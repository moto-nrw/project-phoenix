package active

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// mockGroupRepository is a minimal mock implementation of active.GroupRepository
type mockGroupRepository struct {
	createFunc                      func(ctx context.Context, entity *active.Group) error
	findByIDFunc                    func(ctx context.Context, id interface{}) (*active.Group, error)
	findByIDForUpdateFunc           func(ctx context.Context, id int64) (*active.Group, error)
	listFunc                        func(ctx context.Context, options *base.QueryOptions) ([]*active.Group, error)
	findActiveByDeviceIDFunc        func(ctx context.Context, deviceID int64) (*active.Group, error)
	findActiveByGroupIDFunc         func(ctx context.Context, groupID int64) ([]*active.Group, error)
	endSessionFunc                  func(ctx context.Context, id int64) error
	updateLastActivityFunc          func(ctx context.Context, id int64, lastActivity time.Time) error
	findActiveSessionsOlderThanFunc func(ctx context.Context, cutoffTime time.Time) ([]*active.Group, error)
	checkRoomConflictFunc           func(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *active.Group, error)
	endSessionsByIDsFunc            func(ctx context.Context, ids []int64) (int64, error)
}

func (m *mockGroupRepository) Create(ctx context.Context, entity *active.Group) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, entity)
	}
	return nil
}

func (m *mockGroupRepository) FindByID(ctx context.Context, id interface{}) (*active.Group, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return &active.Group{
		Model: base.Model{ID: 1},
	}, nil
}

func (m *mockGroupRepository) FindByIDForUpdate(ctx context.Context, id int64) (*active.Group, error) {
	if m.findByIDForUpdateFunc != nil {
		return m.findByIDForUpdateFunc(ctx, id)
	}
	if m.findByIDFunc != nil {
		return m.FindByID(ctx, id)
	}
	return &active.Group{
		Model: base.Model{ID: id},
	}, nil
}

func (m *mockGroupRepository) Update(ctx context.Context, entity *active.Group) error {
	return nil
}

func (m *mockGroupRepository) Delete(ctx context.Context, id interface{}) error {
	return nil
}

