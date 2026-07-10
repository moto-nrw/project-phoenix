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
	// ValidateCareOfferingSeries rejects archival when a care offering still
	// depends on the template being live. Production always wires it; partial
	// read-only test facades may leave it nil.
	ValidateCareOfferingSeries func(context.Context, int64) error
	DB                         *bun.DB
}

// TimetableDataService is the service boundary behind api/timetable (issue
// #584: handlers must not hold repositories). Most read methods return
// repository results verbatim; multi-repository writes, such as bounded
// template replacement, own their validation and transaction here.
type TimetableDataService struct {
	deps TimetableDataDependencies
}

// NewTimetableDataService creates the data facade behind api/timetable.
func NewTimetableDataService(deps TimetableDataDependencies) *TimetableDataService {
	return &TimetableDataService{deps: deps}
}

func (s *TimetableDataService) GetInstanceStudents(ctx context.Context, instanceID int64) ([]*scheduleModel.InstanceStudent, error) {
	return s.deps.InstanceStudentRepo.FindByInstanceID(ctx, instanceID)
}

func (s *TimetableDataService) GetExpectedInstanceStudentsByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModel.InstanceStudent, error) {
	return s.deps.InstanceStudentRepo.FindExpectedByInstanceIDs(ctx, instanceIDs)
}

func (s *TimetableDataService) GetStudentInstancesWithAttendance(ctx context.Context, studentID int64, from, to timezone.Date) ([]*scheduleModel.ScheduledInstanceRow, error) {
	return s.deps.InstanceStudentRepo.FindInstancesWithAttendanceByStudentAndDateRange(ctx, studentID, from, to)
}

func (s *TimetableDataService) GetInstanceStudent(ctx context.Context, instanceID, studentID int64) (*scheduleModel.InstanceStudent, error) {
	return s.deps.InstanceStudentRepo.FindByInstanceAndStudent(ctx, instanceID, studentID)
}

func (s *TimetableDataService) UpdateInstanceStudentAttendance(ctx context.Context, id int64, patch scheduleModel.AttendanceFieldPatch) error {
	return s.deps.InstanceStudentRepo.UpdateAttendanceFields(ctx, id, patch)
}

func (s *TimetableDataService) GetActivityInstance(ctx context.Context, id int64) (*scheduleModel.ActivityInstance, error) {
	return s.deps.ActivityInstanceRepo.FindByID(ctx, id)
}

func (s *TimetableDataService) GetActivityInstancesByDateRange(ctx context.Context, from, to timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	return s.deps.ActivityInstanceRepo.FindByTenantAndDateRange(ctx, from, to)
}

func (s *TimetableDataService) GetInstanceByActiveGroupID(ctx context.Context, activeGroupID int64) (*scheduleModel.ActivityInstance, error) {
	return s.deps.ActivityInstanceRepo.FindByActiveGroupID(ctx, activeGroupID)
}

func (s *TimetableDataService) CreateActivityInstance(ctx context.Context, instance *scheduleModel.ActivityInstance) error {
	return s.deps.ActivityInstanceRepo.Create(ctx, instance)
}

func (s *TimetableDataService) MarkInstanceCompleted(ctx context.Context, id int64, completedAt time.Time) error {
	return s.deps.ActivityInstanceRepo.MarkCompleted(ctx, id, completedAt)
}

// DetectInstanceStartConflicts runs the planner's start-conflict checks using
// this facade's repositories.
func (s *TimetableDataService) DetectInstanceStartConflicts(ctx context.Context, instance *scheduleModel.ActivityInstance, logger *slog.Logger) []InstanceConflictWarning {
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

func (s *TimetableDataService) GetActivityExceptionsByDateRange(ctx context.Context, from, to timezone.Date) ([]*scheduleModel.ActivityException, error) {
	return s.deps.ActivityExceptionRepo.FindByDateRange(ctx, from, to)
}

func (s *TimetableDataService) CreateActivitySchedule(ctx context.Context, schedule *activitiesModel.Schedule) error {
	return s.deps.ActivityScheduleRepo.Create(ctx, schedule)
}

func (s *TimetableDataService) GetTemplateStartTimesByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activitiesModel.TemplateStartTime, error) {
	return s.deps.ActivityScheduleRepo.FindTemplateStartTimesByGroupIDs(ctx, groupIDs)
}

