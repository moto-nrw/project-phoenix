package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

var (
	ErrTimetableOperationForbidden = errors.New("timetable operation forbidden")
	ErrTimetableOperationNotFound  = errors.New("timetable operation not found")
	ErrTimetableOperationConflict  = errors.New("timetable operation conflict")
)

type OperationSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

type TimetableAttendanceValidationError struct {
	Fields []scheduleModel.AttendancePatchFieldError
}

func (e *TimetableAttendanceValidationError) Error() string {
	return "timetable attendance validation failed"
}

type OperationPersonService interface {
	FindByAccountID(ctx context.Context, accountID int64) (*usersModel.Person, error)
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*usersModel.Person, error)
	GetStaffByPersonID(ctx context.Context, personID int64) (*usersModel.Staff, error)
}

type OperationActiveService interface {
	CreateVisit(ctx context.Context, visit *activeModel.Visit) error
	EndVisit(ctx context.Context, id int64) error
}

type OperationArrivalService interface {
	GetBulkEffectiveArrivalTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*EffectiveArrivalTime, error)
}

type TimetableOperationsService interface {
	PlannedNow(ctx context.Context, accountID int64, isAdmin bool, date timezone.Date, now time.Time, opts PlannedNowOptions) ([]OperationPlannedInstance, error)
	Start(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*StartInstanceResult, error)
	// CreateAndStartSpontaneous creates a spontaneous instance and starts it as
	// one atomic composition in the caller's request transaction.
	CreateAndStartSpontaneous(ctx context.Context, accountID int64, isAdmin bool, in CreateInstanceInput) (*StartInstanceResult, error)
	Complete(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*scheduleModel.ActivityInstance, error)
	Reopen(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*StartInstanceResult, error)
	Roster(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*OperationRoster, error)
	RosterByActiveGroup(ctx context.Context, accountID int64, isAdmin bool, activeGroupID int64) (*OperationRoster, error)
	CheckInStudent(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*OperationRoster, error)
	CheckOutStudent(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*OperationRoster, error)
	PatchAttendance(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64, patch scheduleModel.AttendanceFieldPatch) (*OperationRosterRow, error)
}

type PlannedNowOptions struct {
	HorizonMinutes int
	Limit          int
	IncludeRoster  bool
}

type TimetableOperationsDependencies struct {
	InstanceRepo       scheduleModel.ActivityInstanceRepository
	InstanceStaffRepo  scheduleModel.InstanceStaffRepository
	InstanceStudents   scheduleModel.InstanceStudentRepository
	InstanceService    InstanceService
	ActiveGroupRepo    activeModel.GroupRepository
	ActivityGroupRepo  activitiesModel.GroupRepository
	ActiveService      OperationActiveService
	ArrivalService     OperationArrivalService
	CareDayService     CareDayService
	SupervisorRepo     activeModel.GroupSupervisorRepository
	VisitRepo          activeModel.VisitRepository
	StudentRepo        usersModel.StudentRepository
	EducationGroupRepo educationModel.GroupRepository
	RoomRepo           facilitiesModel.RoomRepository
	PersonService      OperationPersonService
	Settings           OperationSettings
	Broadcaster        realtime.Broadcaster
	DB                 *bun.DB
	Logger             *slog.Logger
	Now                func() time.Time
	RecoveryRepo       scheduleModel.ActivityRecoveryRepository
}

type OperationPlannedInstance struct {
	ID                    int64   `json:"id"`
	Title                 string  `json:"title"`
	Date                  string  `json:"date"`
	StartTime             string  `json:"start_time"`
	EndTime               string  `json:"end_time"`
	RoomID                int64   `json:"room_id"`
	RoomName              *string `json:"room_name,omitempty"`
	Status                string  `json:"status"`
	IsOverdue             bool    `json:"is_overdue"`
	MinutesUntilStart     int     `json:"minutes_until_start"`
	ExpectedStudentsCount int     `json:"expected_students_count"`
	PresentStudentsCount  int     `json:"present_students_count"`
	// NotScheduledCount is how many assigned children are not in care here
	// today (#1747) — not booked on this weekday, or the day was cancelled.
	// They are excluded from ExpectedStudentsCount; this field keeps the
	// reduction visible instead of silently shrinking the number the
	// supervisor knows.
	NotScheduledCount int                       `json:"not_scheduled_students_count"`
	AssignedStaffIDs  []int64                   `json:"assigned_staff_ids"`
	IsAssigned        bool                      `json:"is_assigned"`
	IsPrimary         bool                      `json:"is_primary"`
	IsSubstitute      bool                      `json:"is_substitute"`
	IsAbsent          bool                      `json:"is_absent"`
	RosterPreview     []OperationRosterRow      `json:"roster_preview,omitempty"`
	Warnings          []InstanceConflictWarning `json:"warnings"`
	CanStart          bool                      `json:"can_start"`
	StartAvailableAt  string                    `json:"start_available_at"`
	StartExpiresAt    string                    `json:"start_expires_at"`
}

type OperationRoster struct {
	Instance OperationRosterInstance `json:"instance"`
	Rows     []OperationRosterRow    `json:"rows"`
}

type OperationRosterInstance struct {
	ID                  int64   `json:"id"`
	Title               string  `json:"title"`
	Status              string  `json:"status"`
	IsSpontaneous       bool    `json:"is_spontaneous"`
	ActiveGroupID       *int64  `json:"active_group_id,omitempty"`
	RoomID              int64   `json:"room_id"`
	RoomName            *string `json:"room_name,omitempty"`
	Date                string  `json:"date"`
	StartTime           string  `json:"start_time"`
	EndTime             string  `json:"end_time"`
	CanComplete         bool    `json:"can_complete"`
	CompleteAvailableAt string  `json:"complete_available_at"`
}