func (m *mockGroupRepository) List(ctx context.Context, options *base.QueryOptions) ([]*active.Group, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, options)
	}
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByRoomID(ctx context.Context, roomID int64) ([]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByRoomIDAndDeviceID(ctx context.Context, roomID int64, deviceID int64) (*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByGroupID(ctx context.Context, groupID int64) ([]*active.Group, error) {
	if m.findActiveByGroupIDFunc != nil {
		return m.findActiveByGroupIDFunc(ctx, groupID)
	}
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByGroupIDs(ctx context.Context, groupIDs []int64) ([]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) EndSession(ctx context.Context, id int64) error {
	if m.endSessionFunc != nil {
		return m.endSessionFunc(ctx, id)
	}
	return nil
}

func (m *mockGroupRepository) FindWithRelations(ctx context.Context, id int64) (*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindWithVisits(ctx context.Context, id int64) (*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindWithSupervisors(ctx context.Context, id int64) (*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByGroupIDWithDevice(ctx context.Context, groupID int64) ([]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByDeviceID(ctx context.Context, deviceID int64) (*active.Group, error) {
	if m.findActiveByDeviceIDFunc != nil {
		return m.findActiveByDeviceIDFunc(ctx, deviceID)
	}
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByDeviceIDWithRelations(ctx context.Context, deviceID int64) (*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) CheckRoomConflict(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *active.Group, error) {
	if m.checkRoomConflictFunc != nil {
		return m.checkRoomConflictFunc(ctx, roomID, excludeGroupID)
	}
	return false, nil, nil
}

func (m *mockGroupRepository) UpdateLastActivity(ctx context.Context, id int64, lastActivity time.Time) error {
	if m.updateLastActivityFunc != nil {
		return m.updateLastActivityFunc(ctx, id, lastActivity)
	}
	return nil
}

func (m *mockGroupRepository) FindActiveSessionsOlderThan(ctx context.Context, cutoffTime time.Time) ([]*active.Group, error) {
	if m.findActiveSessionsOlderThanFunc != nil {
		return m.findActiveSessionsOlderThanFunc(ctx, cutoffTime)
	}
	return nil, nil
}

func (m *mockGroupRepository) FindInactiveSessions(ctx context.Context, inactiveDuration time.Duration) ([]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindUnclaimed(ctx context.Context) ([]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveGroups(ctx context.Context) ([]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) GetOccupiedRoomIDs(ctx context.Context, roomIDs []int64) (map[int64]bool, error) {
	return nil, nil
}

func (m *mockGroupRepository) GetOccupiedActivityGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]bool, error) {
	return nil, nil
}

func (m *mockGroupRepository) EndSessionsByIDs(ctx context.Context, ids []int64) (int64, error) {
	if m.endSessionsByIDsFunc != nil {
		return m.endSessionsByIDsFunc(ctx, ids)
	}
	return 0, nil
}

func (m *mockGroupRepository) AggregateRoomSessions(ctx context.Context, roomID int64, start, end time.Time, supervisorStaffID *int64) ([]*active.RoomSessionAggregate, error) {
	return nil, nil
}

// mockVisitRepository is a minimal mock implementation of active.VisitRepository
type mockVisitRepository struct {
	findByActiveGroupIDFunc               func(ctx context.Context, activeGroupID int64) ([]*active.Visit, error)
	findByIDFunc                          func(ctx context.Context, id interface{}) (*active.Visit, error)
	updateFunc                            func(ctx context.Context, entity *active.Visit) error
	endVisitFunc                          func(ctx context.Context, id int64) error
	getCurrentByStudentIDFunc             func(ctx context.Context, studentID int64) (*active.Visit, error)
	getCurrentByStudentIDWithRoomFunc     func(ctx context.Context, studentID int64) (*active.Visit, error)
	countActiveByRoomIDFunc               func(ctx context.Context, roomID int64) (int, error)
	countActiveByGroupIDFunc              func(ctx context.Context, activeGroupID int64) (int, error)
	listActiveStudentIDsByRoomIDFunc      func(ctx context.Context, roomID int64) ([]int64, error)
	getTodayVisitNamesFunc                func(ctx context.Context, studentIDs []int64) ([]active.VisitGroupNames, error)
	endVisitsByActiveGroupIDsFunc         func(ctx context.Context, activeGroupIDs []int64) (int64, error)
	endVisitsByIDsFunc                    func(ctx context.Context, ids []int64, at time.Time) ([]*active.Visit, error)
	transferVisitsFromRecentSessionsFunc  func(ctx context.Context, newActiveGroupID, deviceID int64) (int, error)
	transferActiveVisitsBetweenGroupsFunc func(ctx context.Context, oldActiveGroupID, newActiveGroupID int64) (int, error)
}

func (m *mockVisitRepository) Create(ctx context.Context, entity *active.Visit) error {
	return nil
}

func (m *mockVisitRepository) FindByID(ctx context.Context, id interface{}) (*active.Visit, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockVisitRepository) Update(ctx context.Context, entity *active.Visit) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, entity)
	}
	return nil
}

func (m *mockVisitRepository) Delete(ctx context.Context, id interface{}) error {
	return nil
}

func (m *mockVisitRepository) List(ctx context.Context, options *base.QueryOptions) ([]*active.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) FindActiveByStudentID(ctx context.Context, studentID int64) ([]*active.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.Visit, error) {
	if m.findByActiveGroupIDFunc != nil {
		return m.findByActiveGroupIDFunc(ctx, activeGroupID)
	}
	return []*active.Visit{}, nil
}

func (m *mockVisitRepository) FindByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) ([]*active.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) FindActiveWithStudentDisplayByGroup(_ context.Context, _ int64) ([]*active.VisitWithStudentDisplay, error) {
	return nil, nil
}

func (m *mockVisitRepository) FindByStudentAndTimeRange(ctx context.Context, studentID int64, start, end time.Time) ([]*active.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) FindByStudentAndActiveGroupIDs(ctx context.Context, studentID int64, activeGroupIDs []int64) ([]*active.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) EndVisit(ctx context.Context, id int64) error {
	if m.endVisitFunc != nil {
		return m.endVisitFunc(ctx, id)
	}
	return nil
}

func (m *mockVisitRepository) TransferVisitsFromRecentSessions(ctx context.Context, newActiveGroupID, deviceID int64) (int, error) {
	if m.transferVisitsFromRecentSessionsFunc != nil {
		return m.transferVisitsFromRecentSessionsFunc(ctx, newActiveGroupID, deviceID)
	}
	return 0, nil
}

func (m *mockVisitRepository) TransferActiveVisitsBetweenGroups(ctx context.Context, oldActiveGroupID, newActiveGroupID int64) (int, error) {
	if m.transferActiveVisitsBetweenGroupsFunc != nil {
		return m.transferActiveVisitsBetweenGroupsFunc(ctx, oldActiveGroupID, newActiveGroupID)
	}
	return 0, nil
}

func (m *mockVisitRepository) DeleteExpiredVisits(ctx context.Context, studentID int64, retentionDays int) (int64, error) {
	return 0, nil
}

func (m *mockVisitRepository) DeleteVisitsBeforeDate(ctx context.Context, studentID int64, beforeDate time.Time) (int64, error) {
	return 0, nil
}

func (m *mockVisitRepository) GetVisitRetentionStats(ctx context.Context) (map[int64]int, error) {
	return nil, nil
}

func (m *mockVisitRepository) CountExpiredVisits(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockVisitRepository) GetCurrentByStudentID(ctx context.Context, studentID int64) (*active.Visit, error) {
	if m.getCurrentByStudentIDFunc != nil {
		return m.getCurrentByStudentIDFunc(ctx, studentID)
	}
	return nil, nil
}

func (m *mockVisitRepository) GetCurrentByStudentIDWithRoom(ctx context.Context, studentID int64) (*active.Visit, error) {
	if m.getCurrentByStudentIDWithRoomFunc != nil {
		return m.getCurrentByStudentIDWithRoomFunc(ctx, studentID)
	}
	return nil, nil
}

func (m *mockVisitRepository) GetCurrentByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*active.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) CountActiveByRoomID(ctx context.Context, roomID int64) (int, error) {
	if m.countActiveByRoomIDFunc != nil {
		return m.countActiveByRoomIDFunc(ctx, roomID)
	}
	return 0, nil
}

func (m *mockVisitRepository) CountActiveByGroupID(ctx context.Context, activeGroupID int64) (int, error) {
	if m.countActiveByGroupIDFunc != nil {
		return m.countActiveByGroupIDFunc(ctx, activeGroupID)
	}
	return 0, nil
}

func (m *mockVisitRepository) ListActiveStudentIDsByRoomID(ctx context.Context, roomID int64) ([]int64, error) {
	if m.listActiveStudentIDsByRoomIDFunc != nil {
		return m.listActiveStudentIDsByRoomIDFunc(ctx, roomID)
	}
	return nil, nil
}

func (m *mockVisitRepository) FindActiveVisits(ctx context.Context) ([]*active.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) ListOpenVisitStudentIDsByRoom(ctx context.Context) (map[int64][]int64, error) {
	return nil, nil
}

func (m *mockVisitRepository) EndVisitsByIDs(ctx context.Context, ids []int64, at time.Time) ([]*active.Visit, error) {
	if m.endVisitsByIDsFunc != nil {
		return m.endVisitsByIDsFunc(ctx, ids, at)
	}
	return nil, nil
}

func (m *mockVisitRepository) EndVisitsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) (int64, error) {
	if m.endVisitsByActiveGroupIDsFunc != nil {
		return m.endVisitsByActiveGroupIDsFunc(ctx, activeGroupIDs)
	}
	return 0, nil
}

func (m *mockVisitRepository) CountActiveByStudentID(ctx context.Context, studentID int64) (int, error) {
	return 0, nil
}

func (m *mockVisitRepository) GetTodayVisitNamesForStudents(ctx context.Context, studentIDs []int64) ([]active.VisitGroupNames, error) {
	if m.getTodayVisitNamesFunc != nil {
		return m.getTodayVisitNamesFunc(ctx, studentIDs)
	}
	return nil, nil
}

// mockGroupSupervisorRepository is a minimal mock implementation of active.GroupSupervisorRepository
type mockGroupSupervisorRepository struct {
	findByIDFunc             func(ctx context.Context, id interface{}) (*active.GroupSupervisor, error)
	findByActiveGroupIDFunc  func(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*active.GroupSupervisor, error)
	endSupervisionFunc       func(ctx context.Context, id int64) error
	createFunc               func(ctx context.Context, entity *active.GroupSupervisor) error
	createBulkFunc           func(ctx context.Context, supervisors []*active.GroupSupervisor) error
	findAllActiveFunc        func(ctx context.Context) ([]*active.GroupSupervisor, error)
	updateFunc               func(ctx context.Context, entity *active.GroupSupervisor) error
	endSupervisionsByIDsFunc func(ctx context.Context, activeGroupIDs []int64) (int64, error)
	findStaleOpenFunc        func(ctx context.Context, before timezone.Date) ([]*active.GroupSupervisor, error)
	updateColumnsFunc        func(ctx context.Context, supervisor *active.GroupSupervisor, columns ...string) (int64, error)
}

func (m *mockGroupSupervisorRepository) Create(ctx context.Context, entity *active.GroupSupervisor) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, entity)
	}
	return nil
}

