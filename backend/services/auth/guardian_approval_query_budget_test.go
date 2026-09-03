package auth_test

import (
	"fmt"
	"testing"
	"time"

	authService "github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPendingGuardianApprovalsQueryBudget(t *testing.T) {
	t.Parallel()
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()
	requester := testpkg.CreateTestAccount(t, env.db, "guardian-approval-budget")
	add := func(from, to int) {
		for i := from; i < to; i++ {
			student := testpkg.CreateTestStudent(t, env.db, fmt.Sprintf("Approval%d", i), "Budget", "1a")
			email := fmt.Sprintf("approval-budget-%d-%d@example.test", i, time.Now().UnixNano())
			_, err := env.service.InviteToStudent(testpkg.Ctx(t), authService.InviteToStudentRequest{
				StudentID: student.ID, Email: email, FirstName: "Guardian", LastName: fmt.Sprintf("Budget%d", i),
				CreatedBy: requester.ID, RequestedByParentAccountID: &requester.ID, RequireApproval: true,
			})
			require.NoError(t, err)
		}
	}
	add(0, 3)
	counter := testpkg.CaptureQueriesForContext(t, env.db)
	ctx := counter.Context(testpkg.Ctx(t))
	run := func() []string {
		counter.Reset()
		_, err := env.service.ListPendingApprovalsDetailed(ctx)
		require.NoError(t, err)
		return counter.Operation("SELECT")
	}
	small := run()
	add(3, 8)
	large := run()
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "services.auth.pending_guardian_approvals.reads", large)
}
