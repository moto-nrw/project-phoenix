package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

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
	scheduler.tenantRuntimeObserver = func(_, _ string) { legacyObservations++ }
	scheduler.workerTracer = WorkerTracer{
		StartJob: func(ctx context.Context, operation string) (context.Context, error) {
			if operation != "token-cleanup" {
				t.Fatalf("StartJob() operation = %q, want token-cleanup", operation)
			}
			return context.WithValue(ctx, workerCorrelationKey{}, true), nil
		},
		Failure: func(ctx context.Context, operation, outcome string, _ error) {
			gotOperation = operation
			gotOutcome = outcome
			gotCorrelation, _ = ctx.Value(workerCorrelationKey{}).(bool)
		},
	}

	err := scheduler.RunCleanupJobs()
	if !errors.Is(err, tenant.ErrRuntimeRequired) {
		t.Fatalf("RunCleanupJobs() error = %v, want tenant.ErrRuntimeRequired", err)
	}
	if runCalled {
		t.Fatal("cleanup job ran without Worker runtime")
	}
	if gotOperation != "token-cleanup" || gotOutcome != "missing_tenant" || !gotCorrelation {
		t.Fatalf("failure = (%q, %q, correlated=%v), want (token-cleanup, missing_tenant, true)", gotOperation, gotOutcome, gotCorrelation)
	}
	if legacyObservations != 0 {
		t.Fatalf("legacy observations = %d, want 0 to avoid duplicate metrics", legacyObservations)
	}
}

func TestRunCleanupJobsReturnsTraceSetupFailure(t *testing.T) {
	t.Parallel()

	scheduler := &Scheduler{}
	scheduler.workerTracer = WorkerTracer{
		StartJob: func(context.Context, string) (context.Context, error) {
			return nil, errors.New("random source unavailable")
		},
	}

	if err := scheduler.RunCleanupJobs(); !errors.Is(err, errWorkerTraceStart) {
		t.Fatalf("RunCleanupJobs() error = %v, want errWorkerTraceStart", err)
	}
}

func TestRunCleanupJobsStartsCorrelationBeforeEmptyJobExit(t *testing.T) {
	t.Parallel()

	started := false
	correlatedLog := false
	scheduler := &Scheduler{}
	scheduler.workerTracer = WorkerTracer{
		StartJob: func(ctx context.Context, _ string) (context.Context, error) {
			started = true
			return context.WithValue(ctx, workerCorrelationKey{}, true), nil
		},
		Logger: func(ctx context.Context) *slog.Logger {
			correlatedLog, _ = ctx.Value(workerCorrelationKey{}).(bool)
			return slog.New(slog.DiscardHandler)
		},
	}

	if err := scheduler.RunCleanupJobs(); err != nil {
		t.Fatalf("RunCleanupJobs() error = %v", err)
	}
	if !started || !correlatedLog {
		t.Fatalf("empty job exit = (started=%v, correlated log=%v), want both true", started, correlatedLog)
	}
}

func TestPollingJobsReportPanicsAndDrain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*Scheduler, *ScheduledTask, func(context.Context, *ScheduledTask))
	}{
		{
			name: "minute polling",
			run: func(scheduler *Scheduler, task *ScheduledTask, check func(context.Context, *ScheduledTask)) {
				scheduler.runMinutePolling(task, "minute panic", "minute start", check)
			},
		},
		{
			name: "interval polling",
			run: func(scheduler *Scheduler, task *ScheduledTask, check func(context.Context, *ScheduledTask)) {
				scheduler.runIntervalPolling(task, "interval panic", "interval start", 0, func() time.Duration {
					return time.Hour
				}, check)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testPollingPanicAndDrain(t, tt.run)
		})
	}
}

func TestRunJobCheckRecordsDurationByStableID(t *testing.T) {
	t.Parallel()

	scheduler := newUnitScheduler(nil, nil, nil, nil, nil, nil, slog.New(slog.DiscardHandler))
	var gotJobID JobID
	var gotOutcome string
	var gotDuration time.Duration
	scheduler.workerTracer.Run = func(jobID JobID, outcome string, duration time.Duration) {
		gotJobID = jobID
		gotOutcome = outcome
		gotDuration = duration
	}

	scheduler.runJobCheck(&ScheduledTask{Name: "duration-job"}, func(context.Context, *ScheduledTask) {})

	if gotJobID != "duration-job" || gotOutcome != "completed" || gotDuration < 0 {
		t.Fatalf("run evidence = (%q, %q, %s), want (duration-job, completed, non-negative)", gotJobID, gotOutcome, gotDuration)
	}
}

func TestFormerCustomPollingJobsRecordStableIDs(t *testing.T) {
	t.Parallel()

	scheduler := unitScheduler(&Scheduler{
		activeService:  &mockActiveService{},
		breakAutoEnder: &countingBreakAutoEnder{},
		logger:         slog.New(slog.DiscardHandler),
	})
	var got []JobID
	scheduler.workerTracer.Run = func(jobID JobID, _ string, _ time.Duration) {
		got = append(got, jobID)
	}
	checks := []struct {
		id    JobID
		check func(context.Context, *ScheduledTask)
	}{
		{id: "session-cleanup", check: scheduler.checkAndRunSessionCleanup},
		{id: "break-auto-end", check: scheduler.checkAndRunBreakAutoEnd},
		{id: "auto-checkout", check: scheduler.checkAndRunAutoCheckout},
	}
	for _, check := range checks {
		scheduler.runJobCheck(&ScheduledTask{Name: string(check.id)}, check.check)
	}

	want := []JobID{"session-cleanup", "break-auto-end", "auto-checkout"}
	if !slices.Equal(got, want) {
		t.Fatalf("run job IDs = %v, want %v", got, want)
	}
}

func testPollingPanicAndDrain(t *testing.T, run func(*Scheduler, *ScheduledTask, func(context.Context, *ScheduledTask))) {
	t.Helper()
	var logs bytes.Buffer
	failure := make(chan string, 1)
	scheduler := newUnitScheduler(nil, nil, nil, nil, nil, nil,
		slog.New(slog.NewTextHandler(&logs, nil)))
	scheduler.workerTracer.Failure = func(_ context.Context, operation, outcome string, _ error) {
		failure <- operation + ":" + outcome
	}
	task := &ScheduledTask{Name: "panic-job"}
	scheduler.wg.Add(1)
	go run(scheduler, task, func(context.Context, *ScheduledTask) { panic("boom") })
	select {
	case got := <-failure:
		if got != "panic-job:panic" {
			t.Fatalf("failure = %q, want panic-job:panic", got)
		}
	case <-time.After(time.Second):
		t.Fatal("panic was not reported")
	}
	scheduler.Stop()
	if !strings.Contains(logs.String(), "job_id=panic-job") {
		t.Fatalf("panic log lacks job ID: %s", logs.String())
	}
}