type OperationRosterRow struct {
	StudentID        int64                    `json:"student_id"`
	StudentName      string                   `json:"student_name"`
	SchoolClass      string                   `json:"school_class"`
	GroupName        string                   `json:"group_name"`
	Planned          bool                     `json:"planned"`
	IsUnplanned      bool                     `json:"is_unplanned"`
	CurrentlyPresent bool                     `json:"currently_present"`
	VisitID          *int64                   `json:"visit_id,omitempty"`
	Status           string                   `json:"status"`
	Substatus        *string                  `json:"substatus,omitempty"`
	Note             *string                  `json:"note,omitempty"`
	CheckedInAt      *string                  `json:"checked_in_at,omitempty"`
	CheckedOutAt     *string                  `json:"checked_out_at,omitempty"`
	VisitEntryTime   *string                  `json:"visit_entry_time,omitempty"`
	Warnings         []OperationRosterWarning `json:"warnings,omitempty"`
	// CareDayStatus is the care-plan verdict for this child on the instance's
	// date (#1747): "scheduled" | "not_scheduled" | "cancelled" | "unknown".
	// "not_scheduled" (not booked that weekday) and "cancelled" (someone said
	// "kommt heute nicht") both mean the child is not expected — the frontend
	// sets those rows apart and leaves them out of the expected count. The rows
	// stay in the payload so a child who turns up anyway can still be checked
	// in with one tap.
	CareDayStatus CareDayStatus `json:"care_day_status"`
}

type OperationRosterWarning struct {
	Kind                  string  `json:"kind"`
	Message               string  `json:"message"`
	ExpectedArrival       *string `json:"expected_arrival,omitempty"`
	SlotStart             *string `json:"slot_start,omitempty"`
	ExpectedGroupID       *int64  `json:"expected_group_id,omitempty"`
	ExpectedGroupName     *string `json:"expected_group_name,omitempty"`
	CurrentEducationGroup *int64  `json:"current_education_group_id,omitempty"`
}

type timetableOperationsService struct {
	deps TimetableOperationsDependencies
}

func NewTimetableOperationsService(deps TimetableOperationsDependencies) TimetableOperationsService {
	if deps.InstanceRepo == nil || deps.InstanceStaffRepo == nil || deps.InstanceStudents == nil ||
		deps.InstanceService == nil || deps.ActiveGroupRepo == nil || deps.ActivityGroupRepo == nil ||
		deps.ActiveService == nil || deps.ArrivalService == nil || deps.CareDayService == nil || deps.SupervisorRepo == nil ||
		deps.VisitRepo == nil || deps.StudentRepo == nil || deps.EducationGroupRepo == nil || deps.RoomRepo == nil || deps.PersonService == nil || deps.Settings == nil || deps.DB == nil {
		panic("schedule.NewTimetableOperationsService: required dependency is nil")
	}
	return &timetableOperationsService{deps: deps}
}

func (s *timetableOperationsService) PlannedNow(ctx context.Context, accountID int64, isAdmin bool, date timezone.Date, now time.Time, opts PlannedNowOptions) ([]OperationPlannedInstance, error) {
	startLead := 15
	if s.deps.Settings != nil {
		var err error
		startLead, err = s.deps.Settings.ResolveInt(ctx, configModel.KeyTimetableStartLeadMinutes)
		if err != nil {
			return nil, fmt.Errorf("%w: resolve start lead: %v", ErrLifecycleSettings, err)
		}
	}
	staffID, hasStaff, err := s.resolveStaffID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	allOperational := s.adminOverviewEnabled(ctx, isAdmin) || (hasStaff && s.openCareMode(ctx))
	if !hasStaff && !allOperational {
		return nil, ErrTimetableOperationForbidden
	}

	instances, err := s.deps.InstanceRepo.FindByTenantAndDate(ctx, date)
	if err != nil {
		return nil, err
	}
	roomNames, err := s.roomNameMap(ctx)
	if err != nil {
		return nil, err
	}
	// Collect first, resolve care days once, map second. Every instance here
	// falls on the same date, so one care-day resolution covers them all —
	// resolving inside the loop would be a query burst per instance (#1747).
	horizon := opts.HorizonMinutes
	if horizon <= 0 {
		horizon = 15
	}
	if startLead > horizon {
		horizon = startLead
	}
	candidates := make([]plannedNowCandidate, 0, len(instances))
	for _, inst := range instances {
		if inst.Status != scheduleModel.InstanceStatusPlanned || !plannedNowWindow(inst, now, horizon) {
			continue
		}
		roomName := roomNames[inst.RoomID]
		staffRows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, inst.ID)
		if err != nil {
			return nil, err
		}
		if !allOperational && !staffAssigned(staffRows, staffID) {
			continue
		}
		studentRows, err := s.deps.InstanceStudents.FindByInstanceID(ctx, inst.ID)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, plannedNowCandidate{
			instance:    inst,
			staffRows:   staffRows,
			studentRows: studentRows,
			roomName:    roomName,
		})
		if opts.Limit > 0 && len(candidates) >= opts.Limit {
			break
		}
	}

	careDay, err := s.deps.CareDayService.ResolveForDate(ctx, plannedNowStudentIDs(candidates), date)
	if err != nil {
		return nil, err
	}

	out := make([]OperationPlannedInstance, 0, len(candidates))
	for _, candidate := range candidates {
		mapped := mapPlannedInstance(candidate.instance, candidate.staffRows, candidate.studentRows, now, staffID, candidate.roomName, careDay)
		availability := EvaluateLifecycleAvailability(candidate.instance, now, startLead, true)
		mapped.CanStart = availability.CanStart
		mapped.StartAvailableAt = availability.StartAvailableAt.Format(time.RFC3339)
		mapped.StartExpiresAt = availability.CompleteAvailableAt.Format(time.RFC3339)
		if opts.IncludeRoster {
			roster, err := s.buildRosterWithCareDay(ctx, candidate.instance.ID, careDay)
			if err != nil {
				return nil, err
			}
			mapped.RosterPreview = roster.Rows
		}
		out = append(out, mapped)
	}
	return out, nil
}

