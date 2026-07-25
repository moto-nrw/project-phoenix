// Coverage tests for the scheduler's Set* injection methods, the
// checkAndRunBreakAutoEnd fallback path, and the instance-overdue tick's
// startup + shutdown loop. These paths are reachable without a real DB by
// exercising the non-tenant-aware branches (forEachTenant / forEachTenantSettings
// fall back to the plain ctx when db/schoolRepo aren't wired).
package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Setter coverage — pure wiring methods that the factory calls at startup.
// -----------------------------------------------------------------------------

func TestScheduler_SetWorkSessionCleaner(t *testing.T) {
	s := &Scheduler{logger: slog.Default()}
	cleaner := &fakeWorkSessionCleaner{}
	s.SetWorkSessionCleaner(cleaner)
	assert.Same(t, cleaner, s.workSessionCleanup)
}

func TestScheduler_SetBreakAutoEnder(t *testing.T) {
	s := &Scheduler{logger: slog.Default()}
	ender := &mockBreakAutoEnder{}
	s.SetBreakAutoEnder(ender)
	assert.Same(t, ender, s.breakAutoEnder)
}

func TestScheduler_SetDB(t *testing.T) {
	s := &Scheduler{logger: slog.Default()}
	// Passing nil is legitimate at test time — the field is only consulted
	// by forEachTenant, and a nil db falls back to the plain-ctx branch.
	s.SetDB(nil)
	assert.Nil(t, s.db)
}

func TestScheduler_SetSchoolRepo(t *testing.T) {
	s := &Scheduler{logger: slog.Default()}
	s.SetSchoolRepo(nil)
	assert.Nil(t, s.schoolRepo)
}

func TestScheduler_SetSettingsService(t *testing.T) {
	s := &Scheduler{logger: slog.Default()}
	resolver := &stubSettingsResolver{}
	s.SetSettingsService(resolver)
	assert.Same(t, resolver, s.settings)
}

func TestScheduler_SetAutoStartService(t *testing.T) {
	s := &Scheduler{logger: slog.Default()}
	autoStart := &fakeAutoStartService{}
	s.SetAutoStartService(autoStart)
	assert.Same(t, autoStart, s.autoStart)
}

// -----------------------------------------------------------------------------
// checkAndRunBreakAutoEnd — the non-tenant-aware branch is exercised by
// leaving db/schoolRepo unset; forEachTenant then falls back to fn(ctx).
// -----------------------------------------------------------------------------

type fakeWorkSessionCleaner struct{}

func (f *fakeWorkSessionCleaner) CleanupOpenSessions(_ context.Context) (int, error) {
	return 0, nil
}

type countingBreakAutoEnder struct {
	mu    sync.Mutex
	calls int
	count int
	err   error
}

func (c *countingBreakAutoEnder) AutoEndExpiredBreaks(_ context.Context) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.count, c.err
}

func TestCheckAndRunBreakAutoEnd_HappyPath(t *testing.T) {
	ender := &countingBreakAutoEnder{count: 3}
	s := &Scheduler{
		breakAutoEnder: ender,
		logger:         slog.Default(),
	}
	task := &ScheduledTask{Name: "break-auto-end", Schedule: "60s-poll"}

	s.checkAndRunBreakAutoEnd(task)

	assert.Equal(t, 1, ender.calls, "break auto-ender should run once")
}

func TestCheckAndRunBreakAutoEnd_ServiceError(t *testing.T) {
	ender := &countingBreakAutoEnder{err: errors.New("db broke")}
	s := &Scheduler{
		breakAutoEnder: ender,
		logger:         slog.Default(),
	}
	task := &ScheduledTask{Name: "break-auto-end", Schedule: "60s-poll"}

	// Error path: the func inside forEachTenant returns the err, which
	// logs and continues — no panic, no blocked task. Running is reset.
	s.checkAndRunBreakAutoEnd(task)

	task.mu.Lock()
	defer task.mu.Unlock()
	assert.False(t, task.Running, "Running flag must be reset after failure")
}

func TestCheckAndRunBreakAutoEnd_AlreadyRunning(t *testing.T) {
	ender := &countingBreakAutoEnder{}
	s := &Scheduler{
		breakAutoEnder: ender,
		logger:         slog.Default(),
	}
	task := &ScheduledTask{Name: "break-auto-end", Running: true}

	s.checkAndRunBreakAutoEnd(task)

	assert.Equal(t, 0, ender.calls, "must skip when task already running")
}

