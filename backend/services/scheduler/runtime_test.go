package scheduler

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/jobruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// mustTenantRuntime binds a composed tenant runtime through the Worker's
// production adapter.
func mustTenantRuntime(runtime jobruntime.Runtime, err error) ports.TenantRuntime {
	if err != nil {
		panic(err)
	}
	return runtime
}

// unitTenantRuntime is the database-less runtime of the test support.
func unitTenantRuntime() ports.TenantRuntime {
	return mustTenantRuntime(jobruntime.New(testpkg.PassthroughTenantRuntime()))
}

// scriptedTenantRuntime is the database-less runtime with a scripted tenant
// transaction and retry decision.
func scriptedTenantRuntime(withinTenant func(context.Context, int64, func(context.Context, any) error) error, retryable func(error) bool) ports.TenantRuntime {
	return mustTenantRuntime(jobruntime.New(testpkg.ScriptedTenantRuntime(withinTenant, retryable)))
}

// dbTenantRuntime is the production tenant runtime against the test database.
func dbTenantRuntime(t *testing.T, db *bun.DB) ports.TenantRuntime {
	t.Helper()
	return mustTenantRuntime(jobruntime.New(testpkg.TenantRuntime(t, db)))
}

const schedulerUnitTenantID int64 = 1

func testEnv(values ...string) func(string) string {
	if len(values)%2 != 0 {
		panic("testEnv requires key/value pairs")
	}
	env := make(map[string]string, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		env[values[i]] = values[i+1]
	}
	return func(key string) string { return env[key] }
}

func TestStopCancelsRunningTaskContexts(t *testing.T) {
	t.Parallel()

	scheduler := newUnitScheduler(nil, nil, nil, nil, nil, nil, slog.Default())
	ctx, cancel := scheduler.taskContext(scheduler.lifecycleContext(), time.Hour)
	defer cancel()

	stopped := make(chan struct{})
	scheduler.wg.Add(1)
	go func() {
		defer scheduler.wg.Done()
		<-ctx.Done()
		close(stopped)
	}()

	scheduler.Stop()

	require.ErrorIs(t, ctx.Err(), context.Canceled)
	select {
	case <-stopped:
	default:
		t.Fatal("scheduler stopped before its task context was cancelled")
	}
}

func newUnitScheduler(
	activeService ports.SessionMaintenance,
	cleanupService ports.PresenceCleanup,
	authService ports.AuthCleanup,
	invitationService ports.InvitationCleaner,
	emailChangeCleaner ports.EmailChangeTokenCleaner,
	operatorInvitationCleaner ports.OperatorInvitationCleaner,
	logger *slog.Logger,
) *Scheduler {
	scheduler := newScheduler(ports.WorkerDependencies{
		Logger:                    logger,
		TenantRuntime:             unitTenantRuntime(),
		Active:                    activeService,
		ActiveCleanup:             cleanupService,
		AuthCleanup:               authService,
		InvitationCleanup:         invitationService,
		EmailChangeCleanup:        emailChangeCleaner,
		OperatorInvitationCleanup: operatorInvitationCleaner,
		FeedbackCleaner:           &fakeFeedbackCleaner{},
	})
	jobs := scheduler.jobDefinitions()
	required := make([]ports.JobID, 0, len(jobs))
	for _, job := range jobs {
		required = append(required, job.ID())
	}
	registry, err := NewRegistry(required, jobs...)
	if err != nil {
		panic(err)
	}
	scheduler.registry = registry
	scheduler.minuteSnapshotLoader = func(context.Context) (*schedulerMinuteSnapshot, error) {
		return &schedulerMinuteSnapshot{tenantIDs: []int64{schedulerUnitTenantID}}, errSchedulerSettingsBatchUnsupported
	}
	scheduler.allTenantIDsLoader = func(context.Context) ([]int64, error) {
		return []int64{schedulerUnitTenantID}, nil
	}
	return scheduler
}

// unitScheduler builds a scheduler with the fields configure sets and the
// unit defaults for every runtime dependency configure leaves unset.
func unitScheduler(configure func(*Scheduler)) *Scheduler {
	scheduler := &Scheduler{}
	if configure != nil {
		configure(scheduler)
	}
	configured := newUnitScheduler(nil, nil, nil, nil, nil, nil, scheduler.logger)
	if scheduler.tenantRuntime == nil {
		scheduler.tenantRuntime = configured.tenantRuntime
	}
	if scheduler.minuteSnapshotLoader == nil && scheduler.schoolRepo == nil {
		scheduler.minuteSnapshotLoader = configured.minuteSnapshotLoader
	}
	if scheduler.allTenantIDsLoader == nil && scheduler.schoolRepo == nil {
		scheduler.allTenantIDsLoader = configured.allTenantIDsLoader
	}
	if scheduler.registry == nil {
		scheduler.registry = configured.registry
	}
	if scheduler.feedbackCleaner == nil {
		scheduler.feedbackCleaner = configured.feedbackCleaner
	}
	return scheduler
}
