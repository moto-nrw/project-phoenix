// Coverage tests for the checkAndRunBreakAutoEnd fallback path and the instance-overdue tick's
// startup + shutdown loop. These paths are reachable without a real DB by
// exercising the non-tenant-aware branches (forEachTenant / forEachTenantSettings
// fall back to the plain ctx when db/schoolRepo aren't wired).
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// checkAndRunBreakAutoEnd — the non-tenant-aware branch is exercised by
// leaving db/schoolRepo unset; forEachTenant then falls back to fn(ctx).
// -----------------------------------------------------------------------------

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
	t.Parallel()

	ender := &countingBreakAutoEnder{count: 3}
	s := unitScheduler(&Scheduler{
		breakAutoEnder: ender,
		logger:         slog.Default()})

	task := &ScheduledTask{Name: "break-auto-end", Schedule: "60s-poll"}

	s.checkAndRunBreakAutoEnd(context.Background(), task)

	assert.Equal(t, 1, ender.calls, "break auto-ender should run once")
}

func TestCheckAndRunBreakAutoEnd_ServiceError(t *testing.T) {
	t.Parallel()

	ender := &countingBreakAutoEnder{err: errors.New("db broke")}
	s := unitScheduler(&Scheduler{
		breakAutoEnder: ender,
		logger:         slog.Default()})

	task := &ScheduledTask{Name: "break-auto-end", Schedule: "60s-poll"}

	// Error path: the func inside forEachTenant returns the err, which
	// logs and continues — no panic, no blocked task. Running is reset.
	s.checkAndRunBreakAutoEnd(context.Background(), task)

	task.mu.Lock()
	defer task.mu.Unlock()
	assert.False(t, task.Running, "Running flag must be reset after failure")
}

func TestCheckAndRunBreakAutoEnd_AlreadyRunning(t *testing.T) {
	t.Parallel()

	ender := &countingBreakAutoEnder{}
	s := unitScheduler(&Scheduler{
		breakAutoEnder: ender,
		logger:         slog.Default()})

	task := &ScheduledTask{Name: "break-auto-end", Running: true}

	s.checkAndRunBreakAutoEnd(context.Background(), task)

	assert.Equal(t, 0, ender.calls, "must skip when task already running")
}

func TestCheckAndRunBreakAutoEnd_ZeroCount(t *testing.T) {
	t.Parallel()

	// count=0 path: the "if count > 0" branch is skipped, no Info log line.
	ender := &countingBreakAutoEnder{count: 0}
	s := unitScheduler(&Scheduler{
		breakAutoEnder: ender,
		logger:         slog.Default()})

	task := &ScheduledTask{Name: "break-auto-end"}

	s.checkAndRunBreakAutoEnd(context.Background(), task)
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

func (f *fakeInstanceRepo) MaxID(_ context.Context) (int64, error) {
	return 0, nil
}
func (f *fakeInstanceRepo) MarkCompleted(_ context.Context, _ int64, _ time.Time) error {
	return nil
}

func TestCheckAndRunOverdue_AlreadyRunning(t *testing.T) {
	t.Parallel()

	repo := &fakeInstanceRepo{}
	spy := testpkg.NewRecordingBroadcaster()
	s := unitScheduler(&Scheduler{
		instanceRepo:       repo,
		overdueBroadcaster: spy,
		logger:             slog.Default()})

	task := &ScheduledTask{Name: "instance-overdue", Running: true}

	s.checkAndRunOverdue(context.Background(), task)

	assert.Equal(t, 0, repo.calls, "must skip when task running")
}

func TestCheckAndRunOverdue_NoTenantContext(t *testing.T) {
	t.Parallel()

	// The unit runtime provides one explicit tenant —
	// the threshold is resolved from registry default (5), and runOverdueForTenant
	// runs with no instances, which means no broadcasts.
	repo := &fakeInstanceRepo{instances: nil}
	spy := testpkg.NewRecordingBroadcaster()
	s := unitScheduler(&Scheduler{
		instanceRepo:       repo,
		overdueBroadcaster: spy,
		logger:             slog.Default()})

	task := &ScheduledTask{Name: "instance-overdue"}

	s.checkAndRunOverdue(context.Background(), task)

	assert.Equal(t, 1, repo.calls, "one call for the configured unit-test tenant")
	assert.Empty(t, spy.CallsByMethod("tenant"), "no overdue instances → no broadcast")
	// Running flag must be cleared after the call.
	task.mu.Lock()
	defer task.mu.Unlock()
	assert.False(t, task.Running)
}

// Since #2161 the Schulhof is a regular plannable room: overdue planned
// yard blocks emit the same overdue events as any other room's blocks.
func TestRunOverdueForTenant_EmitsSchulhofLikeAnyRoom(t *testing.T) {
	t.Parallel()

	today := timezone.NewDate(2026, 4, 20)
	now := time.Date(today.Year(), today.Month(), today.Day(), 10, 30, 0, 0, time.Local)
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
		{ID: 251, Name: constants.SchulhofRoomName},
		{ID: 252, Name: "Lernraum"},
	}}
	spy := testpkg.NewRecordingBroadcaster()
	s := unitScheduler(&Scheduler{
		logger:             slog.Default(),
		instanceRepo:       repo,
		instanceRoomRepo:   roomRepo,
		overdueBroadcaster: spy})

	s.runOverdueForTenant(context.Background(), testpkg.Tenant(t), 5, now)

	assert.Equal(t, 1, spyFilter(spy, schulhofInstance.ID, realtime.EventInstanceOverdue))
	assert.Equal(t, 1, spyFilter(spy, schulhofInstance.ID, realtime.EventActiveSupervisionChanged))
	assert.Equal(t, 1, spyFilter(spy, normalInstance.ID, realtime.EventInstanceOverdue))
	assert.Equal(t, 1, spyFilter(spy, normalInstance.ID, realtime.EventActiveSupervisionChanged))
}