func TestCheckAndRunBreakAutoEnd_ZeroCount(t *testing.T) {
	// count=0 path: the "if count > 0" branch is skipped, no Info log line.
	ender := &countingBreakAutoEnder{count: 0}
	s := &Scheduler{
		breakAutoEnder: ender,
		logger:         slog.Default(),
	}
	task := &ScheduledTask{Name: "break-auto-end"}

	s.checkAndRunBreakAutoEnd(task)
	assert.Equal(t, 1, ender.calls)
}

// -----------------------------------------------------------------------------
// checkAndRunOverdue — exercised with a fake repo in non-tenant-aware mode.
// -----------------------------------------------------------------------------

type fakeInstanceRepo struct {
	mu        sync.Mutex
	calls     int
	instances []*scheduleModel.ActivityInstance
	err       error
}

type fakeOverdueRoomRepo struct {
	facilitiesModel.RoomRepository
	rooms []*facilitiesModel.Room
	err   error
}

func (f *fakeOverdueRoomRepo) FindByIDs(_ context.Context, _ []int64) ([]*facilitiesModel.Room, error) {
	return f.rooms, f.err
}

func (f *fakeInstanceRepo) Create(_ context.Context, _ *scheduleModel.ActivityInstance) error {
	return nil
}
func (f *fakeInstanceRepo) CreateTemplateBackedIfAbsent(_ context.Context, _ *scheduleModel.ActivityInstance) (bool, error) {
	return false, nil
}
func (f *fakeInstanceRepo) FindByID(_ context.Context, _ any) (*scheduleModel.ActivityInstance, error) {
	return nil, nil
}
func (f *fakeInstanceRepo) Update(_ context.Context, _ *scheduleModel.ActivityInstance) error {
	return nil
}
func (f *fakeInstanceRepo) Delete(_ context.Context, _ any) error { return nil }
func (f *fakeInstanceRepo) FindByTenantAndDate(_ context.Context, _ timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.instances, f.err
}
func (f *fakeInstanceRepo) List(_ context.Context, _ *base.QueryOptions) ([]*scheduleModel.ActivityInstance, error) {
	return nil, nil
}
func (f *fakeInstanceRepo) FindByTenantAndDateRange(_ context.Context, _, _ timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	return nil, nil
}
func (f *fakeInstanceRepo) FindByActivityGroupAndDate(_ context.Context, _ int64, _ timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	return nil, nil
}
func (f *fakeInstanceRepo) FindByActivityGroupAndDateRange(_ context.Context, _ int64, _, _ timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	return nil, nil
}
func (f *fakeInstanceRepo) FindByActiveGroupID(_ context.Context, _ int64) (*scheduleModel.ActivityInstance, error) {
	return nil, nil
}
func (f *fakeInstanceRepo) FindByIDs(_ context.Context, _ []int64) ([]*scheduleModel.ActivityInstance, error) {
	return nil, nil
}

func (f *fakeInstanceRepo) FindPlannedTemplateBackedFrom(_ context.Context, _ timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	return nil, nil
}
func (f *fakeInstanceRepo) MarkCompleted(_ context.Context, _ int64, _ time.Time) error {
	return nil
}

func TestCheckAndRunOverdue_AlreadyRunning(t *testing.T) {
	repo := &fakeInstanceRepo{}
	spy := testpkg.NewRecordingBroadcaster()
	s := &Scheduler{
		instanceRepo:       repo,
		overdueBroadcaster: spy,
		logger:             slog.Default(),
	}
	task := &ScheduledTask{Name: "instance-overdue", Running: true}

	s.checkAndRunOverdue(task)

	assert.Equal(t, 0, repo.calls, "must skip when task running")
}

func TestCheckAndRunOverdue_NoTenantContext(t *testing.T) {
	// Without db/schoolRepo, forEachTenantSettings calls fn(ctx, 0) once —
	// the threshold is resolved from registry default (5), and runOverdueForTenant
	// runs for tenant 0 with no instances, which means no broadcasts.
	repo := &fakeInstanceRepo{instances: nil}
	spy := testpkg.NewRecordingBroadcaster()
	s := &Scheduler{
		instanceRepo:       repo,
		overdueBroadcaster: spy,
		logger:             slog.Default(),
	}
	task := &ScheduledTask{Name: "instance-overdue"}

	s.checkAndRunOverdue(task)

	assert.Equal(t, 1, repo.calls, "one per-tenant call in fallback mode")
	assert.Empty(t, spy.CallsByMethod("tenant"), "no overdue instances → no broadcast")
	// Running flag must be cleared after the call.
	task.mu.Lock()
	defer task.mu.Unlock()
	assert.False(t, task.Running)
}

