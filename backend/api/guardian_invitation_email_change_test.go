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

// #6294: the staff list tells a dead link from an open one. A link whose
// address changed, and a link the address binding ended, read as
// "link_invalid", so staff see that a new invitation is needed. A link that
// simply ran out keeps the plain "none".
func TestGuardianComposition_ListMarksDeadInvitationLinks(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	db := testpkg.SetupTestDB(t)
	invitations, err := NewGuardianInvitationServiceForTests(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), GuardianInvitationTestConfig{
		Outbox: NewCapturingOutbox(),
	})
	require.NoError(t, err)
	inviter := ctx.account("dead-link-inviter")
	statusOf := func(studentID, guardianID int64) string {
		t.Helper()
		for _, row := range ctx.studentGuardians(t, studentID) {
			if row.Guardian.ID == guardianID {
				return row.AccountStatus
			}
		}
		t.Fatalf("guardian %d is not listed for student %d", guardianID, studentID)
		return ""
	}

	t.Run("address changed after the link was mailed", func(t *testing.T) {
		guardianID, studentID, _ := ctx.createLinkedGuardian(t, "dead-link-address")
		_, err := invitations.CreateGuardianInvitation(testpkg.Ctx(t), identityaccess.GuardianInvitationRequest{
			GuardianProfileID: guardianID, CreatedBy: inviter,
		})
		require.NoError(t, err)
		require.Equal(t, "pending", statusOf(studentID, guardianID))

		corrected := fmt.Sprintf("dead-link-corrected-%d@example.test", time.Now().UnixNano())
		rr := ctx.do(t, testutil.AdminTestClaims(999), http.MethodPut, fmt.Sprintf("/%d", guardianID), map[string]any{"email": corrected})
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
		assert.Equal(t, "link_invalid", statusOf(studentID, guardianID))
	})

	t.Run("link ended by the address binding", func(t *testing.T) {
		guardianID, studentID, _ := ctx.createLinkedGuardian(t, "dead-link-migrated")
		invitation := testpkg.InsertTestGuardianInvitation(t, db, &testpkg.GuardianInvitation{
			Token: fmt.Sprintf("dead-link-migrated-%d", time.Now().UnixNano()), GuardianProfileID: guardianID,
			CreatedBy: inviter, ExpiresAt: time.Now().Add(-time.Hour),
		})
		// 1.15.855 ends a legacy link without a recorded recipient.
		_, err := db.NewRaw(`UPDATE auth.guardian_invitations SET email = NULL WHERE id = ?`, invitation.ID).Exec(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "link_invalid", statusOf(studentID, guardianID))
	})

	t.Run("link that simply ran out", func(t *testing.T) {
		guardianID, studentID, _ := ctx.createLinkedGuardian(t, "dead-link-expired")
		testpkg.InsertTestGuardianInvitation(t, db, &testpkg.GuardianInvitation{
			Token: fmt.Sprintf("dead-link-expired-%d", time.Now().UnixNano()), GuardianProfileID: guardianID,
			CreatedBy: inviter, ExpiresAt: time.Now().Add(-time.Hour),
		})
		assert.Equal(t, "none", statusOf(studentID, guardianID))
	})
}
