package behavior_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The operator invitation list reads the redeemable links in one statement,
// however many there are.
func TestOperatorInvitationsListQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildInvitationService(t, db)
	ctx := context.Background()
	// Each link is created by its own operator: one inviter may only create
	// OperatorInvitationRateLimit links per hour, and the listing must stay
	// flat over the inviters too, not just over the links.
	add := func(from, to int) {
		for i := from; i < to; i++ {
			inviter := createInvitingOperator(t, db, uniqueOperatorEmail("operator-budget-inviter"))
			require.NoError(t, service.InviteOperator(ctx, uniqueOperatorEmail("operator-budget-invitee"), nil, inviter, nil))
		}
	}
	add(0, 3)

	counter := testpkg.CaptureQueriesForContext(t, db)
	counted := counter.Context(ctx)
	run := func() []string {
		counter.Reset()
		_, err := service.ListPendingOperatorInvitations(counted)
		require.NoError(t, err)
		return counter.Operation("SELECT")
	}
	small := run()
	add(3, 8)
	large := run()
	assert.Equal(t, len(small), len(large), "the listing must not grow a statement per link")
	testpkg.AssertQueryBudget(t, "services.platform.pending_operator_invitations.reads", large)
}