func TestRunOverdueForTenant_FailsClosedWhenRoomResolutionFails(t *testing.T) {
	t.Parallel()

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
			s := unitScheduler(&Scheduler{
				logger:             slog.Default(),
				instanceRepo:       &fakeInstanceRepo{instances: []*scheduleModel.ActivityInstance{inst}},
				instanceRoomRepo:   tt.roomRepo,
				overdueBroadcaster: spy})

			s.runOverdueForTenant(
				context.Background(),
				1,
				5,
				time.Date(today.Year(), today.Month(), today.Day(), 10, 30, 0, 0, time.Local),
			)

			assert.Empty(t, spy.CallsByMethod("tenant"))
		})
	}
}

// -----------------------------------------------------------------------------
// scheduleInstanceOverdueTask — registers when all dependencies are set.
// -----------------------------------------------------------------------------

func TestScheduleInstanceOverdueTask_MissingRepo(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		logger: slog.Default(),
		tasks:  make(map[string]*ScheduledTask),
		done:   make(chan struct{}),

		overdueBroadcaster: testpkg.NewRecordingBroadcaster()})

	s.scheduleInstanceOverdueTask()
	assert.Empty(t, s.tasks, "no repo → no task registered")
}

func TestScheduleInstanceOverdueTask_MissingBroadcaster(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		logger:       slog.Default(),
		tasks:        make(map[string]*ScheduledTask),
		done:         make(chan struct{}),
		instanceRepo: &fakeInstanceRepo{}})

	s.scheduleInstanceOverdueTask()
	assert.Empty(t, s.tasks, "no broadcaster → no task registered")
}

func TestScheduleInstanceOverdueTask_MissingRoomRepo(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		logger:             slog.Default(),
		tasks:              make(map[string]*ScheduledTask),
		done:               make(chan struct{}),
		instanceRepo:       &fakeInstanceRepo{},
		overdueBroadcaster: testpkg.NewRecordingBroadcaster()})

	s.scheduleInstanceOverdueTask()
	assert.Empty(t, s.tasks, "no room repo → no task registered")
}