func TestRunOverdueForTenant_SkipsSchulhof(t *testing.T) {
	today := timezone.NewDate(2026, 4, 20)
	now := time.Date(today.Year, today.Month, today.Day, 10, 30, 0, 0, time.Local)
	newInstance := func(id, roomID int64) *scheduleModel.ActivityInstance {
		inst := &scheduleModel.ActivityInstance{
			Date:          today,
			StartTime:     time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
			EndTime:       time.Date(1, 1, 1, 11, 0, 0, 0, time.UTC),
			Status:        scheduleModel.InstanceStatusPlanned,
			IsSpontaneous: true,
			RoomID:        roomID,
		}
		inst.ID = id
		return inst
	}
	schulhofInstance := newInstance(151, 251)
	normalInstance := newInstance(152, 252)
	repo := &fakeInstanceRepo{instances: []*scheduleModel.ActivityInstance{schulhofInstance, normalInstance}}
	roomRepo := &fakeOverdueRoomRepo{rooms: []*facilitiesModel.Room{
		{Model: base.Model{ID: 251}, Name: constants.SchulhofRoomName},
		{Model: base.Model{ID: 252}, Name: "Lernraum"},
	}}
	spy := testpkg.NewRecordingBroadcaster()
	s := &Scheduler{
		logger:             slog.Default(),
		instanceRepo:       repo,
		instanceRoomRepo:   roomRepo,
		overdueBroadcaster: spy,
	}

	s.runOverdueForTenant(context.Background(), 1, 5, now)

	assert.Equal(t, 0, spyFilter(spy, schulhofInstance.ID, realtime.EventInstanceOverdue))
	assert.Equal(t, 0, spyFilter(spy, schulhofInstance.ID, realtime.EventActiveSupervisionChanged))
	assert.Equal(t, 1, spyFilter(spy, normalInstance.ID, realtime.EventInstanceOverdue))
	assert.Equal(t, 1, spyFilter(spy, normalInstance.ID, realtime.EventActiveSupervisionChanged))
}

func TestRunOverdueForTenant_FailsClosedWhenRoomResolutionFails(t *testing.T) {
	today := timezone.NewDate(2026, 4, 20)
	inst := &scheduleModel.ActivityInstance{
		Date:          today,
		StartTime:     time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
		EndTime:       time.Date(1, 1, 1, 11, 0, 0, 0, time.UTC),
		Status:        scheduleModel.InstanceStatusPlanned,
		IsSpontaneous: true,
		RoomID:        253,
	}
	inst.ID = 153

	tests := []struct {
		name     string
		roomRepo *fakeOverdueRoomRepo
	}{
		{name: "lookup error", roomRepo: &fakeOverdueRoomRepo{err: errors.New("rooms unavailable")}},
		{name: "unresolved room", roomRepo: &fakeOverdueRoomRepo{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := testpkg.NewRecordingBroadcaster()
			s := &Scheduler{
				logger:             slog.Default(),
				instanceRepo:       &fakeInstanceRepo{instances: []*scheduleModel.ActivityInstance{inst}},
				instanceRoomRepo:   tt.roomRepo,
				overdueBroadcaster: spy,
			}

			s.runOverdueForTenant(
				context.Background(),
				1,
				5,
				time.Date(today.Year, today.Month, today.Day, 10, 30, 0, 0, time.Local),
			)

			assert.Empty(t, spy.CallsByMethod("tenant"))
		})
	}
}

// -----------------------------------------------------------------------------
// scheduleInstanceOverdueTask — registers when all dependencies are set.
// -----------------------------------------------------------------------------

func TestScheduleInstanceOverdueTask_MissingRepo(t *testing.T) {
	s := &Scheduler{
		logger: slog.Default(),
		tasks:  make(map[string]*ScheduledTask),
		done:   make(chan struct{}),
		// instanceRepo not set → task should not register
		overdueBroadcaster: testpkg.NewRecordingBroadcaster(),
	}
	s.scheduleInstanceOverdueTask()
	assert.Empty(t, s.tasks, "no repo → no task registered")
}

