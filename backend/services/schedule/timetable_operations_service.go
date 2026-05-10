package schedule

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/device"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
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
}

type OperationPersonService interface {
	FindByAccountID(ctx context.Context, accountID int64) (*usersModel.Person, error)
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*usersModel.Person, error)
	StaffRepository() usersModel.StaffRepository
}

type OperationActiveService interface {
	CreateVisit(ctx context.Context, visit *activeModel.Visit) error
	EndVisit(ctx context.Context, id int64) error
}

type TimetableOperationsService interface {
	PlannedNow(ctx context.Context, accountID int64, isAdmin bool, date time.Time, now time.Time) ([]OperationPlannedInstance, error)
	Start(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*StartInstanceResult, error)
	Complete(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*scheduleModel.ActivityInstance, error)
	Roster(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*OperationRoster, error)
	RosterByActiveGroup(ctx context.Context, accountID int64, isAdmin bool, activeGroupID int64) (*OperationRoster, error)
	CheckInStudent(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*OperationRoster, error)
	CheckOutStudent(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*OperationRoster, error)
	PatchAttendance(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64, patch scheduleModel.AttendanceFieldPatch) (*OperationRosterRow, error)
}

type TimetableOperationsDependencies struct {
	InstanceRepo       scheduleModel.ActivityInstanceRepository
	InstanceStaffRepo  scheduleModel.InstanceStaffRepository
	InstanceStudents   scheduleModel.InstanceStudentRepository
	InstanceService    InstanceService
	ActiveGroupRepo    activeModel.GroupRepository
	ActiveService      OperationActiveService
	SupervisorRepo     activeModel.GroupSupervisorRepository
	VisitRepo          activeModel.VisitRepository
	StudentRepo        usersModel.StudentRepository
	EducationGroupRepo educationModel.GroupRepository
	PersonService      OperationPersonService
	Settings           OperationSettings
	DB                 *bun.DB
	Logger             *slog.Logger
}

type OperationPlannedInstance struct {
	ID                    int64                     `json:"id"`
	Title                 string                    `json:"title"`
	Date                  string                    `json:"date"`
	StartTime             string                    `json:"start_time"`
	EndTime               string                    `json:"end_time"`
	RoomID                int64                     `json:"room_id"`
	Status                string                    `json:"status"`
	IsOverdue             bool                      `json:"is_overdue"`
	MinutesUntilStart     int                       `json:"minutes_until_start"`
	ExpectedStudentsCount int                       `json:"expected_students_count"`
	PresentStudentsCount  int                       `json:"present_students_count"`
	AssignedStaffIDs      []int64                   `json:"assigned_staff_ids"`
	Warnings              []InstanceConflictWarning `json:"warnings"`
}

type OperationRoster struct {
	Instance OperationRosterInstance `json:"instance"`
	Rows     []OperationRosterRow    `json:"rows"`
}

type OperationRosterInstance struct {
	ID            int64   `json:"id"`
	Title         string  `json:"title"`
	Status        string  `json:"status"`
	ActiveGroupID *int64  `json:"active_group_id,omitempty"`
	RoomID        int64   `json:"room_id"`
	RoomName      *string `json:"room_name,omitempty"`
}

type OperationRosterRow struct {
	StudentID        int64   `json:"student_id"`
	StudentName      string  `json:"student_name"`
	SchoolClass      string  `json:"school_class"`
	GroupName        string  `json:"group_name"`
	Planned          bool    `json:"planned"`
	IsUnplanned      bool    `json:"is_unplanned"`
	CurrentlyPresent bool    `json:"currently_present"`
	VisitID          *int64  `json:"visit_id,omitempty"`
	Status           string  `json:"status"`
	Substatus        *string `json:"substatus,omitempty"`
	Note             *string `json:"note,omitempty"`
	CheckedInAt      *string `json:"checked_in_at,omitempty"`
	VisitEntryTime   *string `json:"visit_entry_time,omitempty"`
}

type timetableOperationsService struct {
	deps TimetableOperationsDependencies
}