func TestScheduleInstanceOverdueTask_Registers(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		logger:             slog.Default(),
		tasks:              make(map[string]*ScheduledTask),
		done:               make(chan struct{}),
		instanceRepo:       &fakeInstanceRepo{},
		instanceRoomRepo:   &fakeOverdueRoomRepo{},
		overdueBroadcaster: testpkg.NewRecordingBroadcaster()})

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
	t.Parallel()

	// Pre-close done before launching so waitUntilNextMinute returns false
	// immediately after the startup check.
	repo := &fakeInstanceRepo{}
	spy := testpkg.NewRecordingBroadcaster()
	s := unitScheduler(&Scheduler{
		logger:             slog.Default(),
		tasks:              make(map[string]*ScheduledTask),
		done:               make(chan struct{}),
		instanceRepo:       repo,
		overdueBroadcaster: spy})

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
	run    func(context.Context) (*scheduleSvc.AutoStartResult, error)
	mu     sync.Mutex
	calls  int
	err    error
	result *scheduleSvc.AutoStartResult
}

func (f *fakeAutoStartService) RunForTenant(ctx context.Context, _ time.Time) (*scheduleSvc.AutoStartResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.run != nil {
		return f.run(ctx)
	}
	if f.result != nil {
		return f.result, f.err
	}
	return &scheduleSvc.AutoStartResult{}, f.err
}

type autoStartFailureObservations struct {
	returned []error
	called   []int64
	batches  []TenantBatchEvidence
	reported error
	runs     []string
}

func TestAutoStartTenantFailureOutcomes(t *testing.T) {
	t.Parallel()
	for _, cancelled := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancelled=%t", cancelled), func(t *testing.T) {
			const jobID JobID = "timetable-auto-start"
			tenantIDs := []int64{81, 82}
			commandErr := errors.New("auto-start command failed")
			var cancel context.CancelFunc
			s, observed := autoStartFailureScheduler(t, tenantIDs, commandErr, func() {
				if cancelled {
					cancel()
				}
			})
			s.workerTracer.Batch = func(event TenantBatchEvidence) { observed.batches = append(observed.batches, event) }
			s.workerTracer.Failure = func(_ context.Context, name, outcome string, err error) {
				assert.Equal(t, string(jobID), name)
				assert.Equal(t, "command_failure", outcome)
				observed.reported = err
			}
			s.workerTracer.Run = func(id JobID, outcome string, _ time.Duration) {
				assert.Equal(t, jobID, id)
				observed.runs = append(observed.runs, outcome)
			}
			task := &ScheduledTask{Name: string(jobID)}
			s.runJobCheck(task, func(runCtx context.Context, task *ScheduledTask) {
				ctx, cancelRun := context.WithCancel(runCtx)
				cancel = cancelRun
				defer cancel()
				s.checkAndRunAutoStart(ctx, task)
			})
			assertAutoStartFailureOutcomes(t, s, observed, tenantIDs, commandErr, cancelled)
			assert.False(t, task.Running)
		})
	}
}

func autoStartFailureScheduler(t *testing.T, tenantIDs []int64, commandErr error, onFailure func()) (*Scheduler, *autoStartFailureObservations) {
	t.Helper()
	observed := &autoStartFailureObservations{}
	s := newTenantBatchTestSchedulerWithTenantRunner(t, func(ctx context.Context, _ int64, run func(context.Context, any) error) error {
		err := run(ctx, struct{}{})
		observed.returned = append(observed.returned, err)
		return err
	}, func(error) bool { return false })
	s.settings = &stubSettingsResolver{hasOverride: true, boolVal: true}
	s.minuteSnapshotLoader = func(context.Context) (*schedulerMinuteSnapshot, error) {
		return &schedulerMinuteSnapshot{tenantIDs: tenantIDs}, errSchedulerSettingsBatchUnsupported
	}
	s.autoStart = &fakeAutoStartService{run: func(ctx context.Context) (*scheduleSvc.AutoStartResult, error) {
		id := tenant.FromContext(ctx)
		observed.called = append(observed.called, id)
		if id == tenantIDs[0] {
			onFailure()
			return nil, commandErr
		}
		_, stored := s.tenantBatchCursors.Load(JobID("timetable-auto-start"))
		assert.False(t, stored, "failed tenant must not advance the successful cursor")
		return &scheduleSvc.AutoStartResult{}, nil
	}}
	return s, observed
}

