package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type unitOfWorkOutboxRunner struct {
	called  bool
	backlog int
	err     error
}

func (*unitOfWorkOutboxRunner) SetMaxAttempts(int) {}

func (w *unitOfWorkOutboxRunner) RunOnce(ctx context.Context, _ int) (int, error) {
	w.called = true
	if w.err != nil {
		return 0, w.err
	}
	err := tenant.WithinAdmin(ctx, func(context.Context) error { return nil })
	return 1, err
}

func (w *unitOfWorkOutboxRunner) Backlog(context.Context) (int, error) {
	return w.backlog, nil
}

func TestRunOutboxOnceReportsWorkerUnitOfWorkEvidence(t *testing.T) {
	t.Parallel()
	uow, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error { return fn(ctx, struct{}{}) },
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)
	runner := &unitOfWorkOutboxRunner{backlog: 7}
	var results []string
	var logs bytes.Buffer
	scheduler := newScheduler(WorkerDependencies{
		Logger:        slog.New(slog.NewTextHandler(&logs, nil)),
		TenantRuntime: &uow,
		OutboxWorker:  runner,
		UnitOfWorkObserver: func(entryPoint, kind, result string, _ time.Duration, _ int) {
			if entryPoint == "worker" && kind == "transaction" {
				results = append(results, result)
			}
		},
	})

	scheduler.runOutboxOnce(context.Background(), &ScheduledTask{})

	assert.True(t, runner.called)
	assert.Equal(t, []string{"commit"}, results)
	assert.True(t, strings.Contains(logs.String(), "job_id=email-outbox"))
	assert.True(t, strings.Contains(logs.String(), "backlog=7"))
}

func TestRunOutboxOnceReportsFailure(t *testing.T) {
	t.Parallel()

	runner := &unitOfWorkOutboxRunner{err: errors.New("claim failed")}
	var operation, outcome string
	scheduler := newScheduler(WorkerDependencies{
		Logger:        slog.Default(),
		TenantRuntime: newTestUnitOfWork(t),
		OutboxWorker:  runner,
		Tracer: WorkerTracer{Failure: func(_ context.Context, gotOperation, gotOutcome string, _ error) {
			operation, outcome = gotOperation, gotOutcome
		}},
	})

	scheduler.runOutboxOnce(context.Background(), &ScheduledTask{})

	assert.Equal(t, "email-outbox", operation)
	assert.Equal(t, "run_failure", outcome)
}

func TestRunOutboxOncePropagatesJobCorrelationToFailure(t *testing.T) {
	t.Parallel()

	runner := &unitOfWorkOutboxRunner{err: errors.New("claim failed")}
	correlated := false
	scheduler := newScheduler(WorkerDependencies{
		Logger:        slog.Default(),
		TenantRuntime: newTestUnitOfWork(t),
		OutboxWorker:  runner,
		Tracer: WorkerTracer{
			StartJob: func(ctx context.Context, _ string) (context.Context, error) {
				return context.WithValue(ctx, workerCorrelationKey{}, true), nil
			},
			Failure: func(ctx context.Context, _, _ string, _ error) {
				correlated, _ = ctx.Value(workerCorrelationKey{}).(bool)
			},
		},
	})

	scheduler.runJobCheck(&ScheduledTask{Name: "email-outbox"}, scheduler.runOutboxOnce)

	assert.True(t, correlated)
}

func newTestUnitOfWork(t *testing.T) *tenant.UnitOfWork {
	t.Helper()
	runtime, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)
	return &runtime
}
