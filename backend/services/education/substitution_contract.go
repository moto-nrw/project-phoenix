// Package education owns the shared, storage-neutral substitution module contract.
package education

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

type TargetType string

const (
	TargetGroupHandover        TargetType = "group_handover"
	TargetScheduleSubstitution TargetType = "schedule_substitution"
)

var (
	ErrNotFound        = errors.New("substitution target not found")
	ErrForbidden       = errors.New("substitution action forbidden")
	ErrInvalidTarget   = errors.New("invalid substitution target")
	ErrInvalidPeriod   = errors.New("invalid substitution period")
	ErrNotRunning      = errors.New("substitution is not running")
	ErrAlreadyAssigned = errors.New("substitution already assigned")
	ErrConflict        = errors.New("substitution conflict")
)

type OperationError struct {
	Target  error
	Code    string
	Message string
	Cause   error
}

func (e *OperationError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *OperationError) Is(target error) bool { return target == e.Target }
func (e *OperationError) Unwrap() error        { return e.Cause }

type GroupRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type StaffRef struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
}

type Period struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type GroupHandover struct {
	ID     int64      `json:"id"`
	Type   TargetType `json:"type"`
	Group  GroupRef   `json:"group"`
	Target StaffRef   `json:"target"`
	Period Period     `json:"period"`
	CanEnd bool       `json:"can_end"`
}

type OverviewResult struct {
	GroupHandovers       []GroupHandover               `json:"group_handovers"`
	Targets              []StaffRef                    `json:"targets"`
	ScheduleAppointments []ScheduleAppointmentOverview `json:"schedule_appointments"`
	ScheduleTargets      []StaffRef                    `json:"schedule_targets"`
}

type OverviewQuery struct {
	GroupID                int64
	On                     *timezone.Date
	IncludeTargets         bool
	ScheduleFrom           *timezone.Date
	ScheduleTo             *timezone.Date
	IncludeScheduleTargets bool
}

type ScheduleAppointmentOverview struct {
	ID        int64                      `json:"id"`
	Type      TargetType                 `json:"type"`
	Date      timezone.Date              `json:"date"`
	StartTime string                     `json:"start_time"`
	EndTime   string                     `json:"end_time"`
	Title     string                     `json:"title"`
	Status    string                     `json:"status"`
	Staff     []ScheduleAppointmentStaff `json:"staff"`
}

type ScheduleAppointmentStaff struct {
	AssignmentID int64    `json:"assignment_id"`
	Staff        StaffRef `json:"staff"`
	IsAbsent     bool     `json:"is_absent"`
	IsSubstitute bool     `json:"is_substitute"`
	CanEnd       bool     `json:"can_end"`
}

type ScheduleOverview struct {
	Appointments []ScheduleAppointmentOverview
	Targets      []StaffRef
}

type GroupHandoverAssignment struct {
	GroupID       int64
	TargetStaffID int64
	StartDate     *timezone.Date
	EndDate       *timezone.Date
}

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

type ScheduleWholeDayAssignment struct {
	AbsentStaffID     int64
	SubstituteStaffID *int64
	Dates             []timezone.Date
	Reason            *string
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

type ScheduleSubstitutionResult struct {
	InstanceID           int64                           `json:"instance_id"`
	Cancelled            bool                            `json:"cancelled"`
	UnderstaffedAck      bool                            `json:"understaffed_ack"`
	AffectedAppointments []ScheduleAffectedAppointment   `json:"affected_instances"`
	Warnings             []ScheduleTimeConflict          `json:"warnings"`
	Days                 []ScheduleSubstitutionDayResult `json:"days,omitempty"`
	TotalAffected        int                             `json:"total_affected"`
	AfterCommit          func(context.Context)           `json:"-"`
}

type ScheduleSubstitutionDayResult struct {
	Date                 timezone.Date                 `json:"date"`
	AffectedAppointments []ScheduleAffectedAppointment `json:"affected_instances"`
	Warnings             []ScheduleTimeConflict        `json:"warnings"`
}

type AssignmentResult struct {
	*GroupHandover
	ScheduleSubstitution *ScheduleSubstitutionResult `json:"schedule_substitution,omitempty"`
}

type Assignment struct {
	Type                 TargetType
	GroupHandover        *GroupHandoverAssignment
	ScheduleSubstitution *ScheduleSubstitutionAssignment
}

type EndRequest struct {
	Type TargetType
	ID   int64
}

type Caller struct {
	AccountID     int64
	TenantID      int64
	Scope         string
	Roles         []string
	HasPermission func(string) bool
	Admin         bool
}

type Module interface {
	Overview(context.Context, Caller, OverviewQuery) (*OverviewResult, error)
	Assign(context.Context, Caller, Assignment) (*AssignmentResult, error)
	End(context.Context, Caller, EndRequest) error
}

type ScheduleAdapter interface {
	Overview(context.Context, timezone.Date, timezone.Date, bool, bool) (*ScheduleOverview, error)
	Assign(context.Context, ScheduleSubstitutionAssignment, int64) (*ScheduleSubstitutionResult, error)
	End(context.Context, int64, int64) (*ScheduleSubstitutionResult, error)
}

type SubstitutionCaller = Caller
type SubstitutionModule = Module
type ScheduleSubstitutionAdapter = ScheduleAdapter
