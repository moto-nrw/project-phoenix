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
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func (m *mockVisitRepository) FindVisit(ctx context.Context, id int64) (*studentpresence.Visit, error) {
	row, err := m.FindByID(ctx, id)
	if base.IsNoRows(err) {
		return nil, studentpresence.ErrVisitNotFound
	}
	if err != nil || row == nil {
		return nil, err
	}
	value := *row
	return &value, nil
}

func (m *mockVisitRepository) ReviseVisit(ctx context.Context, row studentpresence.Visit) (studentpresence.Visit, error) {
	return row, m.Update(ctx, &row)
}

func (m *mockVisitRepository) CloseVisits(ctx context.Context, ids []int64, at time.Time) ([]studentpresence.Visit, error) {
	rows := make([]studentpresence.Visit, 0, len(ids))
	for _, id := range ids {
		if err := m.EndVisit(ctx, id); err != nil {
			return nil, err
		}
		row, err := m.FindVisit(ctx, id)
		if err != nil {
			return nil, err
		}
		if row != nil {
			if row.ExitTime == nil {
				row.ExitTime = &at
			}
			rows = append(rows, *row)
		}
	}
	return rows, nil
}

func (m *mockVisitRepository) LockStudentAttendance(context.Context, int64) error { return nil }

func (m *mockVisitRepository) ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error) {
	return nil, nil
}

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

func (m *mockGroupRepository) LockRoomSessionWrites(context.Context, int64) error {
	return nil
}

func (m *mockGroupRepository) ListRoomOccupancy(context.Context, []int64) ([]active.RoomOccupancy, error) {
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

func (m *mockGroupRepository) AggregateRoomSessions(ctx context.Context, roomID int64, start, end time.Time, supervisorStaffID *int64) ([]*active.RoomSessionAggregate, error) {
	return nil, nil
}

// mockVisitRepository supplies presence operations for service failure injection.
type mockVisitRepository struct {
	StudentPresence

	findByActiveGroupIDFunc               func(ctx context.Context, activeGroupID int64) ([]*studentpresence.Visit, error)
	findByIDFunc                          func(ctx context.Context, id interface{}) (*studentpresence.Visit, error)
	updateFunc                            func(ctx context.Context, entity *studentpresence.Visit) error
	endVisitFunc                          func(ctx context.Context, id int64) error
	getCurrentByStudentIDFunc             func(ctx context.Context, studentID int64) (*studentpresence.Visit, error)
	getCurrentByStudentIDWithRoomFunc     func(ctx context.Context, studentID int64) (*studentpresence.VisitLocation, error)
	countActiveByRoomIDFunc               func(ctx context.Context, roomID int64) (int, error)
	countActiveByGroupIDFunc              func(ctx context.Context, activeGroupID int64) (int, error)
	listActiveStudentIDsByRoomIDFunc      func(ctx context.Context, roomID int64) ([]int64, error)
	getTodayVisitNamesFunc                func(ctx context.Context, studentIDs []int64) ([]visitGroupNames, error)
	endVisitsByActiveGroupIDsFunc         func(ctx context.Context, activeGroupIDs []int64) (int64, error)
	endGroupSessionFunc                   func(ctx context.Context, activeGroupID int64, at time.Time) (studentpresence.EndedGroupSession, error)
	endGroupSessionsFunc                  func(ctx context.Context, activeGroupIDs []int64, at time.Time) (studentpresence.EndedGroupSessions, error)
	endVisitsByIDsFunc                    func(ctx context.Context, ids []int64, at time.Time) ([]*studentpresence.Visit, error)
	transferVisitsFromRecentSessionsFunc  func(ctx context.Context, newActiveGroupID, deviceID int64) (int, error)
	transferActiveVisitsBetweenGroupsFunc func(ctx context.Context, oldActiveGroupID, newActiveGroupID int64) (int, error)
}

func (m *mockVisitRepository) Create(ctx context.Context, entity *studentpresence.Visit) error {
	return nil
}

func (m *mockVisitRepository) FindByID(ctx context.Context, id interface{}) (*studentpresence.Visit, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockVisitRepository) Update(ctx context.Context, entity *studentpresence.Visit) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, entity)
	}
	return nil
}