func (m *mockGroupSupervisorRepository) FindByID(ctx context.Context, id interface{}) (*active.GroupSupervisor, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockGroupSupervisorRepository) Update(ctx context.Context, entity *active.GroupSupervisor) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, entity)
	}
	return nil
}

func (m *mockGroupSupervisorRepository) Delete(ctx context.Context, id interface{}) error {
	return nil
}

func (m *mockGroupSupervisorRepository) List(ctx context.Context, options *base.QueryOptions) ([]*active.GroupSupervisor, error) {
	return nil, nil
}

func (m *mockGroupSupervisorRepository) FindActiveByStaffID(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	return nil, nil
}

func (m *mockGroupSupervisorRepository) ListActiveSupervisedRooms(ctx context.Context) ([]active.StaffRoomSupervision, error) {
	return nil, nil
}

func (m *mockGroupSupervisorRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*active.GroupSupervisor, error) {
	if m.findByActiveGroupIDFunc != nil {
		return m.findByActiveGroupIDFunc(ctx, activeGroupID, activeOnly)
	}
	return []*active.GroupSupervisor{}, nil
}

func (m *mockGroupSupervisorRepository) FindByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, activeOnly bool) ([]*active.GroupSupervisor, error) {
	return nil, nil
}

func (m *mockGroupSupervisorRepository) EndSupervision(ctx context.Context, id int64) error {
	if m.endSupervisionFunc != nil {
		return m.endSupervisionFunc(ctx, id)
	}
	return nil
}

