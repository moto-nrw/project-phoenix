package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
)

type unitOfWorkOutboxRunner struct {
	maxAttempts int
	called      bool
	backlog     int
	err         error
}

func (w *unitOfWorkOutboxRunner) RunOnce(ctx context.Context, _ int, maxAttempts int) (int, error) {
	w.called = true
	w.maxAttempts = maxAttempts
	if w.err != nil {
		return 0, w.err
	}
	err := testpkg.WithinAdminTransaction(ctx, func(context.Context) error { return nil })
	return 1, err
}

func (w *unitOfWorkOutboxRunner) Backlog(context.Context) (int, error) {
	return w.backlog, nil
}

func TestRunOutboxOnceReportsWorkerUnitOfWorkEvidence(t *testing.T) {
	t.Parallel()
	runner := &unitOfWorkOutboxRunner{backlog: 7}
	var results []string
	var logs bytes.Buffer
	scheduler := newScheduler(WorkerDependencies{
		Logger:        slog.New(slog.NewTextHandler(&logs, nil)),
		TenantRuntime: unitTenantRuntime(),
		OutboxWorker:  runner,
		UnitOfWorkObserver: func(entryPoint, kind, result string, _ time.Duration, _ int) {
			if entryPoint == "worker" && kind == "transaction" {
				results = append(results, result)
			}
		},
	})

	scheduler.runOutboxOnce(context.Background(), &ScheduledTask{})

	assert.True(t, runner.called)
	assert.Equal(t, 6, runner.maxAttempts)
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
		TenantRuntime: unitTenantRuntime(),
		OutboxWorker:  runner,
		Tracer: WorkerTracer{Failure: func(_ context.Context, gotOperation, gotOutcome string, _ error) {
			operation, outcome = gotOperation, gotOutcome
		}},
	})

	scheduler.runOutboxOnce(context.Background(), &ScheduledTask{})

	assert.Equal(t, "email-outbox", operation)
	assert.Equal(t, "run_failure", outcome)
}

func TestRunOutboxOncePassesRetryLimitOnEachTick(t *testing.T) {
	t.Parallel()
	runner := &unitOfWorkOutboxRunner{}
	settings := &fakeSettingsResolver{intValues: map[string]int{settingEnrollmentOutboxMaxAttempts: 3}}
	scheduler := newScheduler(WorkerDependencies{
		Logger: slog.Default(), TenantRuntime: unitTenantRuntime(), OutboxWorker: runner, Settings: settings,
	})
	task := &ScheduledTask{}
	scheduler.runOutboxOnce(context.Background(), task)
	assert.Equal(t, 3, runner.maxAttempts)
	settings.intValues[settingEnrollmentOutboxMaxAttempts] = 5
	scheduler.runOutboxOnce(context.Background(), task)
	assert.Equal(t, 5, runner.maxAttempts)
}

func TestRunOutboxOncePropagatesJobCorrelationToFailure(t *testing.T) {
	t.Parallel()

	runner := &unitOfWorkOutboxRunner{err: errors.New("claim failed")}
	correlated := false
	scheduler := newScheduler(WorkerDependencies{
		Logger:        slog.Default(),
		TenantRuntime: unitTenantRuntime(),
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
