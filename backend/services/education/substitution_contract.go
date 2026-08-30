package education

import substitution "github.com/moto-nrw/project-phoenix/services/substitution"

type TargetType = substitution.TargetType

const (
	TargetGroupHandover        = substitution.TargetGroupHandover
	TargetScheduleSubstitution = substitution.TargetScheduleSubstitution
)

var (
	ErrNotFound        = substitution.ErrNotFound
	ErrForbidden       = substitution.ErrForbidden
	ErrInvalidTarget   = substitution.ErrInvalidTarget
	ErrInvalidPeriod   = substitution.ErrInvalidPeriod
	ErrNotRunning      = substitution.ErrNotRunning
	ErrAlreadyAssigned = substitution.ErrAlreadyAssigned
	ErrConflict        = substitution.ErrConflict
)

type OperationError = substitution.OperationError
type GroupRef = substitution.GroupRef
type StaffRef = substitution.StaffRef
type Period = substitution.Period
type GroupHandover = substitution.GroupHandover
type OverviewResult = substitution.OverviewResult
type OverviewQuery = substitution.OverviewQuery
type ScheduleAppointmentOverview = substitution.ScheduleAppointmentOverview
type ScheduleAppointmentStaff = substitution.ScheduleAppointmentStaff
type ScheduleOverview = substitution.ScheduleOverview
type GroupHandoverAssignment = substitution.GroupHandoverAssignment
type ScheduleAbsenceChange = substitution.ScheduleAbsenceChange
type ScheduleSubstitutionChange = substitution.ScheduleSubstitutionChange
type SchedulePresenceChange = substitution.SchedulePresenceChange
type ScheduleSubstitutionRemoval = substitution.ScheduleSubstitutionRemoval
type ScheduleSubstitutionAssignment = substitution.ScheduleSubstitutionAssignment
type ScheduleWholeDayAssignment = substitution.ScheduleWholeDayAssignment
type ScheduleAffectedAppointment = substitution.ScheduleAffectedAppointment
type ScheduleTimeConflict = substitution.ScheduleTimeConflict
type ScheduleSubstitutionResult = substitution.ScheduleSubstitutionResult
type ScheduleSubstitutionDayResult = substitution.ScheduleSubstitutionDayResult
type AssignmentResult = substitution.AssignmentResult
type Assignment = substitution.Assignment
type EndRequest = substitution.EndRequest
type SubstitutionModule = substitution.Module
type SubstitutionCaller = substitution.Caller
type ScheduleSubstitutionAdapter = substitution.ScheduleAdapter