func (m *mockGroupSupervisorRepository) GetStaffIDsWithSupervisionToday(ctx context.Context) ([]int64, error) {
	return nil, nil
}

func (m *mockGroupSupervisorRepository) EndAllActiveByStaffID(ctx context.Context, staffID int64) (int, error) {
	return 0, nil
}

func (m *mockGroupSupervisorRepository) CreateBulk(ctx context.Context, supervisors []*active.GroupSupervisor) error {
	if m.createBulkFunc != nil {
		return m.createBulkFunc(ctx, supervisors)
	}
	return nil
}

func (m *mockGroupSupervisorRepository) EndSupervisionsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) (int64, error) {
	if m.endSupervisionsByIDsFunc != nil {
		return m.endSupervisionsByIDsFunc(ctx, activeGroupIDs)
	}
	return 0, nil
}

func (m *mockGroupSupervisorRepository) EndByActiveGroupAndStaffID(ctx context.Context, activeGroupID, staffID int64) (int, error) {
	return 0, nil
}

func (m *mockGroupSupervisorRepository) FindAllActive(ctx context.Context) ([]*active.GroupSupervisor, error) {
	if m.findAllActiveFunc != nil {
		return m.findAllActiveFunc(ctx)
	}
	return nil, nil
}

func TestEndActivitySessionLocksGroupBeforeEnding(t *testing.T) {
	t.Parallel()

	locked := false
	groupRepo := &mockGroupRepository{
		findByIDForUpdateFunc: func(context.Context, int64) (*active.Group, error) {
			locked = true
			return &active.Group{Model: base.Model{ID: 1}}, nil
		},
		endSessionFunc: func(context.Context, int64) error {
			require.True(t, locked)
			return nil
		},
	}
	visitRepo := &mockVisitRepository{findByActiveGroupIDFunc: func(context.Context, int64) ([]*active.Visit, error) {
		return []*active.Visit{}, nil
	}}
	supervisorRepo := &mockGroupSupervisorRepository{findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*active.GroupSupervisor, error) {
		return []*active.GroupSupervisor{}, nil
	}}
	svc := &service{ServiceDependencies: ServiceDependencies{
		GroupRepo: groupRepo, VisitRepo: visitRepo, SupervisorRepo: supervisorRepo,
	}}

	require.NoError(t, svc.EndActivitySession(context.Background(), 1))
	require.True(t, locked)
}

