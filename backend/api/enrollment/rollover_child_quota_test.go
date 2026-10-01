package enrollment_test

import (
	"context"
	"testing"
	"time"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/api/enrollment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The automatic approval at the school-year change keeps the Kinderkontingent
// (#3570): a renewal that would raise the Kontingentzahl above it is skipped,
// stays open for a manual decision and is marked as held by the
// Kinderkontingent, while the rest of the run goes on.
func TestRolloverService_AutoApprove_SkipsChildrenAboveTheKinderkontingent(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	env, cleanup := setupAutoApproveIntegrationEnv(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	countedSource, counted := seedApprovedChildWithStudent(t, env,
		"Anna", "Gezählt", "gezaehlt@example.com", "Lina", "Gezählt", int16(1))
	heldSource, uncounted := seedApprovedChildWithStudent(t, env,
		"Berta", "Inaktiv", "inaktiv@example.com", "Mia", "Inaktiv", int16(1))
	uncounted.Status = usersModels.StudentStatusInactive
	require.NoError(t, env.repos.Student.Update(ctx, uncounted))
	_, err := env.db.NewUpdate().TableExpr("platform.schools").
		Set("child_quota_bundles = 1").Set("child_quota_bundle_size = 1").
		Where("id = ?", testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)

	req := validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true)
	req.RolloverAutoApprove = true
	req.RolloverDeadline = time.Now().Add(-time.Hour)
	req.Name = "child-quota-auto-approve-target"
	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 2, result.RolledCount)

	var summary *enrollmentAPI.DeadlineWorkerSummary
	err = testpkg.WithTenantTx(t, context.Background(), env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var workerErr error
		summary, workerErr = env.rolloverSvc.RunDeadlineWorker(txCtx, time.Now())
		return workerErr
	})
	require.NoError(t, err, "a child above the Kinderkontingent must not abort the run")
	require.NotNil(t, summary)
	assert.Equal(t, 1, summary.AutoRenewedToApproved, "the counted child is renewed")
	assert.Equal(t, 1, summary.AutoApproveChildQuotaHeld, "the uncounted child is skipped")
	assert.Equal(t, 0, summary.AutoApproveErrors, "a full Kinderkontingent is no error")

	rows, err := env.repos.Enrollment().ChildrenByPhaseStatuses(ctx, result.Phase.ID, []string{
		enrollmentModels.ChildStatusApproved, enrollmentModels.ChildStatusSubmitted, enrollmentModels.ChildStatusAutoRenewed,
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.NotNil(t, row.RolloverSourceChildID)
		switch *row.RolloverSourceChildID {
		case countedSource.ID:
			assert.Equal(t, enrollmentModels.ChildStatusApproved, row.Status)
			assert.Nil(t, row.ReviewReason)
		case heldSource.ID:
			assert.Equal(t, enrollmentModels.ChildStatusSubmitted, row.Status, "the held renewal stays open for a manual decision")
			require.NotNil(t, row.ReviewReason)
			assert.Equal(t, enrollmentModels.ReviewReasonChildQuotaReached, *row.ReviewReason)
			assert.Nil(t, row.CreatedStudentID, "the held renewal links no student")
		default:
			t.Fatalf("unexpected rolled row from source %d", *row.RolloverSourceChildID)
		}
	}

	refreshed, err := env.repos.Student.FindByID(ctx, uncounted.ID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.StudentStatusInactive, refreshed.Status, "the held child is not brought back")
	stillCounted, err := env.repos.Student.FindByID(ctx, counted.ID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.StudentStatusActive, stillCounted.Status)

	// A later tick leaves the held renewal to the school instead of
	// approving it once a place frees up.
	_, err = env.db.NewUpdate().TableExpr("platform.schools").Set("child_quota_bundles = NULL").
		Where("id = ?", testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	err = testpkg.WithTenantTx(t, context.Background(), env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var workerErr error
		summary, workerErr = env.rolloverSvc.RunDeadlineWorker(txCtx, time.Now())
		return workerErr
	})
	require.NoError(t, err)
	assert.Zero(t, summary.AutoRenewedToApproved)
	held, err := enrollmentAPI.ReadOwnerChildForTest(ctx, env.repos.Enrollment(), heldRow(t, rows, heldSource.ID))
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildStatusSubmitted, held.Status)
}

func heldRow(t *testing.T, rows []*capability.RequestChild, sourceID int64) int64 {
	t.Helper()
	for _, row := range rows {
		if row.RolloverSourceChildID != nil && *row.RolloverSourceChildID == sourceID {
			return row.ID
		}
	}
	t.Fatalf("no rolled row from source %d", sourceID)
	return 0
}