func (m *mockVisitRepository) Delete(ctx context.Context, id interface{}) error {
	return nil
}

func (m *mockVisitRepository) List(ctx context.Context, options *base.QueryOptions) ([]*studentpresence.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) FindActiveByStudentID(ctx context.Context, studentID int64) ([]*studentpresence.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*studentpresence.Visit, error) {
	if m.findByActiveGroupIDFunc != nil {
		return m.findByActiveGroupIDFunc(ctx, activeGroupID)
	}
	return []*studentpresence.Visit{}, nil
}

func (m *mockVisitRepository) FindByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) ([]*studentpresence.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*studentpresence.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) FindActiveWithStudentDisplayByGroup(_ context.Context, _ int64) ([]*VisitWithStudentDisplay, error) {
	return nil, nil
}

func (m *mockVisitRepository) FindByStudentAndTimeRange(ctx context.Context, studentID int64, start, end time.Time) ([]*studentpresence.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) FindByStudentAndActiveGroupIDs(ctx context.Context, studentID int64, activeGroupIDs []int64) ([]*studentpresence.Visit, error) {
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

func (m *mockVisitRepository) GetCurrentByStudentID(ctx context.Context, studentID int64) (*studentpresence.Visit, error) {
	if m.getCurrentByStudentIDFunc != nil {
		return m.getCurrentByStudentIDFunc(ctx, studentID)
	}
	return nil, nil
}

func (m *mockVisitRepository) GetCurrentByStudentIDWithRoom(ctx context.Context, studentID int64) (*studentpresence.VisitLocation, error) {
	if m.getCurrentByStudentIDWithRoomFunc != nil {
		return m.getCurrentByStudentIDWithRoomFunc(ctx, studentID)
	}
	return nil, nil
}

func (m *mockVisitRepository) GetCurrentByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*studentpresence.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) GetCurrentByStudentIDsForUpdate(ctx context.Context, studentIDs []int64) (map[int64]*studentpresence.Visit, error) {
	return m.GetCurrentByStudentIDs(ctx, studentIDs)
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

func (m *mockVisitRepository) FindActiveVisits(ctx context.Context) ([]*studentpresence.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) ListOpenVisitStudentIDsByRoom(ctx context.Context) (map[int64][]int64, error) {
	return nil, nil
}

func (m *mockVisitRepository) EndVisitsByIDs(ctx context.Context, ids []int64, at time.Time) ([]*studentpresence.Visit, error) {
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

func (m *mockVisitRepository) GetTodayVisitNamesForStudents(ctx context.Context, studentIDs []int64) ([]visitGroupNames, error) {
	if m.getTodayVisitNamesFunc != nil {
		return m.getTodayVisitNamesFunc(ctx, studentIDs)
	}
	return nil, nil
}

// mockGroupSupervisorRepository is a minimal mock implementation of active.GroupSupervisorRepository
type mockGroupSupervisorRepository struct {
	findByIDFunc            func(ctx context.Context, id interface{}) (*active.GroupSupervisor, error)
	findByActiveGroupIDFunc func(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*active.GroupSupervisor, error)
	endSupervisionFunc      func(ctx context.Context, id int64) error
	createFunc              func(ctx context.Context, entity *active.GroupSupervisor) error
	createBulkFunc          func(ctx context.Context, supervisors []*active.GroupSupervisor) error
	findAllActiveFunc       func(ctx context.Context) ([]*active.GroupSupervisor, error)
	updateFunc              func(ctx context.Context, entity *active.GroupSupervisor) error
	findStaleOpenFunc       func(ctx context.Context, before timezone.Date) ([]*active.GroupSupervisor, error)
	updateColumnsFunc       func(ctx context.Context, supervisor *active.GroupSupervisor, columns ...string) (int64, error)
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

func (m *mockGroupSupervisorRepository) FindActiveByStaffIDForUpdate(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	return m.FindActiveByStaffID(ctx, staffID)
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

func (m *mockGroupSupervisorRepository) FindByActiveGroupIDForUpdate(ctx context.Context, activeGroupID int64) ([]*active.GroupSupervisor, error) {
	return m.FindByActiveGroupID(ctx, activeGroupID, true)
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

func (m *mockGroupSupervisorRepository) EndByActiveGroupAndStaffID(ctx context.Context, activeGroupID, staffID int64) (int, error) {
	return 0, nil
}

func (m *mockGroupSupervisorRepository) FindAllActive(ctx context.Context) ([]*active.GroupSupervisor, error) {
	if m.findAllActiveFunc != nil {
		return m.findAllActiveFunc(ctx)
	}
	return nil, nil
}

// The group lock is taken before the Student Presence owner closes the
// session (#2697): the presence command runs only after FindByIDForUpdate.
func TestEndActivitySessionLocksGroupBeforeEnding(t *testing.T) {
	t.Parallel()

	locked := false
	ended := false
	groupRepo := &mockGroupRepository{
		findByIDForUpdateFunc: func(context.Context, int64) (*active.Group, error) {
			locked = true
			return &active.Group{Model: base.Model{ID: 1}}, nil
		},
		endSessionFunc: func(context.Context, int64) error {
			t.Fatal("the legacy group end must not run: the presence owner ends the session")
			return nil
		},
	}
	visitRepo := &mockVisitRepository{
		findByActiveGroupIDFunc: func(context.Context, int64) ([]*studentpresence.Visit, error) {
			return []*studentpresence.Visit{}, nil
		},
		endGroupSessionFunc: func(_ context.Context, id int64, at time.Time) (studentpresence.EndedGroupSession, error) {
			require.True(t, locked, "the presence close runs under the group lock")
			ended = true
			return studentpresence.EndedGroupSession{GroupID: id, EndedAt: at}, nil
		},
	}
	supervisorRepo := &mockGroupSupervisorRepository{endSupervisionFunc: func(context.Context, int64) error {
		t.Fatal("the legacy supervision end must not run: the presence owner ends the supervisions")
		return nil
	}}
	svc := &service{ServiceDependencies: ServiceDependencies{
		GroupRepo: groupRepo, SchoolPresence: visitRepo, SupervisorRepo: supervisorRepo,
	}}

	require.NoError(t, svc.EndActivitySession(context.Background(), 1))
	require.True(t, locked)
	require.True(t, ended)
}

func TestProcessSessionTimeoutLocksGroupBeforeEnding(t *testing.T) {
	t.Parallel()
	locked := false
	ended := false
	groupRepo := &mockGroupRepository{
		findByIDForUpdateFunc: func(context.Context, int64) (*active.Group, error) {
			locked = true
			return &active.Group{Model: base.Model{ID: 1}}, nil
		},
		endSessionFunc: func(context.Context, int64) error {
			t.Fatal("the legacy group end must not run: the presence owner ends the session")
			return nil
		},
	}
	svc := &service{ServiceDependencies: ServiceDependencies{
		GroupRepo: groupRepo,
		SchoolPresence: &mockVisitRepository{
			findByActiveGroupIDFunc: func(context.Context, int64) ([]*studentpresence.Visit, error) {
				return []*studentpresence.Visit{}, nil
			},
			endGroupSessionFunc: func(_ context.Context, id int64, at time.Time) (studentpresence.EndedGroupSession, error) {
				require.True(t, locked, "the presence close runs under the group lock")
				ended = true
				return studentpresence.EndedGroupSession{GroupID: id, EndedAt: at, ClosedVisits: []studentpresence.Visit{{ID: 5, StudentID: 9, ActiveGroupID: id}}}, nil
			},
		},
		SupervisorRepo: &mockGroupSupervisorRepository{},
	}}

	result, err := svc.ProcessSessionTimeoutByID(context.Background(), 1)
	require.NoError(t, err)
	require.True(t, locked)
	require.True(t, ended)
	require.Equal(t, 1, result.StudentsCheckedOut, "the timeout reports the children the owner checked out")
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
	visitRepo := &mockVisitRepository{findByActiveGroupIDFunc: func(context.Context, int64) ([]*studentpresence.Visit, error) {
		return []*studentpresence.Visit{}, nil
	}}
	supervisorRepo := &mockGroupSupervisorRepository{findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*active.GroupSupervisor, error) {
		return []*active.GroupSupervisor{}, nil
	}}
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := &service{ServiceDependencies: ServiceDependencies{
		DB: db, GroupRepo: groupRepo, SchoolPresence: visitRepo,
		SupervisorRepo: supervisorRepo, Broadcaster: broadcaster,
	}}

	ctx := withSessionTestRuntime(t, context.Background(), db)
	require.ErrorContains(t, svc.EndActivitySession(ctx, 1), "commit failed")
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
		SchoolPresence: &mockVisitRepository{},
		SupervisorRepo: &mockGroupSupervisorRepository{findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*active.GroupSupervisor, error) {
			return []*active.GroupSupervisor{}, nil
		}},
		Broadcaster: broadcaster,
	}}

	ctx := withSessionTestRuntime(t, context.Background(), db)
	require.ErrorContains(t, svc.EndActiveGroupSession(ctx, 1), "outer commit failed")
	require.Empty(t, broadcaster.Events())
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestEndActivitySession_FindByActiveGroupIDError tests the error path when
// the read before the close fails: the visit lookup for the SSE payload
// returns an error, and nothing is closed. The handler layer owns the
// transaction via WithTenantTx; the service no longer wraps with RunInTx.
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

	// The visit lookup that feeds the SSE payload fails before any write.
	mockError := errors.New("mock visit lookup error")
	visitRepo := &mockVisitRepository{
		findByActiveGroupIDFunc: func(ctx context.Context, activeGroupID int64) ([]*studentpresence.Visit, error) {
			return nil, mockError
		},
		endGroupSessionFunc: func(context.Context, int64, time.Time) (studentpresence.EndedGroupSession, error) {
			t.Fatal("a failed read must stop the close before the owner writes")
			return studentpresence.EndedGroupSession{}, nil
		},
	}
	supervisorRepo := &mockGroupSupervisorRepository{}

	// Create service with mocks
	svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: groupRepo, SchoolPresence: visitRepo, SupervisorRepo: supervisorRepo, Broadcaster: nil}}

	// ACT
	err = svc.EndActivitySession(ctx, 1)

	// ASSERT
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDatabaseOperation, "a read failure is reported as a stable database error")
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