func TestProcessSessionTimeoutLocksGroupBeforeEnding(t *testing.T) {
	t.Parallel()
	locked := false
	groupRepo := &mockGroupRepository{
		findByIDForUpdateFunc: func(context.Context, int64) (*active.Group, error) {
			locked = true
			return &active.Group{Model: base.Model{ID: 1}}, nil
		},
		endSessionFunc: func(context.Context, int64) error {
			require.True(t, locked)
			return nil
		},
	}
	svc := &service{ServiceDependencies: ServiceDependencies{
		GroupRepo: groupRepo,
		VisitRepo: &mockVisitRepository{findByActiveGroupIDFunc: func(context.Context, int64) ([]*active.Visit, error) {
			return []*active.Visit{}, nil
		}},
		SupervisorRepo: &mockGroupSupervisorRepository{},
	}}

	_, err := svc.ProcessSessionTimeoutByID(context.Background(), 1)
	require.NoError(t, err)
	require.True(t, locked)
}

func TestEndSupervisionLocksGroupBeforeRelease(t *testing.T) {
	t.Parallel()
	order := make([]string, 0, 3)
	lookups := 0
	supervisors := &mockGroupSupervisorRepository{
		findByIDFunc: func(context.Context, interface{}) (*active.GroupSupervisor, error) {
			lookups++
			order = append(order, "supervision")
			return &active.GroupSupervisor{Model: base.Model{ID: 2}, GroupID: 1}, nil
		},
		endSupervisionFunc: func(context.Context, int64) error {
			order = append(order, "end")
			return nil
		},
	}
	svc := &service{ServiceDependencies: ServiceDependencies{
		GroupRepo: &mockGroupRepository{findByIDForUpdateFunc: func(context.Context, int64) (*active.Group, error) {
			order = append(order, "group-lock")
			return &active.Group{Model: base.Model{ID: 1}}, nil
		}},
		SupervisorRepo: supervisors,
	}}

	require.NoError(t, svc.EndSupervision(context.Background(), 2))
	require.Equal(t, 2, lookups)
	require.Equal(t, []string{"supervision", "group-lock", "supervision", "end"}, order)
}

