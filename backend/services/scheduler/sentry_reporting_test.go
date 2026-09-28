package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Issue #3640: a job run that fails for good after all retries, or panics,
// becomes exactly one Sentry event tagged with the job. Failed attempts are
// breadcrumbs of that event and never events of their own.

type sentryRecordingTransport chan *sentry.Event

func (t sentryRecordingTransport) Configure(sentry.ClientOptions)        {}
func (t sentryRecordingTransport) Flush(time.Duration) bool              { return true }
func (t sentryRecordingTransport) FlushWithContext(context.Context) bool { return true }
func (t sentryRecordingTransport) Close()                                {}
func (t sentryRecordingTransport) SendEvent(event *sentry.Event)         { t <- event }

func (t sentryRecordingTransport) recorded() []*sentry.Event {
	var events []*sentry.Event
	for {
		select {
		case event := <-t:
			events = append(events, event)
		default:
			return events
		}
	}
}

// newRecordingHub returns a hub whose client records instead of sending.
func newRecordingHub(t *testing.T) (*sentry.Hub, sentryRecordingTransport) {
	t.Helper()
	transport := make(sentryRecordingTransport, 16)
	client, err := sentry.NewClient(sentry.ClientOptions{Transport: transport})
	require.NoError(t, err)
	return sentry.NewHub(client, sentry.NewScope()), transport
}

type sentryTestSchedulerOptions struct {
	retryable func(error) bool
	run       func(JobID, string, time.Duration)
	// stopped starts every job run on a context the scheduler already
	// cancelled, as during shutdown.
	stopped bool
}

// newSentryTestScheduler builds a scheduler whose job runs report to a
// recording client. The worker tracer binds the recording hub to every job
// context, so the test never touches the global hub.
func newSentryTestScheduler(t *testing.T, options sentryTestSchedulerOptions) (*Scheduler, sentryRecordingTransport) {
	t.Helper()
	retryable := options.retryable
	if retryable == nil {
		retryable = func(error) bool { return false }
	}
	run := options.run
	if run == nil {
		run = func(JobID, string, time.Duration) {}
	}
	runtime, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		retryable,
	)
	require.NoError(t, err)
	hub, transport := newRecordingHub(t)
	scheduler := newScheduler(WorkerDependencies{
		Logger:        slog.New(slog.DiscardHandler),
		TenantRuntime: &runtime,
		Tracer: WorkerTracer{
			StartJob: func(ctx context.Context, _ string) (context.Context, error) {
				ctx = sentry.SetHubOnContext(ctx, hub)
				if options.stopped {
					stoppedCtx, stop := context.WithCancel(ctx)
					stop()
					return stoppedCtx, nil
				}
				return ctx, nil
			},
			Run: run,
		},
	})
	return scheduler, transport
}

func sentryBreadcrumbMessages(event *sentry.Event) []string {
	messages := make([]string, 0, len(event.Breadcrumbs))
	for _, crumb := range event.Breadcrumbs {
		messages = append(messages, crumb.Message)
	}
	return messages
}

func TestJobRunFailingForGoodSendsOneEventWithAttemptBreadcrumbs(t *testing.T) {
	t.Parallel()

	deadlock := errors.New("deadlock detected")
	scheduler, transport := newSentryTestScheduler(t, sentryTestSchedulerOptions{
		retryable: func(err error) bool { return errors.Is(err, deadlock) },
	})
	attempts := 0

	scheduler.runJobCheck(&ScheduledTask{Name: "timetable-auto-end"}, func(ctx context.Context, _ *ScheduledTask) {
		scheduler.runTenantBatches(ctx, []int64{71}, "timetable-auto-end", RetrySafeTenantCommandFunc(func(context.Context, tenant.TenantID) error {
			attempts++
			return deadlock
		}))
	})

	events := transport.recorded()
	require.Len(t, events, 1, "one run that fails for good is exactly one event")
	event := events[0]
	assert.Equal(t, "timetable-auto-end", event.Tags["job"])
	assert.Equal(t, "71", event.Tags["school_id"], "every failure of the run belongs to school 71")
	require.NotEmpty(t, event.Exception)
	assert.Equal(t, "timetable-auto-end tenant 71: deadlock detected", event.Exception[len(event.Exception)-1].Value)
	assert.NotContains(t, event.Tags, "portal")
	assert.NotContains(t, event.Tags, "request_id")
	assert.Empty(t, event.User.ID)

	assert.Equal(t, 4, attempts, "the runtime retries a retry-safe command three times")
	want := make([]string, 0, attempts+1)
	for range attempts {
		want = append(want, "tenant command attempt failed")
	}
	want = append(want, "tenant command failed")
	assert.Equal(t, want, sentryBreadcrumbMessages(event))
	assert.Equal(t, "deadlock detected", event.Breadcrumbs[0].Data["error"])
	assert.Equal(t, string(TenantOutcomeRetryExhausted), event.Breadcrumbs[attempts].Data["classification"])
	assert.Equal(t, int64(71), event.Breadcrumbs[attempts].Data["school_id"])
}

