package scheduler

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type unitOfWorkOutboxRunner struct {
	called bool
}

func (*unitOfWorkOutboxRunner) SetMaxAttempts(int) {}

func (w *unitOfWorkOutboxRunner) RunOnce(ctx context.Context, _ int) (int, error) {
	w.called = true
	err := tenant.WithinAdmin(ctx, func(context.Context) error { return nil })
	return 1, err
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
	runner := &unitOfWorkOutboxRunner{}
	scheduler := NewScheduler(nil, nil, nil, nil, nil, nil, slog.Default())
	scheduler.SetTenantRuntime(uow)
	scheduler.SetOutboxWorker(runner)
	var results []string
	scheduler.SetUnitOfWorkObserver(func(entryPoint, kind, result string, _ time.Duration, _ int) {
		if entryPoint == "worker" && kind == "transaction" {
			results = append(results, result)
		}
	})

	scheduler.runOutboxOnce(&ScheduledTask{})

	assert.True(t, runner.called)
	assert.Equal(t, []string{"commit"}, results)
}