// plannedNowCandidate is one instance that survived the PlannedNow filters,
// with the rows already loaded for it.
type plannedNowCandidate struct {
	instance    *scheduleModel.ActivityInstance
	staffRows   []*scheduleModel.InstanceStaff
	studentRows []*scheduleModel.InstanceStudent
	roomName    *string
}

func plannedNowStudentIDs(candidates []plannedNowCandidate) []int64 {
	seen := map[int64]bool{}
	ids := make([]int64, 0)
	for _, candidate := range candidates {
		for _, row := range candidate.studentRows {
			if !seen[row.StudentID] {
				seen[row.StudentID] = true
				ids = append(ids, row.StudentID)
			}
		}
	}
	return ids
}

// SpontaneousCreateError wraps a CreateAndStartSpontaneous failure that
// occurred during the Create phase (vs. the Start phase). The handler branches
// on it to pick the create-specific error renderer while the Start phase keeps
// the operations renderer. Unwrap exposes the underlying sentinel for errors.Is.
type SpontaneousCreateError struct{ Err error }

func (e *SpontaneousCreateError) Error() string { return e.Err.Error() }
func (e *SpontaneousCreateError) Unwrap() error { return e.Err }

// CreateAndStartSpontaneous atomically creates a spontaneous instance and
// starts it within the caller's request transaction. Create and Start share
// that transaction, so either half failing with a non-5xx error must roll back
// the rows the other half (and the preceding activity/category resolution)
// already wrote — the middleware only auto-rolls-back on 5xx, so both phases
// mark rollback explicitly. A Create-phase failure is wrapped in
// SpontaneousCreateError so the handler renders it as a create error.
func (s *timetableOperationsService) CreateAndStartSpontaneous(ctx context.Context, accountID int64, isAdmin bool, in CreateInstanceInput) (*StartInstanceResult, error) {
	inst, err := s.deps.InstanceService.Create(ctx, in)
	if err != nil {
		tenant.MarkRollback(ctx)
		return nil, &SpontaneousCreateError{Err: err}
	}
	result, err := s.Start(ctx, accountID, isAdmin, inst.ID)
	if err != nil {
		tenant.MarkRollback(ctx)
		return nil, err
	}
	return result, nil
}

func (s *timetableOperationsService) Start(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*StartInstanceResult, error) {
	staffID, err := s.requireCanOperate(ctx, accountID, isAdmin, instanceID)
	if err != nil {
		return nil, err
	}
	if staffID <= 0 {
		return nil, ErrTimetableOperationForbidden
	}
	return s.deps.InstanceService.Start(ctx, instanceID, staffID)
}

func (s *timetableOperationsService) Complete(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	if _, err := s.requireCanOperate(ctx, accountID, isAdmin, instanceID); err != nil {
		return nil, err
	}
	return s.deps.InstanceService.Complete(WithLifecycleActor(ctx, accountID), instanceID)
}

func (s *timetableOperationsService) Reopen(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*StartInstanceResult, error) {
	if _, err := s.requireCanOperate(ctx, accountID, isAdmin, instanceID); err != nil {
		return nil, err
	}
	return s.deps.InstanceService.Reopen(ctx, instanceID, accountID, isAdmin)
}

func (s *timetableOperationsService) Roster(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*OperationRoster, error) {
	if _, err := s.requireCanOperate(ctx, accountID, isAdmin, instanceID); err != nil {
		return nil, err
	}
	return s.buildRoster(ctx, instanceID)
}

func (s *timetableOperationsService) RosterByActiveGroup(ctx context.Context, accountID int64, isAdmin bool, activeGroupID int64) (*OperationRoster, error) {
	inst, err := s.deps.InstanceRepo.FindByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, ErrTimetableOperationNotFound
	}
	return s.Roster(ctx, accountID, isAdmin, inst.ID)
}

func (s *timetableOperationsService) CheckInStudent(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*OperationRoster, error) {
	staffID, err := s.requireCanOperate(ctx, accountID, isAdmin, instanceID)
	if err != nil {
		return nil, err
	}
	if staffID <= 0 {
		return nil, ErrTimetableOperationForbidden
	}
	inst, err := s.loadInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if inst.Status != scheduleModel.InstanceStatusActive || inst.ActiveGroupID == nil {
		return nil, fmt.Errorf("%w: instance is not active", ErrTimetableOperationConflict)
	}
	current, err := s.deps.VisitRepo.GetCurrentByStudentID(ctx, studentID)
	if err != nil && !modelBase.IsNoRows(err) {
		return nil, err
	}
	if current != nil {
		if current.ActiveGroupID != *inst.ActiveGroupID {
			return nil, fmt.Errorf("%w: student already has active visit", ErrTimetableOperationConflict)
		}
		if err := s.markPlannedStudentPresent(ctx, instanceID, studentID); err != nil {
			return nil, err
		}
		return s.buildRoster(ctx, instanceID)
	}
	now := time.Now()
	visit := &activeModel.Visit{
		StudentID:     studentID,
		ActiveGroupID: *inst.ActiveGroupID,
		EntryTime:     now,
	}
	visit.SetTenantID(tenant.FromContext(ctx))
	staff := &usersModel.Staff{}
	staff.ID = staffID
	visitCtx := context.WithValue(ctx, device.CtxStaff, staff)
	if err := s.deps.ActiveService.CreateVisit(visitCtx, visit); err != nil {
		return nil, err
	}
	if err := s.markPlannedStudentPresent(ctx, instanceID, studentID); err != nil {
		return nil, err
	}
	if err := s.deps.ActiveGroupRepo.UpdateLastActivity(ctx, *inst.ActiveGroupID, now); err != nil {
		s.logger().WarnContext(ctx, "failed to update active group activity after timetable check-in",
			slog.Int64("active_group_id", *inst.ActiveGroupID),
			slog.String("error", err.Error()))
	}
	return s.buildRoster(ctx, instanceID)
}