func (s *TimetableDataService) GetInstanceStaffByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) ([]*scheduleModel.InstanceStaff, error) {
	return s.deps.InstanceStaffRepo.FindByStaffAndDate(ctx, staffID, date)
}

func (s *TimetableDataService) GetInstanceStaff(ctx context.Context, instanceID int64) ([]*scheduleModel.InstanceStaff, error) {
	return s.deps.InstanceStaffRepo.FindByInstanceID(ctx, instanceID)
}

func (s *TimetableDataService) UpdateInstanceStaff(ctx context.Context, staff *scheduleModel.InstanceStaff) error {
	return s.deps.InstanceStaffRepo.Update(ctx, staff)
}

func (s *TimetableDataService) CreateInstanceStaff(ctx context.Context, staff *scheduleModel.InstanceStaff) error {
	return s.deps.InstanceStaffRepo.Create(ctx, staff)
}

func (s *TimetableDataService) CountNonAbsentInstanceStaffByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error) {
	return s.deps.InstanceStaffRepo.CountNonAbsentByInstanceIDs(ctx, instanceIDs)
}

func (s *TimetableDataService) CheckRoomConflict(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *activeModel.Group, error) {
	return s.deps.ActiveGroupRepo.CheckRoomConflict(ctx, roomID, excludeGroupID)
}

func (s *TimetableDataService) EndGroupSupervisor(ctx context.Context, activeGroupID, staffID int64) (int, error) {
	return s.deps.SupervisorRepo.EndByActiveGroupAndStaffID(ctx, activeGroupID, staffID)
}

func (s *TimetableDataService) CreateGroupSupervisor(ctx context.Context, supervisor *activeModel.GroupSupervisor) error {
	return s.deps.SupervisorRepo.Create(ctx, supervisor)
}

func (s *TimetableDataService) GetArrivalSchedulesByStudentIDsAndWeekday(ctx context.Context, studentIDs []int64, weekday int) ([]*scheduleModel.StudentArrivalSchedule, error) {
	return s.deps.ArrivalScheduleRepo.FindByStudentIDsAndWeekday(ctx, studentIDs, weekday)
}

func (s *TimetableDataService) GetArrivalSchedulesByStudent(ctx context.Context, studentID int64) ([]*scheduleModel.StudentArrivalSchedule, error) {
	return s.deps.ArrivalScheduleRepo.FindByStudentID(ctx, studentID)
}

func (s *TimetableDataService) GetArrivalExceptionsByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*scheduleModel.StudentArrivalException, error) {
	return s.deps.ArrivalExceptionRepo.FindByStudentIDsAndDate(ctx, studentIDs, date)
}

func (s *TimetableDataService) GetArrivalExceptionsByStudentAndDateRange(ctx context.Context, studentID int64, from, to timezone.Date) ([]*scheduleModel.StudentArrivalException, error) {
	return s.deps.ArrivalExceptionRepo.FindByStudentIDAndDateRange(ctx, studentID, from, to)
}

func (s *TimetableDataService) GetPickupSchedulesByStudent(ctx context.Context, studentID int64) ([]*scheduleModel.StudentPickupSchedule, error) {
	return s.deps.PickupScheduleRepo.FindByStudentID(ctx, studentID)
}

func (s *TimetableDataService) GetPickupExceptionsByStudentAndDateRange(ctx context.Context, studentID int64, from, to timezone.Date) ([]*scheduleModel.StudentPickupException, error) {
	return s.deps.PickupExceptionRepo.FindByStudentIDAndDateRange(ctx, studentID, from, to)
}