func TestJobRunRecoveringAfterFailedAttemptSendsNoEvent(t *testing.T) {
	t.Parallel()

	serialization := errors.New("serialization failure")
	var outcome string
	scheduler, transport := newSentryTestScheduler(t, sentryTestSchedulerOptions{
		retryable: func(err error) bool { return errors.Is(err, serialization) },
		run:       func(_ JobID, got string, _ time.Duration) { outcome = got },
	})
	attempts := 0

	scheduler.runJobCheck(&ScheduledTask{Name: "session-end"}, func(ctx context.Context, _ *ScheduledTask) {
		scheduler.runTenantBatches(ctx, []int64{72}, "session-end", RetrySafeTenantCommandFunc(func(context.Context, tenant.TenantID) error {
			attempts++
			if attempts == 1 {
				return serialization
			}
			return nil
		}))
	})

	assert.Equal(t, 2, attempts)
	assert.Equal(t, "completed", outcome)
	assert.Empty(t, transport.recorded(), "a failed attempt that a retry made good is no event")
}

func TestJobRunFailingInSeveralSchoolsLeavesSchoolAbsent(t *testing.T) {
	t.Parallel()

	scheduler, transport := newSentryTestScheduler(t, sentryTestSchedulerOptions{})

	scheduler.runJobCheck(&ScheduledTask{Name: "auto-checkout"}, func(ctx context.Context, _ *ScheduledTask) {
		scheduler.runTenantBatches(ctx, []int64{81, 82, 83}, "auto-checkout", TenantCommandFunc(func(_ context.Context, id tenant.TenantID) error {
			if id.Int64() == 82 {
				return nil
			}
			return errors.New("constraint violated")
		}))
	})

	events := transport.recorded()
	require.Len(t, events, 1, "failures in several schools are still one run and one event")
	assert.Equal(t, "auto-checkout", events[0].Tags["job"])
	assert.NotContains(t, events[0].Tags, "school_id")
	assert.Equal(t, []string{"tenant command failed", "tenant command failed"}, sentryBreadcrumbMessages(events[0]))
}

func TestJobRunFailingOutsideSchoolsLeavesSchoolAbsent(t *testing.T) {
	t.Parallel()

	scheduler, transport := newSentryTestScheduler(t, sentryTestSchedulerOptions{})

	scheduler.runJobCheck(&ScheduledTask{Name: "email-outbox"}, func(ctx context.Context, _ *ScheduledTask) {
		scheduler.runTenantBatches(ctx, []int64{91}, "email-outbox", TenantCommandFunc(func(context.Context, tenant.TenantID) error {
			return errors.New("claim failed")
		}))
		recordJobCommandFailure(ctx, errors.New("backlog query failed"))
	})

	events := transport.recorded()
	require.Len(t, events, 1)
	assert.Equal(t, "email-outbox", events[0].Tags["job"])
	assert.NotContains(t, events[0].Tags, "school_id", "a failure outside school 91 makes the run's school ambiguous")
	assert.Equal(t, []string{"tenant command failed", "job command failed"}, sentryBreadcrumbMessages(events[0]))
}

func TestJobRunPanicSendsOneEventAndNextRunIsNormal(t *testing.T) {
	t.Parallel()

	var outcomes []string
	scheduler, transport := newSentryTestScheduler(t, sentryTestSchedulerOptions{
		run: func(_ JobID, outcome string, _ time.Duration) { outcomes = append(outcomes, outcome) },
	})
	task := &ScheduledTask{Name: "status-flag-clear"}
	runs := 0
	check := func(context.Context, *ScheduledTask) {
		runs++
		if runs == 1 {
			panic("nil map write")
		}
	}

	scheduler.runJobCheck(task, check)
	scheduler.runJobCheck(task, check)

	assert.Equal(t, []string{"panic", "completed"}, outcomes)
	events := transport.recorded()
	require.Len(t, events, 1, "the panic is one event; the normal run after it is none")
	assert.Equal(t, "status-flag-clear", events[0].Tags["job"])
	assert.Equal(t, sentry.LevelFatal, events[0].Level)
}