func assertAutoStartFailureOutcomes(t *testing.T, s *Scheduler, observed *autoStartFailureObservations, tenantIDs []int64, commandErr error, cancelled bool) {
	t.Helper()
	const jobID JobID = "timetable-auto-start"
	require.NotEmpty(t, observed.returned)
	assert.Same(t, commandErr, observed.returned[0])
	assert.ErrorIs(t, observed.reported, commandErr)
	assert.ErrorContains(t, observed.reported, "timetable-auto-start tenant 81: auto-start command failed")
	assert.Equal(t, []string{"failed"}, observed.runs)
	require.Len(t, observed.batches, 1)
	batch := observed.batches[0]
	assert.Equal(t, jobID, batch.JobID)
	assert.Equal(t, 1, batch.Failed)
	cursor, stored := s.tenantBatchCursors.Load(jobID)
	if cancelled {
		assert.Equal(t, tenantIDs[:1], observed.called)
		assert.Equal(t, 1, batch.Backlog)
		assert.False(t, stored)
		assert.ErrorIs(t, observed.reported, context.Canceled)
	} else {
		assert.Equal(t, tenantIDs, observed.called)
		require.Len(t, observed.returned, 2)
		assert.NoError(t, observed.returned[1])
		assert.Zero(t, batch.Backlog)
		assert.Equal(t, tenantIDs[1], cursor)
	}
	assert.Equal(t, len(observed.called), batch.Processed)
}

func TestScheduleAutoStartTask_MissingService(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		logger: slog.Default(),
		tasks:  make(map[string]*ScheduledTask),
		done:   make(chan struct{})})

	s.scheduleAutoStartTask()
	assert.Empty(t, s.tasks, "no service → no task registered")
}

func TestScheduleAutoStartTask_Registers(t *testing.T) {
	t.Parallel()

	svc := &fakeAutoStartService{}
	s := unitScheduler(&Scheduler{
		logger:    slog.Default(),
		tasks:     make(map[string]*ScheduledTask),
		done:      make(chan struct{}),
		autoStart: svc})

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
	t.Parallel()

	svc := &fakeAutoStartService{}
	s := unitScheduler(&Scheduler{
		autoStart: svc,
		logger:    slog.Default(),
		settings:  &stubSettingsResolver{}})

	task := &ScheduledTask{Name: "timetable-auto-start"}

	s.checkAndRunAutoStart(context.Background(), task)

	svc.mu.Lock()
	defer svc.mu.Unlock()
	assert.Equal(t, 0, svc.calls, "registry default false must not auto-start")
}

func TestCheckAndRunAutoStart_EnabledBySettings(t *testing.T) {
	t.Parallel()

	svc := &fakeAutoStartService{result: &scheduleSvc.AutoStartResult{Checked: 2, Started: 1}}
	s := unitScheduler(&Scheduler{
		autoStart: svc,
		logger:    slog.Default(),
		settings:  &stubSettingsResolver{hasOverride: true, boolVal: true}})

	task := &ScheduledTask{Name: "timetable-auto-start"}

	s.checkAndRunAutoStart(context.Background(), task)

	svc.mu.Lock()
	defer svc.mu.Unlock()
	assert.Equal(t, 1, svc.calls, "enabled timetable + auto-start should run once for the configured tenant")
}

// -----------------------------------------------------------------------------
// timetable-auto-end — optional service, gated by tenant settings.
// -----------------------------------------------------------------------------

type fakeAutoEndService struct {
	mu     sync.Mutex
	calls  int
	grace  time.Duration
	err    error
	result *scheduleSvc.AutoEndResult
}

func (f *fakeAutoEndService) RunForTenant(_ context.Context, _ time.Time, grace time.Duration) (*scheduleSvc.AutoEndResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.grace = grace
	if f.result != nil {
		return f.result, f.err
	}
	return &scheduleSvc.AutoEndResult{}, f.err
}

func TestScheduleAutoEndTask_MissingService(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{logger: slog.Default(), tasks: make(map[string]*ScheduledTask), done: make(chan struct{})})
	s.scheduleAutoEndTask()
	assert.Empty(t, s.tasks)
}

func TestScheduleAutoEndTask_Registers(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		logger:  slog.Default(),
		tasks:   make(map[string]*ScheduledTask),
		done:    make(chan struct{}),
		autoEnd: &fakeAutoEndService{}})

	s.scheduleAutoEndTask()

	s.mu.RLock()
	task, ok := s.tasks["timetable-auto-end"]
	s.mu.RUnlock()
	require.True(t, ok)
	assert.Equal(t, "1m-poll", task.Schedule)

	close(s.done)
	s.wg.Wait()
}

