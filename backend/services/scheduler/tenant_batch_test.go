package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantBatchesBoundWorkAndPreserveEveryOutcome(t *testing.T) {
	t.Parallel()

	commandErr := errors.New("owner command failed")
	var evidence []TenantBatchEvidence
	scheduler := newTenantBatchTestScheduler(t, func(error) bool { return false })
	scheduler.workerTracer.Batch = func(event TenantBatchEvidence) {
		evidence = append(evidence, event)
	}

	tenantIDs := make([]int64, tenantBatchSize*2+1)
	for index := range tenantIDs {
		tenantIDs[index] = int64(index + 11)
	}
	var called []int64
	result := scheduler.runTenantBatches(context.Background(), tenantIDs, "bounded-job", TenantCommandFunc(func(_ context.Context, tenantID tenant.TenantID) error {
		called = append(called, tenantID.Int64())
		if tenantID.Int64() == tenantIDs[1] {
			return commandErr
		}
		return nil
	}))

	assert.Equal(t, tenantIDs, called)
	assert.Equal(t, 3, result.Batches)
	assert.Equal(t, 1, result.Failed())
	assert.Zero(t, result.Backlog)
	assert.ErrorIs(t, result.Err, commandErr)
	assert.Len(t, result.CompletedTenantIDs(), len(tenantIDs)-1)
	assert.NotContains(t, result.CompletedTenantIDs(), tenantIDs[1])
	require.Len(t, evidence, 3)
	assert.Equal(t, []int{tenantBatchSize, tenantBatchSize, 1}, []int{evidence[0].Processed, evidence[1].Processed, evidence[2].Processed})
	assert.Equal(t, []int{tenantBatchSize + 1, 1, 0}, []int{evidence[0].Backlog, evidence[1].Backlog, evidence[2].Backlog})
}

func TestTenantBatchErrorsKeepOnlyRepresentativeFailures(t *testing.T) {
	t.Parallel()

	scheduler := newTenantBatchTestScheduler(t, func(error) bool { return false })
	failures := []error{
		errors.New("tenant failure 1"),
		errors.New("tenant failure 2"),
		errors.New("tenant failure 3"),
		errors.New("tenant failure 4"),
		errors.New("tenant failure 5"),
	}
	tenantIDs := []int64{71, 72, 73, 74, 75}

	result := scheduler.runTenantBatches(context.Background(), tenantIDs, "bounded-errors", TenantCommandFunc(func(_ context.Context, tenantID tenant.TenantID) error {
		return failures[tenantID.Int64()-tenantIDs[0]]
	}))

	assert.Len(t, result.Outcomes, len(failures))
	for _, failure := range failures[:3] {
		assert.ErrorIs(t, result.Err, failure)
	}
	for _, failure := range failures[3:] {
		assert.NotErrorIs(t, result.Err, failure)
	}
	assert.ErrorContains(t, result.Err, "2 additional tenant failures omitted")
}

func TestTenantBatchesStopOnCancellationAndReportBacklog(t *testing.T) {
	t.Parallel()

	scheduler := newTenantBatchTestScheduler(t, func(error) bool { return false })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	tenantIDs := []int64{21, 22, 23}
	var called []int64

	result := scheduler.runTenantBatches(ctx, tenantIDs, "cancelled-job", TenantCommandFunc(func(_ context.Context, tenantID tenant.TenantID) error {
		called = append(called, tenantID.Int64())
		cancel()
		return nil
	}))

	assert.Equal(t, tenantIDs[:1], called)
	assert.Equal(t, 2, result.Backlog)
	assert.ErrorIs(t, result.Err, context.Canceled)

	called = nil
	resumed := scheduler.runTenantBatches(context.Background(), tenantIDs, "cancelled-job", TenantCommandFunc(func(_ context.Context, tenantID tenant.TenantID) error {
		called = append(called, tenantID.Int64())
		return nil
	}))
	assert.Equal(t, []int64{22, 23, 21}, called)
	assert.Zero(t, resumed.Backlog)
}