func (s *timetableOperationsService) markPlannedStudentPresent(ctx context.Context, instanceID, studentID int64) error {
	row, err := s.deps.InstanceStudents.FindByInstanceAndStudent(ctx, instanceID, studentID)
	if err != nil || row == nil {
		return err
	}
	status := scheduleModel.AttendanceStatusPresent
	return s.deps.InstanceStudents.UpdateAttendanceFields(ctx, row.ID, scheduleModel.AttendanceFieldPatch{
		Status:         &status,
		SubstatusClear: true,
		NoteClear:      true,
	})
}

func (s *timetableOperationsService) CheckOutStudent(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*OperationRoster, error) {
	staffID, err := s.requireCanOperate(ctx, accountID, isAdmin, instanceID)
	if err != nil {
		return nil, err
	}
	if staffID <= 0 {
		return nil, ErrTimetableOperationForbidden
	}
	inst, err := s.loadInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if inst.ActiveGroupID == nil {
		return nil, fmt.Errorf("%w: instance has no active group", ErrTimetableOperationConflict)
	}
	visit, err := s.findActiveVisitForInstanceStudent(ctx, *inst.ActiveGroupID, studentID)
	if err != nil {
		return nil, err
	}
	if visit == nil {
		return nil, ErrTimetableOperationNotFound
	}
	staff := &usersModel.Staff{}
	staff.ID = staffID
	visitCtx := context.WithValue(ctx, device.CtxStaff, staff)
	if err := s.deps.ActiveService.EndVisit(visitCtx, visit.ID); err != nil {
		if errors.Is(err, activeSvc.ErrVisitAlreadyEnded) {
			return s.buildRoster(ctx, instanceID)
		}
		return nil, err
	}
	return s.buildRoster(ctx, instanceID)
}

