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
	portal, notices, pushes, db, _ := documentInboxWithMails(t)
	return portal, notices, pushes, db
}

// documentInboxWithMails is documentInbox with the mails of the care
// contracts' signature recorded (#4679).
func documentInboxWithMails(t *testing.T) (*parentService.Portal, parentService.DocumentNotices, *recordedPushes, *bun.DB, *owners.CareContractTestMails) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	mails := &owners.CareContractTestMails{}
	deps := documentInboxDependencies(t, db, mails)
	pushes := &recordedPushes{}
	deps.DocumentNotifier = pushes
	return parentportalcompose.New(deps), parentportalcompose.NewDocumentNotices(deps), pushes, db, mails
}

// documentInboxDependencies are the parents portal's dependencies of the
// document inbox over db, without the push of a document; the signature's
// mails go to mails.
func documentInboxDependencies(t *testing.T, db *bun.DB, mails *owners.CareContractTestMails) parentportalcompose.Dependencies {
	t.Helper()
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	people := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	identity, err := api.NewIdentityAccessForTests(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	return parentportalcompose.Dependencies{
		GuardianChanges: owners.ParentGuardianChanges(), DeparturePolicy: owners.ParentDeparturePolicy(),
		GuardianPolicy: owners.NewParentGuardianPolicy(), Fingerprints: owners.SecurityFingerprints{},
		Declarations: owners.ParentDeclarationPolicy(), Locales: owners.ParentPortalLocales(), Runtime: owners.ParentPortalRuntime{},
		ChildRepo: repos.ParentChildReads(), PersonRepo: owners.NewParentPersonReads(people.Person),
		GuardianProfileRepo: owners.NewParentGuardianProfileReads(people.GuardianProfile),
		Logger:              slog.Default(), CarePlanPolicy: owners.NewParentCarePlanPolicy(),
		BillingDocuments: owners.NewParentBillingDocuments(), ContractDocuments: owners.NewParentContractDocuments(owners.NewParentContractSigningForTests(mails)), TraegerRuntime: owners.ParentPortalRuntime{},
		SchoolTraeger: owners.NewParentSchoolTraeger(organizations), ParentAccounts: owners.NewParentAccounts(identity),
	}
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

	documents, err := portal.ListBillingDocuments(ctx, chain.AccountID, parentService.DocumentInboxLetters)
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
	_, err = portal.ListBillingDocuments(ctx, pickup.AccountID, parentService.DocumentInboxLetters)
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
	counter := testpkg.CaptureQueriesForContext(t, db)
	created, smallStatements := 0, 0
	for _, n := range []int{1, 3, 12} {
		for created < n {
			own.document(chain.Email, fmt.Sprintf("Kind-%02d", created), true)
			created++
		}
		counter.Reset()
		documents, err := portal.ListBillingDocuments(counter.Context(ctx), chain.AccountID, parentService.DocumentInboxLetters)
		require.NoError(t, err)
		assert.Len(t, documents, n)
		assert.EqualValues(t, 1, counter.WriteRows(), "one read trail entry per inbox request, not per document")
		t.Logf("billing documents N=%d total=%d", n, counter.Total())
		querybudgets.AssertQueryBudget(t, "workflows.parentportal.billing_documents.list", counter.Queries())
		if n == 1 {
			smallStatements = counter.Total()
		} else {
			assert.Equal(t, smallStatements, counter.Total(), "document growth adds no roundtrips")
		}
	}

}

// The contract inbox reads flat in the number of contracts: the account's
// children, their Träger, the account, then per Träger its binding, the
// contracts and one trail entry for the whole list (Rule 15, ADR 0153).
func TestContractInboxQueryBudget(t *testing.T) {
	t.Parallel()
	portal, _, _, db := documentInbox(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	testpkg.OwnTraegerCareContracts(t, db, organization)
	for _, child := range []string{"Felix", "Mia", "Lena"} {
		inboxContract(t, db, organization, chain.TenantID, chain.StudentID, child, chain.Email)
	}

	counter := testpkg.CaptureQueriesForContext(t, db)
	documents, err := portal.ListBillingDocuments(counter.Context(ctx), chain.AccountID, parentService.DocumentInboxContracts)
	require.NoError(t, err)
	assert.Len(t, documents, 3)
	querybudgets.AssertQueryBudget(t, "workflows.parentportal.contract_documents.list", counter.Queries())
}

// inboxContract stores a care contract of the Träger for child as the
// Betreuungsvertrag does; sentTo records a send of its PDF to that address.
func inboxContract(t *testing.T, db *bun.DB, organization, school, studentID int64, child, sentTo string) int64 {
	t.Helper()
	ctx := context.Background()
	status := "entwurf"
	if sentTo != "" {
		status = "versendet"
	}
	var contract, version, document int64
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id, student_id, status, sent_at)
		VALUES (?, ?, ?, ?, ?, CASE WHEN ? = 'entwurf' THEN NULL ELSE NOW() END) RETURNING id`,
		organization, school, testpkg.UniqueTestTenantID(t), studentID, status, status).Scan(ctx, &contract))
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contract_versions (organization_id, contract_id, version_number, effective_from,
			child_first_name, child_last_name, school_name, term_kind, notice_months, notice_to, special_termination,
			special_termination_note, statute_required, statute_title, statute_url, contract_partner)
		VALUES (?, ?, 1, '2027-08-01', ?, 'Schneider', 'OGS Am Lindenhof', 'open_ended', 1, 'month_end', false, '', false, '', '', '')
		RETURNING id`, organization, contract, child).Scan(ctx, &version))
	_, err := db.ExecContext(ctx, `INSERT INTO care_contract.contract_version_services (organization_id, contract_version_id,
			weekday, care_offering_id, module_name, ends_at) VALUES (?, ?, 'mon', 3, 'Frühbetreuung', '08:00'),
			(?, ?, 'mon', 4, 'Ganztag', '16:30'), (?, ?, 'tue', 4, 'Ganztag', '16:30')`,
		organization, version, organization, version, organization, version)
	require.NoError(t, err)
	// The contract party, as the intake writes it from the application.
	_, err = db.ExecContext(ctx, `INSERT INTO care_contract.contract_parties (organization_id, contract_id, first_name, last_name, email)
		VALUES (?, ?, 'Erika', 'Schneider', ?)`, organization, contract, sentTo)
	require.NoError(t, err)
	if sentTo == "" {
		return contract
	}
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contract_documents (organization_id, contract_id, contract_version_id,
			template_version, filename, pdf) VALUES (?, ?, ?, 0, 'betreuungsvertrag.pdf', ?) RETURNING id`,
		organization, contract, version, []byte("%PDF-1.4 Vertrag "+child)).Scan(ctx, &document))
	_, err = db.ExecContext(ctx, `INSERT INTO care_contract.contract_mailings (organization_id, contract_id, contract_document_id,
			outbox_id, recipient_email) VALUES (?, ?, ?, ?, ?)`, organization, contract, document, testpkg.UniqueTestTenantID(t), sentTo)
	require.NoError(t, err)
	// The signing link a send makes, with the PDF and version it sent (#4679).
	require.NoError(t, testpkg.WithinTraeger(testpkg.WithPackageTenantRuntime(ctx), organization, func(txCtx context.Context) error {
		return owners.RecordCareContractSendForTests(txCtx, organization, contract)
	}))
	return contract
}

// TestDocumentInboxShowsTheFamilysSentContracts pins ADR 0153 point 7: a care
// contract lies in the inbox of the account its mail went to, beside the
// letters; a draft and a contract sent to another family never show, and
// every read lands in the contract's trail with the account.
func TestDocumentInboxShowsTheFamilysSentContracts(t *testing.T) {
	t.Parallel()
	portal, _, _, db := documentInbox(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	testpkg.OwnTraegerCareContracts(t, db, organization)
	sent := inboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Felix", chain.Email)
	otherFamily := inboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Tom", "andere.familie@example.org")
	draft := inboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Mia", "")

	letters, err := portal.ListBillingDocuments(ctx, chain.AccountID, parentService.DocumentInboxLetters)
	require.NoError(t, err)
	assert.Empty(t, letters, "the letters stay a list of their own")
	documents, err := portal.ListBillingDocuments(ctx, chain.AccountID, parentService.DocumentInboxContracts)
	require.NoError(t, err)
	require.Len(t, documents, 1, "only the contract sent to the own address")
	assert.Equal(t, sent, documents[0].ID)
	assert.Equal(t, parentService.DocumentKindContract, documents[0].Kind)
	assert.Equal(t, []string{"Felix"}, documents[0].ChildNames)
	assert.Equal(t, "Frühbetreuung, Ganztag · OGS Am Lindenhof", documents[0].PeriodName, "the modules once each, then the place")

	file, err := portal.ContractDocumentFile(ctx, chain.AccountID, organization, sent)
	require.NoError(t, err)
	assert.Equal(t, []byte("%PDF-1.4 Vertrag Felix"), file.Data, "the stored PDF")
	for name, contract := range map[string]int64{"contract of another family": otherFamily, "draft": draft} {
		_, err := portal.ContractDocumentFile(ctx, chain.AccountID, organization, contract)
		assert.ErrorIs(t, err, parentService.ErrBillingDocumentNotFound, name)
	}
	var reads int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_events WHERE organization_id = ? AND account_id = ?
		AND action = 'read' AND details->>'inbox' IS NOT NULL`, organization, chain.AccountID).Scan(ctx, &reads))
	assert.Equal(t, 2, reads, "the list and the download are in the trail")
}