func (s *TimetableDataService) GetVisitsByStudentAndActiveGroupIDs(ctx context.Context, studentID int64, activeGroupIDs []int64) ([]*activeModel.Visit, error) {
	return s.deps.VisitRepo.FindByStudentAndActiveGroupIDs(ctx, studentID, activeGroupIDs)
}

func (s *TimetableDataService) GetRoom(ctx context.Context, id int64) (*facilitiesModel.Room, error) {
	return s.deps.RoomRepo.FindByID(ctx, id)
}

func (s *TimetableDataService) GetActivityGroup(ctx context.Context, id int64) (*activitiesModel.Group, error) {
	return s.deps.ActivityGroupRepo.FindByID(ctx, id)
}

func (s *TimetableDataService) GetActivityGroupByName(ctx context.Context, name string) (*activitiesModel.Group, error) {
	return s.deps.ActivityGroupRepo.FindByName(ctx, name)
}

func (s *TimetableDataService) CreateActivityGroup(ctx context.Context, group *activitiesModel.Group) error {
	return s.deps.ActivityGroupRepo.Create(ctx, group)
}

func (s *TimetableDataService) GetActivityCategoryByName(ctx context.Context, name string) (*activitiesModel.Category, error) {
	return s.deps.ActivityCategoryRepo.FindByName(ctx, name)
}

func (s *TimetableDataService) CreateActivityCategory(ctx context.Context, category *activitiesModel.Category) error {
	return s.deps.ActivityCategoryRepo.Create(ctx, category)
}

func (s *TimetableDataService) CreateStudentEnrollment(ctx context.Context, enrollment *activitiesModel.StudentEnrollment) error {
	return s.deps.StudentEnrollmentRepo.Create(ctx, enrollment)
}

func (s *TimetableDataService) CreatePlannedSupervisor(ctx context.Context, supervisor *activitiesModel.SupervisorPlanned) error {
	return s.deps.ActivitySupervisorRepo.Create(ctx, supervisor)
}

func (s *TimetableDataService) CloseOpenEnrollmentsByGroupAndPeriod(ctx context.Context, groupID int64, calendarPeriodID *int64, validFrom timezone.Date) error {
	return s.deps.StudentEnrollmentRepo.CloseOpenByGroupAndPeriod(ctx, groupID, calendarPeriodID, validFrom)
}

func (s *TimetableDataService) CloseOpenSupervisorsByGroupAndPeriod(ctx context.Context, groupID int64, calendarPeriodID *int64, validFrom timezone.Date) error {
	return s.deps.ActivitySupervisorRepo.CloseOpenByGroupAndPeriod(ctx, groupID, calendarPeriodID, validFrom)
}

func (s *TimetableDataService) GetTimeframesByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*scheduleModel.Timeframe, error) {
	return s.deps.TimeframeRepo.FindByTimeRange(ctx, startTime, endTime)
}

func (s *TimetableDataService) CreateTimeframe(ctx context.Context, timeframe *scheduleModel.Timeframe) error {
	return s.deps.TimeframeRepo.Create(ctx, timeframe)
}

func (s *TimetableDataService) ListTemplateRows(
	ctx context.Context,
	templateID *int64,
	childrenPerStaffRatio int,
) ([]activitiesModel.TemplateListRow, error) {
	rows, err := s.deps.ActivityGroupRepo.ListTemplateRows(ctx, templateID)
	if err != nil {
		return nil, err
	}
	setDisplayRosterCapacity(rows)
	// Detail has no period query parameter, so evaluate every active period.
	// The repository still applies materialization's deterministic period
	// selection for globally-unpinned templates on overlapping dates.
	if err := s.attachWorstTemplateCapacity(ctx, rows, nil, distinctTemplateIDs(rows), childrenPerStaffRatio); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *TimetableDataService) ListTemplateRowsForPeriod(
	ctx context.Context,
	periodID *int64,
	childrenPerStaffRatio int,
) ([]activitiesModel.TemplateListRow, error) {
	rows, err := s.deps.ActivityGroupRepo.ListTemplateRowsForPeriod(ctx, periodID)
	if err != nil {
		return nil, err
	}
	setDisplayRosterCapacity(rows)
	if err := s.attachWorstTemplateCapacity(
		ctx,
		rows,
		periodID,
		distinctTemplateIDs(rows),
		childrenPerStaffRatio,
	); err != nil {
		return nil, err
	}
	return rows, nil
}