func (s *timetableOperationsService) PatchAttendance(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64, patch scheduleModel.AttendanceFieldPatch) (*OperationRosterRow, error) {
	if _, err := s.requireCanOperate(ctx, accountID, isAdmin, instanceID); err != nil {
		return nil, err
	}
	inst, err := s.loadInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if inst.Status == scheduleModel.InstanceStatusCompleted || inst.Status == scheduleModel.InstanceStatusCancelled {
		return nil, fmt.Errorf("%w: attendance is frozen after completion", ErrTimetableOperationConflict)
	}
	if s.deps.RecoveryRepo != nil {
		if err := s.deps.RecoveryRepo.LockAttendance(ctx, instanceID); err != nil {
			return nil, err
		}
		inst, err = s.loadInstance(ctx, instanceID)
		if err != nil {
			return nil, err
		}
		if inst.Status == scheduleModel.InstanceStatusCompleted || inst.Status == scheduleModel.InstanceStatusCancelled {
			return nil, fmt.Errorf("%w: attendance is frozen after completion", ErrTimetableOperationConflict)
		}
	}
	row, err := s.deps.InstanceStudents.FindByInstanceAndStudent(ctx, instanceID, studentID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, ErrTimetableOperationNotFound
	}
	if verrs := ValidateAttendancePatch(patch, row); len(verrs) > 0 {
		return nil, &TimetableAttendanceValidationError{Fields: verrs}
	}
	if err := s.deps.InstanceStudents.UpdateAttendanceFields(ctx, row.ID, patch); err != nil {
		return nil, err
	}
	s.broadcastAttendanceChanged(ctx, instanceID)
	roster, err := s.buildRoster(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	for i := range roster.Rows {
		if roster.Rows[i].StudentID == studentID {
			return &roster.Rows[i], nil
		}
	}
	return nil, ErrTimetableOperationNotFound
}

func (s *timetableOperationsService) requireCanOperate(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (int64, error) {
	staffID, hasStaff, err := s.resolveStaffID(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if hasStaff && s.openCareMode(ctx) {
		return staffID, nil
	}
	if s.adminOverviewEnabled(ctx, isAdmin) {
		return staffID, nil
	}
	if !hasStaff {
		return 0, ErrTimetableOperationForbidden
	}
	return s.requireFixedGroupOperationAccess(ctx, staffID, instanceID)
}

func (s *timetableOperationsService) requireFixedGroupOperationAccess(ctx context.Context, staffID, instanceID int64) (int64, error) {
	inst, err := s.loadInstance(ctx, instanceID)
	if err != nil {
		return 0, err
	}
	staffRows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, instanceID)
	if err != nil {
		return 0, err
	}
	if staffAssigned(staffRows, staffID) {
		return staffID, nil
	}
	if inst.ActiveGroupID != nil {
		supervisors, err := s.deps.SupervisorRepo.FindByActiveGroupID(ctx, *inst.ActiveGroupID, true)
		if err != nil {
			return 0, err
		}
		for _, sup := range supervisors {
			if sup.StaffID == staffID {
				return staffID, nil
			}
		}
	}
	return 0, ErrTimetableOperationForbidden
}

func (s *timetableOperationsService) openCareMode(ctx context.Context) bool {
	if s.deps.Settings == nil {
		return false
	}
	mode, err := s.deps.Settings.ResolveString(ctx, configModel.KeyGroupMode)
	if err != nil {
		s.logger().ErrorContext(ctx, "failed to resolve operational group mode", slog.String("error", err.Error()))
		return false
	}
	return mode == configModel.GroupModeOpenCare
}

func (s *timetableOperationsService) buildRoster(ctx context.Context, instanceID int64) (*OperationRoster, error) {
	return s.buildRosterWithCareDay(ctx, instanceID, nil)
}

// rosterExcludedAlumni returns the set of student IDs to drop from a
// current/future roster because the student has graduated (alumnus) and is
// therefore soft-deleted from all staff-facing operations. Frozen history —
// a past-dated, completed, or cancelled instance — excludes nobody so its
// recorded attendance stays intact (#405).
func rosterExcludedAlumni(inst *scheduleModel.ActivityInstance, students map[int64]*usersModel.Student) map[int64]bool {
	excluded := map[int64]bool{}
	if inst == nil {
		return excluded
	}
	if inst.Status == scheduleModel.InstanceStatusCompleted || inst.Status == scheduleModel.InstanceStatusCancelled {
		return excluded
	}
	if inst.Date.Before(timezone.TodayDate()) {
		return excluded
	}
	for id, st := range students {
		if st != nil && st.Status == usersModel.StudentStatusAlumnus {
			excluded[id] = true
		}
	}
	return excluded
}

// buildRosterWithCareDay builds the roster, optionally reusing a care-day map
// the caller already resolved. PlannedNow resolves once for every instance of
// the day; passing nil makes this method resolve for itself.
func (s *timetableOperationsService) buildRosterWithCareDay(
	ctx context.Context, instanceID int64, careDay map[int64]CareDayStatus,
) (*OperationRoster, error) {
	inst, err := s.loadInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	plannedRows, err := s.deps.InstanceStudents.FindByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	var visits []*activeModel.Visit
	if inst.ActiveGroupID != nil {
		visits, err = s.deps.VisitRepo.FindByActiveGroupID(ctx, *inst.ActiveGroupID)
		if err != nil {
			return nil, err
		}
	}
	studentIDs := make([]int64, 0, len(plannedRows)+len(visits))
	seen := map[int64]bool{}
	for _, row := range plannedRows {
		if !seen[row.StudentID] {
			seen[row.StudentID] = true
			studentIDs = append(studentIDs, row.StudentID)
		}
	}
	for _, visit := range visits {
		if !seen[visit.StudentID] {
			seen[visit.StudentID] = true
			studentIDs = append(studentIDs, visit.StudentID)
		}
	}
	students, err := s.deps.StudentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	// Graduated (alumnus) students are soft-deleted: their instance_students
	// rows are kept for historical attendance, but they must drop off any
	// roster a supervisor can still act on, so a departed child never appears on
	// an upcoming staff list nor has their attendance patched. Frozen history
	// (past-dated or completed/cancelled instances) keeps every row (#405).
	excludedAlumni := rosterExcludedAlumni(inst, students)
	templateGroup, err := s.loadRosterTemplateGroup(ctx, inst.ActivityGroupID)
	if err != nil {
		return nil, err
	}
	groupIDs := make([]int64, 0, len(students))
	personIDs := make([]int64, 0, len(students))
	for _, st := range students {
		personIDs = append(personIDs, st.PersonID)
		if st.GroupID != nil {
			groupIDs = append(groupIDs, *st.GroupID)
		}
	}
	if templateGroup != nil && templateGroup.EducationGroupID != nil {
		groupIDs = append(groupIDs, *templateGroup.EducationGroupID)
	}
	persons, err := s.deps.PersonService.GetByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	groups, err := s.deps.EducationGroupRepo.FindByIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	latestVisits := map[int64]*activeModel.Visit{}
	for _, visit := range visits {
		if current := latestVisits[visit.StudentID]; current == nil || visit.EntryTime.After(current.EntryTime) {
			latestVisits[visit.StudentID] = visit
		}
	}
	warningsByStudent := s.rosterWarnings(ctx, inst, studentIDs, students, groups, templateGroup)
	if careDay == nil {
		careDay, err = s.deps.CareDayService.ResolveForDate(ctx, studentIDs, inst.Date)
		if err != nil {
			return nil, err
		}
	}
	rows := make([]OperationRosterRow, 0, len(seen))
	for _, planned := range plannedRows {
		if excludedAlumni[planned.StudentID] {
			continue
		}
		rows = append(rows, s.mapRosterRow(inst, planned.StudentID, planned, latestVisits[planned.StudentID], students, persons, groups, warningsByStudent[planned.StudentID], careDay))
	}
	for _, visit := range latestVisits {
		if excludedAlumni[visit.StudentID] {
			continue
		}
		if _, planned := findPlanned(plannedRows, visit.StudentID); planned {
			continue
		}
		rows = append(rows, s.mapRosterRow(inst, visit.StudentID, nil, visit, students, persons, groups, nil, careDay))
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CurrentlyPresent != rows[j].CurrentlyPresent {
			return rows[i].CurrentlyPresent && !rows[j].CurrentlyPresent
		}
		// Children the care plan does not place here today sink below the ones
		// that are actually expected, mirroring the frontend's separate block.
		if expectedI, expectedJ := rows[i].CareDayStatus.Expected(), rows[j].CareDayStatus.Expected(); expectedI != expectedJ {
			return expectedI && !expectedJ
		}
		if rows[i].Planned != rows[j].Planned {
			return rows[i].Planned && !rows[j].Planned
		}
		return rows[i].StudentName < rows[j].StudentName
	})
	roomNames, err := s.roomNameMap(ctx)
	if err != nil {
		return nil, err
	}
	enforcePlannedEnd, err := s.deps.Settings.ResolveBool(ctx, configModel.KeyTimetableEnforcePlannedEnd)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve planned end policy: %v", ErrLifecycleSettings, err)
	}
	now := time.Now()
	if s.deps.Now != nil {
		now = s.deps.Now()
	}
	availability := EvaluateLifecycleAvailability(inst, now, 15, enforcePlannedEnd)
	return &OperationRoster{
		Instance: OperationRosterInstance{
			ID:                  inst.ID,
			Title:               inst.Title,
			Status:              inst.Status,
			IsSpontaneous:       inst.IsSpontaneous,
			ActiveGroupID:       inst.ActiveGroupID,
			RoomID:              inst.RoomID,
			RoomName:            roomNames[inst.RoomID],
			Date:                inst.Date.String(),
			StartTime:           inst.StartTime.Format("15:04"),
			EndTime:             inst.EndTime.Format("15:04"),
			CanComplete:         availability.CanComplete,
			CompleteAvailableAt: availability.CompleteAvailableAt.Format(time.RFC3339),
		},
		Rows: rows,
	}, nil
}

