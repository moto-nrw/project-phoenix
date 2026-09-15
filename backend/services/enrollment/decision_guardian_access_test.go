package enrollment_test

import (
	"context"
	"testing"
	"time"

	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// An approval for a guardian whose e-mail already owns a platform account
// attaches that account through the Identity & Access capability instead of
// queueing an invitation: the school mapping an offboarding left inactive is
// reactivated, the guardian role is assigned exactly once across repeated
// approvals, and the guardian profile is linked to the account (#2699).
func TestDecisionService_Decide_ApprovedAttachesExistingAccountAndReactivatesAccess(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	account := testpkg.CreateTestAccount(t, env.db, "attach-existing-account")
	_, err := env.db.NewRaw("UPDATE auth.account_tenants SET status = 'inactive', deactivated_at = NOW() WHERE account_id = ? AND tenant_id = ?", account.ID, tenantID).Exec(context.Background())
	require.NoError(t, err)

	var studentIDs []int64
	for _, child := range []string{"Erstes", "Zweites"} {
		reqID, childID := submitOneChild(t, env, account.Email, child, "Kind")
		outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
			RequestID: reqID, ChildID: childID, Status: enrollmentService.DecisionApproved, ReviewedBy: env.creatorID,
		})
		require.NoError(t, err)
		require.Nil(t, outcome.PendingInvite, "a guardian with a platform account must not be invited")
		require.NotNil(t, outcome.Child.CreatedStudentID)
		studentIDs = append(studentIDs, *outcome.Child.CreatedStudentID)
	}

	var status string
	var deactivatedAt *time.Time
	require.NoError(t, env.db.NewRaw("SELECT status, deactivated_at FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?", account.ID, tenantID).Scan(context.Background(), &status, &deactivatedAt))
	require.Equal(t, "active", status, "the approval must reactivate the offboarded mapping")
	require.Nil(t, deactivatedAt)

	var guardianRoles int
	require.NoError(t, env.db.NewRaw("SELECT count(*) FROM auth.account_roles ar JOIN auth.roles r ON r.id = ar.role_id WHERE ar.account_id = ? AND ar.tenant_id = ? AND LOWER(r.name) = 'guardian'", account.ID, tenantID).Scan(context.Background(), &guardianRoles))
	require.Equal(t, 1, guardianRoles, "two approvals for the same guardian assign the role once")

	for _, studentID := range studentIDs {
		links, err := env.repos.StudentGuardian.FindByStudentID(ctx, studentID)
		require.NoError(t, err)
		require.Len(t, links, 1)
		profile, err := env.repos.GuardianProfile.FindByID(ctx, links[0].GuardianProfileID)
		require.NoError(t, err)
		require.NotNil(t, profile.AccountID)
		require.Equal(t, account.ID, *profile.AccountID, "the guardian profile must be linked to the existing account")
		require.True(t, profile.HasAccount)
	}
}