func TestEndActivitySessionDoesNotBroadcastWhenCommitFails(t *testing.T) {
	t.Parallel()
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = mockDB.Close() })
	db := bun.NewDB(mockDB, pgdialect.New())
	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(errors.New("commit failed"))

	groupRepo := &mockGroupRepository{
		findByIDForUpdateFunc: func(context.Context, int64) (*active.Group, error) {
			return &active.Group{Model: base.Model{ID: 1}}, nil
		},
	}
	visitRepo := &mockVisitRepository{findByActiveGroupIDFunc: func(context.Context, int64) ([]*active.Visit, error) {
		return []*active.Visit{}, nil
	}}
	supervisorRepo := &mockGroupSupervisorRepository{findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*active.GroupSupervisor, error) {
		return []*active.GroupSupervisor{}, nil
	}}
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := &service{ServiceDependencies: ServiceDependencies{
		DB: db, GroupRepo: groupRepo, VisitRepo: visitRepo,
		SupervisorRepo: supervisorRepo, Broadcaster: broadcaster,
	}}

	require.ErrorContains(t, svc.EndActivitySession(context.Background(), 1), "commit failed")
	require.Empty(t, broadcaster.Events())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEndActiveGroupSessionDoesNotBroadcastWhenOuterCommitFails(t *testing.T) {
	t.Parallel()
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = mockDB.Close() })
	db := bun.NewDB(mockDB, pgdialect.New())
	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(errors.New("outer commit failed"))
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := &service{ServiceDependencies: ServiceDependencies{
		DB: db,
		GroupRepo: &mockGroupRepository{findByIDForUpdateFunc: func(context.Context, int64) (*active.Group, error) {
			return &active.Group{Model: base.Model{ID: 1}}, nil
		}},
		VisitRepo: &mockVisitRepository{findByActiveGroupIDFunc: func(context.Context, int64) ([]*active.Visit, error) {
			return []*active.Visit{}, nil
		}},
		SupervisorRepo: &mockGroupSupervisorRepository{findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*active.GroupSupervisor, error) {
			return []*active.GroupSupervisor{}, nil
		}},
		Broadcaster: broadcaster,
	}}

	require.ErrorContains(t, svc.EndActiveGroupSession(context.Background(), 1), "outer commit failed")
	require.Empty(t, broadcaster.Events())
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestEndActivitySession_FindByActiveGroupIDError tests the error path when finding supervisors fails.
// This covers the error path when supervisorRepo.FindByActiveGroupID returns an error.
// The handler layer now owns the transaction via WithTenantTx; the service no longer wraps with RunInTx.
func TestEndActivitySession_FindByActiveGroupIDError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create a mock DB using sqlmock (no transaction expectations needed --
	// the handler layer manages the transaction, not the service)
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = mockDB.Close() }()

	// Create mock repositories
	groupRepo := &mockGroupRepository{
		findByIDFunc: func(ctx context.Context, id interface{}) (*active.Group, error) {
			// Return an active group (EndTime is nil)
			return &active.Group{
				Model: base.Model{ID: 1},
			}, nil
		},
	}

	visitRepo := &mockVisitRepository{
		findByActiveGroupIDFunc: func(ctx context.Context, activeGroupID int64) ([]*active.Visit, error) {
			// Return empty visits (no visits to process)
			return []*active.Visit{}, nil
		},
	}

	// Configure supervisor repository to return error
	mockError := errors.New("mock supervisor lookup error")
	supervisorRepo := &mockGroupSupervisorRepository{
		findByActiveGroupIDFunc: func(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*active.GroupSupervisor, error) {
			return nil, mockError
		},
	}

	// Create service with mocks
	svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: groupRepo, VisitRepo: visitRepo, SupervisorRepo: supervisorRepo, Broadcaster: nil}}

	// ACT
	err = svc.EndActivitySession(ctx, 1)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mock supervisor lookup error")
	assert.Contains(t, err.Error(), "EndActivitySession")

	// Verify all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestAssignMultipleSupervisorsNonCritical_PreservesBestEffortAssignments(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var createdStaffIDs []int64
	supervisorRepo := &mockGroupSupervisorRepository{
		createFunc: func(_ context.Context, entity *active.GroupSupervisor) error {
			if entity.StaffID == int64(22) {
				return errors.New("insert failed")
			}
			createdStaffIDs = append(createdStaffIDs, entity.StaffID)
			return nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{SupervisorRepo: supervisorRepo}}

	svc.assignMultipleSupervisorsNonCritical(ctx, 77, []int64{11, 22, 33, 11}, time.Now())

	assert.ElementsMatch(t, []int64{11, 33}, createdStaffIDs)
}