func (s *timetableOperationsService) mapRosterRow(inst *scheduleModel.ActivityInstance, studentID int64, planned *scheduleModel.InstanceStudent, visit *activeModel.Visit, students map[int64]*usersModel.Student, persons map[int64]*usersModel.Person, groups map[int64]*educationModel.Group, warnings []OperationRosterWarning, careDay map[int64]CareDayStatus) OperationRosterRow {
	row := OperationRosterRow{
		StudentID:        studentID,
		Planned:          planned != nil && !planned.IsUnplanned,
		IsUnplanned:      (planned != nil && planned.IsUnplanned) || (planned == nil && visit != nil),
		CurrentlyPresent: visit != nil && visit.ExitTime == nil,
		Status:           scheduleModel.AttendanceStatusPresent,
		Warnings:         warnings,
		CareDayStatus:    rosterCareDayStatus(inst, studentID, planned, visit, careDay),
	}
	applyPlannedRosterAttendance(&row, planned)

	if visit != nil {
		id := visit.ID
		row.VisitID = &id
		v := visit.EntryTime.UTC().Format(time.RFC3339)
		row.VisitEntryTime = &v
	}
	applyRosterStudentIdentity(&row, studentID, students, persons, groups)
	return row
}

// rosterCareDayStatus resolves the care-day verdict shown on one roster row.
//
// A child who is actually here outranks any plan: an attendance record or a
// running visit means the question "should they be here?" is already answered
// by reality, and demoting such a row would hide a present child from the
// supervisor. Walk-ins are never in the resolved map to begin with, and an
// absent entry means unknown — never assume a missing fact excludes a child.
//
// Everything else — the frozen verdict on a completed instance, and the
// status-day-owned absence that is really a non-booking — is decided by the
// shared AttendanceRowCareDay, so this roster, the planner list, and the
// planned-now cards can never disagree about the same child (#1747 review).
func rosterCareDayStatus(
	inst *scheduleModel.ActivityInstance,
	studentID int64,
	planned *scheduleModel.InstanceStudent,
	visit *activeModel.Visit,
	careDay map[int64]CareDayStatus,
) CareDayStatus {
	if visit != nil || (planned != nil && planned.Status == scheduleModel.AttendanceStatusPresent) {
		return CareDayScheduled
	}
	if planned == nil {
		return CareDayUnknown
	}
	return AttendanceRowCareDay(
		inst != nil && inst.Status == scheduleModel.InstanceStatusCompleted,
		planned,
		careDay[studentID],
	)
}

func applyPlannedRosterAttendance(row *OperationRosterRow, planned *scheduleModel.InstanceStudent) {
	if planned != nil {
		row.Status = planned.Status
		row.Substatus = planned.Substatus
		row.Note = planned.Note
		if planned.CheckedInAt != nil {
			v := planned.CheckedInAt.UTC().Format(time.RFC3339)
			row.CheckedInAt = &v
		}
		if planned.CheckedOutAt != nil {
			v := planned.CheckedOutAt.UTC().Format(time.RFC3339)
			row.CheckedOutAt = &v
		}
		if planned.Status == scheduleModel.AttendanceStatusPresent && planned.CheckedInAt != nil && planned.CheckedOutAt == nil {
			row.CurrentlyPresent = true
		}
	}
}

func applyRosterStudentIdentity(row *OperationRosterRow, studentID int64, students map[int64]*usersModel.Student, persons map[int64]*usersModel.Person, groups map[int64]*educationModel.Group) {
	if st := students[studentID]; st != nil {
		row.SchoolClass = st.SchoolClass
		if st.GroupID != nil {
			if group := groups[*st.GroupID]; group != nil {
				row.GroupName = group.Name
			}
		}
		if p := persons[st.PersonID]; p != nil {
			row.StudentName = p.GetFullName()
		}
	}
}

type rosterTemplateGroup struct {
	*activitiesModel.Group
	Targets []*activitiesModel.GroupTarget
}

