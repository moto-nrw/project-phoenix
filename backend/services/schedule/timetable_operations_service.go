package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"

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
	GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*usersModel.Staff, error)
}

type OperationActiveService interface {
	CreateVisit(ctx context.Context, visit *activeModel.Visit) error
	EndVisit(ctx context.Context, id int64) error
	MoveStudentsToActiveGroupAuthorized(ctx context.Context, studentIDs []int64, activeGroupID int64, auth activeSvc.StudentMoveAuthorization) (*activeSvc.StudentMoveResult, error)
}

type OperationArrivalService interface {
	GetBulkEffectiveArrivalTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*EffectiveArrivalTime, error)
}

type OperationPickupService interface {
	GetBulkEffectivePickupTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*EffectivePickupTime, error)
}

type TimetableOperationsService interface {
	PlannedNow(ctx context.Context, accountID int64, isAdmin bool, date timezone.Date, now time.Time, opts PlannedNowOptions) ([]OperationPlannedInstance, error)
	// ActiveSessions lists the given day's running instances with their plan
	// windows, keyed by live session (active group), so the supervision UI
	// can label session tabs "Aktivitätsname · Planzeit" (#2265).
	ActiveSessions(ctx context.Context, date timezone.Date) ([]OperationActiveSession, error)
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
	// EarliestPlannedBlockStartForClass returns the "HH:MM" start of the
	// first non-cancelled block of the date that addresses the school class,
	// "" when there is none (#2970).
	EarliestPlannedBlockStartForClass(ctx context.Context, schoolClass string, date timezone.Date) (string, error)
}

// PlannedNowScopePast flips PlannedNow to the complement of its default
// window (#2335): today's finished blocks — completed ones, and planned ones
// whose end time has passed without ever being started (no job moves them out
// of "planned", so a status filter alone would miss them).
const PlannedNowScopePast = "past"

// PlannedNowScopeDay returns the day's blocks in every lifecycle state —
// planned, running, completed, cancelled — with no time window (#2527). Who
// sees which blocks follows the same operational-overview rule as the default
// scope (#2383): under operations.operational_overview_scope = all_staff every
// verified staff member (and every admin) gets the whole day, otherwise only
// their own assignments. School-portal tokens always collapse to "own"
// (#2527), which keeps the school list "Meine Aufsichten heute" exactly that.
const PlannedNowScopeDay = "day"

type PlannedNowOptions struct {
	HorizonMinutes int
	Limit          int
	IncludeRoster  bool
	// Scope is "" for the default upcoming window or PlannedNowScopePast.
	Scope string
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
	PickupService      OperationPickupService
	CareDayService     CareDayService
	SupervisorRepo     activeModel.GroupSupervisorRepository
	VisitRepo          activeModel.VisitRepository
	StudentRepo        usersModel.StudentRepository
	EducationGroupRepo educationModel.GroupRepository
	RoomRepo           facilitiesModel.RoomRepository
	PersonService      OperationPersonService
	// PlanningTrackRepo is optional: without it day-scope blocks simply carry
	// no planning-track colour (#2383).
	PlanningTrackRepo scheduleModel.PlanningTrackRepository
	Settings          OperationSettings
	Broadcaster       realtime.Broadcaster
	DB                *bun.DB
	Logger            *slog.Logger
	Now               func() time.Time
	RecoveryRepo      scheduleModel.ActivityRecoveryRepository
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
	NotScheduledCount   int                       `json:"not_scheduled_students_count"`
	AssignedStaffIDs    []int64                   `json:"assigned_staff_ids"`
	IsAssigned          bool                      `json:"is_assigned"`
	IsPrimary           bool                      `json:"is_primary"`
	IsSubstitute        bool                      `json:"is_substitute"`
	IsAbsent            bool                      `json:"is_absent"`
	RosterPreview       []OperationRosterRow      `json:"roster_preview,omitempty"`
	PickupTimesLoaded   bool                      `json:"pickup_times_loaded"`
	PickupTimesRedacted bool                      `json:"pickup_times_redacted,omitempty"`
	Warnings            []InstanceConflictWarning `json:"warnings"`
	CanStart            bool                      `json:"can_start"`
	StartAvailableAt    string                    `json:"start_available_at"`
	StartExpiresAt      string                    `json:"start_expires_at"`
	// ActiveGroupID is the live session behind a running block, so the
	// Tagesplan (#2383) can jump straight into its supervision list.
	ActiveGroupID *int64 `json:"active_group_id,omitempty"`
	// CancelReason explains a cancelled block ("fällt aus: …", #1840).
	CancelReason *string `json:"cancel_reason,omitempty"`
	// PlanningTrackName/Color carry the planned colour coding into the
	// whole-day scope (#2383) — the Tagesplan paints blocks in the same
	// colours as the Betreuungsplan. Nil outside scope=day and for blocks
	// without a planning track.
	PlanningTrackName  *string `json:"planning_track_name,omitempty"`
	PlanningTrackColor *string `json:"planning_track_color,omitempty"`
	// GroupName is the education group the block's template targets (the
	// "Zielgruppe" on a Tagesplan card). Nil outside scope=day and for
	// blocks without a template group.
	GroupName *string `json:"group_name,omitempty"`
	// StaffNames lists the assigned (non-absent) staff, resolved to display
	// names. Empty outside scope=day.
	StaffNames []OperationStaffName `json:"staff_names,omitempty"`
}

