package parentportal_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// noticePortal is the parents portal with withdrawal and termination over
// the real Betreuungsvertrag and Delivery's outbox.
func noticePortal(t *testing.T) (*parentService.Portal, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	deps := documentInboxDependencies(t, db, &owners.CareContractTestMails{})
	notices, err := api.NewParentContractNoticesForTests(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	deps.ContractNotices = notices
	return parentportalcompose.New(deps), db
}

// activeInboxContract is a contract concluded today and sent to sentTo.
func activeInboxContract(t *testing.T, db *bun.DB, organization, school, studentID int64, child, sentTo string) int64 {
	t.Helper()
	contract := inboxContract(t, db, organization, school, studentID, child, sentTo)
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `UPDATE care_contract.contract_versions SET status = 'wirksam' WHERE contract_id = ?`, contract)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE care_contract.contracts SET status = 'aktiv', concluded_at = NOW(),
		conclusion_method = 'mit_anmeldung' WHERE id = ?`, contract)
	require.NoError(t, err)
	return contract
}

// Parents end only a contract sent to their own address and only with the
// relationship right parent_portal.enrollment.submit for its child (#4682):
// without it 403, another family's contract 404; the declaration lands in
// the trail with the account and its confirmation goes to the account.
func TestParentsEndAContractOnlyWithTheRelationshipRight(t *testing.T) {
	t.Parallel()
	portal, db := noticePortal(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	testpkg.OwnTraegerCareContracts(t, db, organization)
	own := activeInboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Felix", chain.Email)
	otherFamily := activeInboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Tom", "andere.familie@example.org")

	options, err := portal.ContractNoticeOptions(ctx, chain.AccountID, organization, own)
	require.NoError(t, err)
	assert.Equal(t, "aktiv", options.Status)
	require.NotNil(t, options.OrdinaryEndsOn)
	require.NotNil(t, options.WithdrawalUntil, "concluded today, inside the period")
	_, err = portal.ContractNoticeOptions(ctx, chain.AccountID, organization, otherFamily)
	assert.ErrorIs(t, err, parentService.ErrBillingDocumentNotFound, "another family's contract")
	_, err = portal.DeclareContractNotice(ctx, chain.AccountID, organization, otherFamily,
		parentService.ContractNoticeInput{Kind: "widerruf"})
	assert.ErrorIs(t, err, parentService.ErrBillingDocumentNotFound)

	var link int64
	require.NoError(t, db.NewRaw(`SELECT id FROM users.student_guardian_relationships WHERE student_id = ? AND guardian_profile_id = ?`,
		chain.StudentID, chain.GuardianProfileID).Scan(ctx, &link))
	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(ctx, db, link, map[string]interface{}{
		"parent_portal.access": true, "parent_portal.enrollments.view": true,
	}))
	_, err = portal.DeclareContractNotice(ctx, chain.AccountID, organization, own,
		parentService.ContractNoticeInput{Kind: "kuendigung", TerminationKind: "ordentlich"})
	require.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied, "reading the inbox does not end a contract")

	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(ctx, db, link, map[string]interface{}{
		"parent_portal.access": true, "parent_portal.enrollments.view": true, "parent_portal.enrollment.submit": true,
	}))
	_, err = portal.DeclareContractNotice(ctx, chain.AccountID, organization, own,
		parentService.ContractNoticeInput{Kind: "kuendigung", TerminationKind: "ordentlich", Reason: "Umzug"})
	require.Error(t, err, "an ordinary termination carries no reason")
	done, err := portal.DeclareContractNotice(ctx, chain.AccountID, organization, own,
		parentService.ContractNoticeInput{Kind: "kuendigung", TerminationKind: "ordentlich"})
	require.NoError(t, err)
	assert.Equal(t, "gekuendigt", done.Status)
	require.NotNil(t, done.Notice)
	assert.Equal(t, "eltern_app", done.Notice.Channel)
	assert.Equal(t, chain.Email, done.Notice.Recipient, "the confirmation goes to the declaring parent")
	assert.Contains(t, done.Notice.ConfirmationText, "Felix")
	assert.NotEmpty(t, done.TraegerName)

	documents, err := portal.ListBillingDocuments(ctx, chain.AccountID, parentService.DocumentInboxContracts)
	require.NoError(t, err)
	require.Len(t, documents, 1)
	assert.Equal(t, "gekuendigt", documents[0].ContractStatus, "the inbox shows the terminated contract")
	assert.Equal(t, done.Notice.EndsOn, documents[0].EndsOn)

	_, err = portal.DeclareContractNotice(ctx, chain.AccountID, organization, own,
		parentService.ContractNoticeInput{Kind: "widerruf"})
	assert.ErrorIs(t, err, parentService.ErrContractNotEndable, "a contract ends once")

	var declarations, mails int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_events WHERE contract_id = ? AND account_id = ?
		AND action = 'terminate'`, own, chain.AccountID).Scan(ctx, &declarations))
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM platform.email_outbox WHERE organization_id = ? AND kind = ?
		AND related_entity_id = ?`, organization, "care_contract_notice", own).Scan(ctx, &mails))
	assert.Equal(t, 1, declarations)
	assert.Equal(t, 1, mails)
}

// The inbox offers „Verträge hier kündigen“ only where the declaration would
// pass (#4682): the relationship to the contract's child grants
// parent_portal.enrollment.submit. With the inbox right alone the contract
// shows, but cannot be ended from it.
func TestInboxOffersTheEndOnlyWithTheRelationshipRight(t *testing.T) {
	t.Parallel()
	portal, db := noticePortal(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	testpkg.OwnTraegerCareContracts(t, db, organization)
	activeInboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Felix", chain.Email)
	var link int64
	require.NoError(t, db.NewRaw(`SELECT id FROM users.student_guardian_relationships WHERE student_id = ? AND guardian_profile_id = ?`,
		chain.StudentID, chain.GuardianProfileID).Scan(ctx, &link))

	for _, tc := range []struct {
		name   string
		submit bool
	}{{"inbox right only", false}, {"with enrollment.submit", true}} {
		require.NoError(t, testpkg.SetTestStudentGuardianPermissions(ctx, db, link, map[string]interface{}{
			"parent_portal.access": true, "parent_portal.enrollments.view": true, "parent_portal.enrollment.submit": tc.submit,
		}))
		documents, err := portal.ListBillingDocuments(ctx, chain.AccountID, parentService.DocumentInboxContracts)
		require.NoError(t, err, tc.name)
		require.Len(t, documents, 1, tc.name)
		assert.Equal(t, tc.submit, documents[0].CanEndContract, tc.name)
	}
}

// The inbox offers the withdrawal function „Vertrag widerrufen“ (§ 356a
// BGB, #6105) while the withdrawal period may run, and the withdrawal
// notice names exactly that label. Once the right expired the button goes.
func TestInboxOffersTheWithdrawalWhileThePeriodRuns(t *testing.T) {
	t.Parallel()
	portal, db := noticePortal(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	testpkg.OwnTraegerCareContracts(t, db, organization)
	contract := activeInboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Felix", chain.Email)
	var link int64
	require.NoError(t, db.NewRaw(`SELECT id FROM users.student_guardian_relationships WHERE student_id = ? AND guardian_profile_id = ?`,
		chain.StudentID, chain.GuardianProfileID).Scan(ctx, &link))
	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(ctx, db, link, map[string]interface{}{
		"parent_portal.access": true, "parent_portal.enrollments.view": true, "parent_portal.enrollment.submit": true,
	}))

	documents, err := portal.ListBillingDocuments(ctx, chain.AccountID, parentService.DocumentInboxContracts)
	require.NoError(t, err)
	require.Len(t, documents, 1)
	assert.True(t, documents[0].CanWithdrawContract, "concluded today, inside the period")

	_, err = db.ExecContext(ctx, `UPDATE care_contract.contracts SET concluded_at = NOW() - INTERVAL '2 years' WHERE id = ?`, contract)
	require.NoError(t, err)
	documents, err = portal.ListBillingDocuments(ctx, chain.AccountID, parentService.DocumentInboxContracts)
	require.NoError(t, err)
	require.Len(t, documents, 1)
	assert.True(t, documents[0].CanEndContract, "the contract can still be terminated")
	assert.False(t, documents[0].CanWithdrawContract, "the right of withdrawal expired")
}

// The parents' extraordinary termination reaches the Betreuungsvertrag with
// its reason (§ 312k Abs. 2 BGB) and comes back with an open end and the
// receipt with its time in the confirmation (§ 312k Abs. 4 BGB).
func TestParentsTerminateExtraordinarilyWithTheirReason(t *testing.T) {
	t.Parallel()
	portal, db := noticePortal(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	testpkg.OwnTraegerCareContracts(t, db, organization)
	contract := activeInboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Felix", chain.Email)
	var link int64
	require.NoError(t, db.NewRaw(`SELECT id FROM users.student_guardian_relationships WHERE student_id = ? AND guardian_profile_id = ?`,
		chain.StudentID, chain.GuardianProfileID).Scan(ctx, &link))
	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(ctx, db, link, map[string]interface{}{
		"parent_portal.access": true, "parent_portal.enrollments.view": true, "parent_portal.enrollment.submit": true,
	}))

	done, err := portal.DeclareContractNotice(ctx, chain.AccountID, organization, contract,
		parentService.ContractNoticeInput{Kind: "kuendigung", TerminationKind: "ausserordentlich", Reason: "Wir ziehen um."})
	require.NoError(t, err)
	require.NotNil(t, done.Notice)
	assert.Equal(t, "ausserordentlich", done.Notice.TerminationKind)
	assert.Equal(t, "Wir ziehen um.", done.Notice.Reason)
	assert.Nil(t, done.Notice.EndsOn, "the Träger sets the end after its check")
	assert.Contains(t, done.Notice.ConfirmationText, "Wir ziehen um.")
	assert.Regexp(t, `\d{2}\.\d{2}\.\d{4} um \d{2}:\d{2} Uhr`, done.Notice.ConfirmationText, "date and time of the receipt")
	documents, err := portal.ListBillingDocuments(ctx, chain.AccountID, parentService.DocumentInboxContracts)
	require.NoError(t, err)
	require.Len(t, documents, 1)
	assert.Equal(t, "gekuendigt", documents[0].ContractStatus)
	assert.Nil(t, documents[0].EndsOn, "the inbox shows the open end")
}

// A contract the parents signed in the inbox (#4679) and then terminated or
// withdrew (#4682) no longer opens its signing page: the page answers
// ErrContractNotSignable, the confirmation page takes over. Red without the
// status check of the page (InboxSigningView).
func TestSigningPageClosesOnceTheContractEnded(t *testing.T) {
	t.Parallel()
	portal, db := noticePortal(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	testpkg.OwnTraegerCareContracts(t, db, organization)
	signature := parentService.ContractSignatureInput{Method: "drawn", ImagePNG: signaturePNG(t), NoticeAcknowledged: true}

	for _, declaration := range []parentService.ContractNoticeInput{
		{Kind: "kuendigung", TerminationKind: "ordentlich"},
		{Kind: "widerruf"},
	} {
		contract := inboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Felix", chain.Email)
		_, err := portal.SignContract(ctx, chain.AccountID, organization, contract, signature)
		require.NoError(t, err, declaration.Kind)
		_, err = portal.ContractSigningView(ctx, chain.AccountID, organization, contract)
		require.NoError(t, err, "%s: a signed contract shows its signature", declaration.Kind)

		done, err := portal.DeclareContractNotice(ctx, chain.AccountID, organization, contract, declaration)
		require.NoError(t, err, declaration.Kind)
		require.NotEqual(t, "aktiv", done.Status, declaration.Kind)
		_, err = portal.ContractSigningView(ctx, chain.AccountID, organization, contract)
		assert.ErrorIs(t, err, parentService.ErrContractNotSignable, "%s: the ended contract opens no signing page", declaration.Kind)
	}
}