func (s *timetableOperationsService) loadRosterTemplateGroup(ctx context.Context, activityGroupID *int64) (*rosterTemplateGroup, error) {
	if activityGroupID == nil || *activityGroupID <= 0 {
		return nil, nil
	}
	group, err := s.deps.ActivityGroupRepo.FindByID(ctx, *activityGroupID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	if targetRepo, ok := s.deps.ActivityGroupRepo.(interface {
		FindTargetsByGroupIDs(context.Context, []int64) (map[int64][]*activitiesModel.GroupTarget, error)
	}); ok {
		targetsByGroup, err := targetRepo.FindTargetsByGroupIDs(ctx, []int64{*activityGroupID})
		if err != nil {
			return nil, err
		}
		return &rosterTemplateGroup{Group: group, Targets: targetsByGroup[*activityGroupID]}, nil
	}
	return &rosterTemplateGroup{Group: group}, nil
}

func (s *timetableOperationsService) rosterWarnings(
	ctx context.Context,
	inst *scheduleModel.ActivityInstance,
	studentIDs []int64,
	students map[int64]*usersModel.Student,
	groups map[int64]*educationModel.Group,
	templateGroup *rosterTemplateGroup,
) map[int64][]OperationRosterWarning {
	warnings := make(map[int64][]OperationRosterWarning)
	if len(studentIDs) == 0 {
		return warnings
	}

	arrivals, err := s.deps.ArrivalService.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, inst.Date)
	if err != nil {
		s.logger().WarnContext(
			ctx,
			"could not load arrival times for timetable roster warnings",
			slog.String("error", err.Error()),
			slog.Int64("instance_id", inst.ID),
		)
	} else {
		appendArrivalWarnings(warnings, arrivals, inst)
	}

	if expectedGroupIDs := rosterMismatchExpectedGroupIDs(templateGroup); len(expectedGroupIDs) > 0 {
		var expectedGroupID *int64
		var expectedGroupName *string
		if len(expectedGroupIDs) == 1 {
			for groupID := range expectedGroupIDs {
				expectedGroupID = &groupID
				if group := groups[groupID]; group != nil {
					name := group.Name
					expectedGroupName = &name
				}
			}
		}
		for _, studentID := range studentIDs {
			st := students[studentID]
			if st == nil {
				continue
			}
			if st.GroupID != nil {
				if _, matches := expectedGroupIDs[*st.GroupID]; matches {
					continue
				}
			}
			warnings[studentID] = append(warnings[studentID], OperationRosterWarning{
				Kind:                  "template_class_mismatch",
				Message:               "Kind passt nicht zur Klassengruppe der Betreuungsplan-Vorlage.",
				ExpectedGroupID:       expectedGroupID,
				ExpectedGroupName:     expectedGroupName,
				CurrentEducationGroup: st.GroupID,
			})
		}
	}

	return warnings
}

func rosterMismatchExpectedGroupIDs(group *rosterTemplateGroup) map[int64]struct{} {
	if group == nil {
		return nil
	}
	if len(group.Targets) == 0 {
		if group.EducationGroupID == nil {
			return nil
		}
		return map[int64]struct{}{*group.EducationGroupID: {}}
	}
	expected := make(map[int64]struct{}, len(group.Targets))
	for _, target := range group.Targets {
		if target == nil || target.TargetGroupType != activitiesModel.TargetGroupTypeGruppe || target.EducationGroupID == nil {
			continue
		}
		expected[*target.EducationGroupID] = struct{}{}
	}
	return expected
}

func appendArrivalWarnings(warnings map[int64][]OperationRosterWarning, arrivals map[int64]*EffectiveArrivalTime, inst *scheduleModel.ActivityInstance) {
	slotStart := inst.StartTime.Format("15:04")
	slotStartClock := timezone.WallClock(inst.StartTime)
	for studentID, arrival := range arrivals {
		if arrival == nil {
			continue
		}
		if arrival.ArrivalTime == nil {
			if arrival.IsException {
				continue
			}
			warnings[studentID] = append(warnings[studentID], OperationRosterWarning{
				Kind:      "missing_arrival_schedule",
				Message:   "Für diesen Tag ist keine erwartete Ankunft hinterlegt.",
				SlotStart: &slotStart,
			})
			continue
		}
		arrivalClock := timezone.WallClock(*arrival.ArrivalTime)
		if arrivalClock.After(slotStartClock) {
			expectedArrival := arrival.ArrivalTime.Format("15:04")
			warnings[studentID] = append(warnings[studentID], OperationRosterWarning{
				Kind:            "arrival_after_slot_start",
				Message:         "Erwartete Ankunft liegt nach dem Start dieser Betreuung.",
				ExpectedArrival: &expectedArrival,
				SlotStart:       &slotStart,
			})
		}
	}
}

func (s *timetableOperationsService) resolveStaffID(ctx context.Context, accountID int64) (int64, bool, error) {
	if accountID <= 0 {
		return 0, false, ErrTimetableOperationForbidden
	}
	person, err := s.deps.PersonService.FindByAccountID(ctx, accountID)
	if err != nil {
		if errors.Is(err, usersSvc.ErrPersonNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if person == nil {
		return 0, false, nil
	}
	staff, err := s.deps.PersonService.GetStaffByPersonID(ctx, person.ID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if staff == nil {
		return 0, false, nil
	}
	return staff.ID, true, nil
}

// broadcastAttendanceChanged wakes the tenant's clients after a roster
// attendance patch. Deliberately id-less about the child — see
// realtime.EventActiveSupervisionChanged (#2085). Clients refetch the roster /
// supervision views their own permissions allow.
func (s *timetableOperationsService) broadcastAttendanceChanged(ctx context.Context, instanceID int64) {
	if s.deps.Broadcaster == nil {
		return
	}
	inst, err := s.loadInstance(ctx, instanceID)
	if err != nil {
		s.logger().WarnContext(ctx, "failed to load instance for timetable attendance broadcast",
			slog.Int64("instance_id", instanceID),
			slog.String("error", err.Error()))
		return
	}
	if inst.ActiveGroupID == nil {
		return
	}
	activeGroupID := fmt.Sprintf("%d", *inst.ActiveGroupID)
	instanceIDStr := fmt.Sprintf("%d", instanceID)
	reason := "timetable_attendance_updated"
	event := realtime.NewEvent(realtime.EventActiveSupervisionChanged, activeGroupID, realtime.EventData{
		InstanceID: &instanceIDStr,
		Reason:     &reason,
	})
	tenantID := tenant.FromContext(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		if err := s.deps.Broadcaster.BroadcastToTenant(tenantID, event); err != nil {
			s.logger().WarnContext(ctx, "SSE timetable attendance broadcast failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("active_group_id", activeGroupID),
				slog.String("error", err.Error()))
		}
	})
}

func (s *timetableOperationsService) adminOverviewEnabled(ctx context.Context, isAdmin bool) bool {
	if !isAdmin || s.deps.Settings == nil {
		return false
	}
	enabled, err := s.deps.Settings.ResolveBool(ctx, configModel.KeyAdminSupervisionOverview)
	if err != nil {
		s.logger().WarnContext(ctx, "admin supervision overview setting check failed for timetable operations",
			slog.String("error", err.Error()))
		return false
	}
	return enabled
}

func (s *timetableOperationsService) loadInstance(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	inst, err := s.deps.InstanceRepo.FindByID(ctx, instanceID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, ErrTimetableOperationNotFound
		}
		return nil, err
	}
	if inst == nil {
		return nil, ErrTimetableOperationNotFound
	}
	return inst, nil
}

func (s *timetableOperationsService) findActiveVisitForInstanceStudent(ctx context.Context, activeGroupID, studentID int64) (*activeModel.Visit, error) {
	visits, err := s.deps.VisitRepo.FindByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		return nil, err
	}
	for _, visit := range visits {
		if visit.StudentID == studentID && visit.ExitTime == nil {
			return visit, nil
		}
	}
	return nil, nil
}