// TestEndActivitySession_EndSupervisionError tests the error path when the
// Student Presence owner cannot close the session (its command ends the
// visits, the supervisions, and the group together, #2697). The error keeps
// its identity and the operation name; nothing else is written.
// The handler layer owns the transaction via WithTenantTx; the service no longer wraps with RunInTx.
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

	// The owner's close fails while ending the supervisions.
	mockError := errors.New("mock error")
	visitRepo := &mockVisitRepository{
		findByActiveGroupIDFunc: func(ctx context.Context, activeGroupID int64) ([]*studentpresence.Visit, error) {
			// Return empty visits
			return []*studentpresence.Visit{}, nil
		},
		endGroupSessionFunc: func(context.Context, int64, time.Time) (studentpresence.EndedGroupSession, error) {
			return studentpresence.EndedGroupSession{}, mockError
		},
	}
	supervisorRepo := &mockGroupSupervisorRepository{
		endSupervisionFunc: func(ctx context.Context, id int64) error {
			t.Fatal("the legacy supervision end must not run")
			return nil
		},
	}

	// Create service with mocks
	svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: groupRepo, SchoolPresence: visitRepo, SupervisorRepo: supervisorRepo, Broadcaster: nil}}

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

func (m *mockVisitRepository) CountOpenVisitsInRoom(ctx context.Context, id int64) (int, error) {
	return m.CountActiveByRoomID(ctx, id)
}

