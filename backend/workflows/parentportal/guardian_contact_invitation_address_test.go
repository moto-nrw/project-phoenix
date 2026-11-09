package parentportal_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// #6294 through the parents portal (parent_portal.guardian.edit): a parent
// corrects the address of a contact without an account, whose open
// invitation went to the old one. That link must not create the account of
// the corrected address.
func TestUpdateGuardianContact_EmailCorrectionEndsTheMailedLink(t *testing.T) {
	t.Parallel()
	svc, db := buildGuardianService(t)
	invitations, err := api.NewGuardianInvitationServiceForTests(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.GuardianInvitationTestConfig{
		Outbox: api.NewCapturingOutbox(),
	})
	require.NoError(t, err)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	contactID := linkContactOnlyGuardian(t, db, chain.StudentID, "oma-typo")
	invitation, err := invitations.CreateGuardianInvitation(testpkg.Ctx(t), identityaccess.GuardianInvitationRequest{
		GuardianProfileID: contactID, CreatedBy: chain.AccountID,
	})
	require.NoError(t, err)

	corrected := fmt.Sprintf("oma-corrected-%d@example.test", time.Now().UnixNano())
	_, err = svc.UpdateGuardianContact(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, contactID, parentService.GuardianContactInput{
		FirstName: "Helga", LastName: "Schneider", Email: &corrected,
	})
	require.NoError(t, err)

	password := testpkg.NewTestPassword()
	_, err = invitations.AcceptGuardianInvitation(context.Background(), invitation.Token, identityaccess.GuardianRegistration{
		Password: password, ConfirmPassword: password,
	})
	require.ErrorIs(t, err, identityaccess.ErrGuardianInvitationAddressChanged)
	accounts, err := db.NewSelect().TableExpr("auth.accounts").Where("LOWER(email) = ?", corrected).Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, accounts, "the old link must not create the corrected address's account")
}
