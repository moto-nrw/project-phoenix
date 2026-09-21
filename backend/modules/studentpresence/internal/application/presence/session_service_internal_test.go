package presence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/workforce/adapters/timerecords"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type deviceRepoForSessionUnitTest struct {
	window           time.Duration
	updateRoomIDFunc func(ctx context.Context, id int64, roomID int64) error
}

func (d *deviceRepoForSessionUnitTest) OnlineWindow(context.Context) time.Duration {
	return d.window
}

func (d *deviceRepoForSessionUnitTest) ManualAttendanceDeviceID(context.Context) (int64, error) {
	return 0, nil
}
func (d *deviceRepoForSessionUnitTest) UpdateRoomID(ctx context.Context, id int64, roomID int64) error {
	if d.updateRoomIDFunc != nil {
		return d.updateRoomIDFunc(ctx, id, roomID)
	}
	return nil
}

type workSessionServiceForSessionUnitTest struct {
	ensureCheckedInFunc func(ctx context.Context, staffID int64, source string) (*timerecords.WorkSession, error)
}

func (w *workSessionServiceForSessionUnitTest) EnsureCheckedIn(ctx context.Context, staffID int64, source string) (*timerecords.WorkSession, error) {
	if w.ensureCheckedInFunc != nil {
		return w.ensureCheckedInFunc(ctx, staffID, source)
	}
	return &timerecords.WorkSession{}, nil
}

// plannedStartNotReachedForSessionUnitTest stands in for the time clock's
// refusal, matched by presence through its PlannedStartNotReachedError port.
type plannedStartNotReachedForSessionUnitTest struct {
	plannedStartTime string
	currentTime      string
}

func (e plannedStartNotReachedForSessionUnitTest) Error() string { return "planned start not reached" }

func (e plannedStartNotReachedForSessionUnitTest) PlannedStartNotReached() (string, string) {
	return e.plannedStartTime, e.currentTime
}

func newSessionSQLMockDB(t *testing.T) (*testpkg.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := testpkg.NewBunDB(sqlDB)
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, db.Close())
	})
	return db, mock
}

type sessionTestSavepoints struct{}

type sessionStartLockerStub struct {
	lock func(context.Context, int64) error
}

func (s sessionStartLockerStub) LockSessionStart(ctx context.Context, activityID int64) error {
	return s.lock(ctx, activityID)
}

