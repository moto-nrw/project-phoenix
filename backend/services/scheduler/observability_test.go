package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
)

type workerCorrelationKey struct{}

func TestRunCleanupJobsEmitsCorrelatedWorkerFailure(t *testing.T) {
	t.Parallel()

	runCalled := false
	scheduler := &Scheduler{
		cleanupJobs: []CleanupJob{{
			Description: "must not run",
			Run: func(context.Context) (int, error) {
				runCalled = true
				return 0, nil
			},
		}},
	}

	var gotOperation, gotOutcome string
	var gotCorrelation bool
	legacyObservations := 0
	scheduler.SetTenantRuntimeObserver(func(_, _ string) { legacyObservations++ })
	scheduler.SetWorkerTracer(WorkerTracer{
		StartJob: func(ctx context.Context, operation string) (context.Context, error) {
			if operation != "cleanup-jobs" {
				t.Fatalf("StartJob() operation = %q, want cleanup-jobs", operation)
			}
			return context.WithValue(ctx, workerCorrelationKey{}, true), nil
		},
		Failure: func(ctx context.Context, operation, outcome string, _ error) {
			gotOperation = operation
			gotOutcome = outcome
			gotCorrelation, _ = ctx.Value(workerCorrelationKey{}).(bool)
		},
	})

	err := scheduler.RunCleanupJobs()
	if !errors.Is(err, tenant.ErrRuntimeRequired) {
		t.Fatalf("RunCleanupJobs() error = %v, want tenant.ErrRuntimeRequired", err)
	}
	if runCalled {
		t.Fatal("cleanup job ran without Worker runtime")
	}
	if gotOperation != "cleanup-jobs" || gotOutcome != "missing_tenant" || !gotCorrelation {
		t.Fatalf("failure = (%q, %q, correlated=%v), want (cleanup-jobs, missing_tenant, true)", gotOperation, gotOutcome, gotCorrelation)
	}
	if legacyObservations != 0 {
		t.Fatalf("legacy observations = %d, want 0 to avoid duplicate metrics", legacyObservations)
	}
}

func TestExecuteTokenCleanupLogsTraceSetupFailure(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	scheduler := &Scheduler{logger: slog.New(slog.NewTextHandler(&logs, nil))}
	scheduler.SetWorkerTracer(WorkerTracer{
		StartJob: func(context.Context, string) (context.Context, error) {
			return nil, errors.New("random source unavailable")
		},
	})

	scheduler.executeTokenCleanup(&ScheduledTask{})

	if !strings.Contains(logs.String(), "token cleanup trace setup failed") {
		t.Fatalf("logs = %q, want trace setup failure", logs.String())
	}
}

func TestRunCleanupJobsStartsCorrelationBeforeEmptyJobExit(t *testing.T) {
	t.Parallel()

	started := false
	correlatedLog := false
	scheduler := &Scheduler{}
	scheduler.SetWorkerTracer(WorkerTracer{
		StartJob: func(ctx context.Context, _ string) (context.Context, error) {
			started = true
			return context.WithValue(ctx, workerCorrelationKey{}, true), nil
		},
		Logger: func(ctx context.Context) *slog.Logger {
			correlatedLog, _ = ctx.Value(workerCorrelationKey{}).(bool)
			return slog.New(slog.DiscardHandler)
		},
	})

	if err := scheduler.RunCleanupJobs(); err != nil {
		t.Fatalf("RunCleanupJobs() error = %v", err)
	}
	if !started || !correlatedLog {
		t.Fatalf("empty job exit = (started=%v, correlated log=%v), want both true", started, correlatedLog)
	}
}