func TestCheckAndRunAutoEnd_DefaultDisabled(t *testing.T) {
	t.Parallel()

	svc := &fakeAutoEndService{}
	s := unitScheduler(&Scheduler{autoEnd: svc, logger: slog.Default(), settings: &stubSettingsResolver{}})
	s.checkAndRunAutoEnd(context.Background(), &ScheduledTask{Name: "timetable-auto-end"})

	svc.mu.Lock()
	defer svc.mu.Unlock()
	assert.Zero(t, svc.calls)
}

func TestCheckAndRunAutoEnd_PassesConfiguredGrace(t *testing.T) {
	t.Parallel()

	svc := &fakeAutoEndService{result: &scheduleSvc.AutoEndResult{Checked: 1, Completed: 1}}
	s := unitScheduler(&Scheduler{
		autoEnd: svc,
		logger:  slog.Default(),
		settings: &stubSettingsResolver{
			hasOverride: true,
			boolVal:     true,
			intVal:      15,
		}})

	s.checkAndRunAutoEnd(context.Background(), &ScheduledTask{Name: "timetable-auto-end"})

	svc.mu.Lock()
	defer svc.mu.Unlock()
	assert.Equal(t, 1, svc.calls)
	assert.Equal(t, 15*time.Minute, svc.grace)
}

func TestCheckAndRunAutoEnd_UsesEnabledTimetableDefault(t *testing.T) {
	t.Parallel()

	svc := &fakeAutoEndService{}
	s := unitScheduler(&Scheduler{
		autoEnd: svc,
		logger:  slog.Default(),
		settings: &keyedBoolSettingsResolver{values: map[string]bool{
			configModel.KeyTimetableAutoEndEnabled: true,
		}}})

	s.checkAndRunAutoEnd(context.Background(), &ScheduledTask{Name: "timetable-auto-end"})

	svc.mu.Lock()
	defer svc.mu.Unlock()
	assert.Equal(t, 1, svc.calls)
}

type keyedBoolSettingsResolver struct {
	values map[string]bool
}

func (s *keyedBoolSettingsResolver) HasTenantOverride(_ context.Context, key string) (bool, error) {
	_, ok := s.values[key]
	return ok, nil
}

func (s *keyedBoolSettingsResolver) ResolveBool(_ context.Context, key string) (bool, error) {
	return s.values[key], nil
}

func (*keyedBoolSettingsResolver) ResolveString(context.Context, string) (string, error) {
	return "", nil
}

func (*keyedBoolSettingsResolver) ResolveInt(context.Context, string) (int, error) {
	return 0, nil
}

// emitInstanceOverdue's broadcaster-failure branch: spy with fail=true so the
// broadcast call returns an error and the Warn log path is exercised.
func TestRunOverdueForTenant_BroadcastFailure(t *testing.T) {
	t.Parallel()

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
	s := unitScheduler(&Scheduler{
		logger:       slog.Default(),
		instanceRepo: repo,
		instanceRoomRepo: &fakeOverdueRoomRepo{rooms: []*facilitiesModel.Room{
			{ID: 42, Name: "Lernraum"},
		}},
		overdueBroadcaster: spy})

	// Use a `now` set to 10:30 local on the same day → 30 min past threshold=5.
	now := time.Date(today.Year(), today.Month(), today.Day(), 10, 30, 0, 0, time.Local)
	s.runOverdueForTenant(context.Background(), testpkg.Tenant(t), 5, now)

	assert.Len(t, spy.CallsByMethod("tenant"), 1, "broadcast attempted even when failure is expected")
}

// threshold < 1 is a documented no-op — exercises the early-return branch.
func TestRunOverdueForTenant_ThresholdZero(t *testing.T) {
	t.Parallel()

	repo := &fakeInstanceRepo{}
	spy := testpkg.NewRecordingBroadcaster()
	s := unitScheduler(&Scheduler{
		logger:             slog.Default(),
		instanceRepo:       repo,
		overdueBroadcaster: spy})

	s.runOverdueForTenant(context.Background(), testpkg.Tenant(t), 0, time.Now())
	assert.Equal(t, 0, repo.calls, "threshold < 1 must skip repo call")
}

