package schoolstructure

import "github.com/moto-nrw/project-phoenix/modules/schoolstructure/contract"

type GroupManagement = contract.GroupManagement
type GroupRoom = contract.GroupRoom
type GroupListQuery = contract.GroupListQuery
type Teacher = contract.Teacher
type TeacherPerson = contract.TeacherPerson
type EducationError = contract.EducationError
type TargetType = contract.TargetType
type OperationError = contract.OperationError
type GroupRef = contract.GroupRef
type StaffRef = contract.StaffRef
type Period = contract.Period
type GroupHandover = contract.GroupHandover
type OverviewResult = contract.OverviewResult
type RunningSupervision = contract.RunningSupervision
type OverviewQuery = contract.OverviewQuery
type ScheduleAppointmentOverview = contract.ScheduleAppointmentOverview
type ScheduleAppointmentStaff = contract.ScheduleAppointmentStaff
type ScheduleOverview = contract.ScheduleOverview
type GroupHandoverAssignment = contract.GroupHandoverAssignment
type AdditionalSupervisionAssignment = contract.AdditionalSupervisionAssignment
type ScheduleAbsenceChange = contract.ScheduleAbsenceChange
type ScheduleSubstitutionChange = contract.ScheduleSubstitutionChange
type SchedulePresenceChange = contract.SchedulePresenceChange
type ScheduleSubstitutionRemoval = contract.ScheduleSubstitutionRemoval
type ScheduleSubstitutionAssignment = contract.ScheduleSubstitutionAssignment
type ScheduleWholeDayAssignment = contract.ScheduleWholeDayAssignment
type ScheduleAffectedAppointment = contract.ScheduleAffectedAppointment
type ScheduleTimeConflict = contract.ScheduleTimeConflict
type ScheduleSubstitutionResult = contract.ScheduleSubstitutionResult
type ScheduleSubstitutionDayResult = contract.ScheduleSubstitutionDayResult
type AssignmentResult = contract.AssignmentResult
type Assignment = contract.Assignment
type EndRequest = contract.EndRequest
type Caller = contract.Caller
type SubstitutionModule = contract.SubstitutionModule
type ScheduleAdapter = contract.ScheduleAdapter
type SubstitutionCaller = contract.SubstitutionCaller
type ScheduleSubstitutionAdapter = contract.ScheduleSubstitutionAdapter

var (
	ErrEducationGroupNotFound  = contract.ErrEducationGroupNotFound
	ErrTeacherNotFound         = contract.ErrTeacherNotFound
	ErrStaffNotFound           = contract.ErrStaffNotFound
	ErrEmptySchoolClass        = contract.ErrEmptySchoolClass
	ErrGroupTeacherNotFound    = contract.ErrGroupTeacherNotFound
	ErrSubstitutionNotFound    = contract.ErrSubstitutionNotFound
	ErrRoomNotFound            = contract.ErrRoomNotFound
	ErrDuplicateGroup          = contract.ErrDuplicateGroup
	ErrDuplicateTeacherInGroup = contract.ErrDuplicateTeacherInGroup
	ErrSubstitutionConflict    = contract.ErrSubstitutionConflict
	ErrInvalidDateRange        = contract.ErrInvalidDateRange
	ErrSubstitutionBackdated   = contract.ErrSubstitutionBackdated
	ErrGroupHasStudents        = contract.ErrGroupHasStudents
	ErrGroupHasHandover        = contract.ErrGroupHasHandover
	ErrNotFound                = contract.ErrNotFound
	ErrForbidden               = contract.ErrForbidden
	ErrInvalidTarget           = contract.ErrInvalidTarget
	ErrInvalidPeriod           = contract.ErrInvalidPeriod
	ErrNotRunning              = contract.ErrNotRunning
	ErrAlreadyAssigned         = contract.ErrAlreadyAssigned
	ErrConflict                = contract.ErrConflict
	ErrSelfAssignment          = contract.ErrSelfAssignment
)

const (
	TargetGroupHandover         = contract.TargetGroupHandover
	TargetScheduleSubstitution  = contract.TargetScheduleSubstitution
	TargetAdditionalSupervision = contract.TargetAdditionalSupervision
)

type SubstitutionRefusalValues = contract.SubstitutionRefusalValues