// OperationStaffName is one assigned staff member on a day-scope block.
type OperationStaffName struct {
	StaffID      int64  `json:"staff_id"`
	DisplayName  string `json:"display_name"`
	IsSubstitute bool   `json:"is_substitute"`
}

// OperationActiveSession is one running instance seen from its live session
// (#2265). StartTime/EndTime are the PLAN window ("15:04"), not the actual
// start instant.
type OperationActiveSession struct {
	ActiveGroupID int64  `json:"active_group_id"`
	InstanceID    int64  `json:"instance_id"`
	Title         string `json:"title"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
}

type OperationRoster struct {
	Instance            OperationRosterInstance `json:"instance"`
	Rows                []OperationRosterRow    `json:"rows"`
	PickupTimesLoaded   bool                    `json:"pickup_times_loaded"`
	PickupTimesRedacted bool                    `json:"pickup_times_redacted,omitempty"`
	// MovedFrom is set only on check-in responses that auto-moved the child
	// out of another running session (#2386). It carries the origin's display
	// name; an empty string means the move happened but no name resolved.
	MovedFrom *string `json:"moved_from,omitempty"`
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
	PickupTime       *string                  `json:"pickup_time"`
	Warnings         []OperationRosterWarning `json:"warnings,omitempty"`
	// ParallelPresentIn names the other running instance where this child is
	// currently recorded present (#2265). Set only on rosters of active
	// instances; nil when no parallel block holds the child as present.
	ParallelPresentIn *OperationParallelPresence `json:"parallel_present_in,omitempty"`
	// CareDayStatus is the care-plan verdict for this child on the instance's
	// date (#1747): "scheduled" | "not_scheduled" | "cancelled" | "unknown".
	// "not_scheduled" (not booked that weekday) and "cancelled" (someone said
	// "kommt heute nicht") both mean the child is not expected — the frontend
	// sets those rows apart and leaves them out of the expected count. The rows
	// stay in the payload so a child who turns up anyway can still be checked
	// in with one tap.
	CareDayStatus CareDayStatus `json:"care_day_status"`
}

// OperationParallelPresence identifies the other running instance a roster
// row's parallel-presence hint points at (#2265).
type OperationParallelPresence struct {
	InstanceID int64  `json:"instance_id"`
	Title      string `json:"title"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
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
		deps.ActiveService == nil || deps.ArrivalService == nil || deps.PickupService == nil || deps.CareDayService == nil || deps.SupervisorRepo == nil ||
		deps.VisitRepo == nil || deps.StudentRepo == nil || deps.EducationGroupRepo == nil || deps.RoomRepo == nil || deps.PersonService == nil || deps.Settings == nil || deps.DB == nil {
		panic("schedule.NewTimetableOperationsService: required dependency is nil")
	}
	return &timetableOperationsService{deps: deps}
}

func (s *timetableOperationsService) now() time.Time {
	if s.deps.Now != nil {
		return s.deps.Now()
	}
	return time.Now()
}