func (m *mockVisitRepository) ListOpenVisitRooms(ctx context.Context, roomID int64) ([]studentpresence.OpenVisitRoom, error) {
	ids, err := m.ListActiveStudentIDsByRoomID(ctx, roomID)
	rows := make([]studentpresence.OpenVisitRoom, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, studentpresence.OpenVisitRoom{StudentID: id, RoomID: roomID})
	}
	return rows, err
}
func (m *mockVisitRepository) CountOpenVisitsInGroup(ctx context.Context, id int64) (int, error) {
	return m.CountActiveByGroupID(ctx, id)
}

// EndGroupSession is the owner's session end command. Without a hook it
// closes the group's open visits through the visit hooks, so tests that only
// stub visits keep working, and reports no supervisors.
func (m *mockVisitRepository) EndGroupSession(ctx context.Context, activeGroupID int64, at time.Time) (studentpresence.EndedGroupSession, error) {
	if m.endGroupSessionFunc != nil {
		return m.endGroupSessionFunc(ctx, activeGroupID, at)
	}
	visits, err := m.FindByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		return studentpresence.EndedGroupSession{}, err
	}
	result := studentpresence.EndedGroupSession{GroupID: activeGroupID, EndedAt: at}
	for _, visit := range visits {
		if visit == nil || visit.ExitTime != nil {
			continue
		}
		if err := m.EndVisit(ctx, visit.ID); err != nil {
			return studentpresence.EndedGroupSession{}, err
		}
		closed := *visit
		closed.ExitTime = &at
		result.ClosedVisits = append(result.ClosedVisits, closed)
	}
	return result, nil
}