func TestTenantBatchesClassifyTransactionRetry(t *testing.T) {
	t.Parallel()

	retryErr := errors.New("serialization failure")
	scheduler := newTenantBatchTestScheduler(t, func(err error) bool {
		return errors.Is(err, retryErr)
	})
	attempts := 0

	result := scheduler.runTenantBatches(context.Background(), []int64{31}, "retry-job", RetrySafeTenantCommandFunc(func(context.Context, tenant.TenantID) error {
		attempts++
		if attempts < 3 {
			return retryErr
		}
		return nil
	}))

	require.Len(t, result.Outcomes, 1)
	assert.Equal(t, 3, attempts)
	assert.Equal(t, 2, result.Outcomes[0].Retries)
	assert.Equal(t, TenantOutcomeRetriedSuccess, result.Outcomes[0].Classification)
	assert.NoError(t, result.Err)
}

func TestTenantBatchesRetryDefaultCommandOnNextTickAfterRollback(t *testing.T) {
	t.Parallel()

	retryErr := errors.New("serialization failure")
	commits := 0
	scheduler := newTenantBatchTestSchedulerWithTenantRunner(t,
		func(ctx context.Context, _ int64, run func(context.Context, any) error) error {
			if err := run(ctx, struct{}{}); err != nil {
				return err
			}
			commits++
			if commits == 1 {
				return retryErr
			}
			return nil
		},
		func(err error) bool { return errors.Is(err, retryErr) },
	)
	var lastRun sync.Map
	var executed, skipped int
	command := TenantCommandFunc(func(ctx context.Context, tenantID tenant.TenantID) error {
		if wasRunToday(&lastRun, tenantID.Int64()) {
			skipped++
			return nil
		}
		executed++
		markRunTodayAfterCommit(ctx, &lastRun, tenantID.Int64())
		return nil
	})

	failed := scheduler.runTenantBatches(context.Background(), []int64{35}, "default-command", command)
	succeeded := scheduler.runTenantBatches(context.Background(), []int64{35}, "default-command", command)

	assert.Equal(t, 2, executed)
	assert.Zero(t, skipped)
	require.Len(t, failed.Outcomes, 1)
	assert.Zero(t, failed.Outcomes[0].Retries)
	assert.ErrorIs(t, failed.Err, retryErr)
	assert.NoError(t, succeeded.Err)
	assert.True(t, wasRunToday(&lastRun, 35))
}

func TestTenantBatchesObserveBacklogWithoutCompletedBatch(t *testing.T) {
	t.Parallel()

	scheduler := newTenantBatchTestScheduler(t, func(error) bool { return false })
	var jobID JobID
	backlog := -1
	scheduler.workerTracer.Backlog = func(gotJobID JobID, gotBacklog int) {
		jobID = gotJobID
		backlog = gotBacklog
	}

	result := scheduler.runTenantBatches(context.Background(), nil, "empty-job", TenantCommandFunc(func(context.Context, tenant.TenantID) error {
		return nil
	}))

	assert.Zero(t, result.Batches)
	assert.Zero(t, result.Backlog)
	assert.Equal(t, JobID("empty-job"), jobID)
	assert.Zero(t, backlog)
}