func NewTimetableOperationsService(deps TimetableOperationsDependencies) TimetableOperationsService {
	if deps.InstanceRepo == nil || deps.InstanceStaffRepo == nil || deps.InstanceStudents == nil ||
		deps.InstanceService == nil || deps.ActiveGroupRepo == nil || deps.ActiveService == nil || deps.SupervisorRepo == nil ||
		deps.VisitRepo == nil || deps.StudentRepo == nil || deps.EducationGroupRepo == nil || deps.PersonService == nil || deps.DB == nil {
		panic("schedule.NewTimetableOperationsService: required dependency is nil")
	}
	return &timetableOperationsService{deps: deps}
}

func (s *timetableOperationsService) PlannedNow(ctx context.Context, accountID int64, isAdmin bool, date time.Time, now time.Time) ([]OperationPlannedInstance, error) {
	staffID, hasStaff, err := s.resolveStaffID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	adminAll := s.adminOverviewEnabled(ctx, isAdmin)
	if !hasStaff && !adminAll {
		return nil, ErrTimetableOperationForbidden
	}

	instances, err := s.deps.InstanceRepo.FindByTenantAndDate(ctx, date)
	if err != nil {
		return nil, err
	}
	out := make([]OperationPlannedInstance, 0)
	for _, inst := range instances {
		if inst.Status != scheduleModel.InstanceStatusPlanned || !plannedNowWindow(inst, now) {
			continue
		}
		staffRows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, inst.ID)
		if err != nil {
			return nil, err
		}
		if !adminAll && !staffAssigned(staffRows, staffID) {
			continue
		}
		studentRows, err := s.deps.InstanceStudents.FindByInstanceID(ctx, inst.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, mapPlannedInstance(inst, staffRows, studentRows, now))
	}
	return out, nil
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
	return s.deps.InstanceService.Complete(ctx, instanceID)
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
	if err != nil && !isNoRows(err) {
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
		return nil, err
	}
	return s.buildRoster(ctx, instanceID)
}