// EndGroupSessions is the owner's bulk session end. Without a hook it closes
// visits through the bulk visit hook and counts every given group as ended.
func (m *mockVisitRepository) EndGroupSessions(ctx context.Context, activeGroupIDs []int64, at time.Time) (studentpresence.EndedGroupSessions, error) {
	if m.endGroupSessionsFunc != nil {
		return m.endGroupSessionsFunc(ctx, activeGroupIDs, at)
	}
	visits, err := m.EndVisitsByActiveGroupIDs(ctx, activeGroupIDs)
	if err != nil {
		return studentpresence.EndedGroupSessions{}, err
	}
	return studentpresence.EndedGroupSessions{
		VisitsClosed: visits, SessionsEnded: int64(len(activeGroupIDs)), EndedActiveGroupIDs: activeGroupIDs,
	}, nil
}

func (m *mockVisitRepository) TransferOpenVisits(ctx context.Context, from, to int64) (int64, error) {
	count, err := m.TransferActiveVisitsBetweenGroups(ctx, from, to)
	return int64(count), err
}
func (m *mockVisitRepository) TransferRecentDeviceVisits(ctx context.Context, to, deviceID int64) (int64, error) {
	count, err := m.TransferVisitsFromRecentSessions(ctx, to, deviceID)
	return int64(count), err
}

func (m *mockVisitRepository) ListVisitLocations(ctx context.Context, filter studentpresence.VisitLocationFilter) ([]studentpresence.VisitLocation, error) {
	if len(filter.StudentIDs) != 1 || !filter.OpenOnly || filter.Limit != 1 {
		panic("unexpected visit location filter")
	}
	row, err := m.GetCurrentByStudentIDWithRoom(ctx, filter.StudentIDs[0])
	if base.IsNoRows(err) {
		return nil, nil
	}
	if err != nil || row == nil {
		return nil, err
	}
	return []studentpresence.VisitLocation{*row}, nil
}

func (m *mockVisitRepository) ListVisits(ctx context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
	if len(filter.StudentIDs) == 1 && filter.OpenOnly && filter.Limit == 1 {
		row, err := m.GetCurrentByStudentID(ctx, filter.StudentIDs[0])
		if base.IsNoRows(err) {
			return nil, nil
		}
		if err != nil || row == nil {
			return nil, err
		}
		return []studentpresence.Visit{*row}, nil
	}
	if len(filter.ActiveGroupIDs) != 1 {
		panic("unexpected visit filter")
	}
	rows, err := m.FindByActiveGroupID(ctx, filter.ActiveGroupIDs[0])
	result := make([]studentpresence.Visit, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.Visit{
			ID: row.ID, TenantID: row.TenantID, StudentID: row.StudentID,
			ActiveGroupID: row.ActiveGroupID, EntryTime: row.EntryTime, ExitTime: row.ExitTime,
		})
	}
	return result, err
}