func TestGetActivityEndNameKeepsTransactionUsableAfterDatabaseFailure(t *testing.T) {
	t.Parallel()

	svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, Logger: slog.New(slog.DiscardHandler)}}
	var lookupErr error
	err := tenant.WithinCurrentTenant(testpkg.Ctx(t), func(txCtx context.Context) error {
		name, err := svc.getActivityEndName(txCtx, func(lookupCtx context.Context) (string, error) {
			rawTx, ok := tenant.TransactionFromContext(lookupCtx)
			require.True(t, ok)
			tx, ok := rawTx.(testpkg.Tx)
			require.True(t, ok)
			_, lookupErr = tx.ExecContext(lookupCtx, "SELECT 1 / 0")
			return "", lookupErr
		}, "SSE room name lookup failed", slog.Int64("room_id", 1))
		require.NoError(t, err)
		assert.Empty(t, name)

		rawTx, ok := tenant.TransactionFromContext(txCtx)
		require.True(t, ok)
		tx, ok := rawTx.(testpkg.Tx)
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
	svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SessionStartLock: sessionStartLockerStub{
		lock: func(ctx context.Context, activityID int64) error {
			called = true
			assert.Equal(t, int64(17), tenant.FromContext(ctx))
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
	svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SessionStartLock: sessionStartLockerStub{
		lock: func(context.Context, int64) error { return expected },
	}}}

	err := svc.acquireActivitySessionLock(tenant.WithTenantID(context.Background(), 17), 23, "start")
	require.ErrorIs(t, err, expected)
}

func (sessionTestSavepoints) exec(ctx context.Context, statement string) error {
	raw, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return tenant.ErrRuntimeRequired
	}
	var tx testpkg.Tx
	switch value := raw.(type) {
	case testpkg.Tx:
		tx = value
	case *testpkg.Tx:
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

func withSessionTestRuntime(t *testing.T, ctx context.Context, db *testpkg.DB) context.Context {
	t.Helper()
	tenantID := testpkg.Tenant(t)
	within := func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx testpkg.Tx) error { return fn(ctx, tx) })
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

	svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, Logger: slog.Default(), GroupRepo: &mockGroupRepository{
		findByIDFunc: func(context.Context, interface{}) (*ports.ActiveGroup, error) {
			return &ports.ActiveGroup{ID: 100, GroupID: &activityID}, nil
		},
	}, SchoolPresence: &mockVisitRepository{
		findByActiveGroupIDFunc: func(context.Context, int64) ([]*studentpresence.Visit, error) {
			findCalls++
			if findCalls == 1 {
				return nil, errors.New("student lookup prefetch failed")
			}
			return []*studentpresence.Visit{
				{ID: 300, ActiveGroupID: 100, StudentID: 400, EntryTime: entryTime},
			}, nil
		},
		findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
			visit := &studentpresence.Visit{
				ID: 300, ActiveGroupID: 100, StudentID: 400, EntryTime: entryTime,
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
	activeGroup := &ports.ActiveGroup{ID: 100}

	t.Run("checkout lookup failure", func(t *testing.T) {
		findCalls := 0
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{
			findByIDFunc: func(context.Context, interface{}) (*ports.ActiveGroup, error) {
				return activeGroup, nil
			},
		}, SchoolPresence: &mockVisitRepository{
			findByActiveGroupIDFunc: func(context.Context, int64) ([]*studentpresence.Visit, error) {
				findCalls++
				if findCalls == 1 {
					return []*studentpresence.Visit{}, nil
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
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{
			findByIDFunc: func(context.Context, interface{}) (*ports.ActiveGroup, error) {
				return activeGroup, nil
			},
		}, SchoolPresence: &mockVisitRepository{
			endGroupSessionFunc: func(context.Context, int64, time.Time) (studentpresence.EndedGroupSession, error) {
				return studentpresence.EndedGroupSession{}, errors.New("session end failed")
			},
		}, SupervisorRepo: &mockGroupSupervisorRepository{}},
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
	activeGroup := &ports.ActiveGroup{ID: 100}

	newService := func(order *[]string, bridgeErr error) *service {
		return &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal,
			Logger: slog.Default(),
			GroupRepo: &mockGroupRepository{
				findByIDFunc: func(context.Context, interface{}) (*ports.ActiveGroup, error) {
					return activeGroup, nil
				},
			},
			SchoolPresence: &mockVisitRepository{
				endGroupSessionFunc: func(_ context.Context, id int64, at time.Time) (studentpresence.EndedGroupSession, error) {
					*order = append(*order, "session")
					return studentpresence.EndedGroupSession{GroupID: id, EndedAt: at}, nil
				},
			},
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
	activeGroup := &ports.ActiveGroup{ID: 100}

	newService := func(db *testpkg.DB, endSessionErr error) *service {
		return &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal,
			Logger: slog.Default(),
			DB:     db,
			GroupRepo: &mockGroupRepository{
				findByIDFunc: func(context.Context, interface{}) (*ports.ActiveGroup, error) {
					return activeGroup, nil
				},
			},
			SchoolPresence: &mockVisitRepository{
				endGroupSessionFunc: func(_ context.Context, id int64, at time.Time) (studentpresence.EndedGroupSession, error) {
					return studentpresence.EndedGroupSession{GroupID: id, EndedAt: at}, endSessionErr
				},
			},
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
// against. sessionEndErr fails the Student Presence owner's session end, i.e.
// the session end AFTER the bridge has already completed the mirrored instance.
func newEndGroupService(
	t *testing.T,
	order *[]string,
	group *ports.ActiveGroup,
	bridgeErr error,
	sessionEndErr error,
) *service {
	t.Helper()

	return &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal,
		Logger: slog.Default(),
		GroupRepo: &mockGroupRepository{
			findByIDFunc: func(context.Context, interface{}) (*ports.ActiveGroup, error) {
				return group, nil
			},
		},
		SchoolPresence: &mockVisitRepository{
			endGroupSessionFunc: func(_ context.Context, id int64, at time.Time) (studentpresence.EndedGroupSession, error) {
				*order = append(*order, "session")
				return studentpresence.EndedGroupSession{GroupID: id, EndedAt: at}, sessionEndErr
			},
		},
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

	newService := func(order *[]string, group *ports.ActiveGroup, bridgeErr error) *service {
		return newEndGroupService(t, order, group, bridgeErr, nil)
	}

	t.Run("closes the timetable first", func(t *testing.T) {
		var order []string

		err := newService(&order, &ports.ActiveGroup{ID: 100}, nil).
			EndActiveGroupSession(ctx, 100)

		require.NoError(t, err)
		assert.Equal(t, []string{"timetable", "session"}, order)
	})

	t.Run("a failing bridge leaves the session open", func(t *testing.T) {
		var order []string

		err := newService(&order, &ports.ActiveGroup{ID: 100}, errors.New("bridge down")).
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
		ended := &ports.ActiveGroup{ID: 100, EndTime: &endTime}

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

		err := newEndGroupService(t, &order, &ports.ActiveGroup{ID: 100}, nil,
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
	missErr := &DatabaseError{Op: "update last activity - session not found", Err: ErrNotFound}

	tests := []struct {
		name      string
		findFunc  func(context.Context, interface{}) (*ports.ActiveGroup, error)
		wantError string
	}{
		{
			name: "find by id no rows becomes not found",
			findFunc: func(context.Context, interface{}) (*ports.ActiveGroup, error) {
				return nil, &DatabaseError{Op: "find by id", Err: ErrNotFound}
			},
			wantError: ErrActiveGroupNotFound.Error(),
		},
		{
			name: "nil session becomes not found",
			findFunc: func(context.Context, interface{}) (*ports.ActiveGroup, error) {
				return nil, nil
			},
			wantError: ErrActiveGroupNotFound.Error(),
		},
		{
			name: "ended session becomes already ended",
			findFunc: func(context.Context, interface{}) (*ports.ActiveGroup, error) {
				endedAt := time.Now()
				return &ports.ActiveGroup{ID: 100, EndTime: &endedAt}, nil
			},
			wantError: ErrActiveGroupAlreadyEnded.Error(),
		},
		{
			name: "unexpected lookup error is preserved",
			findFunc: func(context.Context, interface{}) (*ports.ActiveGroup, error) {
				return nil, errors.New("lookup exploded")
			},
			wantError: "lookup exploded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{
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

	svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{
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
	assert.Equal(t, defaultDeviceOnlineWindow, (&service{}).deviceOnlineWindow(ctx))
	for _, window := range []time.Duration{0, -time.Minute, 2 * time.Minute} {
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, DeviceRepo: &deviceRepoForSessionUnitTest{window: window}}}
		want := defaultDeviceOnlineWindow
		if window > 0 {
			want = window
		}
		assert.Equal(t, want, svc.deviceOnlineWindow(ctx))
	}
}

func TestSessionIsDeviceOnline(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-time.Minute)
	old := now.Add(-10 * time.Minute)
	svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, DeviceRepo: &deviceRepoForSessionUnitTest{window: 5 * time.Minute}}}

	assert.False(t, svc.isDeviceOnline(context.Background(), nil, now))
	assert.False(t, svc.isDeviceOnline(context.Background(), &ports.SessionDevice{}, now))
	assert.True(t, svc.isDeviceOnline(context.Background(), &ports.SessionDevice{LastSeen: &recent}, now))
	assert.False(t, svc.isDeviceOnline(context.Background(), &ports.SessionDevice{LastSeen: &old}, now))
}

func TestUpdateDeviceLocationBestEffort(t *testing.T) {
	t.Parallel()

	calls := 0
	svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, Logger: slog.Default(), DeviceRepo: &deviceRepoForSessionUnitTest{
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
		20: plannedStartNotReachedForSessionUnitTest{
			plannedStartTime: "09:00",
			currentTime:      "08:45",
		},
		30: nil,
	}
	checked := map[int64]bool{}
	created := map[int64]bool{}
	svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, Logger: slog.Default(), SupervisorRepo: &mockGroupSupervisorRepository{
		createFunc: func(_ context.Context, supervisor *ports.GroupSupervisor) error {
			created[supervisor.StaffID] = true
			return nil
		},
	}, WorkSessionService: &workSessionServiceForSessionUnitTest{
		ensureCheckedInFunc: func(_ context.Context, staffID int64, source string) (*timerecords.WorkSession, error) {
			checked[staffID] = true
			assert.Equal(t, stampSourceNFC, source)
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
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, Logger: slog.Default()}}
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
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, Logger: slog.Default()}}
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
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, Logger: slog.Default()}}
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
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{
			createFunc: func(context.Context, *ports.ActiveGroup) error {
				return expectedErr
			},
		}, SchoolPresence: &mockVisitRepository{
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
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{
			createFunc: func(_ context.Context, group *ports.ActiveGroup) error {
				group.ID = 44
				return nil
			},
		}, SchoolPresence: &mockVisitRepository{
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
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{
			createFunc: func(_ context.Context, group *ports.ActiveGroup) error {
				group.ID = 45
				return nil
			},
		}, SchoolPresence: &mockVisitRepository{
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

func TestOpenSessionForActivityJoinsOnlyIndependentDeviceLessStays(t *testing.T) {
	t.Parallel()

	activityID := int64(10)
	otherID := int64(11)
	deviceID := int64(20)
	kiosk := &ports.ActiveGroup{GroupID: &activityID, DeviceID: &deviceID, RoomID: 1}
	independent := &ports.ActiveGroup{GroupID: &activityID, RoomID: 1}
	planner := &ports.ActiveGroup{GroupID: &activityID, RoomID: 1}
	other := &ports.ActiveGroup{GroupID: &otherID, RoomID: 1}

	t.Run("device-owned session of the activity wins", func(t *testing.T) {
		got := joinableOpenSession([]*ports.ActiveGroup{independent, kiosk}, activityID, true)
		assert.Equal(t, kiosk, got)
	})

	t.Run("independent room stay is joinable", func(t *testing.T) {
		got := joinableOpenSession([]*ports.ActiveGroup{independent}, activityID, true)
		assert.Equal(t, independent, got)
	})

	t.Run("planner session of a regular activity is not joinable", func(t *testing.T) {
		got := joinableOpenSession([]*ports.ActiveGroup{planner}, activityID, false)
		assert.Nil(t, got)
	})

	t.Run("other activities are ignored", func(t *testing.T) {
		got := joinableOpenSession([]*ports.ActiveGroup{other, independent}, activityID, true)
		assert.Equal(t, independent, got)
	})
}

func TestActivityRunningElsewhereTreatsPlannerSessionsAsConflicts(t *testing.T) {
	t.Parallel()

	activityID := int64(10)
	deviceID := int64(20)
	otherDevice := int64(21)
	sameDevice := &ports.ActiveGroup{GroupID: &activityID, DeviceID: &deviceID, RoomID: 1}
	independent := &ports.ActiveGroup{GroupID: &activityID, RoomID: 1}
	planner := &ports.ActiveGroup{GroupID: &activityID, RoomID: 1}
	elsewhere := &ports.ActiveGroup{GroupID: &activityID, DeviceID: &otherDevice, RoomID: 2}
	otherDeviceSameRoom := &ports.ActiveGroup{GroupID: &activityID, DeviceID: &otherDevice, RoomID: 1}

	assert.False(t, activityRunningElsewhere([]*ports.ActiveGroup{sameDevice}, 1, deviceID, false))
	assert.False(t, activityRunningElsewhere([]*ports.ActiveGroup{independent}, 1, deviceID, true))
	assert.True(t, activityRunningElsewhere([]*ports.ActiveGroup{planner}, 1, deviceID, false),
		"a device-less planner session in the same room must conflict")
	assert.True(t, activityRunningElsewhere([]*ports.ActiveGroup{elsewhere}, 1, deviceID, false))
	assert.True(t, activityRunningElsewhere([]*ports.ActiveGroup{otherDeviceSameRoom}, 1, deviceID, false))
}

func TestActivityConflictDestination(t *testing.T) {
	t.Parallel()

	activityID := int64(10)
	inGym := &ports.ActiveGroup{GroupID: &activityID, RoomID: 1}
	inYard := &ports.ActiveGroup{GroupID: &activityID, RoomID: 2}

	assert.Equal(t, int64(9), activityConflictDestination([]*ports.ActiveGroup{inGym}, 9),
		"a planned room is the kiosk preflight destination")
	assert.Equal(t, int64(1), activityConflictDestination([]*ports.ActiveGroup{inGym}, 0),
		"a single running copy supplies the destination when none is planned")
	assert.Zero(t, activityConflictDestination([]*ports.ActiveGroup{inGym, inYard}, 0),
		"copies in different rooms stay a conflict")
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
	svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, Logger: slog.Default(), GroupRepo: &mockGroupRepository{
		createFunc: func(_ context.Context, group *ports.ActiveGroup) error {
			group.ID = 60
			return nil
		},
	}, SchoolPresence: &mockVisitRepository{
		transferVisitsFromRecentSessionsFunc: func(context.Context, int64, int64) (int, error) {
			return 2, nil
		},
	}, SupervisorRepo: &mockGroupSupervisorRepository{
		createFunc: func(_ context.Context, supervisor *ports.GroupSupervisor) error {
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
	t.Run("lock failure stops before conflict lookup", func(t *testing.T) {
		failure := errors.New("room lock failed")
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{}, SchoolPresence: &mockVisitRepository{
			lockRoomSessionWritesFunc: func(_ context.Context, id int64) error { require.Equal(t, int64(10), id); return failure },
		}}}
		_, err := svc.validateManualRoomSelection(ctx, 10, RoomConflictFail, true)
		require.ErrorIs(t, err, failure)
	})

	t.Run("ignore strategy skips conflict check", func(t *testing.T) {
		svc := &service{}
		roomID, err := svc.validateManualRoomSelection(ctx, 10, RoomConflictIgnore, true)
		require.NoError(t, err)
		assert.Equal(t, int64(10), roomID)
	})

	t.Run("repository error is returned", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SchoolPresence: &mockVisitRepository{}, GroupRepo: &mockGroupRepository{
			checkRoomConflictFunc: func(context.Context, int64, int64) (bool, *ports.ActiveGroup, error) {
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
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SchoolPresence: &mockVisitRepository{}, Logger: slog.Default(), GroupRepo: &mockGroupRepository{
			checkRoomConflictFunc: func(context.Context, int64, int64) (bool, *ports.ActiveGroup, error) {
				return true, &ports.ActiveGroup{ID: 20}, nil
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
	AttendanceActivityGroups
	group   *ports.SessionActivity
	findErr error
}

func (a *activityGroupRepoForRoomUnitTest) FindByID(context.Context, any) (*ports.SessionActivity, error) {
	return a.group, a.findErr
}

func TestDetermineRoomIDWithStrategy_NoSelectionAndNoPlannedRoom(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("planned room is used when configured", func(t *testing.T) {
		plannedRoom := int64(7)
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal,
			ActivityGroupRepo: &activityGroupRepoForRoomUnitTest{
				group: &ports.SessionActivity{PlannedRoomID: &plannedRoom},
			},
		}}

		roomID, err := svc.determineRoomIDWithStrategy(ctx, 1, nil, RoomConflictFail, true)

		require.NoError(t, err)
		assert.Equal(t, plannedRoom, roomID)
	})

	// No hardcoded fallback: room id 1 belongs to a single school, so a default
	// would trip fk_active_groups_room_tenant for every other tenant.
	t.Run("no room and no planned room fails", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal,
			ActivityGroupRepo: &activityGroupRepoForRoomUnitTest{
				group: &ports.SessionActivity{},
			},
		}}

		roomID, err := svc.determineRoomIDWithStrategy(ctx, 1, nil, RoomConflictFail, true)

		require.ErrorIs(t, err, ErrNoRoomAvailable)
		assert.Zero(t, roomID)
	})

	t.Run("repository error is returned", func(t *testing.T) {
		lookupErr := errors.New("planned room lookup failed")
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal,
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
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{
			findActiveByGroupIDFunc: func(context.Context, int64) ([]*ports.ActiveGroup, error) {
				return []*ports.ActiveGroup{
					nil,
					{ID: 0},
					{ID: 30},
				}, nil
			},
		}, SchoolPresence: &mockVisitRepository{
			endGroupFunc: func(_ context.Context, id int64, _ time.Time) error {
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
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{
			findActiveByGroupIDFunc: func(context.Context, int64) ([]*ports.ActiveGroup, error) {
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
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{
			findActiveByGroupIDFunc: func(context.Context, int64) ([]*ports.ActiveGroup, error) {
				return []*ports.ActiveGroup{{ID: 31}}, nil
			},
		}, SchoolPresence: &mockVisitRepository{
			endGroupFunc: func(context.Context, int64, time.Time) error {
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
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SchoolPresence: &mockVisitRepository{
			transferActiveVisitsBetweenGroupsFunc: func(context.Context, int64, int64) (int, error) {
				return 0, visitErr
			},
		}},
		}

		err := svc.transferForceStartedActivityState(ctx, []int64{1}, 2, time.Now())

		require.ErrorIs(t, err, visitErr)
	})

	t.Run("supervisor transfer error", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SchoolPresence: &mockVisitRepository{}, SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*ports.GroupSupervisor, error) {
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
	svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, TimetableBridgeCompleter: &timetableBridgeCompleterForSessionUnitTest{
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
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*ports.GroupSupervisor, error) {
				call++
				if call == 2 {
					return nil, expectedErr
				}
				return []*ports.GroupSupervisor{}, nil
			},
		}},
		}

		count, err := svc.transferActiveSupervisorsBetweenGroups(ctx, 1, 2, time.Now())

		require.ErrorIs(t, err, expectedErr)
		assert.Zero(t, count)
	})

	t.Run("end supervision error returns partial count", func(t *testing.T) {
		expectedErr := errors.New("end supervision failed")
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(_ context.Context, activeGroupID int64, _ bool) ([]*ports.GroupSupervisor, error) {
				if activeGroupID == 1 {
					return []*ports.GroupSupervisor{
						{ID: 10, StaffID: 20, Role: "helper", StartDate: today},
					}, nil
				}
				return []*ports.GroupSupervisor{}, nil
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
		var created []*ports.GroupSupervisor
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(_ context.Context, activeGroupID int64, _ bool) ([]*ports.GroupSupervisor, error) {
				if activeGroupID == 1 {
					return []*ports.GroupSupervisor{
						nil,
						{ID: 10, StaffID: 20, Role: "Supervisor", StartDate: today},
						{ID: 11, StaffID: 30, Role: "helper", StartDate: today},
					}, nil
				}
				return []*ports.GroupSupervisor{
					nil,
					{ID: 12, StaffID: 20, Role: "supervisor", StartDate: today},
				}, nil
			},
			endSupervisionFunc: func(_ context.Context, id int64) error {
				ended = append(ended, id)
				return nil
			},
			createFunc: func(_ context.Context, supervisor *ports.GroupSupervisor) error {
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
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(_ context.Context, activeGroupID int64, _ bool) ([]*ports.GroupSupervisor, error) {
				if activeGroupID == 1 {
					return []*ports.GroupSupervisor{
						{ID: 10, StaffID: 20, Role: "Supervisor", StartDate: today},
					}, nil
				}
				return []*ports.GroupSupervisor{}, nil
			},
			createFunc: func(context.Context, *ports.GroupSupervisor) error {
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
	svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SchoolPresence: &mockVisitRepository{
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
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{
			findActiveByDeviceIDFunc: func(context.Context, int64) (*ports.ActiveGroup, error) {
				return nil, expectedErr
			},
		}},
		}

		endedID, err := svc.endExistingDeviceSessionForForceStart(ctx, 100)

		require.ErrorIs(t, err, expectedErr)
		assert.Zero(t, endedID)
	})

	t.Run("nil existing session is zero", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{}}}

		endedID, err := svc.endExistingDeviceSessionForForceStart(ctx, 100)

		require.NoError(t, err)
		assert.Zero(t, endedID)
	})

	t.Run("end error is returned", func(t *testing.T) {
		expectedErr := errors.New("force end failed")
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{
			findActiveByDeviceIDFunc: func(context.Context, int64) (*ports.ActiveGroup, error) {
				return &ports.ActiveGroup{ID: 302}, nil
			},
		}, SchoolPresence: &mockVisitRepository{
			endGroupFunc: func(context.Context, int64, time.Time) error {
				return expectedErr
			},
		}},
		}

		endedID, err := svc.endExistingDeviceSessionForForceStart(ctx, 100)

		require.ErrorIs(t, err, expectedErr)
		assert.Zero(t, endedID)
	})

	t.Run("returns ended session id", func(t *testing.T) {
		var released []int64
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{
			findActiveByDeviceIDFunc: func(context.Context, int64) (*ports.ActiveGroup, error) {
				return &ports.ActiveGroup{ID: 303}, nil
			},
		}, SchoolPresence: &mockVisitRepository{
			endGroupFunc: func(_ context.Context, id int64, _ time.Time) error {
				released = append(released, id)
				return nil
			},
		}},
		}

		endedID, err := svc.endExistingDeviceSessionForForceStart(ctx, 100)

		require.NoError(t, err)
		assert.Equal(t, int64(303), endedID)
		assert.Equal(t, []int64{303}, released, "the presence owner releases the replaced group")
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
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*ports.GroupSupervisor, error) {
				return nil, expectedErr
			},
		}},
		}

		err := svc.replaceSupervisorsInTransaction(ctx, 100, map[int64]bool{10: true})

		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("ending current supervisor error", func(t *testing.T) {
		expectedErr := errors.New("end current supervisor failed")
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*ports.GroupSupervisor, error) {
				return []*ports.GroupSupervisor{
					{ID: 20, StaffID: 10, Role: "supervisor", StartDate: startDate},
				}, nil
			},
			updateFunc: func(context.Context, *ports.GroupSupervisor) error {
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
		supervisors := []*ports.GroupSupervisor{
			{ID: 20, StaffID: 10, Role: "supervisor", StartDate: startDate, EndDate: &endedDate},
		}
		updateCalls := 0
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*ports.GroupSupervisor, error) {
				return supervisors, nil
			},
			updateFunc: func(context.Context, *ports.GroupSupervisor) error {
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
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SupervisorRepo: &mockGroupSupervisorRepository{
			findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*ports.GroupSupervisor, error) {
				return []*ports.GroupSupervisor{}, nil
			},
			createFunc: func(context.Context, *ports.GroupSupervisor) error {
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
	additional := &ports.GroupSupervisor{
		ID:        21,
		StaffID:   11,
		Role:      "additional_supervisor",
		StartDate: timezone.TodayDate(),
	}
	primary := &ports.GroupSupervisor{
		ID:        20,
		StaffID:   10,
		Role:      "supervisor",
		StartDate: timezone.TodayDate(),
	}

	var updatedIDs, createdStaffIDs []int64
	svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SupervisorRepo: &mockGroupSupervisorRepository{
		findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*ports.GroupSupervisor, error) {
			return []*ports.GroupSupervisor{primary, additional}, nil
		},
		updateFunc: func(_ context.Context, supervisor *ports.GroupSupervisor) error {
			updatedIDs = append(updatedIDs, supervisor.ID)
			return nil
		},
		createFunc: func(_ context.Context, supervisor *ports.GroupSupervisor) error {
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
	activeGroup := &ports.ActiveGroup{ID: 100}

	t.Run("list failure returns active error", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{
			listFunc: func(context.Context) ([]*ports.ActiveGroup, error) {
				return nil, errors.New("list failed")
			},
		}},
		}

		svc.settings = &stubSettingsResolver{stringValues: map[string]string{presenceModeQuestion: PresenceModeDetailed}}
		result, err := svc.EndDailySessions(ctx)

		require.Error(t, err)
		require.NotNil(t, result)
		assert.False(t, result.Success)
	})

	t.Run("visit bulk failure aborts later bulk steps", func(t *testing.T) {
		db, mock := newSessionSQLMockDB(t)
		mock.ExpectBegin()
		mock.ExpectRollback()
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, DB: db, GroupRepo: &mockGroupRepository{
			listFunc: func(context.Context) ([]*ports.ActiveGroup, error) {
				return []*ports.ActiveGroup{activeGroup}, nil
			},
		}, SchoolPresence: &mockVisitRepository{
			endVisitsByActiveGroupIDsFunc: func(context.Context, []int64) (int64, error) {
				return 0, errors.New("bulk visit close failed")
			},
		}, SupervisorRepo: &mockGroupSupervisorRepository{}},
		}

		svc.settings = &stubSettingsResolver{stringValues: map[string]string{presenceModeQuestion: PresenceModeDetailed}}
		result, err := svc.EndDailySessions(withSessionTestRuntime(t, ctx, db))

		require.Error(t, err)
		require.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Contains(t, result.Errors[0], "bulk visit close failed")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	// The Student Presence owner closes visits, sessions, and supervisions in
	// one command (#2697): a failure inside it leaves no partial counts behind.
	t.Run("owner bulk close failure records the error without partial counts", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{
			listFunc: func(context.Context) ([]*ports.ActiveGroup, error) {
				return []*ports.ActiveGroup{activeGroup}, nil
			},
		}, SchoolPresence: &mockVisitRepository{
			endGroupSessionsFunc: func(context.Context, []int64, time.Time) (studentpresence.EndedGroupSessions, error) {
				return studentpresence.EndedGroupSessions{}, errors.New("bulk session close failed")
			},
		}, SupervisorRepo: &mockGroupSupervisorRepository{}},
		}

		svc.settings = &stubSettingsResolver{stringValues: map[string]string{presenceModeQuestion: PresenceModeDetailed}}
		result, err := svc.EndDailySessions(ctx)

		require.Error(t, err)
		assert.False(t, result.Success)
		assert.Zero(t, result.VisitsEnded)
		assert.Zero(t, result.SessionsEnded)
		assert.Zero(t, result.SupervisorsEnded)
		assert.Empty(t, result.EndedActiveGroupIDs)
		assert.Contains(t, result.Errors[0], "bulk session close failed")
	})

	t.Run("owner bulk close reports every count", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{
			listFunc: func(context.Context) ([]*ports.ActiveGroup, error) {
				return []*ports.ActiveGroup{activeGroup}, nil
			},
		}, SchoolPresence: &mockVisitRepository{
			endGroupSessionsFunc: func(_ context.Context, ids []int64, _ time.Time) (studentpresence.EndedGroupSessions, error) {
				return studentpresence.EndedGroupSessions{VisitsClosed: 2, SessionsEnded: 1, SupervisorsEnded: 3, EndedActiveGroupIDs: ids}, nil
			},
		}, SupervisorRepo: &mockGroupSupervisorRepository{}},
		}

		svc.settings = &stubSettingsResolver{stringValues: map[string]string{presenceModeQuestion: PresenceModeDetailed}}
		result, err := svc.EndDailySessions(ctx)

		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 2, result.VisitsEnded)
		assert.Equal(t, 1, result.SessionsEnded)
		assert.Equal(t, 3, result.SupervisorsEnded)
		assert.Equal(t, []int64{activeGroup.ID}, result.EndedActiveGroupIDs)
	})
}

func TestCleanupOrphanedSupervisors_ErrorBranches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	today := timezone.TodayDate()

	t.Run("find failure is captured", func(t *testing.T) {
		result := &DailySessionCleanupResult{Success: true}
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SupervisorRepo: &mockGroupSupervisorRepository{
			findStaleOpenFunc: func(context.Context, timezone.Date) ([]*ports.GroupSupervisor, error) {
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
		record := &ports.GroupSupervisor{ID: 10, GroupID: 20, StartDate: today.AddDays(-1)}
		svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, GroupRepo: &mockGroupRepository{
			findByIDForUpdateFunc: func(context.Context, int64) (*ports.ActiveGroup, error) {
				return &ports.ActiveGroup{ID: 20}, nil
			},
		}, SupervisorRepo: &mockGroupSupervisorRepository{
			findStaleOpenFunc: func(context.Context, timezone.Date) ([]*ports.GroupSupervisor, error) {
				return []*ports.GroupSupervisor{record}, nil
			},
			findByIDFunc: func(context.Context, interface{}) (*ports.GroupSupervisor, error) {
				return record, nil
			},
			updateColumnsFunc: func(context.Context, *ports.GroupSupervisor, ...string) (int64, error) {
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