func TestScheduleInstanceOverdueTask_MissingBroadcaster(t *testing.T) {
	s := &Scheduler{
		logger:       slog.Default(),
		tasks:        make(map[string]*ScheduledTask),
		done:         make(chan struct{}),
		instanceRepo: &fakeInstanceRepo{},
	}
	s.scheduleInstanceOverdueTask()
	assert.Empty(t, s.tasks, "no broadcaster → no task registered")
}

func TestScheduleInstanceOverdueTask_MissingRoomRepo(t *testing.T) {
	s := &Scheduler{
		logger:             slog.Default(),
		tasks:              make(map[string]*ScheduledTask),
		done:               make(chan struct{}),
		instanceRepo:       &fakeInstanceRepo{},
		overdueBroadcaster: testpkg.NewRecordingBroadcaster(),
	}
	s.scheduleInstanceOverdueTask()
	assert.Empty(t, s.tasks, "no room repo → no task registered")
}

func TestScheduleInstanceOverdueTask_Registers(t *testing.T) {
	s := &Scheduler{
		logger:             slog.Default(),
		tasks:              make(map[string]*ScheduledTask),
		done:               make(chan struct{}),
		instanceRepo:       &fakeInstanceRepo{},
		instanceRoomRepo:   &fakeOverdueRoomRepo{},
		overdueBroadcaster: testpkg.NewRecordingBroadcaster(),
	}
	s.scheduleInstanceOverdueTask()

	s.mu.RLock()
	task, ok := s.tasks["instance-overdue"]
	s.mu.RUnlock()
	require.True(t, ok, "task should be registered")
	assert.Equal(t, "1m-poll", task.Schedule)

	// Shut down the goroutine to avoid leaking it across tests.
	close(s.done)
	s.wg.Wait()
}

func TestRunInstanceOverdueTaskPolling_ExitsOnDone(t *testing.T) {
	// Pre-close done before launching so waitUntilNextMinute returns false
	// immediately after the startup check.
	repo := &fakeInstanceRepo{}
	spy := testpkg.NewRecordingBroadcaster()
	s := &Scheduler{
		logger:             slog.Default(),
		tasks:              make(map[string]*ScheduledTask),
		done:               make(chan struct{}),
		instanceRepo:       repo,
		overdueBroadcaster: spy,
	}
	close(s.done)
	task := &ScheduledTask{Name: "instance-overdue"}

	finished := make(chan struct{})
	s.wg.Add(1)
	go func() {
		s.runInstanceOverdueTaskPolling(task)
		close(finished)
	}()

	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("runInstanceOverdueTaskPolling did not exit on done signal")
	}

	// Startup check ran once before waitUntilNextMinute returned false.
	assert.GreaterOrEqual(t, repo.calls, 1)
}

// -----------------------------------------------------------------------------
// timetable-auto-start — optional service, gated by tenant settings.
// -----------------------------------------------------------------------------

type fakeAutoStartService struct {
	mu     sync.Mutex
	calls  int
	err    error
	result *scheduleSvc.AutoStartResult
}

func (f *fakeAutoStartService) RunForTenant(_ context.Context, _ time.Time) (*scheduleSvc.AutoStartResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.result != nil {
		return f.result, f.err
	}
	return &scheduleSvc.AutoStartResult{}, f.err
}

func TestScheduleAutoStartTask_MissingService(t *testing.T) {
	s := &Scheduler{
		logger: slog.Default(),
		tasks:  make(map[string]*ScheduledTask),
		done:   make(chan struct{}),
	}
	s.scheduleAutoStartTask()
	assert.Empty(t, s.tasks, "no service → no task registered")
}

func TestScheduleAutoStartTask_Registers(t *testing.T) {
	svc := &fakeAutoStartService{}
	s := &Scheduler{
		logger:    slog.Default(),
		tasks:     make(map[string]*ScheduledTask),
		done:      make(chan struct{}),
		autoStart: svc,
	}
	s.scheduleAutoStartTask()

	s.mu.RLock()
	task, ok := s.tasks["timetable-auto-start"]
	s.mu.RUnlock()
	require.True(t, ok, "task should be registered")
	assert.Equal(t, "1m-poll", task.Schedule)

	close(s.done)
	s.wg.Wait()
}

