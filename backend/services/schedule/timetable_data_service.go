package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/uptrace/bun"

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// TimetableDataService is the data facade behind the api/timetable handlers
// (issue #584: handlers must not hold repositories). CONTRACT: repository
// results and errors are returned VERBATIM — the handlers keep their
// composition, validation, and error-to-status mapping, so responses stay
// byte-identical. The planner endpoints' assembly logic remains in api/ as
// accepted frozen scope; this facade owns only the data access.
type TimetableDataService interface {
	// Instance students
	GetInstanceStudents(ctx context.Context, instanceID int64) ([]*scheduleModel.InstanceStudent, error)
	GetExpectedInstanceStudentsByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModel.InstanceStudent, error)
	GetStudentInstancesWithAttendance(ctx context.Context, studentID int64, from, to timezone.Date) ([]*scheduleModel.ScheduledInstanceRow, error)
	GetInstanceStudent(ctx context.Context, instanceID, studentID int64) (*scheduleModel.InstanceStudent, error)
	UpdateInstanceStudentAttendance(ctx context.Context, id int64, patch scheduleModel.AttendanceFieldPatch) error

	// Activity instances
	GetActivityInstance(ctx context.Context, id int64) (*scheduleModel.ActivityInstance, error)
	GetActivityInstancesByDateRange(ctx context.Context, from, to timezone.Date) ([]*scheduleModel.ActivityInstance, error)
	GetInstanceByActiveGroupID(ctx context.Context, activeGroupID int64) (*scheduleModel.ActivityInstance, error)
	CreateActivityInstance(ctx context.Context, instance *scheduleModel.ActivityInstance) error
	MarkInstanceCompleted(ctx context.Context, id int64, completedAt time.Time) error

	// DetectInstanceStartConflicts runs the planner's start-conflict checks
	// using this facade's repositories (replaces the api-layer repo handoff).
	DetectInstanceStartConflicts(ctx context.Context, instance *scheduleModel.ActivityInstance, logger *slog.Logger) []InstanceConflictWarning

	// Activity exceptions + template schedules
	GetActivityExceptionsByDateRange(ctx context.Context, from, to timezone.Date) ([]*scheduleModel.ActivityException, error)
	CreateActivitySchedule(ctx context.Context, schedule *activitiesModel.Schedule) error
	GetTemplateStartTimesByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activitiesModel.TemplateStartTime, error)

	// Instance staff
	GetInstanceStaffByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) ([]*scheduleModel.InstanceStaff, error)
	GetInstanceStaff(ctx context.Context, instanceID int64) ([]*scheduleModel.InstanceStaff, error)
	UpdateInstanceStaff(ctx context.Context, staff *scheduleModel.InstanceStaff) error
	CreateInstanceStaff(ctx context.Context, staff *scheduleModel.InstanceStaff) error
	CountNonAbsentInstanceStaffByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error)

	// Active groups + supervisors (substitute / spontaneous start)
	CheckRoomConflict(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *activeModel.Group, error)
	EndGroupSupervisor(ctx context.Context, activeGroupID, staffID int64) (int, error)
	CreateGroupSupervisor(ctx context.Context, supervisor *activeModel.GroupSupervisor) error

	// Arrival / pickup schedules and exceptions
	GetArrivalSchedulesByStudentIDsAndWeekday(ctx context.Context, studentIDs []int64, weekday int) ([]*scheduleModel.StudentArrivalSchedule, error)
	GetArrivalSchedulesByStudent(ctx context.Context, studentID int64) ([]*scheduleModel.StudentArrivalSchedule, error)
	GetArrivalExceptionsByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*scheduleModel.StudentArrivalException, error)
	GetArrivalExceptionsByStudentAndDateRange(ctx context.Context, studentID int64, from, to timezone.Date) ([]*scheduleModel.StudentArrivalException, error)
	GetPickupSchedulesByStudent(ctx context.Context, studentID int64) ([]*scheduleModel.StudentPickupSchedule, error)
	GetPickupExceptionsByStudentAndDateRange(ctx context.Context, studentID int64, from, to timezone.Date) ([]*scheduleModel.StudentPickupException, error)

	// Visits
	GetVisitsByStudentAndActiveGroupIDs(ctx context.Context, studentID int64, activeGroupIDs []int64) ([]*activeModel.Visit, error)

	// Rooms / activity groups / categories (planner lookups)
	GetRoom(ctx context.Context, id int64) (*facilitiesModel.Room, error)
	GetActivityGroup(ctx context.Context, id int64) (*activitiesModel.Group, error)
	GetActivityGroupByName(ctx context.Context, name string) (*activitiesModel.Group, error)
	CreateActivityGroup(ctx context.Context, group *activitiesModel.Group) error
	GetActivityCategoryByName(ctx context.Context, name string) (*activitiesModel.Category, error)
	CreateActivityCategory(ctx context.Context, category *activitiesModel.Category) error

	// Template people + timeframes
	CreateStudentEnrollment(ctx context.Context, enrollment *activitiesModel.StudentEnrollment) error
	CreatePlannedSupervisor(ctx context.Context, supervisor *activitiesModel.SupervisorPlanned) error
	CloseOpenEnrollmentsByGroupAndPeriod(ctx context.Context, groupID int64, calendarPeriodID *int64, validFrom timezone.Date) error
	CloseOpenSupervisorsByGroupAndPeriod(ctx context.Context, groupID int64, calendarPeriodID *int64, validFrom timezone.Date) error
	GetTimeframesByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*scheduleModel.Timeframe, error)
	CreateTimeframe(ctx context.Context, timeframe *scheduleModel.Timeframe) error

	// Templates (raw aggregation queries live in the activities repositories)
	ListTemplateRows(ctx context.Context, templateID *int64) ([]activitiesModel.TemplateListRow, error)
	ListTemplateRowsForPeriod(ctx context.Context, periodID *int64) ([]activitiesModel.TemplateListRow, error)
	UpdateTemplateFields(ctx context.Context, id int64, name, groupType string, categoryID, roomID int64, educationGroupID *int64, maxParticipants int) (int64, error)
	ArchiveTemplate(ctx context.Context, id int64) (int64, error)
	DeleteSchedulesByGroupID(ctx context.Context, groupID int64) error
	EducationGroupExists(ctx context.Context, id int64) (bool, error)

	// Spontaneous-start advisory locks (transaction-scoped; the underlying
	// ExecContext lives in repositories/base per Rule 11).
	LockSpontaneousStartRoom(ctx context.Context, roomID int64) error
	LockSpontaneousActivityName(ctx context.Context, name string) error
	LockSpontaneousActivityCategory(ctx context.Context) error
}