// TestEndActivitySession_EndSupervisionError tests the error path when ending supervision fails.
// This covers the error path when supervisorRepo.EndSupervision returns an error.
// The handler layer now owns the transaction via WithTenantTx; the service no longer wraps with RunInTx.
func TestEndActivitySession_EndSupervisionError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create a mock DB using sqlmock (no transaction expectations needed --
	// the handler layer manages the transaction, not the service)
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = mockDB.Close() }()

	// Create mock repositories
	groupRepo := &mockGroupRepository{
		findByIDFunc: func(ctx context.Context, id interface{}) (*active.Group, error) {
			// Return an active group
			return &active.Group{
				Model: base.Model{ID: 1},
			}, nil
		},
		endSessionFunc: func(ctx context.Context, id int64) error {
			return nil // Should not be reached
		},
	}

	visitRepo := &mockVisitRepository{
		findByActiveGroupIDFunc: func(ctx context.Context, activeGroupID int64) ([]*active.Visit, error) {
			// Return empty visits
			return []*active.Visit{}, nil
		},
		endVisitFunc: func(ctx context.Context, id int64) error {
			return nil // Should not be reached
		},
	}

	// Configure supervisor repository to return a supervisor, then error when ending it
	mockError := errors.New("mock error")
	supervisorRepo := &mockGroupSupervisorRepository{
		findByActiveGroupIDFunc: func(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*active.GroupSupervisor, error) {
			// Return one supervisor
			return []*active.GroupSupervisor{
				{Model: base.Model{ID: 1}},
			}, nil
		},
		endSupervisionFunc: func(ctx context.Context, id int64) error {
			// Error when trying to end supervision
			return mockError
		},
	}

	// Create service with mocks
	svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: groupRepo, VisitRepo: visitRepo, SupervisorRepo: supervisorRepo, Broadcaster: nil}}

	// ACT
	err = svc.EndActivitySession(ctx, 1)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mock error")
	assert.Contains(t, err.Error(), "EndActivitySession")

	// Verify all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

// Stub for the issue #585 refactor interface addition — unused here.
func (m *mockGroupRepository) CountWithOptions(context.Context, *base.QueryOptions) (int, error) {
	return 0, nil
}

// Stubs for the issue #585 refactor interface additions — unused here.
func (m *mockVisitRepository) GetCurrentRoomNamesForStudents(context.Context, []int64) (map[int64]string, error) {
	return nil, nil
}

func (m *mockGroupSupervisorRepository) ListActiveSupervisionBlockers(context.Context, int64, int64) ([]userModels.BlockerSupervision, error) {
	return nil, nil
}

func (m *mockVisitRepository) OldestExpiredVisitDate(context.Context) (*time.Time, error) {
	return nil, nil
}

func (m *mockVisitRepository) ExpiredVisitMonthlyCounts(context.Context) (map[string]int64, error) {
	return nil, nil
}

func (m *mockGroupSupervisorRepository) FindStaleOpen(ctx context.Context, before timezone.Date) ([]*active.GroupSupervisor, error) {
	if m.findStaleOpenFunc != nil {
		return m.findStaleOpenFunc(ctx, before)
	}
	return nil, nil
}

func (m *mockGroupSupervisorRepository) UpdateColumns(ctx context.Context, supervisor *active.GroupSupervisor, columns ...string) (int64, error) {
	if m.updateColumnsFunc != nil {
		return m.updateColumnsFunc(ctx, supervisor, columns...)
	}
	return 0, nil
}