func TestCheckAndRunAutoStart_DefaultDisabled(t *testing.T) {
	svc := &fakeAutoStartService{}
	s := &Scheduler{
		autoStart: svc,
		logger:    slog.Default(),
		settings:  &stubSettingsResolver{},
	}
	task := &ScheduledTask{Name: "timetable-auto-start"}

	s.checkAndRunAutoStart(task)

	svc.mu.Lock()
	defer svc.mu.Unlock()
	assert.Equal(t, 0, svc.calls, "registry default false must not auto-start")
}

func TestCheckAndRunAutoStart_EnabledBySettings(t *testing.T) {
	svc := &fakeAutoStartService{result: &scheduleSvc.AutoStartResult{Checked: 2, Started: 1}}
	s := &Scheduler{
		autoStart: svc,
		logger:    slog.Default(),
		settings:  &stubSettingsResolver{hasOverride: true, boolVal: true},
	}
	task := &ScheduledTask{Name: "timetable-auto-start"}

	s.checkAndRunAutoStart(task)

	svc.mu.Lock()
	defer svc.mu.Unlock()
	assert.Equal(t, 1, svc.calls, "enabled timetable + auto-start should run once in fallback mode")
}

// emitInstanceOverdue's broadcaster-failure branch: spy with fail=true so the
// broadcast call returns an error and the Warn log path is exercised.
func TestRunOverdueForTenant_BroadcastFailure(t *testing.T) {
	today := timezone.NewDate(2026, 4, 20)
	startTime := time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC) // 10:00 local
	inst := &scheduleModel.ActivityInstance{
		Date:          today,
		StartTime:     startTime,
		EndTime:       time.Date(1, 1, 1, 11, 0, 0, 0, time.UTC),
		Status:        scheduleModel.InstanceStatusPlanned,
		IsSpontaneous: true,
		RoomID:        42,
	}
	inst.ID = int64(101)

	repo := &fakeInstanceRepo{instances: []*scheduleModel.ActivityInstance{inst}}
	spy := testpkg.NewRecordingBroadcaster()
	spy.Err = errors.New("forced failure")
	s := &Scheduler{
		logger:       slog.Default(),
		instanceRepo: repo,
		instanceRoomRepo: &fakeOverdueRoomRepo{rooms: []*facilitiesModel.Room{
			{Model: base.Model{ID: 42}, Name: "Lernraum"},
		}},
		overdueBroadcaster: spy,
	}

	// Use a `now` set to 10:30 local on the same day → 30 min past threshold=5.
	now := time.Date(today.Year, today.Month, today.Day, 10, 30, 0, 0, time.Local)
	s.runOverdueForTenant(context.Background(), 1, 5, now)

	assert.Len(t, spy.CallsByMethod("tenant"), 1, "broadcast attempted even when failure is expected")
}

// threshold < 1 is a documented no-op — exercises the early-return branch.
func TestRunOverdueForTenant_ThresholdZero(t *testing.T) {
	repo := &fakeInstanceRepo{}
	spy := testpkg.NewRecordingBroadcaster()
	s := &Scheduler{
		logger:             slog.Default(),
		instanceRepo:       repo,
		overdueBroadcaster: spy,
	}
	s.runOverdueForTenant(context.Background(), 1, 0, time.Now())
	assert.Equal(t, 0, repo.calls, "threshold < 1 must skip repo call")
}

// Exercises the repo error-log branch.
func TestRunOverdueForTenant_RepoError(t *testing.T) {
	repo := &fakeInstanceRepo{err: errors.New("db down")}
	spy := testpkg.NewRecordingBroadcaster()
	s := &Scheduler{
		logger:             slog.Default(),
		instanceRepo:       repo,
		overdueBroadcaster: spy,
	}
	s.runOverdueForTenant(context.Background(), 1, 5, time.Now())
	assert.Empty(t, spy.CallsByMethod("tenant"), "repo error must not result in any broadcast")
}

