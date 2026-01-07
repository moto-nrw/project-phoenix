package mocks

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/mock"
	"github.com/uptrace/bun"
)

// ActiveServiceMock provides a testify mock for active.Service
type ActiveServiceMock struct {
	mock.Mock
}

func NewActiveServiceMock() *ActiveServiceMock {
	return &ActiveServiceMock{}
}

// TransactionalService
func (m *ActiveServiceMock) WithTx(tx bun.Tx) interface{} {
	return m
}

// Most commonly used methods in policy tests
func (m *ActiveServiceMock) GetVisit(ctx context.Context, visitID int64) (*active.Visit, error) {
	args := m.Called(ctx, visitID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.Visit), args.Error(1)
}

func (m *ActiveServiceMock) FindSupervisorsByStaffID(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	args := m.Called(ctx, staffID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.GroupSupervisor), args.Error(1)
}

func (m *ActiveServiceMock) GetStudentCurrentVisit(ctx context.Context, studentID int64) (*active.Visit, error) {
	args := m.Called(ctx, studentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.Visit), args.Error(1)
}

func (m *ActiveServiceMock) GetDashboardAnalytics(ctx context.Context) (*activeSvc.DashboardAnalytics, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*activeSvc.DashboardAnalytics), args.Error(1)
}

// Attendance tracking
func (m *ActiveServiceMock) GetStudentAttendanceStatus(ctx context.Context, studentID int64) (*activeSvc.AttendanceStatus, error) {
	args := m.Called(ctx, studentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*activeSvc.AttendanceStatus), args.Error(1)
}

func (m *ActiveServiceMock) GetStudentsAttendanceStatuses(ctx context.Context, studentIDs []int64) (map[int64]*activeSvc.AttendanceStatus, error) {
	args := m.Called(ctx, studentIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[int64]*activeSvc.AttendanceStatus), args.Error(1)
}

func (m *ActiveServiceMock) ToggleStudentAttendance(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (*activeSvc.AttendanceResult, error) {
	args := m.Called(ctx, studentID, staffID, deviceID, skipAuthCheck)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*activeSvc.AttendanceResult), args.Error(1)
}

func (m *ActiveServiceMock) CheckTeacherStudentAccess(ctx context.Context, teacherID, studentID int64) (bool, error) {
	args := m.Called(ctx, teacherID, studentID)
	return args.Bool(0), args.Error(1)
}

// --- Active Group operations ---

func (m *ActiveServiceMock) GetActiveGroup(ctx context.Context, id int64) (*active.Group, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.Group), args.Error(1)
}

func (m *ActiveServiceMock) CreateActiveGroup(ctx context.Context, group *active.Group) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *ActiveServiceMock) UpdateActiveGroup(ctx context.Context, group *active.Group) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *ActiveServiceMock) DeleteActiveGroup(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *ActiveServiceMock) ListActiveGroups(ctx context.Context, options *base.QueryOptions) ([]*active.Group, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.Group), args.Error(1)
}

func (m *ActiveServiceMock) FindActiveGroupsByRoomID(ctx context.Context, roomID int64) ([]*active.Group, error) {
	args := m.Called(ctx, roomID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.Group), args.Error(1)
}

func (m *ActiveServiceMock) FindActiveGroupsByGroupID(ctx context.Context, groupID int64) ([]*active.Group, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.Group), args.Error(1)
}

func (m *ActiveServiceMock) FindActiveGroupsByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Group, error) {
	args := m.Called(ctx, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.Group), args.Error(1)
}

func (m *ActiveServiceMock) EndActiveGroupSession(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *ActiveServiceMock) GetActiveGroupWithVisits(ctx context.Context, id int64) (*active.Group, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.Group), args.Error(1)
}

func (m *ActiveServiceMock) GetActiveGroupWithSupervisors(ctx context.Context, id int64) (*active.Group, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.Group), args.Error(1)
}

func (m *ActiveServiceMock) GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*active.Group, error) {
	args := m.Called(ctx, groupIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[int64]*active.Group), args.Error(1)
}

// --- Visit operations ---

func (m *ActiveServiceMock) CreateVisit(ctx context.Context, visit *active.Visit) error {
	args := m.Called(ctx, visit)
	return args.Error(0)
}

func (m *ActiveServiceMock) UpdateVisit(ctx context.Context, visit *active.Visit) error {
	args := m.Called(ctx, visit)
	return args.Error(0)
}

