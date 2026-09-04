package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type deviceRepoForSessionUnitTest struct {
	updateRoomIDFunc func(ctx context.Context, id int64, roomID int64) error
}

func (d *deviceRepoForSessionUnitTest) Create(context.Context, *iotModels.Device) error {
	return nil
}
func (d *deviceRepoForSessionUnitTest) FindByID(context.Context, interface{}) (*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) FindByIDForUpdate(context.Context, int64) (*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) Update(context.Context, *iotModels.Device) error {
	return nil
}
func (d *deviceRepoForSessionUnitTest) Delete(context.Context, interface{}) error {
	return nil
}
func (d *deviceRepoForSessionUnitTest) List(context.Context, map[string]interface{}) ([]*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) FindByDeviceID(context.Context, string) (*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) FindByAPIKey(context.Context, string) (*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) FindByType(context.Context, string) ([]*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) FindByStatus(context.Context, iotModels.DeviceStatus) ([]*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) FindByRegisteredBy(context.Context, int64) ([]*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) UpdateLastSeen(context.Context, int64, time.Time) error {
	return nil
}
func (d *deviceRepoForSessionUnitTest) UpdateRoomID(ctx context.Context, id int64, roomID int64) error {
	if d.updateRoomIDFunc != nil {
		return d.updateRoomIDFunc(ctx, id, roomID)
	}
	return nil
}
func (d *deviceRepoForSessionUnitTest) UpdateStatus(context.Context, string, iotModels.DeviceStatus) error {
	return nil
}
func (d *deviceRepoForSessionUnitTest) FindOfflineDevices(context.Context, time.Duration) ([]*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) CountDevicesByType(context.Context) (map[string]int, error) {
	return nil, nil
}

type settingsResolverForSessionUnitTest struct {
	has    bool
	hasErr error
	intVal int
	intErr error
}

func (s *settingsResolverForSessionUnitTest) HasTenantOverride(context.Context, string) (bool, error) {
	return s.has, s.hasErr
}
func (s *settingsResolverForSessionUnitTest) ResolveString(context.Context, string) (string, error) {
	return "", nil
}
func (s *settingsResolverForSessionUnitTest) ResolveInt(context.Context, string) (int, error) {
	return s.intVal, s.intErr
}

type workSessionServiceForSessionUnitTest struct {
	ensureCheckedInFunc func(ctx context.Context, staffID int64, source string) (*activeModels.WorkSession, error)
}

func (w *workSessionServiceForSessionUnitTest) CheckIn(context.Context, int64, string, string, string) (*activeModels.WorkSession, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) CheckInOn(context.Context, int64, timezone.Date, string, string, string) (*activeModels.WorkSession, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) CheckOut(context.Context, int64, string) (*activeModels.WorkSession, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) StartBreak(context.Context, int64, *int) (*activeModels.WorkSessionBreak, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) EndBreak(context.Context, int64) (*activeModels.WorkSession, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) CheckOutOn(context.Context, int64, timezone.Date, string) (*activeModels.WorkSession, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) StartBreakOn(context.Context, int64, timezone.Date, *int) (*activeModels.WorkSessionBreak, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) EndBreakOn(context.Context, int64, timezone.Date) (*activeModels.WorkSession, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) GetSessionBreaks(context.Context, int64, int64) ([]*activeModels.WorkSessionBreak, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) UpdateSession(context.Context, int64, int64, SessionUpdateRequest) (*activeModels.WorkSession, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) UpdateSessionAsAdmin(context.Context, int64, int64, int64, SessionUpdateRequest) (*activeModels.WorkSession, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) CreateSessionAsAdmin(context.Context, int64, int64, AdminCreateSessionRequest) (*activeModels.WorkSession, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) GetCurrentSession(context.Context, int64) (*activeModels.WorkSession, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) GetLatestOpenSession(context.Context, int64) (*activeModels.WorkSession, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) GetHistory(context.Context, int64, timezone.Date, timezone.Date) (*HistoryResponse, error) {
	return nil, nil
}

func (w *workSessionServiceForSessionUnitTest) GetHistoryIntersecting(context.Context, int64, timezone.Date, timezone.Date) (*HistoryResponse, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) GetSessionEdits(context.Context, int64, int64) ([]*WorkSessionEditView, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) GetSessionEditsForStaff(context.Context, int64, int64) ([]*WorkSessionEditView, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) GetTodayPresenceMap(context.Context) (map[int64]string, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) CleanupOpenSessions(context.Context) (int, error) {
	return 0, nil
}
func (w *workSessionServiceForSessionUnitTest) AutoCheckoutDueSessions(context.Context, time.Duration) (int, error) {
	return 0, nil
}
func (w *workSessionServiceForSessionUnitTest) SetStaffShiftRepo(scheduleModels.StaffShiftRepository) {
}
func (w *workSessionServiceForSessionUnitTest) EnsureCheckedIn(ctx context.Context, staffID int64, source string) (*activeModels.WorkSession, error) {
	if w.ensureCheckedInFunc != nil {
		return w.ensureCheckedInFunc(ctx, staffID, source)
	}
	return &activeModels.WorkSession{}, nil
}
func (w *workSessionServiceForSessionUnitTest) ExportSessions(context.Context, int64, timezone.Date, timezone.Date, string) (*ExportFile, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) DayExportRows(context.Context, int64, timezone.Date, timezone.Date) ([]DayExportRow, error) {
	return nil, nil
}

func (w *workSessionServiceForSessionUnitTest) DayExportRowsByStaffIDs(context.Context, []int64, timezone.Date, timezone.Date) (map[int64][]DayExportRow, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) AutoEndExpiredBreaks(context.Context) (int, error) {
	return 0, nil
}
func (w *workSessionServiceForSessionUnitTest) GetStaffIDsWithSupervisionToday(context.Context) ([]int64, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) GetWorkTimeModelByID(context.Context, int64) (*configModels.WorkTimeModel, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) GetCurrentScheduleRows(context.Context, int64) ([]*configModels.StaffWorkSchedule, error) {
	return nil, nil
}
func (w *workSessionServiceForSessionUnitTest) AssignScheduleTemplate(context.Context, *userModels.Staff, int64) error {
	return nil
}
func (w *workSessionServiceForSessionUnitTest) ApplyCustomScheduleRows(context.Context, *userModels.Staff, []*configModels.StaffWorkSchedule, timezone.Date) error {
	return nil
}
func (w *workSessionServiceForSessionUnitTest) SaveCustomScheduleAsTemplate(context.Context, *userModels.Staff, string, int, timezone.Date, []*configModels.WorkTimeModelEntry) error {
	return nil
}

func (w *workSessionServiceForSessionUnitTest) UpdateSchedule(context.Context, *userModels.Staff, ScheduleUpdateInput) error {
	return nil
}

func newSessionSQLMockDB(t *testing.T) (*bun.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, db.Close())
	})
	return db, mock
}

type sessionTestSavepoints struct{}

type sessionStartLockerStub struct {
	lock func(context.Context, int64, int64) error
}

func (s sessionStartLockerStub) LockSessionStart(ctx context.Context, tenantID, activityID int64) error {
	return s.lock(ctx, tenantID, activityID)
}