func TestJobRunStoppedByShutdownSendsNoEvent(t *testing.T) {
	t.Parallel()

	scheduler, transport := newSentryTestScheduler(t, sentryTestSchedulerOptions{stopped: true})

	scheduler.runJobCheck(&ScheduledTask{Name: "visit-cleanup"}, func(ctx context.Context, _ *ScheduledTask) {
		scheduler.runTenantBatches(ctx, []int64{1, 2}, "visit-cleanup", TenantCommandFunc(func(context.Context, tenant.TenantID) error {
			return nil
		}))
	})

	assert.Empty(t, transport.recorded())
}

func TestJobRunReportsCommandFailureDespiteShutdown(t *testing.T) {
	t.Parallel()

	scheduler, transport := newSentryTestScheduler(t, sentryTestSchedulerOptions{stopped: true})
	scheduler.runJobCheck(&ScheduledTask{Name: "visit-cleanup"}, func(ctx context.Context, _ *ScheduledTask) {
		recordJobCommandFailure(ctx, errors.New("cleanup failed"))
		recordJobCommandFailure(ctx, context.Canceled)
	})

	events := transport.recorded()
	require.Len(t, events, 1, "shutdown must not hide a separate command failure")
	assert.Equal(t, "visit-cleanup", events[0].Tags["job"])
	assert.Equal(t, []string{"job command failed", "job command failed"}, sentryBreadcrumbMessages(events[0]))
}

func TestJobRunReportsKeepTheirBreadcrumbsApart(t *testing.T) {
	t.Parallel()
	hub, transport := newRecordingHub(t)
	ctx := sentry.SetHubOnContext(context.Background(), hub)

	succeeded := startJobRunReport(ctx, "email-outbox")
	recordFailedJobAttempt(succeeded, "transient failure", errors.New("timeout"), nil)

	failed := startJobRunReport(ctx, "email-outbox")
	recordStandingJobFailure(failed, 0, "final failure", errors.New("refused"), nil)
	reportJobRunFailure(failed, errors.New("refused"))

	events := transport.recorded()
	require.Len(t, events, 1)
	assert.Equal(t, []string{"final failure"}, sentryBreadcrumbMessages(events[0]))
}

// A job run reports without the scope of whatever started it: job events
// have no user.id, portal or request_id.
func TestJobRunReportDropsTheStartingScope(t *testing.T) {
	t.Parallel()
	hub, transport := newRecordingHub(t)
	hub.Scope().SetUser(sentry.User{ID: "42"})
	hub.Scope().SetTag("portal", "tenant")
	hub.Scope().SetTag("request_id", "req-1")
	hub.AddBreadcrumb(&sentry.Breadcrumb{Message: "request breadcrumb"}, nil)

	run := startJobRunReport(sentry.SetHubOnContext(context.Background(), hub), "token-cleanup")
	reportJobRunFailure(run, errors.New("cleanup failed"))
	reportJobRunFailure(run, nil)

	events := transport.recorded()
	require.Len(t, events, 1, "no error, no event")
	assert.Empty(t, events[0].User.ID)
	assert.Equal(t, map[string]string{"job": "token-cleanup"}, events[0].Tags)
	assert.Empty(t, events[0].Breadcrumbs)
}

func TestJobRunReportSetsSchoolOnlyForExactlyOneSchool(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		schools []int64
		want    string
	}{
		{name: "one school, several failures", schools: []int64{4, 4}, want: "4"},
		{name: "two schools", schools: []int64{4, 5}},
		{name: "one school and an unscoped failure", schools: []int64{4, 0}},
		{name: "unscoped failure only", schools: []int64{0}},
		{name: "no recorded failure", schools: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hub, transport := newRecordingHub(t)
			run := startJobRunReport(sentry.SetHubOnContext(context.Background(), hub), "session-end")
			for _, schoolID := range tc.schools {
				recordStandingJobFailure(run, schoolID, "failed", errors.New("boom"), nil)
			}

			reportJobRunFailure(run, errors.New("run failed"))

			events := transport.recorded()
			require.Len(t, events, 1)
			if tc.want == "" {
				assert.NotContains(t, events[0].Tags, "school_id")
				return
			}
			assert.Equal(t, tc.want, events[0].Tags["school_id"])
		})
	}
}
