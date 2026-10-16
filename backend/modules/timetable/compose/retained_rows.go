package compose

import "github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"

// The retained repository adapters, the care plan and workforce rows, the
// cleanup and the legacy repository vocabulary live in internal/application
// (#4084). The roots, the retained services and the suites name them
// through the declarations below.

type (
	ActivityError                      = application.ActivityError
	ActivityInstanceDate               = application.ActivityInstanceDate
	ActivityInstanceQueryOptions       = application.ActivityInstanceQueryOptions
	CalendarAssignment                 = application.CalendarAssignment
	CalendarAssignmentRows             = application.CalendarAssignmentRows
	CalendarInstance                   = application.CalendarInstance
	CalendarInstanceRows               = application.CalendarInstanceRows
	CalendarPeriodReferences           = application.CalendarPeriodReferences
	CalendarPeriodUsage                = application.CalendarPeriodUsage
	CalendarPeriodUsageRepository      = application.CalendarPeriodUsageRepository
	CalendarShift                      = application.CalendarShift
	CalendarShiftRows                  = application.CalendarShiftRows
	CalendarShiftType                  = application.CalendarShiftType
	CalendarShiftTypeRows              = application.CalendarShiftTypeRows
	CallerActivityGroups               = application.CallerActivityGroups
	CareArrivalException               = application.CareArrivalException
	CareArrivalExceptionRows           = application.CareArrivalExceptionRows
	CareArrivalNote                    = application.CareArrivalNote
	CareArrivalNoteRows                = application.CareArrivalNoteRows
	CareArrivalSchedule                = application.CareArrivalSchedule
	CareArrivalScheduleRows            = application.CareArrivalScheduleRows
	CareDate                           = application.CareDate
	CarePickupException                = application.CarePickupException
	CarePickupExceptionRows            = application.CarePickupExceptionRows
	CarePickupNote                     = application.CarePickupNote
	CarePickupNoteRows                 = application.CarePickupNoteRows
	CarePickupSchedule                 = application.CarePickupSchedule
	CarePickupScheduleRows             = application.CarePickupScheduleRows
	CarePlanRows                       = application.CarePlanRows
	CareScheduleFilter                 = application.CareScheduleFilter
	CareScheduleRepositories           = application.CareScheduleRepositories
	CareScheduleRequest                = application.CareScheduleRequest
	CareScheduleRequestDecision        = application.CareScheduleRequestDecision
	CareScheduleRequestFilter          = application.CareScheduleRequestFilter
	CareScheduleRequestRows            = application.CareScheduleRequestRows
	EndedSessionCompletionDependencies = application.EndedSessionCompletionDependencies
	PersonName                         = application.PersonName
	PickupExceptionFilter              = application.PickupExceptionFilter
	PickupExceptionProjection          = application.PickupExceptionProjection
	RecurrenceRuleQueryOptions         = application.RecurrenceRuleQueryOptions
	RepositoryDirectories              = application.RepositoryDirectories
	RetentionSettings                  = application.RetentionSettings
	ShiftType                          = application.ShiftType
	StudentDeletionRecord              = application.StudentDeletionRecord
	StudentStatusDayFilter             = application.StudentStatusDayFilter
	StudentStatusDayProjection         = application.StudentStatusDayProjection
	TimetableCleanupDependencies       = application.TimetableCleanupDependencies
	WorkforceShift                     = application.WorkforceShift
	WorkforceShiftFilter               = application.WorkforceShiftFilter
	WorkforceShiftRows                 = application.WorkforceShiftRows
	WorkforceShiftType                 = application.WorkforceShiftType
	WorkforceShiftTypeRows             = application.WorkforceShiftTypeRows
)