func TestGetActivityEndNameKeepsTransactionUsableAfterDatabaseFailure(t *testing.T) {
	t.Parallel()

	svc := &service{ServiceDependencies: ServiceDependencies{Logger: slog.New(slog.DiscardHandler)}}
	var lookupErr error
	err := tenant.WithinCurrentTenant(testpkg.Ctx(t), func(txCtx context.Context) error {
		name, err := svc.getActivityEndName(txCtx, func(lookupCtx context.Context) (string, error) {
			rawTx, ok := tenant.TransactionFromContext(lookupCtx)
			require.True(t, ok)
			tx, ok := rawTx.(bun.Tx)
			require.True(t, ok)
			_, lookupErr = tx.ExecContext(lookupCtx, "SELECT 1 / 0")
			return "", lookupErr
		}, "SSE room name lookup failed", slog.Int64("room_id", 1))
		require.NoError(t, err)
		assert.Empty(t, name)

		rawTx, ok := tenant.TransactionFromContext(txCtx)
		require.True(t, ok)
		tx, ok := rawTx.(bun.Tx)
		require.True(t, ok)
		_, err = tx.ExecContext(txCtx, "SELECT 1")
		return err
	})
	require.NoError(t, err)
	require.Error(t, lookupErr)
	var pgErr interface{ Field(byte) string }
	require.True(t, errors.As(lookupErr, &pgErr))
	assert.Equal(t, "22012", pgErr.Field('C'))
}

func TestAcquireActivitySessionLock_UsesRepository(t *testing.T) {
	t.Parallel()
	ctx := tenant.WithTenantID(context.Background(), 17)
	called := false
	svc := &service{ServiceDependencies: ServiceDependencies{SessionStartLock: sessionStartLockerStub{
		lock: func(_ context.Context, tenantID, activityID int64) error {
			called = true
			assert.Equal(t, int64(17), tenantID)
			assert.Equal(t, int64(23), activityID)
			return nil
		},
	}}}

	require.NoError(t, svc.acquireActivitySessionLock(ctx, 23, "start"))
	assert.True(t, called)
}

func TestAcquireActivitySessionLock_PropagatesRepositoryFailure(t *testing.T) {
	t.Parallel()
	expected := errors.New("lock failed")
	svc := &service{ServiceDependencies: ServiceDependencies{SessionStartLock: sessionStartLockerStub{
		lock: func(context.Context, int64, int64) error { return expected },
	}}}

	err := svc.acquireActivitySessionLock(tenant.WithTenantID(context.Background(), 17), 23, "start")
	require.ErrorIs(t, err, expected)
}

func (sessionTestSavepoints) exec(ctx context.Context, statement string) error {
	raw, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return tenant.ErrRuntimeRequired
	}
	var tx bun.Tx
	switch value := raw.(type) {
	case bun.Tx:
		tx = value
	case *bun.Tx:
		if value == nil {
			return tenant.ErrRuntimeRequired
		}
		tx = *value
	default:
		return fmt.Errorf("unsupported transaction type %T", raw)
	}
	_, err := tx.ExecContext(ctx, statement)
	return err
}

func (s sessionTestSavepoints) CreateSavepoint(ctx context.Context) error {
	return s.exec(ctx, "SAVEPOINT phoenix_operation")
}
func (s sessionTestSavepoints) RollbackSavepoint(ctx context.Context) error {
	return s.exec(ctx, "ROLLBACK TO SAVEPOINT phoenix_operation")
}
func (s sessionTestSavepoints) ReleaseSavepoint(ctx context.Context) error {
	return s.exec(ctx, "RELEASE SAVEPOINT phoenix_operation")
}

func withSessionTestRuntime(t *testing.T, ctx context.Context, db *bun.DB) context.Context {
	t.Helper()
	tenantID := testpkg.Tenant(t)
	within := func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error { return fn(ctx, tx) })
	}
	admin := func(ctx context.Context, fn func(context.Context, any) error) error {
		return within(ctx, tenantID, fn)
	}
	runtime, err := tenant.NewUnitOfWork(within, admin, tenant.SavepointFunc(sessionTestSavepoints{}), func(error) bool { return false })
	require.NoError(t, err)
	return tenant.WithTenantID(tenant.WithUnitOfWork(ctx, runtime), tenantID)
}

type timetableBridgeCompleterForSessionUnitTest struct {
	completeFunc func(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error)
}

func (t *timetableBridgeCompleterForSessionUnitTest) CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error) {
	if t.completeFunc != nil {
		return t.completeFunc(ctx, activeGroupIDs, completedAt)
	}
	return 0, nil
}

func TestProcessSessionTimeoutByID_ContinuesWhenSSECollectionFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	activityID := int64(200)
	findCalls := 0
	endedVisits := 0
	visitEnded := false
	entryTime := time.Now().Add(-time.Hour)

	svc := &service{ServiceDependencies: ServiceDependencies{Logger: slog.Default(), GroupRepo: &mockGroupRepository{
		findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
			return &activeModels.Group{Model: modelBase.Model{ID: 100}, GroupID: &activityID}, nil
		},
	}, VisitRepo: &mockVisitRepository{
		findByActiveGroupIDFunc: func(context.Context, int64) ([]*activeModels.Visit, error) {
			findCalls++
			if findCalls == 1 {
				return nil, errors.New("student lookup prefetch failed")
			}
			return []*activeModels.Visit{
				{Model: modelBase.Model{ID: 300}, ActiveGroupID: 100, StudentID: 400, EntryTime: entryTime},
			}, nil
		},
		findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
			visit := &activeModels.Visit{
				Model: modelBase.Model{ID: 300}, ActiveGroupID: 100, StudentID: 400, EntryTime: entryTime,
			}
			if visitEnded {
				exitTime := time.Now()
				visit.ExitTime = &exitTime
			}
			return visit, nil
		},
		endVisitFunc: func(context.Context, int64) error {
			endedVisits++
			visitEnded = true
			return nil
		},
	}, SupervisorRepo: &mockGroupSupervisorRepository{}},
	}

	result, err := svc.ProcessSessionTimeoutByID(ctx, 100)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, activityID, *result.ActivityID)
	assert.Equal(t, 1, result.StudentsCheckedOut)
	assert.Equal(t, 1, endedVisits)
}

func TestProcessSessionTimeoutByID_ReturnsCheckoutAndEndErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	activeGroup := &activeModels.Group{Model: modelBase.Model{ID: 100}}

	t.Run("checkout lookup failure", func(t *testing.T) {
		findCalls := 0
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				return activeGroup, nil
			},
		}, VisitRepo: &mockVisitRepository{
			findByActiveGroupIDFunc: func(context.Context, int64) ([]*activeModels.Visit, error) {
				findCalls++
				if findCalls == 1 {
					return []*activeModels.Visit{}, nil
				}
				return nil, errors.New("visit checkout lookup failed")
			},
		}},
		}

		result, err := svc.ProcessSessionTimeoutByID(ctx, 100)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "visit checkout lookup failed")
	})

	t.Run("session end failure", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				return activeGroup, nil
			},
			endSessionFunc: func(context.Context, int64) error {
				return errors.New("session end failed")
			},
		}, VisitRepo: &mockVisitRepository{}, SupervisorRepo: &mockGroupSupervisorRepository{}},
		}

		result, err := svc.ProcessSessionTimeoutByID(ctx, 100)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "session end failed")
	})
}