// TimetableDataDependencies wires the repositories behind TimetableDataService.
type TimetableDataDependencies struct {
	InstanceStudentRepo    scheduleModel.InstanceStudentRepository
	ActivityInstanceRepo   scheduleModel.ActivityInstanceRepository
	ActivityExceptionRepo  scheduleModel.ActivityExceptionRepository
	ActivityScheduleRepo   activitiesModel.ScheduleRepository
	InstanceStaffRepo      scheduleModel.InstanceStaffRepository
	ActiveGroupRepo        activeModel.GroupRepository
	SupervisorRepo         activeModel.GroupSupervisorRepository
	ArrivalScheduleRepo    scheduleModel.StudentArrivalScheduleRepository
	ArrivalExceptionRepo   scheduleModel.StudentArrivalExceptionRepository
	PickupScheduleRepo     scheduleModel.StudentPickupScheduleRepository
	PickupExceptionRepo    scheduleModel.StudentPickupExceptionRepository
	VisitRepo              activeModel.VisitRepository
	RoomRepo               facilitiesModel.RoomRepository
	ActivityCategoryRepo   activitiesModel.CategoryRepository
	ActivityGroupRepo      activitiesModel.GroupRepository
	ActivitySupervisorRepo activitiesModel.SupervisorPlannedRepository
	StudentEnrollmentRepo  activitiesModel.StudentEnrollmentRepository
	TimeframeRepo          scheduleModel.TimeframeRepository
	EducationGroupRepo     educationModel.GroupRepository
	DB                     *bun.DB
}

type timetableDataService struct {
	deps TimetableDataDependencies
}

// NewTimetableDataService creates the data facade behind api/timetable.
func NewTimetableDataService(deps TimetableDataDependencies) TimetableDataService {
	return &timetableDataService{deps: deps}
}

func (s *timetableDataService) GetInstanceStudents(ctx context.Context, instanceID int64) ([]*scheduleModel.InstanceStudent, error) {
	return s.deps.InstanceStudentRepo.FindByInstanceID(ctx, instanceID)
}

