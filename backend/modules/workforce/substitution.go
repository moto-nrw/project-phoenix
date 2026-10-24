package workforce

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Group substitution target types. A group handover hands a whole education
// group to a substitute for a period; the legacy personnel row predates typed
// targets and is kept readable only.
const (
	GroupSubstitutionTypeGroupHandover = "group_handover"
	GroupSubstitutionTypeLegacy        = "legacy_personnel_substitution"
)

var (
	ErrGroupSubstitutionNotFound = errors.New("group substitution not found")
	ErrGroupSubstitutionExists   = errors.New("group substitution already exists")
	ErrInvalidGroupSubstitution  = errors.New("invalid group substitution input")
)

// InvalidGroupSubstitutionError carries the caller-facing validation reason;
// it unwraps to ErrInvalidGroupSubstitution.
type InvalidGroupSubstitutionError struct{ Reason string }

func (e *InvalidGroupSubstitutionError) Error() string { return e.Reason }
func (e *InvalidGroupSubstitutionError) Unwrap() error { return ErrInvalidGroupSubstitution }

// GroupSubstitution is a temporary assignment of a substitute staff member to
// an education group. Dates are calendar days in DateLayout, inclusive.
type GroupSubstitution struct {
	ID                int64
	TenantID          int64
	TargetType        string
	GroupID           int64
	RegularStaffID    *int64
	SubstituteStaffID int64
	StartDate         string
	EndDate           string
	Reason            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// GroupSubstitutionFilter narrows a listing; set fields combine with AND.
// On selects rows covering the day; OverlapFrom/OverlapTo select rows
// intersecting [from, to]; StaffID matches the regular or the substitute.
// TenantID is an explicit tenant predicate on top of the ambient tenant; a
// value for another tenant therefore matches nothing.
type GroupSubstitutionFilter struct {
	TenantID          *int64
	GroupID           int64
	GroupIDs          []int64
	SubstituteStaffID int64
	RegularStaffID    int64
	StaffID           int64
	TargetType        string
	On                string
	OverlapFrom       string
	OverlapTo         string
	EndsOnOrAfter     string
	ReasonContains    string
	Limit             int
	Offset            int
}

// SubstitutionQuery reads education.group_substitution.
type SubstitutionQuery interface {
	FindGroupSubstitution(context.Context, int64) (GroupSubstitution, error)
	// LockGroupSubstitution is FindGroupSubstitution with a row lock held
	// until the surrounding transaction finishes.
	LockGroupSubstitution(context.Context, int64) (GroupSubstitution, error)
	ListGroupSubstitutions(context.Context, GroupSubstitutionFilter) ([]GroupSubstitution, error)
}

// SubstitutionCommand writes education.group_substitution.
type SubstitutionCommand interface {
	CreateGroupSubstitution(context.Context, GroupSubstitution) (GroupSubstitution, error)
	UpdateGroupSubstitution(context.Context, GroupSubstitution) (GroupSubstitution, error)
	DeleteGroupSubstitution(context.Context, int64) error
	// DeleteGroupSubstitutionsForStaff removes rows involving the staff member
	// as regular or substitute that end on or after from. Past rows stay as
	// history; staff offboarding uses it.
	DeleteGroupSubstitutionsForStaff(ctx context.Context, staffID int64, from string) (int64, error)
	// LockGroupSubstitutions takes a SHARE ROW EXCLUSIVE table lock on the
	// group substitution rows for the caller's transaction, so a caregiver
	// capability re-check cannot race a concurrent substitution write.
	LockGroupSubstitutions(context.Context) error
}

// --- substitution operations contract ---

// SubstitutionTargetType names what a substitution assignment addresses.
type SubstitutionTargetType string

const (
	TargetGroupHandover         SubstitutionTargetType = "group_handover"
	TargetScheduleSubstitution  SubstitutionTargetType = "schedule_substitution"
	TargetAdditionalSupervision SubstitutionTargetType = "additional_supervision"
)

var (
	ErrSubstitutionNotFound        = errors.New("substitution target not found")
	ErrSubstitutionForbidden       = errors.New("substitution action forbidden")
	ErrSubstitutionInvalidTarget   = errors.New("invalid substitution target")
	ErrSubstitutionInvalidPeriod   = errors.New("invalid substitution period")
	ErrSubstitutionNotRunning      = errors.New("substitution is not running")
	ErrSubstitutionAlreadyAssigned = errors.New("substitution already assigned")
	ErrSubstitutionConflict        = errors.New("substitution conflict")
	ErrSubstitutionSelfAssignment  = errors.New("substitution self assignment")
)

// SubstitutionOperationError carries the stable code and caller-facing message
// of a rejected substitution operation. Target is one of the sentinels above.
type SubstitutionOperationError struct {
	Target  error
	Code    string
	Message string
	Cause   error
	// Details are the values a refusal names and Field the request field it
	// is about (#2516); both reach the response unchanged.
	Details SubstitutionRefusalValues
	Field   string
}

// InvalidSubstitutionPeriod refuses a period whose day the caller sent
// malformed or empty, with the German message the substitution page shows.
func InvalidSubstitutionPeriod(cause error) *SubstitutionOperationError {
	return &SubstitutionOperationError{
		Target: ErrSubstitutionInvalidPeriod, Code: codeSubstitutionInvalidPeriod, Message: "Der Zeitraum ist ungültig.", Cause: cause,
	}
}

// codeSubstitutionInvalidPeriod is the registered error code of the refusal.
const codeSubstitutionInvalidPeriod = "substitutions.invalid_period"

// SubstitutionRefusalValues are the values a refused schedule substitution
// names: the failing day (YYYY-MM-DD) or a cap. Unset values stay off the wire.
type SubstitutionRefusalValues struct {
	Date string
	Max  int
}

func (e *SubstitutionOperationError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *SubstitutionOperationError) Is(target error) bool { return target == e.Target }
func (e *SubstitutionOperationError) Unwrap() error        { return e.Cause }

// SubstitutionCaller is the authenticated principal an operation runs for.
type SubstitutionCaller struct {
	AccountID     int64
	TenantID      int64
	Scope         string
	Roles         []string
	HasPermission func(string) bool
	Admin         bool
}

type GroupRef struct {
	ID   int64  `json:"id,string"`
	Name string `json:"name"`
}

type StaffRef struct {
	ID       int64  `json:"id,string"`
	FullName string `json:"full_name"`
	// IsExternal marks an external caregiver, with or without a moto account
	// (#3823, #4573), and ExternalAccountStatus is then "account" |
	// "invitation_pending" | "none", ExternalOrganization the organization
	// of the external caregiver record (empty when none is recorded). The running
	// supervisions' targets and supervisors carry all three; the schedule
	// targets carry IsExternal only.
	IsExternal            bool   `json:"is_external,omitempty"`
	ExternalAccountStatus string `json:"external_account_status,omitempty"`
	ExternalOrganization  string `json:"external_organization,omitempty"`
}

type Period struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type GroupHandover struct {
	ID     int64                  `json:"id,string"`
	Type   SubstitutionTargetType `json:"type"`
	Group  GroupRef               `json:"group"`
	Target StaffRef               `json:"target"`
	Period Period                 `json:"period"`
	CanEnd bool                   `json:"can_end"`
}

type RunningSupervision struct {
	ID                       int64                  `json:"id,string"`
	Type                     SubstitutionTargetType `json:"type"`
	Name                     string                 `json:"name"`
	RoomName                 string                 `json:"room_name,omitempty"`
	Supervisors              []StaffRef             `json:"supervisors"`
	AvailableTargets         []StaffRef             `json:"available_targets"`
	IsCurrentUserSupervising bool                   `json:"is_current_user_supervising"`
	CanAssign                bool                   `json:"can_assign"`
}

type ScheduleAppointmentStaff struct {
	AssignmentID int64    `json:"assignment_id"`
	Staff        StaffRef `json:"staff"`
	IsAbsent     bool     `json:"is_absent"`
	IsSubstitute bool     `json:"is_substitute"`
	CanEnd       bool     `json:"can_end"`
}

// ScheduleAppointmentOverview is one timetable appointment with the staff
// planned for it. Date is a calendar day in DateLayout.
type ScheduleAppointmentOverview struct {
	ID        int64                      `json:"id"`
	Type      SubstitutionTargetType     `json:"type"`
	Date      string                     `json:"date"`
	StartTime string                     `json:"start_time"`
	EndTime   string                     `json:"end_time"`
	Title     string                     `json:"title"`
	Status    string                     `json:"status"`
	Staff     []ScheduleAppointmentStaff `json:"staff"`
}

type SubstitutionOverviewResult struct {
	GroupHandovers       []GroupHandover               `json:"group_handovers"`
	Groups               []GroupRef                    `json:"groups"`
	Targets              []StaffRef                    `json:"targets"`
	ScheduleAppointments []ScheduleAppointmentOverview `json:"schedule_appointments"`
	ScheduleTargets      []StaffRef                    `json:"schedule_targets"`
	RunningSupervisions  []RunningSupervision          `json:"running_supervisions"`
}

// SubstitutionOverviewQuery selects what the overview shows. Dates are
// calendar days in DateLayout; empty means unset.
type SubstitutionOverviewQuery struct {
	GroupID                int64
	ActiveGroupID          int64
	On                     string
	IncludeTargets         bool
	ScheduleFrom           string
	ScheduleTo             string
	IncludeScheduleTargets bool
}

type GroupHandoverAssignment struct {
	GroupID       int64
	TargetStaffID int64
	StartDate     string
	EndDate       string
}

type AdditionalSupervisionAssignment struct{ ActiveGroupID, TargetStaffID int64 }

type ScheduleAbsenceChange struct {
	StaffID     int64    `json:"staff_id"`
	Reason      *string  `json:"reason,omitempty"`
	InstanceIDs *[]int64 `json:"instance_ids,omitempty"`
}

type ScheduleSubstitutionChange struct {
	AbsentStaffID     int64    `json:"absent_staff_id"`
	SubstituteStaffID int64    `json:"substitute_staff_id"`
	Reason            *string  `json:"reason,omitempty"`
	InstanceIDs       *[]int64 `json:"instance_ids,omitempty"`
}

type SchedulePresenceChange struct {
	StaffID     int64    `json:"staff_id"`
	InstanceIDs *[]int64 `json:"instance_ids,omitempty"`
}

type ScheduleSubstitutionRemoval struct {
	StaffID     int64    `json:"staff_id"`
	InstanceIDs *[]int64 `json:"instance_ids,omitempty"`
}

// ScheduleWholeDayAssignment marks a staff member absent for whole days and
// optionally names the substitute. Dates are calendar days in DateLayout.
type ScheduleWholeDayAssignment struct {
	AbsentStaffID     int64
	SubstituteStaffID *int64
	Dates             []string
	Reason            *string
}

// Days parses the whole days of the assignment. A whole-day absence names
// each of its days: an empty or malformed day refuses the period
// (InvalidSubstitutionPeriod).
func (a ScheduleWholeDayAssignment) Days() ([]calendar.Date, error) {
	days := make([]calendar.Date, 0, len(a.Dates))
	for _, raw := range a.Dates {
		if raw == "" {
			return nil, InvalidSubstitutionPeriod(nil)
		}
		day, err := calendar.ParseDate(raw)
		if err != nil {
			return nil, InvalidSubstitutionPeriod(err)
		}
		days = append(days, day)
	}
	return days, nil
}

type ScheduleSubstitutionAssignment struct {
	InstanceID           int64
	UnderstaffedAck      *bool
	UnderstaffedNote     *string
	Absences             []ScheduleAbsenceChange
	Substitutions        []ScheduleSubstitutionChange
	SubstitutionRemovals []ScheduleSubstitutionRemoval
	Presences            []SchedulePresenceChange
	WholeDays            *ScheduleWholeDayAssignment
}

type ScheduleAffectedAppointment struct {
	InstanceID int64  `json:"instance_id"`
	Title      string `json:"title"`
	StartTime  string `json:"start_time"`
	Action     string `json:"action"`
}

type ScheduleTimeConflict struct {
	Kind       string `json:"kind"`
	InstanceID int64  `json:"instance_id"`
	OtherID    int64  `json:"other_instance_id"`
	Message    string `json:"message"`
}

type ScheduleSubstitutionDayResult struct {
	Date                 string                        `json:"date"`
	AffectedAppointments []ScheduleAffectedAppointment `json:"affected_instances"`
	Warnings             []ScheduleTimeConflict        `json:"warnings"`
}

type ScheduleSubstitutionResult struct {
	InstanceID           int64                           `json:"instance_id"`
	Cancelled            bool                            `json:"cancelled"`
	UnderstaffedAck      bool                            `json:"understaffed_ack"`
	AffectedAppointments []ScheduleAffectedAppointment   `json:"affected_instances"`
	Warnings             []ScheduleTimeConflict          `json:"warnings"`
	Days                 []ScheduleSubstitutionDayResult `json:"days,omitempty"`
	TotalAffected        int                             `json:"total_affected"`
}

type SubstitutionAssignment struct {
	Type                  SubstitutionTargetType
	GroupHandover         *GroupHandoverAssignment
	AdditionalSupervision *AdditionalSupervisionAssignment
	ScheduleSubstitution  *ScheduleSubstitutionAssignment
}

type SubstitutionAssignmentResult struct {
	ID                   int64                       `json:"id,string"`
	Type                 SubstitutionTargetType      `json:"type"`
	Group                *GroupRef                   `json:"group,omitempty"`
	ActiveGroupID        int64                       `json:"active_group_id,string,omitempty"`
	Target               StaffRef                    `json:"target"`
	Period               *Period                     `json:"period,omitempty"`
	CanEnd               bool                        `json:"can_end,omitempty"`
	ScheduleSubstitution *ScheduleSubstitutionResult `json:"schedule_substitution,omitempty"`
}

type SubstitutionEndRequest struct {
	Type SubstitutionTargetType
	ID   int64
}