// The retained sentinels keep their identity.
var (
	ErrStudentRepoMissing                = application.ErrStudentRepoMissing
	ErrAlreadyEnrolled                   = application.ErrAlreadyEnrolled
	ErrCareRequestRecordNotDecided       = application.ErrCareRequestRecordNotDecided
	ErrCareRequestRecordNotFound         = application.ErrCareRequestRecordNotFound
	ErrCareRequestRecordNotPending       = application.ErrCareRequestRecordNotPending
	ErrCareScheduleRecordNotFound        = application.ErrCareScheduleRecordNotFound
	ErrCategoryArchived                  = application.ErrCategoryArchived
	ErrCategoryNameExists                = application.ErrCategoryNameExists
	ErrCategoryNotFound                  = application.ErrCategoryNotFound
	ErrEnrollmentNotFound                = application.ErrEnrollmentNotFound
	ErrGroupClosed                       = application.ErrGroupClosed
	ErrGroupFull                         = application.ErrGroupFull
	ErrGroupNotFound                     = application.ErrGroupNotFound
	ErrInvalidAttendanceStatus           = application.ErrInvalidAttendanceStatus
	ErrNotEnrolled                       = application.ErrNotEnrolled
	ErrNotOwner                          = application.ErrNotOwner
	ErrOnlySupervisorRequiresReplacement = application.ErrOnlySupervisorRequiresReplacement
	ErrScheduleNotFound                  = application.ErrScheduleNotFound
	ErrStaffNotFound                     = application.ErrStaffNotFound
	ErrStudentAlreadyEnrolled            = application.ErrStudentAlreadyEnrolled
	ErrStudentCareEnded                  = application.ErrStudentCareEnded
	ErrStudentIsAlumnus                  = application.ErrStudentIsAlumnus
	ErrStudentNotFound                   = application.ErrStudentNotFound
	ErrSupervisorNotFound                = application.ErrSupervisorNotFound
	ErrSystemActivityProtected           = application.ErrSystemActivityProtected
	ErrSystemCategoryNameReserved        = application.ErrSystemCategoryNameReserved
	ErrSystemCategoryProtected           = application.ErrSystemCategoryProtected
	ErrTimetableTemplateProtected        = application.ErrTimetableTemplateProtected
	ErrWorkforceShiftInvalid             = application.ErrWorkforceShiftInvalid
	ErrWorkforceShiftNotFound            = application.ErrWorkforceShiftNotFound
)

// The constructors and helpers of the moved adapters.
var (
	IsRepositoryNotFound             = application.IsRepositoryNotFound
	FindCalendarAssignments          = application.FindCalendarAssignments
	FindCalendarInstances            = application.FindCalendarInstances
	FindCalendarShifts               = application.FindCalendarShifts
	ListCalendarShiftTypes           = application.ListCalendarShiftTypes
	MarkRecordError                  = application.MarkRecordError
	NewActivityCategorySeeder        = application.NewActivityCategorySeeder
	NewCalendarPeriodUsageRepository = application.NewCalendarPeriodUsageRepository
	NewCallerActivityGroups          = application.NewCallerActivityGroups
	NewCareArrivalExceptionRows      = application.NewCareArrivalExceptionRows
	NewCareArrivalNoteRows           = application.NewCareArrivalNoteRows
	NewCareArrivalScheduleRows       = application.NewCareArrivalScheduleRows
	NewCarePickupExceptionRows       = application.NewCarePickupExceptionRows
	NewCarePickupNoteRows            = application.NewCarePickupNoteRows
	NewCarePickupScheduleRows        = application.NewCarePickupScheduleRows
	NewCareScheduleRepositories      = application.NewCareScheduleRepositories
	NewCareScheduleRequestRows       = application.NewCareScheduleRequestRows
	NewCategoryCount                 = application.NewCategoryCount
	NewEndedSessionCompletion        = application.NewEndedSessionCompletion
	NewEnrollmentArrivalSchedules    = application.NewEnrollmentArrivalSchedules
	NewEnrollmentPickupSchedules     = application.NewEnrollmentPickupSchedules
	NewPresenceSessionFacts          = application.NewPresenceSessionFacts
	NewTimetableCleanup              = application.NewTimetableCleanup
	NewTimetableGroupReads           = application.NewTimetableGroupReads
	NewTimetableInstanceReads        = application.NewTimetableInstanceReads
	NewTimetableInstanceStaffReads   = application.NewTimetableInstanceStaffReads
	NewWorkforceShiftRows            = application.NewWorkforceShiftRows
	NewWorkforceShiftTypeRows        = application.NewWorkforceShiftTypeRows
	ParseActivityInstanceDate        = application.ParseActivityInstanceDate
	ShiftTypeCategoryLinker          = application.ShiftTypeCategoryLinker
	WrapDatabaseError                = application.WrapDatabaseError
)

// The workforce shift list orders and the cleanup audit sample cap.
const (
	CleanupAuditSampleCap        = application.CleanupAuditSampleCap
	WorkforceShiftOrderDate      = application.WorkforceShiftOrderDate
	WorkforceShiftOrderStaffID   = application.WorkforceShiftOrderStaffID
	WorkforceShiftOrderStartTime = application.WorkforceShiftOrderStartTime
)