func (m *ActiveServiceMock) DeleteVisit(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *ActiveServiceMock) ListVisits(ctx context.Context, options *base.QueryOptions) ([]*active.Visit, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.Visit), args.Error(1)
}

func (m *ActiveServiceMock) FindVisitsByStudentID(ctx context.Context, studentID int64) ([]*active.Visit, error) {
	args := m.Called(ctx, studentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.Visit), args.Error(1)
}

func (m *ActiveServiceMock) FindVisitsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.Visit, error) {
	args := m.Called(ctx, activeGroupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.Visit), args.Error(1)
}

func (m *ActiveServiceMock) FindVisitsByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Visit, error) {
	args := m.Called(ctx, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.Visit), args.Error(1)
}

func (m *ActiveServiceMock) EndVisit(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *ActiveServiceMock) GetStudentsCurrentVisits(ctx context.Context, studentIDs []int64) (map[int64]*active.Visit, error) {
	args := m.Called(ctx, studentIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[int64]*active.Visit), args.Error(1)
}

// --- Group Supervisor operations ---

func (m *ActiveServiceMock) GetGroupSupervisor(ctx context.Context, id int64) (*active.GroupSupervisor, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.GroupSupervisor), args.Error(1)
}

func (m *ActiveServiceMock) CreateGroupSupervisor(ctx context.Context, supervisor *active.GroupSupervisor) error {
	args := m.Called(ctx, supervisor)
	return args.Error(0)
}

func (m *ActiveServiceMock) UpdateGroupSupervisor(ctx context.Context, supervisor *active.GroupSupervisor) error {
	args := m.Called(ctx, supervisor)
	return args.Error(0)
}

func (m *ActiveServiceMock) DeleteGroupSupervisor(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *ActiveServiceMock) ListGroupSupervisors(ctx context.Context, options *base.QueryOptions) ([]*active.GroupSupervisor, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.GroupSupervisor), args.Error(1)
}

func (m *ActiveServiceMock) FindSupervisorsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.GroupSupervisor, error) {
	args := m.Called(ctx, activeGroupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.GroupSupervisor), args.Error(1)
}

func (m *ActiveServiceMock) FindSupervisorsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) ([]*active.GroupSupervisor, error) {
	args := m.Called(ctx, activeGroupIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.GroupSupervisor), args.Error(1)
}

func (m *ActiveServiceMock) EndSupervision(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *ActiveServiceMock) GetStaffActiveSupervisions(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	args := m.Called(ctx, staffID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.GroupSupervisor), args.Error(1)
}

func (m *ActiveServiceMock) UpdateActiveGroupSupervisors(ctx context.Context, activeGroupID int64, supervisorIDs []int64) (*active.Group, error) {
	args := m.Called(ctx, activeGroupID, supervisorIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.Group), args.Error(1)
}

// --- Combined Group operations ---

func (m *ActiveServiceMock) GetCombinedGroup(ctx context.Context, id int64) (*active.CombinedGroup, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.CombinedGroup), args.Error(1)
}

func (m *ActiveServiceMock) CreateCombinedGroup(ctx context.Context, group *active.CombinedGroup) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *ActiveServiceMock) UpdateCombinedGroup(ctx context.Context, group *active.CombinedGroup) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *ActiveServiceMock) DeleteCombinedGroup(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *ActiveServiceMock) ListCombinedGroups(ctx context.Context, options *base.QueryOptions) ([]*active.CombinedGroup, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.CombinedGroup), args.Error(1)
}

func (m *ActiveServiceMock) FindActiveCombinedGroups(ctx context.Context) ([]*active.CombinedGroup, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.CombinedGroup), args.Error(1)
}

func (m *ActiveServiceMock) FindCombinedGroupsByTimeRange(ctx context.Context, start, end time.Time) ([]*active.CombinedGroup, error) {
	args := m.Called(ctx, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.CombinedGroup), args.Error(1)
}

func (m *ActiveServiceMock) EndCombinedGroup(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *ActiveServiceMock) GetCombinedGroupWithGroups(ctx context.Context, id int64) (*active.CombinedGroup, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.CombinedGroup), args.Error(1)
}

// --- Group Mapping operations ---

func (m *ActiveServiceMock) AddGroupToCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error {
	args := m.Called(ctx, combinedGroupID, activeGroupID)
	return args.Error(0)
}

func (m *ActiveServiceMock) RemoveGroupFromCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error {
	args := m.Called(ctx, combinedGroupID, activeGroupID)
	return args.Error(0)
}

func (m *ActiveServiceMock) GetGroupMappingsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.GroupMapping, error) {
	args := m.Called(ctx, activeGroupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.GroupMapping), args.Error(1)
}