func TestRunInstanceOverdueTaskPolling_TickerFires(t *testing.T) {
	// synctest: drive the fake clock past waitUntilNextMinute + a few ticker
	// intervals so the ticker.C branch + done-exit branch both execute.
	synctest.Test(t, func(t *testing.T) {
		repo := &fakeInstanceRepo{}
		spy := testpkg.NewRecordingBroadcaster()
		s := &Scheduler{
			logger:             slog.Default(),
			tasks:              make(map[string]*ScheduledTask),
			done:               make(chan struct{}),
			instanceRepo:       repo,
			overdueBroadcaster: spy,
		}
		task := &ScheduledTask{Name: "instance-overdue"}

		s.wg.Add(1)
		go s.runInstanceOverdueTaskPolling(task)

		// Advance the fake clock past 1 min alignment + 2 ticker intervals.
		time.Sleep(3 * time.Minute)
		synctest.Wait()

		repo.mu.Lock()
		calls := repo.calls
		repo.mu.Unlock()
		assert.GreaterOrEqual(t, calls, 2, "ticker should have fired checkAndRunOverdue at least twice (startup + 1 tick)")

		close(s.done)
		s.wg.Wait()
	})
}

// -----------------------------------------------------------------------------
// forEachTenant — fallback branch (no db/schoolRepo) + fn error.
// -----------------------------------------------------------------------------

