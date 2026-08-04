package enrollment_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Restore-flow integration tests (#2157). Same env + fixture strategy as
// the Decide tests: real test DB, submissions created through the public
// Submit flow, withdraw through the parent-facing Withdraw flow — so the
// restore undoes exactly the state production produces.

func listRestorationAuditRows(t *testing.T, env *decisionTestEnv, requestID int64) []*auditModels.EnrollmentRestoration {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	var rows []*auditModels.EnrollmentRestoration
	err := env.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`audit.enrollment_restorations AS "enrollment_restoration"`).
		Where(`"enrollment_restoration".request_id = ?`, requestID).
		Scan(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			Model((*auditModels.EnrollmentRestoration)(nil)).
			ModelTableExpr(`audit.enrollment_restorations AS "enrollment_restoration"`).
			Where(`"enrollment_restoration".request_id = ?`, requestID).
			Exec(testpkg.TenantContext(1))
	})
	return rows
}

func TestDecisionService_RestoreWithdrawn_HappyPath(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	result := submitDecisionSiblings(t, env, "restore-happy@example.com")
	reqID := result.Request.ID
	require.NoError(t, env.requestSvc.Withdraw(ctx, result.Request.StatusToken, 0))

	// Precondition: the withdraw stamped everything the restore must undo.
	withdrawnReq, err := env.repos.Request.FindByID(ctx, reqID)
	require.NoError(t, err)
	require.NotNil(t, withdrawnReq.WithdrawnAt)

	outcome, err := env.decision.RestoreWithdrawn(ctx, reqID, env.creatorID)
	require.NoError(t, err)
	require.Len(t, outcome.RestoredChildIDs, 2)

	children, err := env.repos.RequestChild.ListByRequestID(ctx, reqID)
	require.NoError(t, err)
	require.Len(t, children, 2)
	for _, child := range children {
		assert.Equal(t, enrollmentModels.ChildStatusSubmitted, child.Status)
		assert.Nil(t, child.ReviewedAt, "restored child must look un-reviewed")
		assert.Nil(t, child.ReviewedBy)
		assert.Nil(t, child.StatusReason)
	}

	restoredReq, err := env.repos.Request.FindByID(ctx, reqID)
	require.NoError(t, err)
	assert.Nil(t, restoredReq.WithdrawnAt, "withdrawn_at must be cleared")

	auditRows := listRestorationAuditRows(t, env, reqID)
	require.Len(t, auditRows, 1)
	require.NotNil(t, auditRows[0].ActorAccountID)
	assert.Equal(t, env.creatorID, *auditRows[0].ActorAccountID)
	assert.ElementsMatch(t, outcome.RestoredChildIDs, auditRows[0].ChildIDs)
	assert.False(t, auditRows[0].RestoredAt.IsZero())
}

func TestDecisionService_RestoreWithdrawn_TerminalSiblingUntouched(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	result := submitDecisionSiblings(t, env, "restore-partial@example.com")
	reqID := result.Request.ID
	rejectedChildID := result.Children[0].ID

	// Admin rejects the first child (terminal), then the parent withdraws
	// the whole request — the withdraw skips the rejected child.
	_, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  reqID,
		ChildID:    rejectedChildID,
		Status:     enrollmentService.DecisionRejected,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NoError(t, env.requestSvc.Withdraw(ctx, result.Request.StatusToken, 0))

	outcome, err := env.decision.RestoreWithdrawn(ctx, reqID, env.creatorID)
	require.NoError(t, err)
	require.Len(t, outcome.RestoredChildIDs, 1)
	assert.NotContains(t, outcome.RestoredChildIDs, rejectedChildID)

	children, err := env.repos.RequestChild.ListByRequestID(ctx, reqID)
	require.NoError(t, err)
	require.Len(t, children, 2)
	for _, child := range children {
		if child.ID == rejectedChildID {
			assert.Equal(t, enrollmentModels.ChildStatusRejected, child.Status,
				"pre-withdraw terminal child must stay untouched")
			require.NotNil(t, child.ReviewedBy)
			assert.Equal(t, env.creatorID, *child.ReviewedBy)
			continue
		}
		assert.Equal(t, enrollmentModels.ChildStatusSubmitted, child.Status)
		assert.Nil(t, child.ReviewedAt)
	}

	restoredReq, err := env.repos.Request.FindByID(ctx, reqID)
	require.NoError(t, err)
	assert.Nil(t, restoredReq.WithdrawnAt)

	listRestorationAuditRows(t, env, reqID) // registers audit cleanup
}

func TestDecisionService_RestoreWithdrawn_NothingWithdrawn(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	reqID, _ := submitOneChild(t, env, "restore-nothing@example.com", "Nia", "Nichts")

	_, err := env.decision.RestoreWithdrawn(ctx, reqID, env.creatorID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrRestoreNothingWithdrawn))
}

func TestDecisionService_RestoreWithdrawn_NotFound(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	_, err := env.decision.RestoreWithdrawn(ctx, 99_999_999, env.creatorID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrDecisionRequestNotFound))
}

func TestDecisionService_RestoreWithdrawn_PhaseInactive(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	result := submitDecisionSiblings(t, env, "restore-inactive@example.com")
	reqID := result.Request.ID
	require.NoError(t, env.requestSvc.Withdraw(ctx, result.Request.StatusToken, 0))

	env.sourcePhase.IsActive = false
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))
	t.Cleanup(func() {
		env.sourcePhase.IsActive = true
		_ = env.repos.Phase.Update(testpkg.TenantContext(1), env.sourcePhase)
	})

	_, err := env.decision.RestoreWithdrawn(ctx, reqID, env.creatorID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrRestorePhaseInactive))

	// Nothing changed: children stay withdrawn, withdrawn_at stays set.
	children, listErr := env.repos.RequestChild.ListByRequestID(ctx, reqID)
	require.NoError(t, listErr)
	for _, child := range children {
		assert.Equal(t, enrollmentModels.ChildStatusWithdrawn, child.Status)
	}
}

func TestDecisionService_RestoreWithdrawn_BlockedByActiveDuplicate(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	// The production scenario from #2156: withdraw, then re-submit the same
	// child. Restoring the withdrawn request would create a second active
	// request for the same child in the phase.
	first := submitDecisionSiblings(t, env, "restore-dup@example.com")
	require.NoError(t, env.requestSvc.Withdraw(ctx, first.Request.StatusToken, 0))
	_, _ = submitOneChild(t, env, "restore-dup@example.com", "Lina", "Digest")

	_, err := env.decision.RestoreWithdrawn(ctx, first.Request.ID, env.creatorID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrRestoreDuplicateActive))

	// The withdrawn request is untouched.
	children, listErr := env.repos.RequestChild.ListByRequestID(ctx, first.Request.ID)
	require.NoError(t, listErr)
	for _, child := range children {
		assert.Equal(t, enrollmentModels.ChildStatusWithdrawn, child.Status)
	}
	req, findErr := env.repos.Request.FindByID(ctx, first.Request.ID)
	require.NoError(t, findErr)
	assert.NotNil(t, req.WithdrawnAt)
}