func TestTenantBatchesObserveBacklogWhenCancelledBeforeFirstBatch(t *testing.T) {
	t.Parallel()

	scheduler := newTenantBatchTestScheduler(t, func(error) bool { return false })
	backlog := -1
	scheduler.workerTracer.Backlog = func(_ JobID, gotBacklog int) {
		backlog = gotBacklog
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := scheduler.runTenantBatches(ctx, []int64{36, 37}, "cancelled-before-batch", TenantCommandFunc(func(context.Context, tenant.TenantID) error {
		return nil
	}))

	assert.Zero(t, result.Batches)
	assert.Equal(t, 2, result.Backlog)
	assert.ErrorIs(t, result.Err, context.Canceled)
	assert.Equal(t, 2, backlog)
}

func TestTenantBatchesRejectMissingTenantAndContinue(t *testing.T) {
	t.Parallel()

	scheduler := newTenantBatchTestScheduler(t, func(error) bool { return false })
	tenantIDs := []int64{0, 41}
	var called []int64

	result := scheduler.runTenantBatches(context.Background(), tenantIDs, "missing-tenant-job", TenantCommandFunc(func(_ context.Context, tenantID tenant.TenantID) error {
		called = append(called, tenantID.Int64())
		return nil
	}))

	assert.Equal(t, tenantIDs[1:], called)
	require.Len(t, result.Outcomes, 2)
	assert.Equal(t, TenantOutcomeMissingTenant, result.Outcomes[0].Classification)
	assert.Equal(t, TenantOutcomeSuccess, result.Outcomes[1].Classification)
	assert.Error(t, result.Err)
}

func TestTenantBatchesPropagateRetryExhaustionAndContinue(t *testing.T) {
	t.Parallel()

	retryErr := errors.New("deadlock")
	scheduler := newTenantBatchTestScheduler(t, func(err error) bool {
		return errors.Is(err, retryErr)
	})
	tenantIDs := []int64{51, 52}
	var called []int64

	result := scheduler.runTenantBatches(context.Background(), tenantIDs, "failed-retry-job", RetrySafeTenantCommandFunc(func(_ context.Context, tenantID tenant.TenantID) error {
		called = append(called, tenantID.Int64())
		if tenantID.Int64() == tenantIDs[0] {
			return retryErr
		}
		return nil
	}))

	assert.True(t, slices.Contains(called, tenantIDs[1]), "a failed tenant must not stop the next tenant")
	require.Len(t, result.Outcomes, 2)
	assert.Equal(t, TenantOutcomeRetryExhausted, result.Outcomes[0].Classification)
	assert.Equal(t, 3, result.Outcomes[0].Retries)
	assert.ErrorIs(t, result.Err, retryErr)
}

func TestJobRunReportsAggregatedCommandFailure(t *testing.T) {
	t.Parallel()

	commandErr := errors.New("command failed")
	scheduler := newTenantBatchTestScheduler(t, func(error) bool { return false })
	var gotOutcome, gotFailureOutcome string
	var gotErr error
	var gotBatchJobID JobID
	scheduler.workerTracer.Run = func(_ JobID, outcome string, _ time.Duration) {
		gotOutcome = outcome
	}
	scheduler.workerTracer.Failure = func(_ context.Context, _ string, outcome string, err error) {
		gotFailureOutcome = outcome
		gotErr = err
	}
	scheduler.workerTracer.Batch = func(event TenantBatchEvidence) {
		gotBatchJobID = event.JobID
	}

	scheduler.runJobCheck(&ScheduledTask{Name: "command-job"}, func(ctx context.Context, _ *ScheduledTask) {
		scheduler.runTenantBatches(ctx, []int64{61}, "inner-operation-name", TenantCommandFunc(func(context.Context, tenant.TenantID) error {
			return commandErr
		}))
	})

	assert.Equal(t, "failed", gotOutcome)
	assert.Equal(t, "command_failure", gotFailureOutcome)
	assert.ErrorIs(t, gotErr, commandErr)
	assert.Equal(t, JobID("command-job"), gotBatchJobID)
}

func newTenantBatchTestScheduler(t *testing.T, retryable func(error) bool) *Scheduler {
	t.Helper()
	return newTenantBatchTestSchedulerWithTenantRunner(t,
		func(ctx context.Context, _ int64, run func(context.Context, any) error) error {
			return run(ctx, struct{}{})
		},
		retryable,
	)
}

func newTenantBatchTestSchedulerWithTenantRunner(
	t *testing.T,
	withinTenant func(context.Context, int64, func(context.Context, any) error) error,
	retryable func(error) bool,
) *Scheduler {
	t.Helper()
	runtime, err := tenant.NewUnitOfWork(
		withinTenant,
		func(ctx context.Context, run func(context.Context, any) error) error {
			return run(ctx, struct{}{})
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		retryable,
	)
	require.NoError(t, err)
	return newScheduler(WorkerDependencies{
		Logger:        slog.New(slog.DiscardHandler),
		TenantRuntime: &runtime,
		Tracer: WorkerTracer{
			Run: func(JobID, string, time.Duration) {},
		},
	})
}