// A timed-out session is a session end like any other: the mirrored timetable
// instance has to be completed through the bridge, or it stays active forever
// with its expected rows unfinalized (#1747 review). Both timeout entry points
// (the kiosk /timeout endpoint and the abandoned-session sweep) come through
// ProcessSessionTimeoutByID, so one guard covers both.
func TestProcessSessionTimeoutByID_CompletesTimetableMirrorBeforeEndingSession(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	activeGroup := &activeModels.Group{Model: modelBase.Model{ID: 100}}

	newService := func(order *[]string, bridgeErr error) *service {
		return &service{ServiceDependencies: ServiceDependencies{
			Logger: slog.Default(),
			GroupRepo: &mockGroupRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
					return activeGroup, nil
				},
				endSessionFunc: func(context.Context, int64) error {
					*order = append(*order, "session")
					return nil
				},
			},
			VisitRepo:      &mockVisitRepository{},
			SupervisorRepo: &mockGroupSupervisorRepository{},
			TimetableBridgeCompleter: &timetableBridgeCompleterForSessionUnitTest{
				completeFunc: func(_ context.Context, activeGroupIDs []int64, _ time.Time) (int64, error) {
					assert.Equal(t, []int64{100}, activeGroupIDs)
					*order = append(*order, "timetable")
					return 1, bridgeErr
				},
			},
		}}
	}

	t.Run("closes the timetable first", func(t *testing.T) {
		var order []string

		result, err := newService(&order, nil).ProcessSessionTimeoutByID(ctx, 100)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, []string{"timetable", "session"}, order)
	})

	t.Run("a failing bridge leaves the session open", func(t *testing.T) {
		var order []string

		result, err := newService(&order, errors.New("bridge down")).ProcessSessionTimeoutByID(ctx, 100)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "bridge down")
		assert.Equal(t, []string{"timetable"}, order,
			"the session must not be ended while its timetable side is unfinished")
	})
}

// The abandoned-session sweep calls the timeout path straight from the
// scheduler, with no request middleware to open a transaction for it. Without
// one of its own the bridge commits a completed timetable instance and a later
// failure leaves the session open beside it — a split the next sweep cannot
// repair, because it only ever sees the still-active session (#1747 review).
func TestProcessSessionTimeoutByID_IsAtomic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	activeGroup := &activeModels.Group{Model: modelBase.Model{ID: 100}}

	newService := func(db *bun.DB, endSessionErr error) *service {
		return &service{ServiceDependencies: ServiceDependencies{
			Logger: slog.Default(),
			DB:     db,
			GroupRepo: &mockGroupRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
					return activeGroup, nil
				},
				endSessionFunc: func(context.Context, int64) error { return endSessionErr },
			},
			VisitRepo:      &mockVisitRepository{},
			SupervisorRepo: &mockGroupSupervisorRepository{},
			TimetableBridgeCompleter: &timetableBridgeCompleterForSessionUnitTest{
				completeFunc: func(ctx context.Context, _ []int64, _ time.Time) (int64, error) {
					_, inTx := tenant.TransactionFromContext(ctx)
					assert.True(t, inTx, "the bridge must write inside the timeout transaction")
					return 1, nil
				},
			},
		}}
	}

	t.Run("commits both halves together", func(t *testing.T) {
		db, mock := newSessionSQLMockDB(t)
		txCtx := withSessionTestRuntime(t, ctx, db)
		mock.ExpectBegin()
		mock.ExpectCommit()

		result, err := newService(db, nil).ProcessSessionTimeoutByID(txCtx, 100)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("a failing session end takes the bridge write with it", func(t *testing.T) {
		db, mock := newSessionSQLMockDB(t)
		txCtx := withSessionTestRuntime(t, ctx, db)
		mock.ExpectBegin()
		mock.ExpectRollback()

		result, err := newService(db, errors.New("session end failed")).ProcessSessionTimeoutByID(txCtx, 100)

		require.Error(t, err)
		assert.Nil(t, result)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

// Ending a group by hand is a session end like any other: POST
// /active/groups/{id}/end has to close the timetable side too, or the linked
// instance stays active with its expected rows unfinalized — and the nightly
// bridge never repairs it, because it only looks at active.groups that are
// still running (#1747 review).
// newEndGroupService builds the service EndActiveGroupSession is exercised
// against. sessionEndErr fails GroupRepo.EndSession, i.e. the session end AFTER
// the bridge has already completed the mirrored instance.
func newEndGroupService(
	t *testing.T,
	order *[]string,
	group *activeModels.Group,
	bridgeErr error,
	sessionEndErr error,
) *service {
	t.Helper()

	return &service{ServiceDependencies: ServiceDependencies{
		Logger: slog.Default(),
		GroupRepo: &mockGroupRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				return group, nil
			},
			endSessionFunc: func(context.Context, int64) error {
				*order = append(*order, "session")
				return sessionEndErr
			},
		},
		VisitRepo:      &mockVisitRepository{},
		SupervisorRepo: &mockGroupSupervisorRepository{},
		TimetableBridgeCompleter: &timetableBridgeCompleterForSessionUnitTest{
			completeFunc: func(_ context.Context, activeGroupIDs []int64, _ time.Time) (int64, error) {
				assert.Equal(t, []int64{100}, activeGroupIDs)
				*order = append(*order, "timetable")
				return 1, bridgeErr
			},
		},
	}}
}

func TestEndActiveGroupSession_CompletesTimetableMirrorBeforeEndingSession(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	newService := func(order *[]string, group *activeModels.Group, bridgeErr error) *service {
		return newEndGroupService(t, order, group, bridgeErr, nil)
	}

	t.Run("closes the timetable first", func(t *testing.T) {
		var order []string

		err := newService(&order, &activeModels.Group{Model: modelBase.Model{ID: 100}}, nil).
			EndActiveGroupSession(ctx, 100)

		require.NoError(t, err)
		assert.Equal(t, []string{"timetable", "session"}, order)
	})

	t.Run("a failing bridge leaves the session open", func(t *testing.T) {
		var order []string

		err := newService(&order, &activeModels.Group{Model: modelBase.Model{ID: 100}}, errors.New("bridge down")).
			EndActiveGroupSession(ctx, 100)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "bridge down")
		assert.Equal(t, []string{"timetable"}, order,
			"the session must not be ended while its timetable side is unfinished")
	})

	// The handler renders "already ended" as 4xx and the tenant middleware only
	// rolls back on its own for 5xx, so a bridge write here would be committed
	// beside the rejection.
	t.Run("an already ended group is rejected before the bridge writes", func(t *testing.T) {
		var order []string
		endTime := time.Now()
		ended := &activeModels.Group{Model: modelBase.Model{ID: 100}, EndTime: &endTime}

		err := newService(&order, ended, nil).EndActiveGroupSession(ctx, 100)

		require.ErrorIs(t, err, ErrActiveGroupAlreadyEnded)
		assert.Empty(t, order)
	})

	// The bridge has already completed the mirrored instance at this point. The
	// handler maps most session-end errors to 4xx and the tenant middleware only
	// rolls back on its own for 5xx, so the service has to ask for the rollback
	// itself — otherwise a completed instance commits beside an open group.
	t.Run("a failing session end asks for a rollback", func(t *testing.T) {
		var order []string
		rollbackCtx := tenant.WithRollbackMarker(ctx)

		err := newEndGroupService(t, &order, &activeModels.Group{Model: modelBase.Model{ID: 100}}, nil,
			errors.New("session end refused")).EndActiveGroupSession(rollbackCtx, 100)

		require.Error(t, err)
		assert.Equal(t, []string{"timetable", "session"}, order)
		assert.True(t, tenant.RollbackRequested(rollbackCtx),
			"the completed timetable instance must not commit beside a group that is still open")
	})
}

func TestUpdateSessionActivity_RepositoryMissesAreMapped(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	missErr := &modelBase.DatabaseError{Op: "update last activity - session not found", Err: sql.ErrNoRows}

	tests := []struct {
		name      string
		findFunc  func(context.Context, interface{}) (*activeModels.Group, error)
		wantError string
	}{
		{
			name: "find by id no rows becomes not found",
			findFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				return nil, &modelBase.DatabaseError{Op: "find by id", Err: sql.ErrNoRows}
			},
			wantError: ErrActiveGroupNotFound.Error(),
		},
		{
			name: "nil session becomes not found",
			findFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				return nil, nil
			},
			wantError: ErrActiveGroupNotFound.Error(),
		},
		{
			name: "ended session becomes already ended",
			findFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				endedAt := time.Now()
				return &activeModels.Group{Model: modelBase.Model{ID: 100}, EndTime: &endedAt}, nil
			},
			wantError: ErrActiveGroupAlreadyEnded.Error(),
		},
		{
			name: "unexpected lookup error is preserved",
			findFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				return nil, errors.New("lookup exploded")
			},
			wantError: "lookup exploded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
				updateLastActivityFunc: func(context.Context, int64, time.Time) error {
					return missErr
				},
				findByIDFunc: tt.findFunc,
			}},
			}

			err := svc.UpdateSessionActivity(ctx, 100)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestUpdateSessionActivity_NonMissUpdateErrorIsPreserved(t *testing.T) {
	t.Parallel()

	svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
		updateLastActivityFunc: func(context.Context, int64, time.Time) error {
			return errors.New("deadlock")
		},
	}},
	}

	err := svc.UpdateSessionActivity(context.Background(), 100)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadlock")
}