// Exercises the repo error-log branch.
func TestRunOverdueForTenant_RepoError(t *testing.T) {
	t.Parallel()

	repo := &fakeInstanceRepo{err: errors.New("db down")}
	spy := testpkg.NewRecordingBroadcaster()
	s := unitScheduler(&Scheduler{
		logger:             slog.Default(),
		instanceRepo:       repo,
		overdueBroadcaster: spy})

	s.runOverdueForTenant(context.Background(), testpkg.Tenant(t), 5, time.Now())
	assert.Empty(t, spy.CallsByMethod("tenant"), "repo error must not result in any broadcast")
}

func TestRunInstanceOverdueTaskPolling_TickerFires(t *testing.T) {
	t.Parallel()

	// synctest: drive the fake clock past waitUntilNextMinute + a few ticker
	// intervals so the ticker.C branch + done-exit branch both execute.
	synctest.Test(t, func(t *testing.T) {
		repo := &fakeInstanceRepo{}
		spy := testpkg.NewRecordingBroadcaster()
		s := unitScheduler(&Scheduler{
			logger:             slog.Default(),
			tasks:              make(map[string]*ScheduledTask),
			done:               make(chan struct{}),
			instanceRepo:       repo,
			overdueBroadcaster: spy})

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
// forEachTenant — fail-closed branch when tenant runtime is missing.
// -----------------------------------------------------------------------------

func TestForEachTenant_NoTenantConfig_RejectsWork(t *testing.T) {
	t.Parallel()

	s := &Scheduler{logger: slog.Default()}
	var called int
	err := s.forEachTenant(context.Background(), "test-op", func(_ context.Context) error {
		called++
		return nil
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "tenant runtime is not configured")
	assert.Zero(t, called)
}

func TestForEachTenant_NoTenantConfig_DoesNotInvokeFailingWork(t *testing.T) {
	t.Parallel()

	s := &Scheduler{logger: slog.Default()}
	fnErr := errors.New("boom")
	err := s.forEachTenant(context.Background(), "test-op", func(_ context.Context) error {
		return fnErr
	})
	assert.ErrorContains(t, err, "tenant runtime is not configured")
	assert.NotErrorIs(t, err, fnErr)
}

func TestForEachTenantSettings_NoTenantConfig_RejectsWork(t *testing.T) {
	t.Parallel()

	s := &Scheduler{logger: slog.Default()}
	var called int
	var gotTenantID int64
	s.forEachTenantSettings(context.Background(), "test-op", func(_ context.Context, tid int64) error {
		called++
		gotTenantID = tid
		return nil
	})
	assert.Zero(t, called)
	assert.Zero(t, gotTenantID)
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
	t.Parallel()

	s := unitScheduler(&Scheduler{
		settings: &stubSettingsResolver{hasOverride: true, stringVal: "override-value"},
		logger:   slog.Default()})

	val := s.resolveStringSetting(context.Background(), "some.key", "SOME_ENV", "default")
	assert.Equal(t, "override-value", val)
}

func TestResolveStringSetting_OverrideCheckError(t *testing.T) {
	t.Parallel()

	// When HasTenantOverride returns err, fall through to env/default.
	s := unitScheduler(&Scheduler{
		settings: &stubSettingsResolver{hasOverrideErr: errors.New("db down")},
		logger:   slog.Default()})

	val := s.resolveStringSetting(context.Background(), "some.key", "SOME_ENV_NEVER_SET_XYZ", "fallback")
	assert.Equal(t, "fallback", val)
}

func TestResolveStringSetting_OverrideEmptyString(t *testing.T) {
	t.Parallel()

	// Override exists but the resolved value is empty → fall through to default.
	s := unitScheduler(&Scheduler{
		settings: &stubSettingsResolver{hasOverride: true, stringVal: ""},
		logger:   slog.Default()})

	val := s.resolveStringSetting(context.Background(), "some.key", "NEVER_SET_YYZ", "fallback")
	assert.Equal(t, "fallback", val)
}

func TestResolveBoolSetting_TenantOverride(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		settings: &stubSettingsResolver{hasOverride: true, boolVal: true},
		logger:   slog.Default()})

	val := s.resolveBoolSetting(context.Background(), "some.key", "NEVER_SET_BBB", false)
	assert.True(t, val)
}

func TestResolveBoolSetting_OverrideCheckError(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		settings: &stubSettingsResolver{hasOverrideErr: errors.New("db down")},
		logger:   slog.Default()})

	val := s.resolveBoolSetting(context.Background(), "some.key", "NEVER_SET_CCC", true)
	assert.True(t, val)
}

