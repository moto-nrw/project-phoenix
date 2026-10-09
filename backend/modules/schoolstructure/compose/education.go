package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/application/groups"
)

type ActiveSupervisorCreator = groups.ActiveSupervisorCreator
type Actor = groups.Actor
type ActorResolver = groups.ActorResolver
type AdditionalSupervisionAssignment = groups.AdditionalSupervisionAssignment
type Assignment = groups.Assignment
type AssignmentResult = groups.AssignmentResult
type Caller = groups.Caller
type CaregiverDirectory = groups.CaregiverDirectory
type ClassAssignmentAudit = groups.ClassAssignmentAudit
type ClassTeacherStore = groups.ClassTeacherStore
type EducationError = groups.EducationError
type EndRequest = groups.EndRequest
type GraduationPresence = groups.GraduationPresence
type GroupHandover = groups.GroupHandover
type GroupHandoverAssignment = groups.GroupHandoverAssignment
type GroupHandoverStore = groups.GroupHandoverStore
type GroupRecords = groups.GroupRecords
type GroupRef = groups.GroupRef
type GroupStore = groups.GroupStore
type GroupTeacherStore = groups.GroupTeacherStore
type HandoverReader = groups.HandoverReader
type OfferingSourceResyncer = groups.OfferingSourceResyncer
type OperationError = groups.OperationError
type OverviewQuery = groups.OverviewQuery
type OverviewResult = groups.OverviewResult
type Period = groups.Period
type Person = groups.Person
type PersonQuery = groups.PersonQuery
type RoomDirectory = groups.RoomDirectory
type RunningSupervision = groups.RunningSupervision
type Runtime = groups.Runtime
type ScheduleAbsenceChange = groups.ScheduleAbsenceChange
type ScheduleAdapter = groups.ScheduleAdapter
type ScheduleAffectedAppointment = groups.ScheduleAffectedAppointment
type ScheduleAppointmentOverview = groups.ScheduleAppointmentOverview
type ScheduleAppointmentStaff = groups.ScheduleAppointmentStaff
type ScheduleOverview = groups.ScheduleOverview
type SchedulePresenceChange = groups.SchedulePresenceChange
type ScheduleSubstitutionAdapter = groups.ScheduleSubstitutionAdapter
type ScheduleSubstitutionAssignment = groups.ScheduleSubstitutionAssignment
type ScheduleSubstitutionChange = groups.ScheduleSubstitutionChange
type ScheduleSubstitutionDayResult = groups.ScheduleSubstitutionDayResult
type ScheduleSubstitutionRemoval = groups.ScheduleSubstitutionRemoval
type ScheduleSubstitutionResult = groups.ScheduleSubstitutionResult
type ScheduleTimeConflict = groups.ScheduleTimeConflict
type ScheduleWholeDayAssignment = groups.ScheduleWholeDayAssignment
type StaffDirectory = groups.StaffDirectory
type StaffLockStore = groups.StaffLockStore
type StaffRef = groups.StaffRef
type StudentCounter = groups.StudentCounter
type SubstitutionAudit = groups.SubstitutionAudit
type SubstitutionCaller = groups.SubstitutionCaller
type SubstitutionDependencies = groups.SubstitutionDependencies
type SubstitutionModule = groups.SubstitutionModule
type TargetType = groups.TargetType

type Teacher = groups.Teacher
type TeacherDirectory = groups.TeacherDirectory
type TeacherPerson = groups.TeacherPerson

var (
	ErrAlreadyAssigned         = groups.ErrAlreadyAssigned
	ErrConflict                = groups.ErrConflict
	ErrDuplicateGroup          = groups.ErrDuplicateGroup
	ErrDuplicateTeacherInGroup = groups.ErrDuplicateTeacherInGroup
	ErrEmptySchoolClass        = groups.ErrEmptySchoolClass
	ErrForbidden               = groups.ErrForbidden
	ErrGroupHasHandover        = groups.ErrGroupHasHandover
	ErrGroupHasStudents        = groups.ErrGroupHasStudents
	ErrGroupNotFound           = groups.ErrGroupNotFound
	ErrGroupTeacherNotFound    = groups.ErrGroupTeacherNotFound
	ErrInvalidDateRange        = groups.ErrInvalidDateRange
	ErrInvalidPeriod           = groups.ErrInvalidPeriod
	ErrInvalidTarget           = groups.ErrInvalidTarget
	ErrNotFound                = groups.ErrNotFound
	ErrNotRunning              = groups.ErrNotRunning
	ErrRoomNotFound            = groups.ErrRoomNotFound
	ErrSelfAssignment          = groups.ErrSelfAssignment
	ErrStaffNotFound           = groups.ErrStaffNotFound
	ErrSubstitutionBackdated   = groups.ErrSubstitutionBackdated
	ErrSubstitutionConflict    = groups.ErrSubstitutionConflict
	ErrSubstitutionNotFound    = groups.ErrSubstitutionNotFound
	ErrTeacherNotFound         = groups.ErrTeacherNotFound
)

const (
	TargetGroupHandover         = groups.TargetGroupHandover
	TargetScheduleSubstitution  = groups.TargetScheduleSubstitution
	TargetAdditionalSupervision = groups.TargetAdditionalSupervision
)