func TestSessionDeviceOnlineWindowResolution(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("default without settings", func(t *testing.T) {
		svc := &service{}
		assert.Equal(t, defaultDeviceOnlineWindow, svc.deviceOnlineWindow(ctx))
	})

	t.Run("default when override check fails", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{Logger: slog.Default()}, settings: &settingsResolverForSessionUnitTest{hasErr: errors.New("settings db down")}}
		assert.Equal(t, defaultDeviceOnlineWindow, svc.deviceOnlineWindow(ctx))
	})

	t.Run("default without tenant override", func(t *testing.T) {
		svc := &service{settings: &settingsResolverForSessionUnitTest{has: false, intVal: 2}}
		assert.Equal(t, defaultDeviceOnlineWindow, svc.deviceOnlineWindow(ctx))
	})

	t.Run("default for invalid override", func(t *testing.T) {
		svc := &service{settings: &settingsResolverForSessionUnitTest{has: true, intVal: 0}}
		assert.Equal(t, defaultDeviceOnlineWindow, svc.deviceOnlineWindow(ctx))
	})

	t.Run("default when resolve fails", func(t *testing.T) {
		svc := &service{settings: &settingsResolverForSessionUnitTest{has: true, intErr: errors.New("missing")}}
		assert.Equal(t, defaultDeviceOnlineWindow, svc.deviceOnlineWindow(ctx))
	})

	t.Run("valid tenant override", func(t *testing.T) {
		svc := &service{settings: &settingsResolverForSessionUnitTest{has: true, intVal: 2}}
		assert.Equal(t, 2*time.Minute, svc.deviceOnlineWindow(ctx))
	})
}

func TestSessionIsDeviceOnline(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-time.Minute)
	old := now.Add(-10 * time.Minute)
	svc := &service{settings: &settingsResolverForSessionUnitTest{has: true, intVal: 5}}

	assert.False(t, svc.isDeviceOnline(context.Background(), nil, now))
	assert.False(t, svc.isDeviceOnline(context.Background(), &iotModels.Device{}, now))
	assert.True(t, svc.isDeviceOnline(context.Background(), &iotModels.Device{LastSeen: &recent}, now))
	assert.False(t, svc.isDeviceOnline(context.Background(), &iotModels.Device{LastSeen: &old}, now))
}

func TestUpdateDeviceLocationBestEffort(t *testing.T) {
	t.Parallel()

	calls := 0
	svc := &service{ServiceDependencies: ServiceDependencies{Logger: slog.Default(), DeviceRepo: &deviceRepoForSessionUnitTest{
		updateRoomIDFunc: func(context.Context, int64, int64) error {
			calls++
			return errors.New("device table temporarily unavailable")
		},
	}},
	}

	svc.updateDeviceLocation(context.Background(), 50, 60)

	assert.Equal(t, 1, calls)
}

func TestAssignMultipleSupervisorsNonCritical_WorkSessionBestEffortBranches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	checkInResults := map[int64]error{
		10: errors.New("auto check-in failed"),
		20: &PlannedStartNotReachedError{
			PlannedStartTime: "09:00",
			CurrentTime:      "08:45",
		},
		30: nil,
	}
	checked := map[int64]bool{}
	created := map[int64]bool{}
	svc := &service{ServiceDependencies: ServiceDependencies{Logger: slog.Default(), SupervisorRepo: &mockGroupSupervisorRepository{
		createFunc: func(_ context.Context, supervisor *activeModels.GroupSupervisor) error {
			created[supervisor.StaffID] = true
			return nil
		},
	}, WorkSessionService: &workSessionServiceForSessionUnitTest{
		ensureCheckedInFunc: func(_ context.Context, staffID int64, source string) (*activeModels.WorkSession, error) {
			checked[staffID] = true
			assert.Equal(t, activeModels.WorkSessionSourceNFC, source)
			if err := checkInResults[staffID]; err != nil {
				return nil, err
			}
			return nil, nil
		},
	}},
	}

	svc.assignMultipleSupervisorsNonCritical(ctx, 99, []int64{10, 20, 10, 30}, time.Now())

	assert.Equal(t, map[int64]bool{10: true, 20: true, 30: true}, created)
	assert.Equal(t, map[int64]bool{10: true, 20: true, 30: true}, checked)
}