func (s *timetableDataService) GetExpectedInstanceStudentsByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModel.InstanceStudent, error) {
	return s.deps.InstanceStudentRepo.FindExpectedByInstanceIDs(ctx, instanceIDs)
}

func (s *timetableDataService) GetStudentInstancesWithAttendance(ctx context.Context, studentID int64, from, to timezone.Date) ([]*scheduleModel.ScheduledInstanceRow, error) {
	return s.deps.InstanceStudentRepo.FindInstancesWithAttendanceByStudentAndDateRange(ctx, studentID, from, to)
}

func (s *timetableDataService) GetInstanceStudent(ctx context.Context, instanceID, studentID int64) (*scheduleModel.InstanceStudent, error) {
	return s.deps.InstanceStudentRepo.FindByInstanceAndStudent(ctx, instanceID, studentID)
}

func (s *timetableDataService) UpdateInstanceStudentAttendance(ctx context.Context, id int64, patch scheduleModel.AttendanceFieldPatch) error {
	return s.deps.InstanceStudentRepo.UpdateAttendanceFields(ctx, id, patch)
}

func (s *timetableDataService) GetActivityInstance(ctx context.Context, id int64) (*scheduleModel.ActivityInstance, error) {
	return s.deps.ActivityInstanceRepo.FindByID(ctx, id)
}

func (s *timetableDataService) GetActivityInstancesByDateRange(ctx context.Context, from, to timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	return s.deps.ActivityInstanceRepo.FindByTenantAndDateRange(ctx, from, to)
}

func (s *timetableDataService) GetInstanceByActiveGroupID(ctx context.Context, activeGroupID int64) (*scheduleModel.ActivityInstance, error) {
	return s.deps.ActivityInstanceRepo.FindByActiveGroupID(ctx, activeGroupID)
}

func (s *timetableDataService) CreateActivityInstance(ctx context.Context, instance *scheduleModel.ActivityInstance) error {
	return s.deps.ActivityInstanceRepo.Create(ctx, instance)
}

func (s *timetableDataService) MarkInstanceCompleted(ctx context.Context, id int64, completedAt time.Time) error {
	return s.deps.ActivityInstanceRepo.MarkCompleted(ctx, id, completedAt)
}

// DetectInstanceStartConflicts runs the planner's start-conflict checks using
// this facade's repositories.
func (s *timetableDataService) DetectInstanceStartConflicts(ctx context.Context, instance *scheduleModel.ActivityInstance, logger *slog.Logger) []InstanceConflictWarning {
	return DetectStartConflicts(
		ctx,
		ConflictDependencies{
			GroupRepo:         s.deps.ActiveGroupRepo,
			SupervisorRepo:    s.deps.SupervisorRepo,
			VisitRepo:         s.deps.VisitRepo,
			InstanceStaffRepo: s.deps.InstanceStaffRepo,
			InstanceStudents:  s.deps.InstanceStudentRepo,
		},
		instance,
		logger,
	)
}

func (s *timetableDataService) GetActivityExceptionsByDateRange(ctx context.Context, from, to timezone.Date) ([]*scheduleModel.ActivityException, error) {
	return s.deps.ActivityExceptionRepo.FindByDateRange(ctx, from, to)
}

func (s *timetableDataService) CreateActivitySchedule(ctx context.Context, schedule *activitiesModel.Schedule) error {
	return s.deps.ActivityScheduleRepo.Create(ctx, schedule)
}

func (s *timetableDataService) GetTemplateStartTimesByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activitiesModel.TemplateStartTime, error) {
	return s.deps.ActivityScheduleRepo.FindTemplateStartTimesByGroupIDs(ctx, groupIDs)
}

func (s *timetableDataService) GetInstanceStaffByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) ([]*scheduleModel.InstanceStaff, error) {
	return s.deps.InstanceStaffRepo.FindByStaffAndDate(ctx, staffID, date)
}

func (s *timetableDataService) GetInstanceStaff(ctx context.Context, instanceID int64) ([]*scheduleModel.InstanceStaff, error) {
	return s.deps.InstanceStaffRepo.FindByInstanceID(ctx, instanceID)
}

func (s *timetableDataService) UpdateInstanceStaff(ctx context.Context, staff *scheduleModel.InstanceStaff) error {
	return s.deps.InstanceStaffRepo.Update(ctx, staff)
}

func (s *timetableDataService) CreateInstanceStaff(ctx context.Context, staff *scheduleModel.InstanceStaff) error {
	return s.deps.InstanceStaffRepo.Create(ctx, staff)
}