func (s *timetableOperationsService) today() timezone.Date {
	return timezone.DateFromTime(s.now())
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
	allOperational := s.operationalOverview(ctx, isAdmin, hasStaff)
	adminActions := s.hasAdministrativeActionAccess(ctx, isAdmin)
	if !hasStaff && !allOperational {
		return nil, ErrTimetableOperationForbidden
	}

	instances, err := s.deps.InstanceRepo.FindByTenantAndDate(ctx, scheduleModel.Date(date))
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
	past := opts.Scope == PlannedNowScopePast
	wholeDay := opts.Scope == PlannedNowScopeDay
	eligible := make([]*scheduleModel.ActivityInstance, 0, len(instances))
	for _, inst := range instances {
		switch {
		case wholeDay:
			// Every state stays in: a cancelled block ("fällt aus") and a
			// finished one are both answers the person came for.
		case past:
			if !plannedPastToday(inst, now) {
				continue
			}
		default:
			if inst.Status != scheduleModel.InstanceStatusPlanned || !plannedNowWindow(inst, now, horizon) {
				continue
			}
		}
		eligible = append(eligible, inst)
	}

	eligibleIDs := activityInstanceIDs(eligible)
	staffRows, err := s.deps.InstanceStaffRepo.FindByInstanceIDs(ctx, eligibleIDs)
	if err != nil {
		return nil, err
	}
	staffByInstance := indexInstanceStaffRows(staffRows)

	candidateInstances := make([]*scheduleModel.ActivityInstance, 0, len(eligible))
	for _, inst := range eligible {
		assigned := staffAssigned(staffByInstance[inst.ID], staffID)
		if !allOperational && !assigned {
			continue
		}
		candidateInstances = append(candidateInstances, inst)
		if opts.Limit > 0 && len(candidateInstances) >= opts.Limit {
			break
		}
	}
	studentRows, err := s.deps.InstanceStudents.FindByInstanceIDs(ctx, activityInstanceIDs(candidateInstances))
	if err != nil {
		return nil, err
	}
	studentsByInstance := indexInstanceStudentRows(studentRows)
	candidates := make([]plannedNowCandidate, 0, len(candidateInstances))
	for _, inst := range candidateInstances {
		assigned := staffAssigned(staffByInstance[inst.ID], staffID)
		candidates = append(candidates, plannedNowCandidate{
			instance:    inst,
			staffRows:   staffByInstance[inst.ID],
			studentRows: studentsByInstance[inst.ID],
			roomName:    roomNames[inst.RoomID],
			canOperate:  hasStaff && (adminActions || assigned),
		})
	}

	careDay, err := s.deps.CareDayService.ResolveForDate(ctx, plannedNowStudentIDs(candidates), date)
	if err != nil {
		return nil, err
	}

	out := make([]OperationPlannedInstance, 0, len(candidates))
	for _, candidate := range candidates {
		mapped := mapPlannedInstance(candidate.instance, candidate.staffRows, candidate.studentRows, now, staffID, candidate.roomName, careDay)
		// Past blocks are read-only: no start lifecycle, the zero-value
		// CanStart/StartAvailableAt/StartExpiresAt say so. The whole-day scope
		// carries every state, so there the instance's own status decides —
		// a running, finished or cancelled block is not startable either.
		if candidate.canOperate && !past && (!wholeDay || candidate.instance.Status == scheduleModel.InstanceStatusPlanned) {
			availability := EvaluateLifecycleAvailability(candidate.instance, now, startLead, true)
			mapped.CanStart = availability.CanStart
			mapped.StartAvailableAt = availability.StartAvailableAt.Format(time.RFC3339)
			if !candidate.instance.IsSpontaneous {
				mapped.StartExpiresAt = availability.CompleteAvailableAt.Format(time.RFC3339)
			}
		}
		if opts.IncludeRoster {
			roster, err := s.buildRosterWithCareDay(ctx, candidate.instance.ID, careDay)
			if err != nil {
				return nil, err
			}
			mapped.RosterPreview = roster.Rows
			mapped.PickupTimesLoaded = roster.PickupTimesLoaded
		}
		out = append(out, mapped)
	}
	if wholeDay {
		if err := s.enrichDayPlan(ctx, candidates, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func activityInstanceIDs(instances []*scheduleModel.ActivityInstance) []int64 {
	ids := make([]int64, 0, len(instances))
	for _, instance := range instances {
		ids = append(ids, instance.ID)
	}
	return ids
}

func indexInstanceStaffRows(rows []*scheduleModel.InstanceStaff) map[int64][]*scheduleModel.InstanceStaff {
	byInstance := make(map[int64][]*scheduleModel.InstanceStaff)
	for _, row := range rows {
		byInstance[row.InstanceID] = append(byInstance[row.InstanceID], row)
	}
	return byInstance
}

func indexInstanceStudentRows(rows []*scheduleModel.InstanceStudent) map[int64][]*scheduleModel.InstanceStudent {
	byInstance := make(map[int64][]*scheduleModel.InstanceStudent)
	for _, row := range rows {
		byInstance[row.InstanceID] = append(byInstance[row.InstanceID], row)
	}
	return byInstance
}

// enrichDayPlan decorates whole-day-scope blocks (#2383) with what the
// Tagesplan cards show beyond the base payload: staff display names, the
// planning-track colour of the underlying template, and the template's
// education group ("Zielgruppe"). candidates and out are parallel slices.
func (s *timetableOperationsService) enrichDayPlan(ctx context.Context, candidates []plannedNowCandidate, out []OperationPlannedInstance) error {
	staffSeen := map[int64]bool{}
	staffIDs := make([]int64, 0)
	for _, candidate := range candidates {
		for _, row := range candidate.staffRows {
			if !row.IsAbsent && !staffSeen[row.StaffID] {
				staffSeen[row.StaffID] = true
				staffIDs = append(staffIDs, row.StaffID)
			}
		}
	}
	staffByID := map[int64]*usersModel.Staff{}
	if len(staffIDs) > 0 {
		var err error
		staffByID, err = s.deps.PersonService.GetStaffWithPersonByIDs(ctx, staffIDs)
		if err != nil {
			return err
		}
	}

	// One IN query per metadata kind instead of one lookup per distinct
	// template/track: a large school's day plan otherwise fires a query
	// burst proportional to its module count. Missing IDs are simply absent
	// from the maps, which the render loop below already treats as "no
	// metadata" (same behavior as the previous per-ID no-rows handling).
	groupIDs := make([]int64, 0)
	groupSeen := map[int64]bool{}
	for _, candidate := range candidates {
		groupID := candidate.instance.ActivityGroupID
		if groupID == nil || *groupID <= 0 || groupSeen[*groupID] {
			continue
		}
		groupSeen[*groupID] = true
		groupIDs = append(groupIDs, *groupID)
	}
	activityGroups := map[int64]*activitiesModel.Group{}
	educationGroupIDs := make([]int64, 0)
	trackIDs := make([]int64, 0)
	if len(groupIDs) > 0 {
		groups, err := s.deps.ActivityGroupRepo.FindByIDs(ctx, groupIDs)
		if err != nil {
			return err
		}
		educationSeen := map[int64]bool{}
		trackSeen := map[int64]bool{}
		for _, group := range groups {
			activityGroups[group.ID] = group
			if group.EducationGroupID != nil && !educationSeen[*group.EducationGroupID] {
				educationSeen[*group.EducationGroupID] = true
				educationGroupIDs = append(educationGroupIDs, *group.EducationGroupID)
			}
			if group.PlanningTrackID != nil && !trackSeen[*group.PlanningTrackID] {
				trackSeen[*group.PlanningTrackID] = true
				trackIDs = append(trackIDs, *group.PlanningTrackID)
			}
		}
	}
	tracks := map[int64]*scheduleModel.PlanningTrack{}
	if len(trackIDs) > 0 && s.deps.PlanningTrackRepo != nil {
		trackRows, err := s.deps.PlanningTrackRepo.FindByIDs(ctx, trackIDs)
		if err != nil {
			return err
		}
		for _, track := range trackRows {
			tracks[track.ID] = track
		}
	}
	educationGroups := map[int64]*educationModel.Group{}
	if len(educationGroupIDs) > 0 {
		var err error
		educationGroups, err = s.deps.EducationGroupRepo.FindByIDs(ctx, educationGroupIDs)
		if err != nil {
			return err
		}
	}

	for i, candidate := range candidates {
		names := make([]OperationStaffName, 0, len(candidate.staffRows))
		for _, row := range candidate.staffRows {
			if row.IsAbsent {
				continue
			}
			staff := staffByID[row.StaffID]
			if staff == nil || staff.Person == nil {
				continue
			}
			names = append(names, OperationStaffName{
				StaffID:      row.StaffID,
				DisplayName:  staff.Person.GetFullName(),
				IsSubstitute: row.IsSubstitute,
			})
		}
		sort.Slice(names, func(a, b int) bool { return names[a].DisplayName < names[b].DisplayName })
		out[i].StaffNames = names

		groupID := candidate.instance.ActivityGroupID
		if groupID == nil {
			continue
		}
		group := activityGroups[*groupID]
		if group == nil {
			continue
		}
		if group.EducationGroupID != nil {
			if educationGroup := educationGroups[*group.EducationGroupID]; educationGroup != nil {
				name := educationGroup.Name
				out[i].GroupName = &name
			}
		}
		if group.PlanningTrackID != nil {
			if track := tracks[*group.PlanningTrackID]; track != nil {
				trackName := track.Name
				trackColor := track.Color
				out[i].PlanningTrackName = &trackName
				out[i].PlanningTrackColor = &trackColor
			}
		}
	}
	return nil
}

// plannedNowCandidate is one instance that survived the PlannedNow filters,
// with the rows already loaded for it.
type plannedNowCandidate struct {
	instance    *scheduleModel.ActivityInstance
	staffRows   []*scheduleModel.InstanceStaff
	studentRows []*scheduleModel.InstanceStudent
	roomName    *string
	canOperate  bool
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
	result, err := s.Start(withSpontaneousStartWorkdayGuard(ctx), accountID, isAdmin, inst.ID)
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
	inst, err := s.loadInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	// Completing a live group closes its supervisor row, so requireCanOperate
	// would reject the same person who just finished the block. Reopen is
	// gated by the completion actor / admin check instead.
	if !CanReopenAsActor(inst, accountID, isAdmin) {
		return nil, ErrTimetableOperationForbidden
	}
	return s.deps.InstanceService.Reopen(ctx, instanceID, accountID, isAdmin)
}

func (s *timetableOperationsService) Roster(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*OperationRoster, error) {
	if _, err := s.requireCanView(ctx, accountID, isAdmin, instanceID); err != nil {
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
	if err := s.requireRosterStudent(ctx, inst, instanceID, studentID); err != nil {
		return nil, err
	}
	current, err := s.deps.VisitRepo.GetCurrentByStudentID(ctx, studentID)
	if err != nil && !modelBase.IsNoRows(err) {
		return nil, err
	}
	if current != nil {
		return s.checkInStudentWithCurrentVisit(ctx, staffID, inst, instanceID, studentID, current)
	}
	now := s.now()
	visit := &activeModel.Visit{
		StudentID:     studentID,
		ActiveGroupID: *inst.ActiveGroupID,
		EntryTime:     now,
	}
	visit.SetTenantID(tenant.FromContext(ctx))
	staff := &usersModel.Staff{}
	staff.ID = staffID
	visitCtx := context.WithValue(ctx, device.CtxStaff, staff)
	var createErr error
	if _, inTx := tenant.TransactionFromContext(visitCtx); inTx {
		createErr = tenant.WithSavepoint(visitCtx, func(savepointCtx context.Context) error {
			return s.deps.ActiveService.CreateVisit(savepointCtx, visit)
		})
	} else {
		createErr = s.deps.ActiveService.CreateVisit(visitCtx, visit)
	}
	if createErr != nil {
		if errors.Is(createErr, tenant.ErrSavepointControl) {
			return nil, createErr
		}
		if errors.Is(createErr, activeSvc.ErrStudentAlreadyActive) {
			current, lookupErr := s.deps.VisitRepo.GetCurrentByStudentID(ctx, studentID)
			if lookupErr != nil && !modelBase.IsNoRows(lookupErr) {
				return nil, lookupErr
			}
			if current != nil {
				return s.checkInStudentWithCurrentVisit(ctx, staffID, inst, instanceID, studentID, current)
			}
		}
		return nil, createErr
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

func (s *timetableOperationsService) checkInStudentWithCurrentVisit(ctx context.Context, staffID int64, inst *scheduleModel.ActivityInstance, instanceID, studentID int64, current *activeModel.Visit) (*OperationRoster, error) {
	if current.ActiveGroupID != *inst.ActiveGroupID {
		return s.moveStudentFromOtherSession(ctx, staffID, inst, instanceID, studentID)
	}
	if err := s.markPlannedStudentPresent(ctx, instanceID, studentID); err != nil {
		return nil, err
	}
	return s.buildRoster(ctx, instanceID)
}

// moveStudentFromOtherSession resolves the "child is still recorded present
// in another running session" check-in conflict by moving the child instead
// of rejecting (#2386). The shared bulk-move path owns checkout semantics,
// attendance mirroring, and SSE broadcasts for both the old and new visit.
// Target authorization already happened in requireCanOperate, so the move's
// own supervision check is bypassed.
func (s *timetableOperationsService) moveStudentFromOtherSession(ctx context.Context, staffID int64, inst *scheduleModel.ActivityInstance, instanceID, studentID int64) (*OperationRoster, error) {
	result, err := s.deps.ActiveService.MoveStudentsToActiveGroupAuthorized(ctx, []int64{studentID}, *inst.ActiveGroupID, activeSvc.StudentMoveAuthorization{
		StaffID:              staffID,
		BypassResourceChecks: true,
	})
	if err != nil {
		return nil, err
	}
	if len(result.Moved) == 0 && len(result.Unchanged) == 0 {
		tenant.MarkRollback(ctx)
		return nil, fmt.Errorf("%w: student could not be moved from other session", ErrTimetableOperationConflict)
	}
	if err := s.markPlannedStudentPresent(ctx, instanceID, studentID); err != nil {
		return nil, err
	}
	roster, err := s.buildRoster(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if len(result.Moved) > 0 {
		movedFrom := ""
		if previousActiveGroupID := result.PreviousActiveGroupIDs[studentID]; previousActiveGroupID > 0 {
			movedFrom = s.resolveActiveGroupLabel(ctx, previousActiveGroupID)
		}
		roster.MovedFrom = &movedFrom
	}
	return roster, nil
}

// resolveActiveGroupLabel names a running session for the move notice: the
// owning timetable instance's title when one exists, otherwise the session's
// activity group name, otherwise its room name. Purely cosmetic — every
// failure degrades to an empty label instead of an error.
func (s *timetableOperationsService) resolveActiveGroupLabel(ctx context.Context, activeGroupID int64) string {
	if inst, err := s.deps.InstanceRepo.FindByActiveGroupID(ctx, activeGroupID); err == nil && inst != nil {
		return inst.Title
	}
	group, err := s.deps.ActiveGroupRepo.FindByID(ctx, activeGroupID)
	if err != nil || group == nil {
		return ""
	}
	if group.GroupID != nil {
		if activityGroup, err := s.deps.ActivityGroupRepo.FindByID(ctx, *group.GroupID); err == nil && activityGroup != nil {
			return activityGroup.Name
		}
	}
	if room, err := s.deps.RoomRepo.FindByID(ctx, group.RoomID); err == nil && room != nil {
		return room.Name
	}
	return ""
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
	if err := s.requireRosterStudent(ctx, inst, instanceID, studentID); err != nil {
		return nil, err
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
	if err := s.requireRosterStudent(ctx, inst, instanceID, studentID); err != nil {
		return nil, err
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

// requireRosterStudent bounds a per-child write to the children this block
// actually holds (#2527). It runs only for assignment-bound portals: an OGS
// supervisor legitimately pulls a child that walked in off the tenant-wide
// directory, but a Lehrkraft has no directory — her whole reach is the block
// she was planned into, so a student id she did not get from this roster is
// not hers to write.
//
// "Belongs to the block" is the same union the roster renders: a planned
// instance_students row, or a child currently recorded present in the running
// session. That deliberately includes the not-scheduled and cancelled rows —
// they are on her sheet, and a child who turns up anyway must stay one tap
// away.
func (s *timetableOperationsService) requireRosterStudent(ctx context.Context, inst *scheduleModel.ActivityInstance, instanceID, studentID int64) error {
	if !isAssignmentBoundPortal(ctx) {
		return nil
	}
	planned, err := s.deps.InstanceStudents.FindByInstanceID(ctx, instanceID)
	if err != nil {
		return err
	}
	if _, ok := findPlanned(planned, studentID); ok {
		excluded, err := s.rosterStudentExcluded(ctx, inst, studentID)
		if err != nil {
			return err
		}
		if excluded {
			return ErrTimetableOperationForbidden
		}
		return nil
	}
	if inst != nil && inst.ActiveGroupID != nil {
		visits, err := s.deps.VisitRepo.FindByActiveGroupID(ctx, *inst.ActiveGroupID)
		if err != nil {
			return err
		}
		for _, visit := range visits {
			if visit.StudentID == studentID {
				excluded, err := s.rosterStudentExcluded(ctx, inst, studentID)
				if err != nil {
					return err
				}
				if excluded {
					return ErrTimetableOperationForbidden
				}
				return nil
			}
		}
	}
	return ErrTimetableOperationForbidden
}

// rosterStudentExcluded applies the same current-roster exclusion used by the
// read model before an assignment-bound portal mutates a child. A planned row
// is retained as history after graduation or care end, but it must not still
// authorize attendance changes for a current or future supervision.
func (s *timetableOperationsService) rosterStudentExcluded(ctx context.Context, inst *scheduleModel.ActivityInstance, studentID int64) (bool, error) {
	students, err := s.deps.StudentRepo.FindByIDs(ctx, []int64{studentID})
	if err != nil {
		return false, err
	}
	return rosterExcludedAlumni(inst, students, s.today())[studentID], nil
}

func (s *timetableOperationsService) requireCanOperate(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (int64, error) {
	staffID, hasStaff, err := s.resolveStaffID(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if s.hasAdministrativeActionAccess(ctx, isAdmin) {
		return staffID, nil
	}
	if !hasStaff {
		return 0, ErrTimetableOperationForbidden
	}
	return s.requireFixedGroupOperationAccess(ctx, staffID, instanceID)
}

func (s *timetableOperationsService) requireCanView(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (int64, error) {
	staffID, hasStaff, err := s.resolveStaffID(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if s.operationalOverview(ctx, isAdmin, hasStaff) {
		inst, err := s.loadInstance(ctx, instanceID)
		if err != nil {
			return 0, err
		}
		if inst.Status != scheduleModel.InstanceStatusActive || inst.ActiveGroupID == nil {
			return 0, ErrTimetableOperationForbidden
		}
		return staffID, nil
	}
	if !hasStaff {
		return 0, ErrTimetableOperationForbidden
	}
	return s.requireFixedGroupOperationAccess(ctx, staffID, instanceID)
}

func (s *timetableOperationsService) hasAdministrativeActionAccess(ctx context.Context, isAdmin bool) bool {
	if isAssignmentBoundPortal(ctx) {
		return false
	}
	claims := jwt.ClaimsFromCtx(ctx)
	return isAdmin || claims.IsAdmin || authorize.HasAdminWildcard(jwt.PermissionsFromCtx(ctx))
}

func (s *timetableOperationsService) requireFixedGroupOperationAccess(ctx context.Context, staffID, instanceID int64) (int64, error) {
	inst, err := s.loadInstance(ctx, instanceID)
	if err != nil {
		return 0, err
	}
	// An assignment-bound portal reaches TODAY and nothing else (#2527). Its
	// list already answers only for today, but a detail route takes an id, and
	// ids are guessable: without this clamp a Lehrkraft could pull the roster
	// — and with it a child's pickup and emergency contacts — for any block
	// she is planned into next week or was planned into in March. Her access
	// follows the day she stands in front of the children, so the day is part
	// of the boundary, not just the assignment.
	if isAssignmentBoundPortal(ctx) && timezone.Date(inst.Date) != s.today() {
		return 0, ErrTimetableOperationForbidden
	}
	staffRows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, instanceID)
	if err != nil {
		return 0, err
	}
	if staffAssigned(staffRows, staffID) {
		return staffID, nil
	}
	// The school portal's boundary is the concrete timetable assignment, not
	// the active group's supervisor list. Starting a block adds its operator
	// as a supervisor, so using that list here would preserve access after the
	// assignment has been withdrawn.
	if isAssignmentBoundPortal(ctx) {
		return 0, ErrTimetableOperationForbidden
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

func isAssignmentBoundPortal(ctx context.Context) bool {
	claims := jwt.ClaimsFromCtx(ctx)
	return claims.IsSchoolScope()
}

func (s *timetableOperationsService) buildRoster(ctx context.Context, instanceID int64) (*OperationRoster, error) {
	return s.buildRosterWithCareDay(ctx, instanceID, nil)
}

// rosterExcludedAlumni returns the set of student IDs to drop from a
// current/future roster because the student has graduated (alumnus) and is
// therefore soft-deleted from all staff-facing operations. Frozen history —
// a past-dated, completed, or cancelled instance — excludes nobody so its
// recorded attendance stays intact (#405).
func rosterExcludedAlumni(inst *scheduleModel.ActivityInstance, students map[int64]*usersModel.Student, today timezone.Date) map[int64]bool {
	excluded := map[int64]bool{}
	if inst == nil {
		return excluded
	}
	if inst.Status == scheduleModel.InstanceStatusCompleted || inst.Status == scheduleModel.InstanceStatusCancelled {
		return excluded
	}
	if inst.Date.Before(today) {
		return excluded
	}
	for id, st := range students {
		if st == nil {
			continue
		}
		// Graduated children and children whose care ended before this block
		// are excluded from the roster of a not-yet-past occurrence (#2487).
		if st.Status == usersModel.StudentStatusAlumnus || st.CareEndedOn(timezone.Date(inst.Date)) {
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
	excludedAlumni := rosterExcludedAlumni(inst, students, s.today())
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
		careDay, err = s.deps.CareDayService.ResolveForDate(ctx, studentIDs, timezone.Date(inst.Date))
		if err != nil {
			return nil, err
		}
	}
	pickupTimes, pickupTimesLoaded := s.rosterPickupTimes(ctx, inst, studentIDs)
	parallelPresence, err := s.parallelPresenceByStudent(ctx, inst, studentIDs)
	if err != nil {
		return nil, err
	}
	rows := make([]OperationRosterRow, 0, len(seen))
	for _, planned := range plannedRows {
		if excludedAlumni[planned.StudentID] {
			continue
		}
		row := s.mapRosterRow(inst, planned.StudentID, planned, latestVisits[planned.StudentID], students, persons, groups, warningsByStudent[planned.StudentID], careDay)
		row.PickupTime = formatRosterPickupTime(pickupTimes[planned.StudentID])
		row.ParallelPresentIn = parallelPresence[planned.StudentID]
		rows = append(rows, row)
	}
	for _, visit := range latestVisits {
		if excludedAlumni[visit.StudentID] {
			continue
		}
		if _, planned := findPlanned(plannedRows, visit.StudentID); planned {
			continue
		}
		row := s.mapRosterRow(inst, visit.StudentID, nil, visit, students, persons, groups, nil, careDay)
		row.PickupTime = formatRosterPickupTime(pickupTimes[visit.StudentID])
		row.ParallelPresentIn = parallelPresence[visit.StudentID]
		rows = append(rows, row)
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
	now := s.now()
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
		Rows:              rows,
		PickupTimesLoaded: pickupTimesLoaded,
	}, nil
}

func (s *timetableOperationsService) rosterPickupTimes(
	ctx context.Context,
	inst *scheduleModel.ActivityInstance,
	studentIDs []int64,
) (map[int64]*EffectivePickupTime, bool) {
	if len(studentIDs) == 0 {
		return map[int64]*EffectivePickupTime{}, true
	}
	pickups, err := s.deps.PickupService.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, timezone.Date(inst.Date))
	if err != nil {
		s.logger().WarnContext(
			ctx,
			"could not load pickup times for timetable roster",
			slog.String("error", err.Error()),
			slog.Int64("instance_id", inst.ID),
		)
		return map[int64]*EffectivePickupTime{}, false
	}
	return pickups, true
}

func formatRosterPickupTime(effective *EffectivePickupTime) *string {
	if effective == nil || effective.PickupTime == nil {
		return nil
	}
	formatted := effective.PickupTime.Format("15:04")
	return &formatted
}

// ActiveSessions implements TimetableOperationsService. Purely descriptive
// display metadata (titles + plan windows of running blocks), so it carries
// no per-caller assignment filter — route-level SchedulesRead gates access.
func (s *timetableOperationsService) ActiveSessions(ctx context.Context, date timezone.Date) ([]OperationActiveSession, error) {
	instances, err := s.deps.InstanceRepo.FindByTenantAndDate(ctx, scheduleModel.Date(date))
	if err != nil {
		return nil, err
	}
	out := make([]OperationActiveSession, 0, len(instances))
	for _, inst := range instances {
		if inst.Status != scheduleModel.InstanceStatusActive || inst.ActiveGroupID == nil {
			continue
		}
		out = append(out, OperationActiveSession{
			ActiveGroupID: *inst.ActiveGroupID,
			InstanceID:    inst.ID,
			Title:         inst.Title,
			StartTime:     inst.StartTime.Format("15:04"),
			EndTime:       inst.EndTime.Format("15:04"),
		})
	}
	return out, nil
}

// parallelPresenceByStudent resolves, for an active instance's roster, which
// students are recorded present in another running instance right now
// (#2265). Repo ordering is start_time DESC, so the first row per student is
// the latest parallel block. Non-active instances never query — a completed
// or planned roster carries no "right now" claim to contradict.
func (s *timetableOperationsService) parallelPresenceByStudent(ctx context.Context, inst *scheduleModel.ActivityInstance, studentIDs []int64) (map[int64]*OperationParallelPresence, error) {
	if inst.Status != scheduleModel.InstanceStatusActive || len(studentIDs) == 0 {
		return nil, nil
	}
	found, err := s.deps.InstanceStudents.FindPresentInOtherActiveInstances(ctx, inst.ID, inst.Date, studentIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*OperationParallelPresence, len(found))
	for _, row := range found {
		if _, taken := out[row.StudentID]; taken {
			continue
		}
		out[row.StudentID] = &OperationParallelPresence{
			InstanceID: row.InstanceID,
			Title:      row.Title,
			StartTime:  row.StartTime.Format("15:04"),
			EndTime:    row.EndTime.Format("15:04"),
		}
	}
	return out, nil
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

	arrivals, err := s.deps.ArrivalService.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, timezone.Date(inst.Date))
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
	slotStartClock := timezone.NormalizeWallClock(inst.StartTime)
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
		arrivalClock := timezone.NormalizeWallClock(*arrival.ArrivalTime)
		arrivesLate := arrivalClock.After(slotStartClock)
		if arrivesLate {
			expectedArrival := arrival.ArrivalTime.Format("15:04")
			warnings[studentID] = append(warnings[studentID], OperationRosterWarning{
				Kind:            "arrival_after_slot_start",
				Message:         "Erwartete Ankunft liegt nach dem Start dieser Betreuung.",
				ExpectedArrival: &expectedArrival,
				SlotStart:       &slotStart,
			})
		}
		// A class-wide day exception (#2962) is information, not a warning:
		// the class arrives at a different time today and the row says why,
		// so nobody wonders why the child is not under "Kommt später". When
		// the row already carries "Kommt um HH:MM Uhr" from the late-arrival
		// warning, the line adds only the reason instead of repeating the time.
		if arrival.ClassException != nil {
			expectedArrival := arrival.ClassException.ArrivalTime
			message := "Kommt heute um " + expectedArrival + " Uhr (" + arrival.ClassException.Label + ")"
			if arrivesLate {
				message = arrival.ClassException.Label
			}
			warnings[studentID] = append(warnings[studentID], OperationRosterWarning{
				Kind:            "class_arrival_exception",
				Message:         message,
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

// operationalOverview reports whether this caller may see every
// running module of the school (#2380). It reuses the staff record this
// service already resolved instead of looking it up a second time, but asks
// the same setting as every other surface, so a module listed by PlannedNow
// can never 403 on the detail call. Fails closed on a settings fault.
func (s *timetableOperationsService) operationalOverview(ctx context.Context, isAdmin, hasStaff bool) bool {
	claims := jwt.ClaimsFromCtx(ctx)
	allowed, err := authorize.HasOperationalOverviewForResolvedStaff(
		ctx,
		s.deps.Settings,
		claims.IsSchoolScope(),
		s.hasAdministrativeActionAccess(ctx, isAdmin),
		hasStaff,
	)
	if err != nil {
		s.logger().WarnContext(ctx, "operational overview scope check failed for timetable operations",
			slog.String("error", err.Error()))
		return false
	}
	return allowed
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
	if !inst.IsSpontaneous && !now.Before(end) {
		return false
	}
	return (start.After(now.Add(-15*time.Minute)) && start.Before(now.Add(time.Duration(horizonMinutes)*time.Minute))) || start.Before(now)
}

// plannedPastToday is the scope=past complement of plannedNowWindow (#2335):
// completed blocks, plus non-spontaneous planned blocks whose end time has
// passed — those never started and stay "planned" forever, so a pure status
// filter would hide them. Spontaneous planned instances stay in the default
// scope (plannedNowWindow keeps them startable past their end), and cancelled
// and running instances belong to neither list.
func plannedPastToday(inst *scheduleModel.ActivityInstance, now time.Time) bool {
	switch inst.Status {
	case scheduleModel.InstanceStatusCompleted:
		return true
	case scheduleModel.InstanceStatusPlanned:
		return !inst.IsSpontaneous && !now.Before(instanceEndAt(inst, now.Location()))
	default:
		return false
	}
}

func instanceEndAt(inst *scheduleModel.ActivityInstance, loc *time.Location) time.Time {
	return time.Date(inst.Date.Year(), inst.Date.Month(), inst.Date.Day(), inst.EndTime.Hour(), inst.EndTime.Minute(), inst.EndTime.Second(), 0, loc)
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
		ActiveGroupID:         inst.ActiveGroupID,
		CancelReason:          inst.CancelReason,
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
	return time.Date(inst.Date.Year(), inst.Date.Month(), inst.Date.Day(), inst.StartTime.Hour(), inst.StartTime.Minute(), inst.StartTime.Second(), 0, loc)
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