func TestRunBestEffortDB_SavepointBranches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("savepoint create failure skips operation", func(t *testing.T) {
		db, mock := newSessionSQLMockDB(t)
		mock.ExpectBegin()
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		txCtx := tenant.WithTransactionForTest(withSessionTestRuntime(t, ctx, db), &tx)
		mock.ExpectExec("SAVEPOINT phoenix_operation").WillReturnError(errors.New("savepoint failed"))
		mock.ExpectRollback()

		called := false
		svc := &service{ServiceDependencies: ServiceDependencies{Logger: slog.Default()}}
		svc.runBestEffortDB(txCtx, "assign_supervisor", func() error {
			called = true
			return nil
		}, func(error) {
			t.Fatal("operation failure logger must not run when savepoint creation fails")
		})

		assert.False(t, called)
		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("operation failure rolls back to savepoint", func(t *testing.T) {
		db, mock := newSessionSQLMockDB(t)
		mock.ExpectBegin()
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		txCtx := tenant.WithTransactionForTest(withSessionTestRuntime(t, ctx, db), &tx)
		mock.ExpectExec("SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("ROLLBACK TO SAVEPOINT phoenix_operation").WillReturnError(errors.New("rollback failed"))
		mock.ExpectRollback()

		logged := false
		svc := &service{ServiceDependencies: ServiceDependencies{Logger: slog.Default()}}
		svc.runBestEffortDB(txCtx, "nfc_auto_checkin", func() error {
			return errors.New("operation failed")
		}, func(error) {
			logged = true
		})

		assert.True(t, logged)
		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("release failure is logged after successful operation", func(t *testing.T) {
		db, mock := newSessionSQLMockDB(t)
		mock.ExpectBegin()
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		txCtx := tenant.WithTransactionForTest(withSessionTestRuntime(t, ctx, db), &tx)
		mock.ExpectExec("SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("RELEASE SAVEPOINT phoenix_operation").WillReturnError(errors.New("release failed"))
		mock.ExpectRollback()

		called := false
		svc := &service{ServiceDependencies: ServiceDependencies{Logger: slog.Default()}}
		svc.runBestEffortDB(txCtx, "update_device_location", func() error {
			called = true
			return nil
		}, func(error) {
			t.Fatal("operation failure logger must not run on release failure")
		})

		assert.True(t, called)
		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestCreateSessionBase_Branches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("create failure is returned before side effects", func(t *testing.T) {
		expectedErr := errors.New("group insert failed")
		transferCalls := 0
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
			createFunc: func(context.Context, *activeModels.Group) error {
				return expectedErr
			},
		}, VisitRepo: &mockVisitRepository{
			transferVisitsFromRecentSessionsFunc: func(context.Context, int64, int64) (int, error) {
				transferCalls++
				return 0, nil
			},
		}},
		}

		group, transferred, err := svc.createSessionBase(ctx, 10, 20, 30)

		require.ErrorIs(t, err, expectedErr)
		assert.Nil(t, group)
		assert.Zero(t, transferred)
		assert.Zero(t, transferCalls)
	})

	t.Run("transfer failure is returned after group creation", func(t *testing.T) {
		expectedErr := errors.New("visit transfer failed")
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
			createFunc: func(_ context.Context, group *activeModels.Group) error {
				group.ID = 44
				return nil
			},
		}, VisitRepo: &mockVisitRepository{
			transferVisitsFromRecentSessionsFunc: func(_ context.Context, newActiveGroupID, deviceID int64) (int, error) {
				assert.Equal(t, int64(44), newActiveGroupID)
				assert.Equal(t, int64(20), deviceID)
				return 0, expectedErr
			},
		}, DeviceRepo: &deviceRepoForSessionUnitTest{}},
		}

		group, transferred, err := svc.createSessionBase(ctx, 10, 20, 30)

		require.ErrorIs(t, err, expectedErr)
		assert.Nil(t, group)
		assert.Zero(t, transferred)
	})

	t.Run("success without device skips location update", func(t *testing.T) {
		locationUpdates := 0
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
			createFunc: func(_ context.Context, group *activeModels.Group) error {
				group.ID = 45
				return nil
			},
		}, VisitRepo: &mockVisitRepository{
			transferVisitsFromRecentSessionsFunc: func(_ context.Context, newActiveGroupID, deviceID int64) (int, error) {
				assert.Equal(t, int64(45), newActiveGroupID)
				assert.Zero(t, deviceID)
				return 2, nil
			},
		}, DeviceRepo: &deviceRepoForSessionUnitTest{
			updateRoomIDFunc: func(context.Context, int64, int64) error {
				locationUpdates++
				return nil
			},
		}},
		}

		group, transferred, err := svc.createSessionBase(ctx, 10, 0, 30)

		require.NoError(t, err)
		require.NotNil(t, group)
		assert.Equal(t, int64(45), group.ID)
		assert.Equal(t, 2, transferred)
		assert.Zero(t, locationUpdates)
	})
}

func TestMarkRollbackOnRoomCapacity(t *testing.T) {
	t.Parallel()

	ctx := tenant.WithRollbackMarker(context.Background())
	capacityErr := &RoomCapacityError{RoomID: 30, RoomName: "Mensa", CurrentOccupancy: 43, MaxCapacity: 43}

	returned := markRollbackOnRoomCapacity(ctx, capacityErr)

	require.ErrorIs(t, returned, ErrRoomCapacityExceeded)
	assert.True(t, tenant.RollbackRequested(ctx))

	otherCtx := tenant.WithRollbackMarker(context.Background())
	otherErr := errors.New("other failure")
	assert.ErrorIs(t, markRollbackOnRoomCapacity(otherCtx, otherErr), otherErr)
	assert.False(t, tenant.RollbackRequested(otherCtx))
}

func TestCreateSessionWithMultipleSupervisors_TransferredVisitsBranch(t *testing.T) {
	t.Parallel()

	var assigned []int64
	svc := &service{ServiceDependencies: ServiceDependencies{Logger: slog.Default(), GroupRepo: &mockGroupRepository{
		createFunc: func(_ context.Context, group *activeModels.Group) error {
			group.ID = 60
			return nil
		},
	}, VisitRepo: &mockVisitRepository{
		transferVisitsFromRecentSessionsFunc: func(context.Context, int64, int64) (int, error) {
			return 2, nil
		},
	}, SupervisorRepo: &mockGroupSupervisorRepository{
		createFunc: func(_ context.Context, supervisor *activeModels.GroupSupervisor) error {
			assigned = append(assigned, supervisor.StaffID)
			return nil
		},
	}, DeviceRepo: &deviceRepoForSessionUnitTest{}},
	}

	group, err := svc.createSessionWithMultipleSupervisors(context.Background(), 11, 22, []int64{33, 44, 33}, 55)

	require.NoError(t, err)
	require.NotNil(t, group)
	assert.ElementsMatch(t, []int64{33, 44}, assigned)
}

func TestManualRoomSelectionStrategies(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("ignore strategy skips conflict check", func(t *testing.T) {
		svc := &service{}
		roomID, err := svc.validateManualRoomSelection(ctx, 10, RoomConflictIgnore, true)
		require.NoError(t, err)
		assert.Equal(t, int64(10), roomID)
	})

	t.Run("repository error is returned", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
			checkRoomConflictFunc: func(context.Context, int64, int64) (bool, *activeModels.Group, error) {
				return false, nil, errors.New("conflict lookup failed")
			},
		}},
		}

		roomID, err := svc.validateManualRoomSelection(ctx, 10, RoomConflictFail, true)

		require.Error(t, err)
		assert.Zero(t, roomID)
		assert.Contains(t, err.Error(), "conflict lookup failed")
	})

	t.Run("warn strategy permits conflict", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{Logger: slog.Default(), GroupRepo: &mockGroupRepository{
			checkRoomConflictFunc: func(context.Context, int64, int64) (bool, *activeModels.Group, error) {
				return true, &activeModels.Group{Model: modelBase.Model{ID: 20}}, nil
			},
		}},
		}

		roomID, err := svc.validateManualRoomSelection(ctx, 10, RoomConflictWarn, true)

		require.NoError(t, err)
		assert.Equal(t, int64(10), roomID)
	})
}

// activityGroupRepoForRoomUnitTest overrides only FindByID; every other method
// panics through the embedded nil interface, which is what an unexpected call
// should do in a unit test.
type activityGroupRepoForRoomUnitTest struct {
	activitiesModels.GroupRepository
	group   *activitiesModels.Group
	findErr error
}