func TestForEachTenant_NoTenantConfig_InvokesFn(t *testing.T) {
	s := &Scheduler{logger: slog.Default()}
	var called int
	err := s.forEachTenant(context.Background(), "test-op", func(_ context.Context) error {
		called++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, called)
}

func TestForEachTenant_NoTenantConfig_PropagatesErr(t *testing.T) {
	s := &Scheduler{logger: slog.Default()}
	fnErr := errors.New("boom")
	err := s.forEachTenant(context.Background(), "test-op", func(_ context.Context) error {
		return fnErr
	})
	assert.ErrorIs(t, err, fnErr)
}

func TestForEachTenantSettings_NoTenantConfig_InvokesFn(t *testing.T) {
	s := &Scheduler{logger: slog.Default()}
	var called int
	var gotTenantID int64
	s.forEachTenantSettings(context.Background(), "test-op", func(_ context.Context, tid int64) error {
		called++
		gotTenantID = tid
		return nil
	})
	assert.Equal(t, 1, called)
	assert.Equal(t, int64(0), gotTenantID, "fallback passes tenant id 0")
}

// -----------------------------------------------------------------------------
// resolveStringSetting — override path (HasTenantOverride returns true).
// resolveBoolSetting, resolveIntSetting — override check fails (err path).
// -----------------------------------------------------------------------------

type stubSettingsResolver struct {
	hasOverride    bool
	hasOverrideErr error
	stringVal      string
	stringErr      error
	boolVal        bool
	boolErr        error
	intVal         int
	intErr         error
}

func (s *stubSettingsResolver) HasTenantOverride(_ context.Context, _ string) (bool, error) {
	return s.hasOverride, s.hasOverrideErr
}
func (s *stubSettingsResolver) ResolveString(_ context.Context, _ string) (string, error) {
	return s.stringVal, s.stringErr
}
func (s *stubSettingsResolver) ResolveBool(_ context.Context, _ string) (bool, error) {
	return s.boolVal, s.boolErr
}
func (s *stubSettingsResolver) ResolveInt(_ context.Context, _ string) (int, error) {
	return s.intVal, s.intErr
}

func TestResolveStringSetting_TenantOverride(t *testing.T) {
	s := &Scheduler{
		settings: &stubSettingsResolver{hasOverride: true, stringVal: "override-value"},
		logger:   slog.Default(),
	}
	val := s.resolveStringSetting(context.Background(), "some.key", "SOME_ENV", "default")
	assert.Equal(t, "override-value", val)
}

func TestResolveStringSetting_OverrideCheckError(t *testing.T) {
	// When HasTenantOverride returns err, fall through to env/default.
	s := &Scheduler{
		settings: &stubSettingsResolver{hasOverrideErr: errors.New("db down")},
		logger:   slog.Default(),
	}
	val := s.resolveStringSetting(context.Background(), "some.key", "SOME_ENV_NEVER_SET_XYZ", "fallback")
	assert.Equal(t, "fallback", val)
}

func TestResolveStringSetting_OverrideEmptyString(t *testing.T) {
	// Override exists but the resolved value is empty → fall through to default.
	s := &Scheduler{
		settings: &stubSettingsResolver{hasOverride: true, stringVal: ""},
		logger:   slog.Default(),
	}
	val := s.resolveStringSetting(context.Background(), "some.key", "NEVER_SET_YYZ", "fallback")
	assert.Equal(t, "fallback", val)
}

func TestResolveBoolSetting_TenantOverride(t *testing.T) {
	s := &Scheduler{
		settings: &stubSettingsResolver{hasOverride: true, boolVal: true},
		logger:   slog.Default(),
	}
	val := s.resolveBoolSetting(context.Background(), "some.key", "NEVER_SET_BBB", false)
	assert.True(t, val)
}

func TestResolveBoolSetting_OverrideCheckError(t *testing.T) {
	s := &Scheduler{
		settings: &stubSettingsResolver{hasOverrideErr: errors.New("db down")},
		logger:   slog.Default(),
	}
	val := s.resolveBoolSetting(context.Background(), "some.key", "NEVER_SET_CCC", true)
	assert.True(t, val)
}

func TestResolveIntSetting_TenantOverride(t *testing.T) {
	s := &Scheduler{
		settings: &stubSettingsResolver{hasOverride: true, intVal: 99},
		logger:   slog.Default(),
	}
	val := s.resolveIntSetting(context.Background(), "some.key", "NEVER_SET_III", 5)
	assert.Equal(t, 99, val)
}

func TestResolveIntSetting_OverrideCheckError(t *testing.T) {
	s := &Scheduler{
		settings: &stubSettingsResolver{hasOverrideErr: errors.New("db down")},
		logger:   slog.Default(),
	}
	val := s.resolveIntSetting(context.Background(), "some.key", "NEVER_SET_DDD", 7)
	assert.Equal(t, 7, val)
}

func TestResolveIntSetting_OverrideZeroFallsThrough(t *testing.T) {
	// When override resolves to 0 (or negative), resolveIntSetting falls
	// through to env/default — documented invariant.
	s := &Scheduler{
		settings: &stubSettingsResolver{hasOverride: true, intVal: 0},
		logger:   slog.Default(),
	}
	val := s.resolveIntSetting(context.Background(), "some.key", "NEVER_SET_EEE", 11)
	assert.Equal(t, 11, val)
}

func TestResolveNonNegativeIntSetting_OverrideZeroHonored(t *testing.T) {
	// Zero is a meaningful value for settings like
	// tracking.auto_checkout_grace_minutes (checkout exactly at shift end).
	s := &Scheduler{
		settings: &stubSettingsResolver{hasOverride: true, intVal: 0},
		logger:   slog.Default(),
	}
	val := s.resolveNonNegativeIntSetting(context.Background(), "some.key", "NEVER_SET_FFF", 15)
	assert.Equal(t, 0, val)
}

func TestResolveNonNegativeIntSetting_NegativeFallsThrough(t *testing.T) {
	s := &Scheduler{
		settings: &stubSettingsResolver{hasOverride: true, intVal: -3},
		logger:   slog.Default(),
	}
	val := s.resolveNonNegativeIntSetting(context.Background(), "some.key", "NEVER_SET_GGG", 15)
	assert.Equal(t, 15, val)
}

func TestResolveNonNegativeIntSetting_PositiveOverride(t *testing.T) {
	s := &Scheduler{
		settings: &stubSettingsResolver{hasOverride: true, intVal: 30},
		logger:   slog.Default(),
	}
	val := s.resolveNonNegativeIntSetting(context.Background(), "some.key", "NEVER_SET_HHH", 15)
	assert.Equal(t, 30, val)
}

// Stubs for the issue #585 cleanup refactor interface additions — unused by
// the setter tests.
func (f *fakeInstanceRepo) CompleteActiveByActiveGroupIDs(context.Context, []int64, time.Time) (int64, error) {
	return 0, nil
}

func (f *fakeInstanceRepo) CountWithOptions(context.Context, *base.QueryOptions) (int, error) {
	return 0, nil
}

func (f *fakeInstanceRepo) OldestBefore(context.Context, string, *timezone.Date) (*timezone.Date, error) {
	return nil, nil
}

func (f *fakeInstanceRepo) DeleteOlderThan(context.Context, string, timezone.Date) (int64, error) {
	return 0, nil
}

func (f *fakeInstanceRepo) DeletePlannedNonSpontaneousInWindow(context.Context, timezone.Date, *timezone.Date, *int64, bool) (int64, error) {
	return 0, nil
}

func (f *fakeInstanceRepo) PropagateListKindToFutureInstances(context.Context, int64, *string, *string, timezone.Date) (int64, error) {
	return 0, nil
}

func (f *fakeInstanceRepo) UpdateColumns(context.Context, *scheduleModel.ActivityInstance, ...string) (int64, error) {
	return 0, nil
}