func (s *timetableDataService) CountNonAbsentInstanceStaffByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error) {
	return s.deps.InstanceStaffRepo.CountNonAbsentByInstanceIDs(ctx, instanceIDs)
}

func (s *timetableDataService) CheckRoomConflict(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *activeModel.Group, error) {
	return s.deps.ActiveGroupRepo.CheckRoomConflict(ctx, roomID, excludeGroupID)
}

func (s *timetableDataService) EndGroupSupervisor(ctx context.Context, activeGroupID, staffID int64) (int, error) {
	return s.deps.SupervisorRepo.EndByActiveGroupAndStaffID(ctx, activeGroupID, staffID)
}

func (s *timetableDataService) CreateGroupSupervisor(ctx context.Context, supervisor *activeModel.GroupSupervisor) error {
	return s.deps.SupervisorRepo.Create(ctx, supervisor)
}

func (s *timetableDataService) GetArrivalSchedulesByStudentIDsAndWeekday(ctx context.Context, studentIDs []int64, weekday int) ([]*scheduleModel.StudentArrivalSchedule, error) {
	return s.deps.ArrivalScheduleRepo.FindByStudentIDsAndWeekday(ctx, studentIDs, weekday)
}

func (s *timetableDataService) GetArrivalSchedulesByStudent(ctx context.Context, studentID int64) ([]*scheduleModel.StudentArrivalSchedule, error) {
	return s.deps.ArrivalScheduleRepo.FindByStudentID(ctx, studentID)
}

func (s *timetableDataService) GetArrivalExceptionsByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*scheduleModel.StudentArrivalException, error) {
	return s.deps.ArrivalExceptionRepo.FindByStudentIDsAndDate(ctx, studentIDs, date)
}

func (s *timetableDataService) GetArrivalExceptionsByStudentAndDateRange(ctx context.Context, studentID int64, from, to timezone.Date) ([]*scheduleModel.StudentArrivalException, error) {
	return s.deps.ArrivalExceptionRepo.FindByStudentIDAndDateRange(ctx, studentID, from, to)
}

func (s *timetableDataService) GetPickupSchedulesByStudent(ctx context.Context, studentID int64) ([]*scheduleModel.StudentPickupSchedule, error) {
	return s.deps.PickupScheduleRepo.FindByStudentID(ctx, studentID)
}

func (s *timetableDataService) GetPickupExceptionsByStudentAndDateRange(ctx context.Context, studentID int64, from, to timezone.Date) ([]*scheduleModel.StudentPickupException, error) {
	return s.deps.PickupExceptionRepo.FindByStudentIDAndDateRange(ctx, studentID, from, to)
}

func (s *timetableDataService) GetVisitsByStudentAndActiveGroupIDs(ctx context.Context, studentID int64, activeGroupIDs []int64) ([]*activeModel.Visit, error) {
	return s.deps.VisitRepo.FindByStudentAndActiveGroupIDs(ctx, studentID, activeGroupIDs)
}

func (s *timetableDataService) GetRoom(ctx context.Context, id int64) (*facilitiesModel.Room, error) {
	return s.deps.RoomRepo.FindByID(ctx, id)
}

func (s *timetableDataService) GetActivityGroup(ctx context.Context, id int64) (*activitiesModel.Group, error) {
	return s.deps.ActivityGroupRepo.FindByID(ctx, id)
}

func (s *timetableDataService) GetActivityGroupByName(ctx context.Context, name string) (*activitiesModel.Group, error) {
	return s.deps.ActivityGroupRepo.FindByName(ctx, name)
}

func (s *timetableDataService) CreateActivityGroup(ctx context.Context, group *activitiesModel.Group) error {
	return s.deps.ActivityGroupRepo.Create(ctx, group)
}

func (s *timetableDataService) GetActivityCategoryByName(ctx context.Context, name string) (*activitiesModel.Category, error) {
	return s.deps.ActivityCategoryRepo.FindByName(ctx, name)
}

func (s *timetableDataService) CreateActivityCategory(ctx context.Context, category *activitiesModel.Category) error {
	return s.deps.ActivityCategoryRepo.Create(ctx, category)
}

func (s *timetableDataService) CreateStudentEnrollment(ctx context.Context, enrollment *activitiesModel.StudentEnrollment) error {
	return s.deps.StudentEnrollmentRepo.Create(ctx, enrollment)
}

