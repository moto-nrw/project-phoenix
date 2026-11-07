package parentportal_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// confirmationTexts answers the Träger's confirmation templates.
type confirmationTexts struct{}

func (confirmationTexts) TraegerTexts(_ context.Context, _ int64, keys []string) (map[string]string, error) {
	texts := map[string]string{}
	for _, key := range keys {
		texts[key] = "Bestätigung für {Kind}, eingegangen am {Eingang}. {Träger}"
		if key == configModels.KeyCareContractTerminationConfirmation {
			texts[key] = "Kündigung für {Kind} zum {Vertragsende}. {Träger}"
		}
	}
	return texts, nil
}

// noticePortal is the parents portal with withdrawal and termination over
// the real Betreuungsvertrag and Delivery's outbox.
func noticePortal(t *testing.T) (*parentService.Portal, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	delivery, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{
		DB: db, People: owners.EnrollmentFlowGuardianDirectory{}, Observe: func(deliveryContract.Observation) {},
	})
	require.NoError(t, err)
	deps := documentInboxDependencies(t, db)
	deps.ContractNotices = owners.NewParentContractNotices(confirmationTexts{}, delivery.Module)
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
	assert.Equal(t, carecontract.StatusActive, options.Status)
	require.NotNil(t, options.OrdinaryEndsOn)
	require.NotNil(t, options.WithdrawalUntil, "concluded today, inside the period")
	_, err = portal.ContractNoticeOptions(ctx, chain.AccountID, organization, otherFamily)
	assert.ErrorIs(t, err, parentService.ErrBillingDocumentNotFound, "another family's contract")
	_, err = portal.DeclareContractNotice(ctx, chain.AccountID, organization, otherFamily,
		parentService.ContractNoticeInput{Kind: carecontract.NoticeWithdrawal})
	assert.ErrorIs(t, err, parentService.ErrBillingDocumentNotFound)

	var link int64
	require.NoError(t, db.NewRaw(`SELECT id FROM users.student_guardian_relationships WHERE student_id = ? AND guardian_profile_id = ?`,
		chain.StudentID, chain.GuardianProfileID).Scan(ctx, &link))
	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(ctx, db, link, map[string]interface{}{
		"parent_portal.access": true, "parent_portal.enrollments.view": true,
	}))
	_, err = portal.DeclareContractNotice(ctx, chain.AccountID, organization, own,
		parentService.ContractNoticeInput{Kind: carecontract.NoticeTermination, TerminationKind: carecontract.TerminationOrdinary})
	require.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied, "reading the inbox does not end a contract")

	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(ctx, db, link, map[string]interface{}{
		"parent_portal.access": true, "parent_portal.enrollments.view": true, "parent_portal.enrollment.submit": true,
	}))
	done, err := portal.DeclareContractNotice(ctx, chain.AccountID, organization, own,
		parentService.ContractNoticeInput{Kind: carecontract.NoticeTermination, TerminationKind: carecontract.TerminationOrdinary})
	require.NoError(t, err)
	assert.Equal(t, carecontract.StatusTerminated, done.Status)
	require.NotNil(t, done.Notice)
	assert.Equal(t, carecontract.ChannelParents, done.Notice.Channel)
	assert.Equal(t, chain.Email, done.Notice.Recipient, "the confirmation goes to the declaring parent")
	assert.Contains(t, done.Notice.ConfirmationText, "Felix")
	assert.NotEmpty(t, done.TraegerName)

	_, err = portal.DeclareContractNotice(ctx, chain.AccountID, organization, own,
		parentService.ContractNoticeInput{Kind: carecontract.NoticeWithdrawal})
	assert.ErrorIs(t, err, parentService.ErrContractNotEndable, "a contract ends once")

	var declarations, mails int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_events WHERE contract_id = ? AND account_id = ?
		AND action = 'terminate'`, own, chain.AccountID).Scan(ctx, &declarations))
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM platform.email_outbox WHERE organization_id = ? AND kind = ?
		AND related_entity_id = ?`, organization, carecontract.MailKindNotice, own).Scan(ctx, &mails))
	assert.Equal(t, 1, declarations)
	assert.Equal(t, 1, mails)
}
