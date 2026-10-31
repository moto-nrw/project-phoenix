package parentportal_test

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// recordedPushes records the pushes the document notice sends.
type recordedPushes struct {
	mu     sync.Mutex
	pushes []parentService.BillingDocumentPush
}

func (r *recordedPushes) NotifyBillingDocument(_ context.Context, push parentService.BillingDocumentPush) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pushes = append(r.pushes, push)
	return nil
}

func (r *recordedPushes) all() []parentService.BillingDocumentPush {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]parentService.BillingDocumentPush(nil), r.pushes...)
}

// documentInbox is the parents portal over the real Elternabrechnung,
// Organisation & Tenancy and Identity & Access, with the pushes recorded.
func documentInbox(t *testing.T) (*parentService.Portal, parentService.DocumentNotices, *recordedPushes, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	people := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	identity, err := api.NewIdentityAccessForTests(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	pushes := &recordedPushes{}
	deps := parentportalcompose.Dependencies{
		GuardianChanges: owners.ParentGuardianChanges(), DeparturePolicy: owners.ParentDeparturePolicy(),
		GuardianPolicy: owners.NewParentGuardianPolicy(), Fingerprints: owners.SecurityFingerprints{},
		Declarations: owners.ParentDeclarationPolicy(), Locales: owners.ParentPortalLocales(), Runtime: owners.ParentPortalRuntime{},
		ChildRepo: repos.ParentChildReads(), PersonRepo: owners.NewParentPersonReads(people.Person),
		GuardianProfileRepo: owners.NewParentGuardianProfileReads(people.GuardianProfile),
		Logger:              slog.Default(), CarePlanPolicy: owners.NewParentCarePlanPolicy(),
		BillingDocuments: owners.NewParentBillingDocuments(), TraegerRuntime: owners.ParentPortalRuntime{},
		SchoolTraeger: owners.NewParentSchoolTraeger(organizations), ParentAccounts: owners.NewParentAccounts(identity),
		DocumentNotifier: pushes,
	}
	return parentportalcompose.New(deps), parentportalcompose.NewDocumentNotices(deps), pushes, db
}

// inboxTraeger writes the invoices of one Träger as the Elternabrechnung
// stores them: a released run, its invoices with positions, the stored
// letters and, for a sent letter, its send.
type inboxTraeger struct {
	t            *testing.T
	db           *bun.DB
	organization int64
	run          int64
	sequence     int
}

func newInboxTraeger(t *testing.T, db *bun.DB, organization int64) *inboxTraeger {
	t.Helper()
	testpkg.OwnTraegerContributionRules(t, db, organization)
	var run int64
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.runs (organization_id, service, holiday_period_id, period_name, service_from,
			service_until, status, released_at, issued_on, document_count, total_cents)
		VALUES (?, 'holiday', 8, 'Herbstferien 2026', '2026-10-12', '2026-10-23', 'released', NOW(), '2026-10-01', 3, 13500) RETURNING id`, organization).
		Scan(context.Background(), &run))
	return &inboxTraeger{t: t, db: db, organization: organization, run: run}
}

// document stores one invoice to recipient for child, with its letter;
// sent records a send.
func (f *inboxTraeger) document(recipient, child string, sent bool) int64 {
	f.t.Helper()
	ctx := context.Background()
	f.sequence++
	number := fmt.Sprintf("FE-%04d", f.sequence)
	var id int64
	require.NoError(f.t, f.db.NewRaw(`INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year, number_sequence,
			issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name, recipient_email,
			service_from, service_until, total_cents)
		VALUES (?, ?, 'holiday', ?, 0, ?, '2026-10-01', 8, ?, 'Sabine', 'Schneider', ?, '2026-10-12', '2026-10-16', 4500) RETURNING id`,
		f.organization, f.run, number, f.sequence, f.sequence+100, recipient).Scan(ctx, &id))
	_, err := f.db.ExecContext(ctx, `INSERT INTO parent_billing.document_positions (organization_id, document_id, position,
			holiday_registration_child_id, child_first_name, child_last_name, kind, amount_cents, net_cents)
		VALUES (?, ?, 1, ?, ?, 'Schneider', 'base', 4500, 4500)`, f.organization, id, f.sequence+200, child)
	require.NoError(f.t, err)
	_, err = f.db.ExecContext(ctx, `INSERT INTO parent_billing.run_letters (organization_id, run_id, payment_due_on)
		VALUES (?, ?, '2026-10-15') ON CONFLICT DO NOTHING`, f.organization, f.run)
	require.NoError(f.t, err)
	_, err = f.db.ExecContext(ctx, `INSERT INTO parent_billing.document_letters (organization_id, document_id, run_id, filename, pdf)
		VALUES (?, ?, ?, ?, ?)`, f.organization, id, f.run, "bestaetigung-"+number+".pdf", []byte("%PDF-1.4 "+number))
	require.NoError(f.t, err)
	if sent {
		_, err = f.db.ExecContext(ctx, `INSERT INTO parent_billing.document_mailings (organization_id, run_id, document_id, delivery_id, recipient_email)
			VALUES (?, ?, ?, ?, ?)`, f.organization, f.run, id, id+900_000, recipient)
		require.NoError(f.t, err)
	}
	return id
}

// inboxReads counts the reads of the account in the trail of the run.
func (f *inboxTraeger) inboxReads(accountID int64, kind string) int {
	f.t.Helper()
	var reads int
	require.NoError(f.t, f.db.NewRaw(`SELECT COUNT(*) FROM parent_billing.run_events
		WHERE organization_id = ? AND run_id = ? AND action = 'read' AND actor_account_id = ? AND details->>'inbox' = ?`,
		f.organization, f.run, accountID, kind).Scan(context.Background(), &reads))
	return reads
}

// TestDocumentInboxShowsOnlyTheFamilysSentLetters pins ADR 0126: the inbox
// lists the sent letters of the Träger of the account's schools that are
// addressed to the account, never a letter of another family, an unsent
// letter or a letter of another Träger; the PDF is the stored one; every
// read lands in the trail with the account; an account whose relationships
// grant no inbox gets ErrGuardianPermissionDenied.
func TestDocumentInboxShowsOnlyTheFamilysSentLetters(t *testing.T) {
	t.Parallel()
	portal, _, _, db := documentInbox(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))

	own := newInboxTraeger(t, db, organization)
	letter := own.document(chain.Email, "Felix", true)
	otherFamily := own.document("andere.familie@example.org", "Tom", true)
	unsent := own.document(chain.Email, "Mia", false)

	foreignOrganization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Fremder Träger Postfach", Slug: fmt.Sprintf("traeger-postfach-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.CreateTestSchoolInOrganization(t, db, foreignOrganization, "Fremde OGS Postfach")
	foreign := newInboxTraeger(t, db, foreignOrganization)
	foreignLetter := foreign.document(chain.Email, "Felix", true)

	documents, err := portal.ListBillingDocuments(ctx, chain.AccountID)
	require.NoError(t, err)
	require.Len(t, documents, 1, "only the sent letter of the own family at the own Träger")
	document := documents[0]
	assert.Equal(t, letter, document.ID)
	assert.Equal(t, organization, document.OrganizationID)
	assert.NotEmpty(t, document.TraegerName)
	assert.Equal(t, "confirmation_letter", document.Kind)
	assert.Equal(t, "FE-0001", document.Number)
	assert.Equal(t, "Herbstferien 2026", document.PeriodName)
	assert.Equal(t, []string{"Felix"}, document.ChildNames)
	assert.Equal(t, int64(4500), document.TotalCents)
	assert.Equal(t, 1, own.inboxReads(chain.AccountID, "list"), "the list is in the trail of the run")

	file, err := portal.BillingDocumentLetter(ctx, chain.AccountID, organization, letter)
	require.NoError(t, err)
	assert.Equal(t, []byte("%PDF-1.4 FE-0001"), file.Data, "the stored letter, never rendered again")
	assert.Equal(t, "application/pdf", file.ContentType)
	assert.Equal(t, 1, own.inboxReads(chain.AccountID, "download"), "the download is in the trail of the run")

	for name, request := range map[string]struct{ organization, document int64 }{
		"letter of another family": {organization, otherFamily},
		"unsent letter":            {organization, unsent},
		"letter of another Träger": {foreignOrganization, foreignLetter},
		"letter under another":     {foreignOrganization, letter},
	} {
		_, err := portal.BillingDocumentLetter(ctx, chain.AccountID, request.organization, request.document)
		assert.ErrorIs(t, err, parentService.ErrBillingDocumentNotFound, name)
	}
	assert.Zero(t, foreign.inboxReads(chain.AccountID, "download"), "another Träger's trail stays untouched")
	assert.Zero(t, foreign.inboxReads(chain.AccountID, "list"))

	// The same address on an account whose relationship grants no inbox
	// (pickup only) opens nothing.
	pickup := testpkg.CreateTestParentGuardianChain(t, db)
	_, err = db.ExecContext(ctx, `UPDATE auth.guardian_student_access SET permissions = '{"parent_portal.access": true}'::jsonb
		WHERE relationship_id IN (SELECT id FROM users.student_guardian_relationships WHERE guardian_profile_id = ?)`,
		pickup.GuardianProfileID)
	require.NoError(t, err)
	own.document(pickup.Email, "Lena", true)
	_, err = portal.ListBillingDocuments(ctx, pickup.AccountID)
	assert.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied)
	_, err = portal.BillingDocumentLetter(ctx, pickup.AccountID, organization, letter)
	assert.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied)
	assert.Zero(t, own.inboxReads(pickup.AccountID, "list"))
}

// TestDocumentNoticeReachesOnlyThePermittedRecipient pins the push of ADR
// 0126: it goes to the account of the recipient's address at the schools of
// the Träger where its relationship grants the inbox, about the children it
// covers; an unknown address, another Träger's school or a relationship
// without the right gets nothing.
func TestDocumentNoticeReachesOnlyThePermittedRecipient(t *testing.T) {
	t.Parallel()
	_, notices, pushes, db := documentInbox(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))

	require.NoError(t, notices.NotifyBillingDocument(ctx, organization, 77, chain.Email))
	sent := pushes.all()
	require.Len(t, sent, 1)
	assert.Equal(t, parentService.BillingDocumentPush{
		TenantID: chain.TenantID, AccountID: chain.AccountID, StudentIDs: []int64{chain.StudentID}, DocumentID: 77,
	}, sent[0])

	require.NoError(t, notices.NotifyBillingDocument(ctx, organization, 78, "niemand@example.org"))
	require.NoError(t, notices.NotifyBillingDocument(ctx, organization+1_000_000, 79, chain.Email))
	assert.Len(t, pushes.all(), 1, "no account, or no school of that Träger: no push")

	_, err := db.ExecContext(ctx, `UPDATE auth.guardian_student_access SET permissions = '{"parent_portal.access": true}'::jsonb
		WHERE relationship_id IN (SELECT id FROM users.student_guardian_relationships WHERE guardian_profile_id = ?)`,
		chain.GuardianProfileID)
	require.NoError(t, err)
	require.NoError(t, notices.NotifyBillingDocument(ctx, organization, 80, chain.Email))
	assert.Len(t, pushes.all(), 1, "a relationship without the inbox right gets no push")
}

// The inbox reads flat in the number of documents: the account's children,
// their Träger, the account, then per Träger its binding, the documents and
// one trail entry per run (Rule 15, #2940).
func TestDocumentInboxQueryBudget(t *testing.T) {
	t.Parallel()
	portal, _, _, db := documentInbox(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	own := newInboxTraeger(t, db, organization)
	for _, child := range []string{"Felix", "Mia", "Lena"} {
		own.document(chain.Email, child, true)
	}

	counter := testpkg.CaptureQueriesForContext(t, db)
	documents, err := portal.ListBillingDocuments(counter.Context(ctx), chain.AccountID)
	require.NoError(t, err)
	assert.Len(t, documents, 3)
	querybudgets.AssertQueryBudget(t, "workflows.parentportal.billing_documents.list", counter.Queries())
}
