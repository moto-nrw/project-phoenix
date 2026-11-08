package enrollmenthttp_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Two approvals of two requests for the same new child must not both create
// it. Each read no enrolled candidate before the other committed; without a
// lock on the child's identity both created a child (#6170). The second
// approval must wait for the first and then ask the office.
func TestExistingChild_ConcurrentApprovalsCreateTheChildOnce(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
	first := env.submit(t, existingChildSubmission{email: "first-parent@example.com", children: []enrollmentAPI.SubmitChild{
		submitChild("Mila", "Zugleich", existingChildBirthday),
	}})
	second := env.submit(t, existingChildSubmission{email: "second-parent@example.com", children: []enrollmentAPI.SubmitChild{
		submitChild(" mila", "ZUGLEICH ", existingChildBirthday),
	}})
	approve := func(res *enrollmentAPI.SubmitResult) enrollmentAPI.DecideInput {
		return enrollmentAPI.DecideInput{
			RequestID: res.Request.ID, ChildID: res.Children[0].ID,
			Status: enrollmentNative.DecisionApproved, ReviewedBy: env.creatorID,
		}
	}

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()
	done := make(chan error, 1)
	firstErr := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		if _, err := env.decision.Decide(txCtx, approve(first)); err != nil {
			return err
		}
		var pid int
		require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &pid))
		go func() {
			done <- testpkg.WithinTenantContext(t, ctx, env.db, testpkg.Tenant(t), func(otherCtx context.Context) error {
				_, err := env.decision.Decide(otherCtx, approve(second))
				return err
			})
		}()
		if finished := waitForBlockedWriter(t, txCtx, tx, pid, done); finished != nil {
			done <- *finished
			return fmt.Errorf("the second approval finished without waiting for the first: %w", *finished)
		}
		return nil
	})
	var secondErr error
	select {
	case secondErr = <-done:
	case <-ctx.Done():
		t.Fatalf("the second approval did not finish; the first returned %v", firstErr)
	}

	count, err := env.db.NewSelect().TableExpr(`users.student_profiles AS sp`).
		Join(`JOIN users.persons AS p ON p.id = sp.person_id`).
		Where("sp.tenant_id = ?", testpkg.Tenant(t)).
		Where("LOWER(TRIM(p.first_name)) = 'mila' AND LOWER(TRIM(p.last_name)) = 'zugleich'").
		Count(testpkg.Ctx(t))
	require.NoError(t, err)
	assert.Equal(t, 1, int(count), "the child exists once")
	require.NoError(t, firstErr)
	require.ErrorIs(t, secondErr, enrollmentNative.ErrChildPossiblyEnrolled,
		"the second approval asks the office about the child the first created")
	assert.Len(t, env.candidates(t, second.Request.ID, second.Children[0].ID), 1)
}