func TestResolveIntSetting_TenantOverride(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		settings: &stubSettingsResolver{hasOverride: true, intVal: 99},
		logger:   slog.Default()})

	val := s.resolveIntSetting(context.Background(), "some.key", "NEVER_SET_III", 5)
	assert.Equal(t, 99, val)
}

func TestResolveIntSetting_OverrideCheckError(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		settings: &stubSettingsResolver{hasOverrideErr: errors.New("db down")},
		logger:   slog.Default()})

	val := s.resolveIntSetting(context.Background(), "some.key", "NEVER_SET_DDD", 7)
	assert.Equal(t, 7, val)
}

func TestResolveIntSetting_OverrideZeroFallsThrough(t *testing.T) {
	t.Parallel()

	// When override resolves to 0 (or negative), resolveIntSetting falls
	// through to env/default — documented invariant.
	s := unitScheduler(&Scheduler{
		settings: &stubSettingsResolver{hasOverride: true, intVal: 0},
		logger:   slog.Default()})

	val := s.resolveIntSetting(context.Background(), "some.key", "NEVER_SET_EEE", 11)
	assert.Equal(t, 11, val)
}

func TestResolveRequiredPositiveIntSetting(t *testing.T) {
	t.Parallel()

	t.Run("positive value", func(t *testing.T) {
		t.Parallel()
		s := unitScheduler(&Scheduler{
			settings: &stubSettingsResolver{intVal: 30},
			logger:   slog.Default()})

		val, err := s.resolveRequiredPositiveIntSetting(context.Background(), "some.key", "NEVER_SET_REQUIRED_POSITIVE")

		require.NoError(t, err)
		assert.Equal(t, 30, val)
	})

	t.Run("resolver error", func(t *testing.T) {
		t.Parallel()
		s := unitScheduler(&Scheduler{
			settings: &stubSettingsResolver{intErr: errors.New("db down")},
			logger:   slog.Default()})

		_, err := s.resolveRequiredPositiveIntSetting(context.Background(), "some.key", "NEVER_SET_REQUIRED_ERROR")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "db down")
	})

	t.Run("zero rejected", func(t *testing.T) {
		t.Parallel()
		s := unitScheduler(&Scheduler{
			settings: &stubSettingsResolver{intVal: 0},
			logger:   slog.Default()})

		_, err := s.resolveRequiredPositiveIntSetting(context.Background(), "some.key", "NEVER_SET_REQUIRED_ZERO")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be positive")
	})
}

func TestResolveNonNegativeIntSetting_OverrideZeroHonored(t *testing.T) {
	t.Parallel()

	// Zero is a meaningful value for settings like
	// tracking.auto_checkout_grace_minutes (checkout exactly at shift end).
	s := unitScheduler(&Scheduler{
		settings: &stubSettingsResolver{hasOverride: true, intVal: 0},
		logger:   slog.Default()})

	val := s.resolveNonNegativeIntSetting(context.Background(), "some.key", "NEVER_SET_FFF", 15)
	assert.Equal(t, 0, val)
}

func TestResolveNonNegativeIntSetting_NegativeFallsThrough(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		settings: &stubSettingsResolver{hasOverride: true, intVal: -3},
		logger:   slog.Default()})

	val := s.resolveNonNegativeIntSetting(context.Background(), "some.key", "NEVER_SET_GGG", 15)
	assert.Equal(t, 15, val)
}

func TestResolveNonNegativeIntSetting_PositiveOverride(t *testing.T) {
	t.Parallel()

	s := unitScheduler(&Scheduler{
		settings: &stubSettingsResolver{hasOverride: true, intVal: 30},
		logger:   slog.Default()})

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