func (s *timetableOperationsService) logger() *slog.Logger {
	if s.deps.Logger != nil {
		return s.deps.Logger
	}
	return slog.Default()
}

func plannedNowWindow(inst *scheduleModel.ActivityInstance, now time.Time, horizonMinutes int) bool {
	if horizonMinutes <= 0 {
		horizonMinutes = 15
	}
	start := instanceStartAt(inst, now.Location())
	end := instanceEndAt(inst, now.Location())
	if !now.Before(end) {
		return false
	}
	return (start.After(now.Add(-15*time.Minute)) && start.Before(now.Add(time.Duration(horizonMinutes)*time.Minute))) || start.Before(now)
}

func instanceEndAt(inst *scheduleModel.ActivityInstance, loc *time.Location) time.Time {
	return time.Date(inst.Date.Year, inst.Date.Month, inst.Date.Day, inst.EndTime.Hour(), inst.EndTime.Minute(), inst.EndTime.Second(), 0, loc)
}

func mapPlannedInstance(inst *scheduleModel.ActivityInstance, staffRows []*scheduleModel.InstanceStaff, studentRows []*scheduleModel.InstanceStudent, now time.Time, currentStaffID int64, roomName *string, careDay map[int64]CareDayStatus) OperationPlannedInstance {
	assigned := make([]int64, 0, len(staffRows))
	isAssigned := false
	isPrimary := false
	isSubstitute := false
	isAbsent := false
	for _, row := range staffRows {
		if !row.IsAbsent {
			assigned = append(assigned, row.StaffID)
		}
		if currentStaffID > 0 && row.StaffID == currentStaffID {
			isAssigned = !row.IsAbsent
			isPrimary = row.IsPrimary
			isSubstitute = row.IsSubstitute
			isAbsent = row.IsAbsent
		}
	}
	expected, present, notScheduled := 0, 0, 0
	completed := inst.Status == scheduleModel.InstanceStatusCompleted
	for _, row := range studentRows {
		verdict := AttendanceRowCareDay(completed, row, careDay[row.StudentID])
		switch row.Status {
		case scheduleModel.AttendanceStatusExpected:
			// An assignment alone does not make a child expected today: the
			// care plan has to place them here on this weekday, and nobody may
			// have cancelled the day (#1747). A missing entry reads as unknown
			// and keeps the child expected.
			if !verdict.Expected() {
				notScheduled++
				continue
			}
			expected++
		case scheduleModel.AttendanceStatusPresent:
			present++
		case scheduleModel.AttendanceStatusAbsent:
			// A broad day status wrote this absence onto a day the care plan
			// never booked, and nothing has undone it yet. The card has to
			// explain the child the same way the planner list does, or it
			// reports "0 nicht eingeplant" while the roster shows one.
			// AttendanceRowCareDay hands out this verdict for no other absent
			// row, so a manual absence stays uncounted.
			if verdict == CareDayNotScheduled {
				notScheduled++
			}
		}
	}
	start := instanceStartAt(inst, now.Location())
	return OperationPlannedInstance{
		ID:                    inst.ID,
		Title:                 inst.Title,
		Date:                  inst.Date.Format("2006-01-02"),
		StartTime:             inst.StartTime.Format("15:04"),
		EndTime:               inst.EndTime.Format("15:04"),
		RoomID:                inst.RoomID,
		RoomName:              roomName,
		Status:                inst.Status,
		IsOverdue:             start.Before(now),
		MinutesUntilStart:     int(start.Sub(now).Minutes()),
		ExpectedStudentsCount: expected,
		PresentStudentsCount:  present,
		NotScheduledCount:     notScheduled,
		AssignedStaffIDs:      assigned,
		IsAssigned:            isAssigned,
		IsPrimary:             isPrimary,
		IsSubstitute:          isSubstitute,
		IsAbsent:              isAbsent,
		Warnings:              []InstanceConflictWarning{},
	}
}

func (s *timetableOperationsService) roomNameMap(ctx context.Context) (map[int64]*string, error) {
	rooms, err := s.deps.RoomRepo.List(ctx, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	names := make(map[int64]*string, len(rooms))
	for _, room := range rooms {
		if room == nil {
			continue
		}
		name := room.Name
		names[room.ID] = &name
	}
	return names, nil
}

func instanceStartAt(inst *scheduleModel.ActivityInstance, loc *time.Location) time.Time {
	return time.Date(inst.Date.Year, inst.Date.Month, inst.Date.Day, inst.StartTime.Hour(), inst.StartTime.Minute(), inst.StartTime.Second(), 0, loc)
}

func staffAssigned(rows []*scheduleModel.InstanceStaff, staffID int64) bool {
	for _, row := range rows {
		if row.StaffID == staffID && !row.IsAbsent {
			return true
		}
	}
	return false
}

func findPlanned(rows []*scheduleModel.InstanceStudent, studentID int64) (*scheduleModel.InstanceStudent, bool) {
	for _, row := range rows {
		if row.StudentID == studentID {
			return row, true
		}
	}
	return nil, false
}