func (a *activityGroupRepoForRoomUnitTest) FindByID(context.Context, any) (*activitiesModels.Group, error) {
	return a.group, a.findErr
}

func TestDetermineRoomIDWithStrategy_NoSelectionAndNoPlannedRoom(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("planned room is used when configured", func(t *testing.T) {
		plannedRoom := int64(7)
		svc := &service{ServiceDependencies: ServiceDependencies{
			ActivityGroupRepo: &activityGroupRepoForRoomUnitTest{
				group: &activitiesModels.Group{PlannedRoomID: &plannedRoom},
			},
		}}

		roomID, err := svc.determineRoomIDWithStrategy(ctx, 1, nil, RoomConflictFail, true)

		require.NoError(t, err)
		assert.Equal(t, plannedRoom, roomID)
	})

	// No hardcoded fallback: room id 1 belongs to a single school, so a default
	// would trip fk_active_groups_room_tenant for every other tenant.
	t.Run("no room and no planned room fails", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{
			ActivityGroupRepo: &activityGroupRepoForRoomUnitTest{
				group: &activitiesModels.Group{},
			},
		}}

		roomID, err := svc.determineRoomIDWithStrategy(ctx, 1, nil, RoomConflictFail, true)

		require.ErrorIs(t, err, ErrNoRoomAvailable)
		assert.Zero(t, roomID)
	})

	t.Run("repository error is returned", func(t *testing.T) {
		lookupErr := errors.New("planned room lookup failed")
		svc := &service{ServiceDependencies: ServiceDependencies{
			ActivityGroupRepo: &activityGroupRepoForRoomUnitTest{findErr: lookupErr},
		}}

		roomID, err := svc.determineRoomIDWithStrategy(ctx, 1, nil, RoomConflictFail, true)

		require.ErrorIs(t, err, lookupErr)
		assert.Zero(t, roomID)
	})
}

func TestEndExistingActivitySessionsForForceStart_SkipsInvalidRowsAndStopsOnErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("skips nil and invalid sessions", func(t *testing.T) {
		var ended []int64
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
			findActiveByGroupIDFunc: func(context.Context, int64) ([]*activeModels.Group, error) {
				return []*activeModels.Group{
					nil,
					{Model: modelBase.Model{ID: 0}},
					{Model: modelBase.Model{ID: 30}},
				}, nil
			},
			endSessionFunc: func(_ context.Context, id int64) error {
				ended = append(ended, id)
				return nil
			},
		}},
		}

		endedIDs, err := svc.endExistingActivitySessionsForForceStart(ctx, 500)

		require.NoError(t, err)
		assert.Equal(t, []int64{30}, ended)
		assert.Equal(t, []int64{30}, endedIDs)
	})

	t.Run("find error is returned", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
			findActiveByGroupIDFunc: func(context.Context, int64) ([]*activeModels.Group, error) {
				return nil, errors.New("active sessions lookup failed")
			},
		}},
		}

		endedIDs, err := svc.endExistingActivitySessionsForForceStart(ctx, 500)

		require.Error(t, err)
		assert.Nil(t, endedIDs)
		assert.Contains(t, err.Error(), "active sessions lookup failed")
	})

	t.Run("end error stops immediately", func(t *testing.T) {
		expectedErr := errors.New("end active session failed")
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
			findActiveByGroupIDFunc: func(context.Context, int64) ([]*activeModels.Group, error) {
				return []*activeModels.Group{{Model: modelBase.Model{ID: 31}}}, nil
			},
			endSessionFunc: func(context.Context, int64) error {
				return expectedErr
			},
		}},
		}

		endedIDs, err := svc.endExistingActivitySessionsForForceStart(ctx, 500)

		require.ErrorIs(t, err, expectedErr)
		assert.Nil(t, endedIDs)
	})
}

func TestTransferForceStartedActivityState_PropagatesTransferErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	visitErr := errors.New("visit update failed")
	supervisorErr := errors.New("supervisor lookup failed")

	t.Run("visit transfer error", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{
			transferActiveVisitsBetweenGroupsFunc: func(context.Context, int64, int64) (int, error) {
				return 0, visitErr
			},
		}},
		}

		err := svc.transferForceStartedActivityState(ctx, []int64{1}, 2, time.Now())

		require.ErrorIs(t, err, visitErr)
	})

	t.Run("supervisor transfer error", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{}, SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*activeModels.GroupSupervisor, error) {
				return nil, supervisorErr
			},
		}},
		}

		err := svc.transferForceStartedActivityState(ctx, []int64{1}, 2, time.Now())

		require.ErrorIs(t, err, supervisorErr)
	})
}

func TestCompleteTimetableMirrorsForEndedSessions_PropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("bridge update failed")
	svc := &service{ServiceDependencies: ServiceDependencies{TimetableBridgeCompleter: &timetableBridgeCompleterForSessionUnitTest{
		completeFunc: func(_ context.Context, activeGroupIDs []int64, _ time.Time) (int64, error) {
			assert.Equal(t, []int64{10, 20}, activeGroupIDs)
			return 0, expectedErr
		},
	}},
	}

	err := svc.completeTimetableMirrorsForEndedSessions(context.Background(), []int64{10, 20})

	require.ErrorIs(t, err, expectedErr)
	assert.Contains(t, err.Error(), "complete timetable mirrors for ended sessions")
}

func TestTransferActiveSupervisorsBetweenGroups_ErrorBranches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	today := timezone.TodayDate()

	t.Run("new supervisor lookup error", func(t *testing.T) {
		call := 0
		expectedErr := errors.New("new supervisor lookup failed")
		svc := &service{ServiceDependencies: ServiceDependencies{SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*activeModels.GroupSupervisor, error) {
				call++
				if call == 2 {
					return nil, expectedErr
				}
				return []*activeModels.GroupSupervisor{}, nil
			},
		}},
		}

		count, err := svc.transferActiveSupervisorsBetweenGroups(ctx, 1, 2, time.Now())

		require.ErrorIs(t, err, expectedErr)
		assert.Zero(t, count)
	})

	t.Run("end supervision error returns partial count", func(t *testing.T) {
		expectedErr := errors.New("end supervision failed")
		svc := &service{ServiceDependencies: ServiceDependencies{SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(_ context.Context, activeGroupID int64, _ bool) ([]*activeModels.GroupSupervisor, error) {
				if activeGroupID == 1 {
					return []*activeModels.GroupSupervisor{
						{Model: modelBase.Model{ID: 10}, StaffID: 20, Role: "helper", StartDate: today},
					}, nil
				}
				return []*activeModels.GroupSupervisor{}, nil
			},
			endSupervisionFunc: func(context.Context, int64) error {
				return expectedErr
			},
		}},
		}

		count, err := svc.transferActiveSupervisorsBetweenGroups(ctx, 1, 2, time.Now())

		require.ErrorIs(t, err, expectedErr)
		assert.Zero(t, count)
	})

	t.Run("skips nil and duplicate supervisors", func(t *testing.T) {
		var ended []int64
		var created []*activeModels.GroupSupervisor
		svc := &service{ServiceDependencies: ServiceDependencies{SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(_ context.Context, activeGroupID int64, _ bool) ([]*activeModels.GroupSupervisor, error) {
				if activeGroupID == 1 {
					return []*activeModels.GroupSupervisor{
						nil,
						{Model: modelBase.Model{ID: 10}, StaffID: 20, Role: "Supervisor", StartDate: today},
						{Model: modelBase.Model{ID: 11}, StaffID: 30, Role: "helper", StartDate: today},
					}, nil
				}
				return []*activeModels.GroupSupervisor{
					nil,
					{Model: modelBase.Model{ID: 12}, StaffID: 20, Role: "supervisor", StartDate: today},
				}, nil
			},
			endSupervisionFunc: func(_ context.Context, id int64) error {
				ended = append(ended, id)
				return nil
			},
			createFunc: func(_ context.Context, supervisor *activeModels.GroupSupervisor) error {
				created = append(created, supervisor)
				return nil
			},
		}},
		}

		count, err := svc.transferActiveSupervisorsBetweenGroups(ctx, 1, 2, time.Now())

		require.NoError(t, err)
		assert.Equal(t, 1, count)
		assert.Equal(t, []int64{10, 11}, ended)
		require.Len(t, created, 1)
		assert.Equal(t, int64(30), created[0].StaffID)
		assert.Equal(t, "helper", created[0].Role)
	})

	t.Run("create error returns partial count", func(t *testing.T) {
		expectedErr := errors.New("create transferred supervisor failed")
		svc := &service{ServiceDependencies: ServiceDependencies{SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(_ context.Context, activeGroupID int64, _ bool) ([]*activeModels.GroupSupervisor, error) {
				if activeGroupID == 1 {
					return []*activeModels.GroupSupervisor{
						{Model: modelBase.Model{ID: 10}, StaffID: 20, Role: "Supervisor", StartDate: today},
					}, nil
				}
				return []*activeModels.GroupSupervisor{}, nil
			},
			createFunc: func(context.Context, *activeModels.GroupSupervisor) error {
				return expectedErr
			},
		}},
		}

		count, err := svc.transferActiveSupervisorsBetweenGroups(ctx, 1, 2, time.Now())

		require.ErrorIs(t, err, expectedErr)
		assert.Zero(t, count)
	})
}