func (s *timetableOperationsService) PatchAttendance(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64, patch scheduleModel.AttendanceFieldPatch) (*OperationRosterRow, error) {
	if _, err := s.requireCanOperate(ctx, accountID, isAdmin, instanceID); err != nil {
		return nil, err
	}
	row, err := s.deps.InstanceStudents.FindByInstanceAndStudent(ctx, instanceID, studentID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, ErrTimetableOperationNotFound
	}
	if err := s.deps.InstanceStudents.UpdateAttendanceFields(ctx, row.ID, patch); err != nil {
		return nil, err
	}
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
	if s.adminOverviewEnabled(ctx, isAdmin) {
		return staffID, nil
	}
	if !hasStaff {
		return 0, ErrTimetableOperationForbidden
	}
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

func (s *timetableOperationsService) buildRoster(ctx context.Context, instanceID int64) (*OperationRoster, error) {
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
	groupIDs := make([]int64, 0, len(students))
	personIDs := make([]int64, 0, len(students))
	for _, st := range students {
		personIDs = append(personIDs, st.PersonID)
		if st.GroupID != nil {
			groupIDs = append(groupIDs, *st.GroupID)
		}
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
	rows := make([]OperationRosterRow, 0, len(seen))
	for _, planned := range plannedRows {
		rows = append(rows, s.mapRosterRow(planned.StudentID, planned, latestVisits[planned.StudentID], students, persons, groups))
	}
	for _, visit := range latestVisits {
		if _, planned := findPlanned(plannedRows, visit.StudentID); planned {
			continue
		}
		rows = append(rows, s.mapRosterRow(visit.StudentID, nil, visit, students, persons, groups))
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CurrentlyPresent != rows[j].CurrentlyPresent {
			return rows[i].CurrentlyPresent && !rows[j].CurrentlyPresent
		}
		if rows[i].Planned != rows[j].Planned {
			return rows[i].Planned && !rows[j].Planned
		}
		return rows[i].StudentName < rows[j].StudentName
	})
	return &OperationRoster{
		Instance: OperationRosterInstance{
			ID:            inst.ID,
			Title:         inst.Title,
			Status:        inst.Status,
			ActiveGroupID: inst.ActiveGroupID,
			RoomID:        inst.RoomID,
		},
		Rows: rows,
	}, nil
}

func (s *timetableOperationsService) mapRosterRow(studentID int64, planned *scheduleModel.InstanceStudent, visit *activeModel.Visit, students map[int64]*usersModel.Student, persons map[int64]*usersModel.Person, groups map[int64]*educationModel.Group) OperationRosterRow {
	row := OperationRosterRow{
		StudentID:        studentID,
		Planned:          planned != nil,
		IsUnplanned:      planned == nil && visit != nil,
		CurrentlyPresent: visit != nil && visit.ExitTime == nil,
		Status:           scheduleModel.AttendanceStatusPresent,
	}
	if planned != nil {
		row.Status = planned.Status
		row.Substatus = planned.Substatus
		row.Note = planned.Note
		if planned.CheckedInAt != nil {
			v := planned.CheckedInAt.UTC().Format(time.RFC3339)
			row.CheckedInAt = &v
		}
	}
	if visit != nil {
		id := visit.ID
		row.VisitID = &id
		v := visit.EntryTime.UTC().Format(time.RFC3339)
		row.VisitEntryTime = &v
	}
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
	return row
}

func (s *timetableOperationsService) resolveStaffID(ctx context.Context, accountID int64) (int64, bool, error) {
	if accountID <= 0 {
		return 0, false, ErrTimetableOperationForbidden
	}
	person, err := s.deps.PersonService.FindByAccountID(ctx, accountID)
	if err != nil || person == nil {
		return 0, false, nil
	}
	staff, err := s.deps.PersonService.StaffRepository().FindByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		return 0, false, nil
	}
	return staff.ID, true, nil
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
		if isNoRows(err) {
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

func plannedNowWindow(inst *scheduleModel.ActivityInstance, now time.Time) bool {
	start := time.Date(now.Year(), now.Month(), now.Day(), inst.StartTime.Hour(), inst.StartTime.Minute(), inst.StartTime.Second(), 0, now.Location())
	return (start.After(now.Add(-15*time.Minute)) && start.Before(now.Add(15*time.Minute))) || start.Before(now)
}

func mapPlannedInstance(inst *scheduleModel.ActivityInstance, staffRows []*scheduleModel.InstanceStaff, studentRows []*scheduleModel.InstanceStudent, now time.Time) OperationPlannedInstance {
	assigned := make([]int64, 0, len(staffRows))
	for _, row := range staffRows {
		if !row.IsAbsent {
			assigned = append(assigned, row.StaffID)
		}
	}
	expected, present := 0, 0
	for _, row := range studentRows {
		switch row.Status {
		case scheduleModel.AttendanceStatusExpected:
			expected++
		case scheduleModel.AttendanceStatusPresent:
			present++
		}
	}
	start := time.Date(now.Year(), now.Month(), now.Day(), inst.StartTime.Hour(), inst.StartTime.Minute(), inst.StartTime.Second(), 0, now.Location())
	return OperationPlannedInstance{
		ID:                    inst.ID,
		Title:                 inst.Title,
		Date:                  inst.Date.Format("2006-01-02"),
		StartTime:             inst.StartTime.Format("15:04"),
		EndTime:               inst.EndTime.Format("15:04"),
		RoomID:                inst.RoomID,
		Status:                inst.Status,
		IsOverdue:             start.Before(now),
		MinutesUntilStart:     int(start.Sub(now).Minutes()),
		ExpectedStudentsCount: expected,
		PresentStudentsCount:  present,
		AssignedStaffIDs:      assigned,
		Warnings:              []InstanceConflictWarning{},
	}
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

func isNoRows(err error) bool {
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	type unwrapper interface{ Unwrap() error }
	if u, ok := err.(unwrapper); ok {
		return isNoRows(u.Unwrap())
	}
	return false
}