func (s *timetableDataService) CreatePlannedSupervisor(ctx context.Context, supervisor *activitiesModel.SupervisorPlanned) error {
	return s.deps.ActivitySupervisorRepo.Create(ctx, supervisor)
}

func (s *timetableDataService) CloseOpenEnrollmentsByGroupAndPeriod(ctx context.Context, groupID int64, calendarPeriodID *int64, validFrom timezone.Date) error {
	return s.deps.StudentEnrollmentRepo.CloseOpenByGroupAndPeriod(ctx, groupID, calendarPeriodID, validFrom)
}

func (s *timetableDataService) CloseOpenSupervisorsByGroupAndPeriod(ctx context.Context, groupID int64, calendarPeriodID *int64, validFrom timezone.Date) error {
	return s.deps.ActivitySupervisorRepo.CloseOpenByGroupAndPeriod(ctx, groupID, calendarPeriodID, validFrom)
}

func (s *timetableDataService) GetTimeframesByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*scheduleModel.Timeframe, error) {
	return s.deps.TimeframeRepo.FindByTimeRange(ctx, startTime, endTime)
}

func (s *timetableDataService) CreateTimeframe(ctx context.Context, timeframe *scheduleModel.Timeframe) error {
	return s.deps.TimeframeRepo.Create(ctx, timeframe)
}

func (s *timetableDataService) ListTemplateRows(ctx context.Context, templateID *int64) ([]activitiesModel.TemplateListRow, error) {
	return s.deps.ActivityGroupRepo.ListTemplateRows(ctx, templateID)
}

func (s *timetableDataService) ListTemplateRowsForPeriod(ctx context.Context, periodID *int64) ([]activitiesModel.TemplateListRow, error) {
	return s.deps.ActivityGroupRepo.ListTemplateRowsForPeriod(ctx, periodID)
}

func (s *timetableDataService) UpdateTemplateFields(ctx context.Context, id int64, name, groupType string, categoryID, roomID int64, educationGroupID *int64, maxParticipants int) (int64, error) {
	return s.deps.ActivityGroupRepo.UpdateTemplateFields(ctx, id, name, groupType, categoryID, roomID, educationGroupID, maxParticipants)
}

func (s *timetableDataService) ArchiveTemplate(ctx context.Context, id int64) (int64, error) {
	return s.deps.ActivityGroupRepo.ArchiveTemplate(ctx, id)
}

func (s *timetableDataService) DeleteSchedulesByGroupID(ctx context.Context, groupID int64) error {
	return s.deps.ActivityScheduleRepo.DeleteByGroupID(ctx, groupID)
}

func (s *timetableDataService) EducationGroupExists(ctx context.Context, id int64) (bool, error) {
	return s.deps.EducationGroupRepo.Exists(ctx, id)
}

// acquireSpontaneousLock takes a transaction-scoped advisory lock. Mirrors
// the historical handler helpers: missing tenant or transaction is an error,
// except in unit tests where no DB is wired at all (nil DB short-circuits).
func (s *timetableDataService) acquireSpontaneousLock(ctx context.Context, key string) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("tenant id is required")
	}
	if tx, ok := modelBase.TxFromContext(ctx); !ok || tx == nil {
		if s.deps.DB == nil {
			return nil
		}
		return errors.New("tenant transaction is required")
	}
	return repoBase.AcquireXactLock(ctx, s.deps.DB, key)
}

func (s *timetableDataService) LockSpontaneousStartRoom(ctx context.Context, roomID int64) error {
	tenantID := tenant.FromContext(ctx)
	return s.acquireSpontaneousLock(ctx, fmt.Sprintf("timetable:spontaneous-start-room:%d:%d", tenantID, roomID))
}

func (s *timetableDataService) LockSpontaneousActivityName(ctx context.Context, name string) error {
	tenantID := tenant.FromContext(ctx)
	key := fmt.Sprintf("timetable:spontaneous-activity-name:%d:%s", tenantID, strings.ToLower(strings.TrimSpace(name)))
	return s.acquireSpontaneousLock(ctx, key)
}

func (s *timetableDataService) LockSpontaneousActivityCategory(ctx context.Context) error {
	tenantID := tenant.FromContext(ctx)
	return s.acquireSpontaneousLock(ctx, fmt.Sprintf("timetable:spontaneous-activity-category:%d", tenantID))
}
