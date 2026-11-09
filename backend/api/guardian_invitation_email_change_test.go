package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #6294 through the staff contact edit (users:update): a link mailed before
// the address was corrected creates no account for the corrected address.
func TestGuardianComposition_EmailCorrectionEndsTheMailedLink(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	db := testpkg.SetupTestDB(t)
	invitations, err := NewGuardianInvitationServiceForTests(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), GuardianInvitationTestConfig{
		Outbox: NewCapturingOutbox(),
	})
	require.NoError(t, err)
	guardianID, _ := ctx.createGuardian("staff-email-correction")
	invitation, err := invitations.CreateGuardianInvitation(testpkg.Ctx(t), identityaccess.GuardianInvitationRequest{
		GuardianProfileID: guardianID, CreatedBy: ctx.account("staff-email-correction-inviter"),
	})
	require.NoError(t, err)

	corrected := fmt.Sprintf("staff-corrected-%d@example.test", time.Now().UnixNano())
	rr := ctx.do(t, testutil.AdminTestClaims(999), http.MethodPut, fmt.Sprintf("/%d", guardianID), map[string]any{"email": corrected})
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	password := testpkg.NewTestPassword()
	_, err = invitations.AcceptGuardianInvitation(context.Background(), invitation.Token, identityaccess.GuardianRegistration{
		Password: password, ConfirmPassword: password,
	})
	require.ErrorIs(t, err, identityaccess.ErrGuardianInvitationAddressChanged)
	accounts, err := db.NewSelect().TableExpr("auth.accounts").Where("LOWER(email) = ?", corrected).Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, accounts, "the old link must not create the corrected address's account")
}
