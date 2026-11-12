package parentportal_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// inboxNotice stores an issued Jahresbescheid of the Träger for a child at a
// school, addressed to recipient, with its stored original and, when sent,
// one send, as the Elternabrechnung writes them (#6347). It returns the ID
// of the original, which is the ID of the document in the inbox.
func inboxNotice(t *testing.T, db *bun.DB, organization, school, student int64, recipient string, sent bool) int64 {
	t.Helper()
	ctx := context.Background()
	unique := testpkg.UniqueTestTenantID(t)
	var run, notice, letter int64
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_notice_runs (organization_id, billing_year, period_from,
		period_until, school_ids, status, released_at, issued_on, notice_count, total_cents)
		VALUES (?, 2030, DATE '2030-09-01', DATE '2030-12-31', ARRAY[?]::bigint[], 'released', NOW(), DATE '2030-09-24', 1, 12800)
		RETURNING id`, organization, school).Scan(ctx, &run))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_notices (organization_id, run_id, number, number_year,
		number_sequence, issued_on, billing_year, kind, care_contract_id, school_id, school_name, child_first_name,
		child_last_name, student_id, recipient_email, family_email, recipient, payment_method, period_from, period_until, lines,
		previous_cents, total_cents, delta_cents, legal_remedy)
		VALUES (?, ?, ?, 2030, ?, DATE '2030-09-24', 2030, 'initial', ?, ?, 'OGS', 'Felix', 'Schneider', ?, ?, ?,
		'{}'::jsonb, 'transfer', DATE '2030-09-01', DATE '2030-12-31', '[]'::jsonb, 0, 12800, 12800, 'Widerspruch binnen eines Monats.')
		RETURNING id`, organization, run, fmt.Sprintf("JB-%d", unique), unique, unique, school, student, recipient, recipient).
		Scan(ctx, &notice))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_notice_letters (organization_id, run_id, notice_id, filename, pdf)
		VALUES (?, ?, ?, 'gebuehrenbescheid.pdf', ?) RETURNING id`, organization, run, notice, []byte(fmt.Sprintf("%%PDF-1.4 JB-%d", unique))).
		Scan(ctx, &letter))
	if sent {
		_, err := db.ExecContext(ctx, `INSERT INTO parent_billing.care_notice_letter_mailings (organization_id, run_id, letter_id,
			delivery_id, recipient_email) VALUES (?, ?, ?, ?, ?)`, organization, run, letter, letter+900_000, recipient)
		require.NoError(t, err)
	}
	return letter
}

// noticeInbox is the parents portal over the real Elternabrechnung, as
// documentInbox builds it, on db.
func noticeInbox(t *testing.T, db *bun.DB) *parentService.Portal {
	t.Helper()
	return parentportalcompose.New(documentInboxDependencies(t, db, &owners.CareContractTestMails{}))
}

func inboxDocumentIDs(documents []parentService.BillingDocument) []int64 {
	ids := make([]int64, len(documents))
	for i, document := range documents {
		ids[i] = document.ID
	}
	return ids
}

// TestCareNoticeInboxNeedsTheRelationshipOfTheChild pins the access to a
// Jahresbescheid in the parents inbox (#6347, ADR 0126 point 2): only a sent
// notice, only to the account with the address it copied, and only while the
// account's relationship to the child of the notice grants the inbox.
// Membership at the school, a relationship to another child, a deviating
// payer's address, a withdrawn right, a deleted account and another Träger
// open nothing.
func TestCareNoticeInboxNeedsTheRelationshipOfTheChild(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	portal := noticeInbox(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	testpkg.OwnTraegerContributionRules(t, db, organization)

	sent := inboxNotice(t, db, organization, chain.TenantID, chain.StudentID, chain.Email, true)
	unsent := inboxNotice(t, db, organization, chain.TenantID, chain.StudentID, chain.Email, false)
	payer := inboxNotice(t, db, organization, chain.TenantID, chain.StudentID, "oma.zahlt@example.org", true)
	// A child of the same school the account has no relationship to.
	stranger := testpkg.CreateTestStudent(t, db, "Tom", "Fremd", "2b")
	otherChild := inboxNotice(t, db, organization, chain.TenantID, stranger.ID, chain.Email, true)
	foreignOrganization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Fremder Träger Bescheid", Slug: fmt.Sprintf("traeger-bescheid-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	foreignSchool := testpkg.CreateTestSchoolInOrganization(t, db, foreignOrganization, "Fremde OGS Bescheid")
	testpkg.OwnTraegerContributionRules(t, db, foreignOrganization)
	foreign := inboxNotice(t, db, foreignOrganization, foreignSchool, chain.StudentID, chain.Email, true)

	documents, err := portal.ListBillingDocuments(ctx, chain.AccountID, parentService.DocumentInboxLetters)
	require.NoError(t, err)
	assert.Equal(t, []int64{sent}, inboxDocumentIDs(documents), "only the sent notice of the own child to the own address")
	require.Len(t, documents, 1)
	assert.Equal(t, "care_fee_notice", documents[0].Kind)
	assert.Equal(t, "Betreuung 2030", documents[0].PeriodName)
	file, err := portal.BillingDocumentLetter(ctx, chain.AccountID, organization, sent)
	require.NoError(t, err)
	assert.Contains(t, string(file.Data), "%PDF-1.4 JB-", "the stored original, never rendered again")
	var reads int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.care_notice_run_events
		WHERE organization_id = ? AND actor_account_id = ? AND action = 'read' AND details->>'inbox' IS NOT NULL`,
		organization, chain.AccountID).Scan(ctx, &reads))
	assert.Equal(t, 2, reads, "the list and the download are in the trail of the notice run")

	for name, request := range map[string]struct{ organization, document int64 }{
		"unsent notice":                   {organization, unsent},
		"notice to a deviating payer":     {organization, payer},
		"notice of a child of the school": {organization, otherChild},
		"notice of another Träger":        {foreignOrganization, foreign},
		"notice under another Träger":     {foreignOrganization, sent},
	} {
		_, err := portal.BillingDocumentLetter(ctx, chain.AccountID, request.organization, request.document)
		assert.Error(t, err, name)
	}

	// The same school and address with only pickup rights opens nothing.
	_, err = db.ExecContext(ctx, `UPDATE auth.guardian_student_access SET permissions = '{"parent_portal.access": true}'::jsonb
		WHERE relationship_id IN (SELECT id FROM users.student_guardian_relationships WHERE guardian_profile_id = ?)`,
		chain.GuardianProfileID)
	require.NoError(t, err)
	_, err = portal.ListBillingDocuments(ctx, chain.AccountID, parentService.DocumentInboxLetters)
	assert.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied, "a withdrawn right closes the inbox")
	_, err = portal.BillingDocumentLetter(ctx, chain.AccountID, organization, sent)
	assert.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied, "a withdrawn right closes the download")
}