func TestTransferActiveVisitsBetweenGroups_DelegatesToConditionalRepositoryTransfer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var gotOldGroupID, gotNewGroupID int64
	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{
		transferActiveVisitsBetweenGroupsFunc: func(_ context.Context, oldGroupID, newGroupID int64) (int, error) {
			gotOldGroupID = oldGroupID
			gotNewGroupID = newGroupID
			return 3, nil
		},
	}},
	}

	count, err := svc.transferActiveVisitsBetweenGroups(ctx, 100, 200)

	require.NoError(t, err)
	assert.Equal(t, 3, count)
	assert.Equal(t, int64(100), gotOldGroupID)
	assert.Equal(t, int64(200), gotNewGroupID)
}

func TestEndExistingDeviceSessionForForceStart_Branches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("find error is returned", func(t *testing.T) {
		expectedErr := errors.New("force device lookup failed")
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
			findActiveByDeviceIDFunc: func(context.Context, int64) (*activeModels.Group, error) {
				return nil, expectedErr
			},
		}},
		}

		endedID, err := svc.endExistingDeviceSessionForForceStart(ctx, 100)

		require.ErrorIs(t, err, expectedErr)
		assert.Zero(t, endedID)
	})

	t.Run("nil existing session is zero", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{}}}

		endedID, err := svc.endExistingDeviceSessionForForceStart(ctx, 100)

		require.NoError(t, err)
		assert.Zero(t, endedID)
	})

	t.Run("end error is returned", func(t *testing.T) {
		expectedErr := errors.New("force end failed")
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
			findActiveByDeviceIDFunc: func(context.Context, int64) (*activeModels.Group, error) {
				return &activeModels.Group{Model: modelBase.Model{ID: 302}}, nil
			},
			endSessionFunc: func(context.Context, int64) error {
				return expectedErr
			},
		}},
		}

		endedID, err := svc.endExistingDeviceSessionForForceStart(ctx, 100)

		require.ErrorIs(t, err, expectedErr)
		assert.Zero(t, endedID)
	})

	t.Run("returns ended session id", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
			findActiveByDeviceIDFunc: func(context.Context, int64) (*activeModels.Group, error) {
				return &activeModels.Group{Model: modelBase.Model{ID: 303}}, nil
			},
		}},
		}

		endedID, err := svc.endExistingDeviceSessionForForceStart(ctx, 100)

		require.NoError(t, err)
		assert.Equal(t, int64(303), endedID)
	})
}

func TestAppendActiveGroupID_DeduplicatesAndSkipsInvalid(t *testing.T) {
	t.Parallel()

	ids := appendActiveGroupID(nil, 0)
	ids = appendActiveGroupID(ids, -1)
	ids = appendActiveGroupID(ids, 10)
	ids = appendActiveGroupID(ids, 10)
	ids = appendActiveGroupIDs(ids, 11, 10, 12)

	assert.Equal(t, []int64{10, 11, 12}, ids)
}

func TestSupervisorReplacement_ErrorBranches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	startDate := timezone.TodayDate()

	t.Run("current supervisor lookup error", func(t *testing.T) {
		expectedErr := errors.New("current supervisors failed")
		svc := &service{ServiceDependencies: ServiceDependencies{SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*activeModels.GroupSupervisor, error) {
				return nil, expectedErr
			},
		}},
		}

		err := svc.replaceSupervisorsInTransaction(ctx, 100, map[int64]bool{10: true})

		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("ending current supervisor error", func(t *testing.T) {
		expectedErr := errors.New("end current supervisor failed")
		svc := &service{ServiceDependencies: ServiceDependencies{SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*activeModels.GroupSupervisor, error) {
				return []*activeModels.GroupSupervisor{
					{Model: modelBase.Model{ID: 20}, StaffID: 10, Role: "supervisor", StartDate: startDate},
				}, nil
			},
			updateFunc: func(context.Context, *activeModels.GroupSupervisor) error {
				return expectedErr
			},
		}},
		}

		err := svc.replaceSupervisorsInTransaction(ctx, 100, map[int64]bool{10: true})

		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("reactivate existing supervisor update error", func(t *testing.T) {
		expectedErr := errors.New("reactivate failed")
		endedDate := startDate.AddDays(-1)
		supervisors := []*activeModels.GroupSupervisor{
			{Model: modelBase.Model{ID: 20}, StaffID: 10, Role: "supervisor", StartDate: startDate, EndDate: &endedDate},
		}
		updateCalls := 0
		svc := &service{ServiceDependencies: ServiceDependencies{SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*activeModels.GroupSupervisor, error) {
				return supervisors, nil
			},
			updateFunc: func(context.Context, *activeModels.GroupSupervisor) error {
				updateCalls++
				if updateCalls == 2 {
					return expectedErr
				}
				return nil
			},
		}},
		}

		err := svc.replaceSupervisorsInTransaction(ctx, 100, map[int64]bool{10: true})

		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("create new supervisor error", func(t *testing.T) {
		expectedErr := errors.New("create supervisor failed")
		svc := &service{ServiceDependencies: ServiceDependencies{SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*activeModels.GroupSupervisor, error) {
				return []*activeModels.GroupSupervisor{}, nil
			},
			createFunc: func(context.Context, *activeModels.GroupSupervisor) error {
				return expectedErr
			},
		}},
		}

		err := svc.replaceSupervisorsInTransaction(ctx, 100, map[int64]bool{10: true})

		require.ErrorIs(t, err, expectedErr)
	})
}

