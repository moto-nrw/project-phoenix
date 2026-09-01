package scheduler

import (
	"context"
	"log/slog"
	"testing"
	"time"

	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

const schedulerUnitTenantID int64 = 1

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
	activeService activeSvc.Service,
	cleanupService activeSvc.CleanupService,
	authService AuthCleanup,
	invitationService InvitationCleaner,
	emailChangeCleaner EmailChangeTokenCleaner,
	operatorInvitationCleaner OperatorInvitationCleaner,
	logger *slog.Logger,
) *Scheduler {
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
	if err != nil {
		panic(err)
	}
	scheduler := newScheduler(WorkerDependencies{
		Logger:                    logger,
		TenantRuntime:             &runtime,
		Active:                    activeService,
		ActiveCleanup:             cleanupService,
		AuthCleanup:               authService,
		InvitationCleanup:         invitationService,
		EmailChangeCleanup:        emailChangeCleaner,
		OperatorInvitationCleanup: operatorInvitationCleaner,
		FeedbackCleaner:           &fakeFeedbackCleaner{},
	})
	jobs := scheduler.jobDefinitions()
	required := make([]JobID, 0, len(jobs))
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

func unitScheduler(scheduler *Scheduler) *Scheduler {
	configured := newUnitScheduler(nil, nil, nil, nil, nil, nil, scheduler.logger)
	if scheduler.db != nil {
		runtime, ok := testpkg.PackageTenantRuntime()
		if !ok {
			panic("tenant runtime is not configured for the scheduler test package")
		}
		scheduler.tenantRuntime = runtime
		scheduler.tenantRuntimeConfigured = true
		if setter, ok := scheduler.settings.(interface{ SetTenantRuntime(tenant.UnitOfWork) }); ok {
			setter.SetTenantRuntime(runtime)
		}
	}
	if !scheduler.tenantRuntimeConfigured {
		scheduler.tenantRuntime = configured.tenantRuntime
		scheduler.tenantRuntimeConfigured = true
	}
	if scheduler.minuteSnapshotLoader == nil && scheduler.db == nil && scheduler.schoolRepo == nil {
		scheduler.minuteSnapshotLoader = configured.minuteSnapshotLoader
	}
	if scheduler.allTenantIDsLoader == nil && scheduler.db == nil && scheduler.schoolRepo == nil {
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