func setDisplayRosterCapacity(rows []activitiesModel.TemplateListRow) {
	for i := range rows {
		rows[i].CapacityEnrollmentCount = rows[i].EnrollmentCount
		rows[i].CapacitySupervisorCount = rows[i].SupervisorCount
	}
}

func distinctTemplateIDs(rows []activitiesModel.TemplateListRow) []int64 {
	ids := make([]int64, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		if _, exists := seen[row.TemplateID]; exists {
			continue
		}
		seen[row.TemplateID] = struct{}{}
		ids = append(ids, row.TemplateID)
	}
	return ids
}

func (s *TimetableDataService) attachWorstTemplateCapacity(
	ctx context.Context,
	rows []activitiesModel.TemplateListRow,
	periodID *int64,
	templateIDs []int64,
	childrenPerStaffRatio int,
) error {
	if len(templateIDs) == 0 {
		return nil
	}
	occurrences, err := s.deps.ActivityGroupRepo.ListTemplateCapacityOccurrences(ctx, periodID, templateIDs)
	if err != nil {
		return err
	}
	applyWorstTemplateCapacity(rows, templateIDs, occurrences, childrenPerStaffRatio)
	return nil
}

func applyWorstTemplateCapacity(
	rows []activitiesModel.TemplateListRow,
	templateIDs []int64,
	occurrences []activitiesModel.TemplateCapacityOccurrence,
	childrenPerStaffRatio int,
) {
	included := make(map[int64]struct{}, len(templateIDs))
	for _, templateID := range templateIDs {
		included[templateID] = struct{}{}
	}
	byTemplate := make(map[int64][]activitiesModel.TemplateCapacityOccurrence)
	for _, occurrence := range occurrences {
		if _, ok := included[occurrence.TemplateID]; ok {
			byTemplate[occurrence.TemplateID] = append(byTemplate[occurrence.TemplateID], occurrence)
		}
	}

	worst := make(map[int64]activitiesModel.TemplateCapacityOccurrence, len(byTemplate))
	for templateID, candidates := range byTemplate {
		if candidate, ok := worstTemplateOccurrence(candidates, childrenPerStaffRatio); ok {
			worst[templateID] = candidate
		}
	}
	for i := range rows {
		if _, ok := included[rows[i].TemplateID]; !ok {
			continue
		}
		candidate := worst[rows[i].TemplateID]
		rows[i].CapacityEnrollmentCount = candidate.EnrollmentCount
		rows[i].CapacitySupervisorCount = candidate.SupervisorCount
	}
}

func worstTemplateOccurrence(
	occurrences []activitiesModel.TemplateCapacityOccurrence,
	childrenPerStaffRatio int,
) (activitiesModel.TemplateCapacityOccurrence, bool) {
	hasPositiveDemand := false
	for _, occurrence := range occurrences {
		if RequiredStaffForChildren(occurrence.EnrollmentCount, childrenPerStaffRatio) > 0 {
			hasPositiveDemand = true
			break
		}
	}

	var worst activitiesModel.TemplateCapacityOccurrence
	found := false
	for _, candidate := range occurrences {
		if hasPositiveDemand && candidate.EnrollmentCount == 0 {
			continue
		}
		if !found || templateOccurrenceIsWorse(candidate, worst, childrenPerStaffRatio) {
			worst = candidate
			found = true
		}
	}
	return worst, found
}