func TestSupervisorReplacement_PreservesAdditionalSupervisors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	additional := &activeModels.GroupSupervisor{
		Model:     modelBase.Model{ID: 21},
		StaffID:   11,
		Role:      "additional_supervisor",
		StartDate: timezone.TodayDate(),
	}
	primary := &activeModels.GroupSupervisor{
		Model:     modelBase.Model{ID: 20},
		StaffID:   10,
		Role:      "supervisor",
		StartDate: timezone.TodayDate(),
	}

	var updatedIDs, createdStaffIDs []int64
	svc := &service{ServiceDependencies: ServiceDependencies{SupervisorRepo: &mockGroupSupervisorRepository{
		findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*activeModels.GroupSupervisor, error) {
			return []*activeModels.GroupSupervisor{primary, additional}, nil
		},
		updateFunc: func(_ context.Context, supervisor *activeModels.GroupSupervisor) error {
			updatedIDs = append(updatedIDs, supervisor.ID)
			return nil
		},
		createFunc: func(_ context.Context, supervisor *activeModels.GroupSupervisor) error {
			createdStaffIDs = append(createdStaffIDs, supervisor.StaffID)
			return nil
		},
	}}}

	err := svc.replaceSupervisorsInTransaction(ctx, 100, map[int64]bool{10: true, 11: true, 12: true})

	require.NoError(t, err)
	assert.Equal(t, []int64{20, 20}, updatedIDs)
	assert.Equal(t, []int64{12}, createdStaffIDs)
	assert.Nil(t, additional.EndDate)
}

func TestGetDeviceIDString(t *testing.T) {
	t.Parallel()

	deviceID := int64(42)

	assert.Equal(t, "unknown", getDeviceIDString(nil))
	assert.Equal(t, "42", getDeviceIDString(&deviceID))
}

func TestNormalizeTransferredSupervisorRole(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "supervisor", normalizeTransferredSupervisorRole("Supervisor"))
	assert.Equal(t, "helper", normalizeTransferredSupervisorRole("helper"))
}

func TestEndDailySessions_RepositoryFailures(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	activeGroup := &activeModels.Group{Model: modelBase.Model{ID: 100}}

	t.Run("list failure returns active error", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
			listFunc: func(context.Context, *modelBase.QueryOptions) ([]*activeModels.Group, error) {
				return nil, errors.New("list failed")
			},
		}},
		}

		result, err := svc.EndDailySessions(ctx)

		require.Error(t, err)
		require.NotNil(t, result)
		assert.False(t, result.Success)
	})

	t.Run("visit bulk failure aborts later bulk steps", func(t *testing.T) {
		db, mock := newSessionSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()
		svc := &service{ServiceDependencies: ServiceDependencies{DB: db, GroupRepo: &mockGroupRepository{
			listFunc: func(context.Context, *modelBase.QueryOptions) ([]*activeModels.Group, error) {
				return []*activeModels.Group{activeGroup}, nil
			},
		}, VisitRepo: &mockVisitRepository{
			endVisitsByActiveGroupIDsFunc: func(context.Context, []int64) (int64, error) {
				return 0, errors.New("bulk visit close failed")
			},
		}, SupervisorRepo: &mockGroupSupervisorRepository{}},
		}

		result, err := svc.EndDailySessions(withSessionTestRuntime(t, ctx, db))

		require.Error(t, err)
		require.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Contains(t, result.Errors[0], "bulk visit close failed")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("session bulk failure aborts supervisor cleanup", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
			listFunc: func(context.Context, *modelBase.QueryOptions) ([]*activeModels.Group, error) {
				return []*activeModels.Group{activeGroup}, nil
			},
			endSessionsByIDsFunc: func(context.Context, []int64) (int64, error) {
				return 0, errors.New("bulk session close failed")
			},
		}, VisitRepo: &mockVisitRepository{
			endVisitsByActiveGroupIDsFunc: func(context.Context, []int64) (int64, error) {
				return 2, nil
			},
		}, SupervisorRepo: &mockGroupSupervisorRepository{
			endSupervisionsByIDsFunc: func(context.Context, []int64) (int64, error) {
				return 3, nil
			},
		}},
		}

		result, err := svc.EndDailySessions(ctx)

		require.Error(t, err)
		assert.False(t, result.Success)
		assert.Equal(t, 2, result.VisitsEnded)
		assert.Zero(t, result.SupervisorsEnded)
		assert.Contains(t, result.Errors[0], "bulk session close failed")
	})

	t.Run("supervisor bulk failure records error", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
			listFunc: func(context.Context, *modelBase.QueryOptions) ([]*activeModels.Group, error) {
				return []*activeModels.Group{activeGroup}, nil
			},
			endSessionsByIDsFunc: func(context.Context, []int64) (int64, error) {
				return 1, nil
			},
		}, VisitRepo: &mockVisitRepository{
			endVisitsByActiveGroupIDsFunc: func(context.Context, []int64) (int64, error) {
				return 2, nil
			},
		}, SupervisorRepo: &mockGroupSupervisorRepository{
			endSupervisionsByIDsFunc: func(context.Context, []int64) (int64, error) {
				return 0, errors.New("bulk supervisor close failed")
			},
		}},
		}

		result, err := svc.EndDailySessions(ctx)

		require.Error(t, err)
		assert.False(t, result.Success)
		assert.Equal(t, 1, result.SessionsEnded)
		assert.Contains(t, result.Errors[0], "bulk supervisor close failed")
	})
}

func TestCleanupOrphanedSupervisors_ErrorBranches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	today := timezone.TodayDate()

	t.Run("find failure is captured", func(t *testing.T) {
		result := &DailySessionCleanupResult{Success: true}
		svc := &service{ServiceDependencies: ServiceDependencies{SupervisorRepo: &mockGroupSupervisorRepository{
			findStaleOpenFunc: func(context.Context, timezone.Date) ([]*activeModels.GroupSupervisor, error) {
				return nil, errors.New("stale lookup failed")
			},
		}},
		}

		svc.cleanupOrphanedSupervisors(ctx, result)

		assert.False(t, result.Success)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0], "stale lookup failed")
	})

	t.Run("update failure is captured", func(t *testing.T) {
		result := &DailySessionCleanupResult{Success: true}
		record := &activeModels.GroupSupervisor{Model: modelBase.Model{ID: 10}, GroupID: 20, StartDate: today.AddDays(-1)}
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
			findByIDForUpdateFunc: func(context.Context, int64) (*activeModels.Group, error) {
				return &activeModels.Group{Model: modelBase.Model{ID: 20}}, nil
			},
		}, SupervisorRepo: &mockGroupSupervisorRepository{
			findStaleOpenFunc: func(context.Context, timezone.Date) ([]*activeModels.GroupSupervisor, error) {
				return []*activeModels.GroupSupervisor{record}, nil
			},
			findByIDFunc: func(context.Context, interface{}) (*activeModels.GroupSupervisor, error) {
				return record, nil
			},
			updateColumnsFunc: func(context.Context, *activeModels.GroupSupervisor, ...string) (int64, error) {
				return 0, errors.New("stale close failed")
			},
		}},
		}

		svc.cleanupOrphanedSupervisors(ctx, result)

		assert.False(t, result.Success)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0], "stale close failed")
	})
}