// A sibling whose right was withdrawn loses its notice while the other
// child keeps its own, and a deleted account reads nothing (#6347).
func TestCareNoticeInboxEndsWithTheRightOfTheChildAndTheAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	portal := noticeInbox(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	sibling := testpkg.CreateTestStudent(t, db, "Mia", "Schneider", "1a")
	link := testpkg.CreateTestStudentGuardianLink(t, db, sibling.ID, chain.GuardianProfileID, "primary_guardian")
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	testpkg.OwnTraegerContributionRules(t, db, organization)
	first := inboxNotice(t, db, organization, chain.TenantID, chain.StudentID, chain.Email, true)
	second := inboxNotice(t, db, organization, chain.TenantID, sibling.ID, chain.Email, true)

	documents, err := portal.ListBillingDocuments(ctx, chain.AccountID, parentService.DocumentInboxLetters)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{first, second}, inboxDocumentIDs(documents))
	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(ctx, db, link.ID, map[string]interface{}{"parent_portal.access": true}))
	documents, err = portal.ListBillingDocuments(ctx, chain.AccountID, parentService.DocumentInboxLetters)
	require.NoError(t, err)
	assert.Equal(t, []int64{first}, inboxDocumentIDs(documents), "the sibling's notice goes with its right")
	_, err = portal.BillingDocumentLetter(ctx, chain.AccountID, organization, second)
	assert.Error(t, err)

	// The deletion of the account erases its personal data (the address
	// included); the account no longer has a mailbox.
	_, err = db.ExecContext(ctx, `UPDATE auth.accounts SET personal_data_erased_at = NOW() WHERE id = ?`, chain.AccountID)
	require.NoError(t, err)
	documents, err = portal.ListBillingDocuments(ctx, chain.AccountID, parentService.DocumentInboxLetters)
	if err == nil {
		assert.Empty(t, documents, "a deleted account reads nothing")
	}
	_, err = portal.BillingDocumentLetter(ctx, chain.AccountID, organization, first)
	assert.Error(t, err, "a deleted account downloads nothing")
}