type templateCapacityScore struct {
	severity int
	shortage int
	surplus  int
	required int
}

func scoreTemplateOccurrence(
	occurrence activitiesModel.TemplateCapacityOccurrence,
	childrenPerStaffRatio int,
) templateCapacityScore {
	required := RequiredStaffForChildren(occurrence.EnrollmentCount, childrenPerStaffRatio)
	assigned := occurrence.SupervisorCount
	score := templateCapacityScore{required: required}
	if assigned < required {
		score.shortage = required - assigned
		if assigned == 0 {
			score.severity = 2 // danger
		} else {
			score.severity = 1 // warning
		}
	} else {
		score.surplus = assigned - required
	}
	return score
}

func templateOccurrenceIsWorse(
	candidate activitiesModel.TemplateCapacityOccurrence,
	current activitiesModel.TemplateCapacityOccurrence,
	childrenPerStaffRatio int,
) bool {
	candidateScore := scoreTemplateOccurrence(candidate, childrenPerStaffRatio)
	currentScore := scoreTemplateOccurrence(current, childrenPerStaffRatio)
	if candidateScore.severity != currentScore.severity {
		return candidateScore.severity > currentScore.severity
	}
	if candidateScore.shortage != currentScore.shortage {
		return candidateScore.shortage > currentScore.shortage
	}
	if candidateScore.surplus != currentScore.surplus {
		return candidateScore.surplus < currentScore.surplus
	}
	if candidateScore.required != currentScore.required {
		return candidateScore.required > currentScore.required
	}
	return candidate.OccurrenceDate.Before(current.OccurrenceDate)
}

func (s *TimetableDataService) DetectPlannedConflicts(ctx context.Context, query PlannedConflictQuery, logger *slog.Logger) []PlannedConflictWarning {
	return DetectPlannedConflicts(ctx, PlannedConflictDependencies{
		InstanceRepo:      s.deps.ActivityInstanceRepo,
		InstanceStaffRepo: s.deps.InstanceStaffRepo,
		InstanceStudents:  s.deps.InstanceStudentRepo,
	}, query, logger)
}

func (s *TimetableDataService) EducationGroupExists(ctx context.Context, id int64) (bool, error) {
	return s.deps.EducationGroupRepo.Exists(ctx, id)
}

// acquireSpontaneousLock takes a transaction-scoped advisory lock. Mirrors
// the historical handler helpers: missing tenant or transaction is an error,
// except in unit tests where no DB is wired at all (nil DB short-circuits).
func (s *TimetableDataService) acquireSpontaneousLock(ctx context.Context, key string) error {
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

// LockSpontaneousStartRoom takes the spontaneous-start advisory lock for a
// room (transaction-scoped; the underlying ExecContext lives in
// repositories/base per Rule 11).
func (s *TimetableDataService) LockSpontaneousStartRoom(ctx context.Context, roomID int64) error {
	tenantID := tenant.FromContext(ctx)
	return s.acquireSpontaneousLock(ctx, fmt.Sprintf("timetable:spontaneous-start-room:%d:%d", tenantID, roomID))
}

func (s *TimetableDataService) LockSpontaneousActivityName(ctx context.Context, name string) error {
	tenantID := tenant.FromContext(ctx)
	key := fmt.Sprintf("timetable:spontaneous-activity-name:%d:%s", tenantID, strings.ToLower(strings.TrimSpace(name)))
	return s.acquireSpontaneousLock(ctx, key)
}

func (s *TimetableDataService) LockSpontaneousActivityCategory(ctx context.Context) error {
	tenantID := tenant.FromContext(ctx)
	return s.acquireSpontaneousLock(ctx, fmt.Sprintf("timetable:spontaneous-activity-category:%d", tenantID))
}