func (m *ActiveServiceMock) GetGroupMappingsByCombinedGroupID(ctx context.Context, combinedGroupID int64) ([]*active.GroupMapping, error) {
	args := m.Called(ctx, combinedGroupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.GroupMapping), args.Error(1)
}

// --- Activity Session Management ---

func (m *ActiveServiceMock) StartActivitySession(ctx context.Context, activityID, deviceID, staffID int64, roomID *int64) (*active.Group, error) {
	args := m.Called(ctx, activityID, deviceID, staffID, roomID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.Group), args.Error(1)
}

func (m *ActiveServiceMock) StartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*active.Group, error) {
	args := m.Called(ctx, activityID, deviceID, supervisorIDs, roomID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.Group), args.Error(1)
}

func (m *ActiveServiceMock) CheckActivityConflict(ctx context.Context, activityID, deviceID int64) (*activeSvc.ActivityConflictInfo, error) {
	args := m.Called(ctx, activityID, deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*activeSvc.ActivityConflictInfo), args.Error(1)
}

func (m *ActiveServiceMock) EndActivitySession(ctx context.Context, activeGroupID int64) error {
	args := m.Called(ctx, activeGroupID)
	return args.Error(0)
}

func (m *ActiveServiceMock) ForceStartActivitySession(ctx context.Context, activityID, deviceID, staffID int64, roomID *int64) (*active.Group, error) {
	args := m.Called(ctx, activityID, deviceID, staffID, roomID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.Group), args.Error(1)
}

func (m *ActiveServiceMock) ForceStartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*active.Group, error) {
	args := m.Called(ctx, activityID, deviceID, supervisorIDs, roomID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.Group), args.Error(1)
}

func (m *ActiveServiceMock) GetDeviceCurrentSession(ctx context.Context, deviceID int64) (*active.Group, error) {
	args := m.Called(ctx, deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.Group), args.Error(1)
}

// --- Session timeout operations ---

func (m *ActiveServiceMock) ProcessSessionTimeout(ctx context.Context, deviceID int64) (*activeSvc.TimeoutResult, error) {
	args := m.Called(ctx, deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*activeSvc.TimeoutResult), args.Error(1)
}

func (m *ActiveServiceMock) UpdateSessionActivity(ctx context.Context, activeGroupID int64) error {
	args := m.Called(ctx, activeGroupID)
	return args.Error(0)
}

func (m *ActiveServiceMock) ValidateSessionTimeout(ctx context.Context, deviceID int64, timeoutMinutes int) error {
	args := m.Called(ctx, deviceID, timeoutMinutes)
	return args.Error(0)
}

func (m *ActiveServiceMock) GetSessionTimeoutInfo(ctx context.Context, deviceID int64) (*activeSvc.SessionTimeoutInfo, error) {
	args := m.Called(ctx, deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*activeSvc.SessionTimeoutInfo), args.Error(1)
}

func (m *ActiveServiceMock) CleanupAbandonedSessions(ctx context.Context, olderThan time.Duration) (int, error) {
	args := m.Called(ctx, olderThan)
	return args.Int(0), args.Error(1)
}

// --- Daily session management ---

func (m *ActiveServiceMock) EndDailySessions(ctx context.Context) (*activeSvc.DailySessionCleanupResult, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*activeSvc.DailySessionCleanupResult), args.Error(1)
}

// --- Analytics and statistics ---

func (m *ActiveServiceMock) GetActiveGroupsCount(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *ActiveServiceMock) GetTotalVisitsCount(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *ActiveServiceMock) GetActiveVisitsCount(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *ActiveServiceMock) GetRoomUtilization(ctx context.Context, roomID int64) (float64, error) {
	args := m.Called(ctx, roomID)
	return args.Get(0).(float64), args.Error(1)
}

func (m *ActiveServiceMock) GetStudentAttendanceRate(ctx context.Context, studentID int64) (float64, error) {
	args := m.Called(ctx, studentID)
	return args.Get(0).(float64), args.Error(1)
}

// --- Unclaimed groups management ---

func (m *ActiveServiceMock) GetUnclaimedActiveGroups(ctx context.Context) ([]*active.Group, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.Group), args.Error(1)
}

func (m *ActiveServiceMock) ClaimActiveGroup(ctx context.Context, groupID, staffID int64, role string) (*active.GroupSupervisor, error) {
	args := m.Called(ctx, groupID, staffID, role)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.GroupSupervisor), args.Error(1)
}