func TestContractInboxRevokesOnlyTheChildWhosePermissionWasRemoved(t *testing.T) {
	t.Parallel()
	portal, _, _, db := documentInbox(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	sibling := testpkg.CreateTestStudent(t, db, "Mia", "Schneider", "1a")
	link := testpkg.CreateTestStudentGuardianLink(t, db, sibling.ID, chain.GuardianProfileID, "primary_guardian")
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	testpkg.OwnTraegerCareContracts(t, db, organization)
	first := inboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Felix", chain.Email)
	second := inboxContract(t, db, organization, chain.TenantID, sibling.ID, "Mia", chain.Email)
	documents, err := portal.ListBillingDocuments(ctx, chain.AccountID, parentService.DocumentInboxContracts)
	require.NoError(t, err)
	require.Len(t, documents, 2)
	_, err = portal.ContractDocumentFile(ctx, chain.AccountID, organization, second)
	require.NoError(t, err)
	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(ctx, db, link.ID, map[string]interface{}{"parent_portal.access": true}))
	documents, err = portal.ListBillingDocuments(ctx, chain.AccountID, parentService.DocumentInboxContracts)
	require.NoError(t, err)
	require.Len(t, documents, 1)
	assert.Equal(t, first, documents[0].ID)
	_, err = portal.ContractDocumentFile(ctx, chain.AccountID, organization, second)
	assert.ErrorIs(t, err, parentService.ErrBillingDocumentNotFound)
	_, err = portal.ContractDocumentFile(ctx, chain.AccountID, organization, first)
	assert.NoError(t, err)
}
