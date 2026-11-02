package behavior_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The staff invitation list reads the school's pending invitations in one
// statement, however many there are.
func TestPendingInvitationsQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	env := newInvitationEnv(t, db)
	creator := testpkg.CreateTestAccount(t, db, "invitation-budget-creator")
	role := testpkg.CreateTestRole(t, db, "invitation-budget")
	add := func(from, to int) {
		for i := from; i < to; i++ {
			_, err := env.service.CreateSchoolInvitation(testpkg.Ctx(t), identityaccess.SchoolInvitationRequest{
				Email: inviteeAddress("invitation-budget"), RoleID: role.ID, CreatedBy: creator.ID,
				FirstName: testpkg.StrPtr("Budget"), LastName: testpkg.StrPtr("Invitee"),
				ActorPermissions: []string{usersManagePermission},
			})
			require.NoError(t, err)
		}
	}
	add(0, 3)

	require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.TenantIDFromContext(testpkg.Ctx(t)), func(ctx context.Context) error {
		counter := testpkg.CaptureQueriesForContext(t, db)
		countedCtx := counter.Context(ctx)
		run := func() []string {
			counter.Reset()
			_, err := env.service.ListPendingSchoolInvitations(countedCtx)
			require.NoError(t, err)
			return counter.Operation("SELECT")
		}
		small := run()
		add(3, 8)
		large := run()
		assert.Equal(t, len(small), len(large), "the listing must not grow a statement per invitation")
		querybudgets.AssertQueryBudget(t, "services.auth.pending_invitations.reads", large)
		return nil
	}))
}
