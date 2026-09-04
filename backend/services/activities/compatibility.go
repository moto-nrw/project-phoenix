// Package activities retains the established service contract while the
// Timetable & Activities owner composes its implementation.
package activities

import legacy "github.com/moto-nrw/project-phoenix/modules/timetable/compose/legacy"

type ActivityError = legacy.ActivityError
type ActivityGroupWithOccupancy = legacy.ActivityGroupWithOccupancy
type ActivityService = legacy.ActivityService
type CategoryInput = legacy.CategoryInput
type Service = legacy.Service

var NewService = legacy.NewService

var (
	ErrCategoryNotFound                  = legacy.ErrCategoryNotFound
	ErrGroupNotFound                     = legacy.ErrGroupNotFound
	ErrScheduleNotFound                  = legacy.ErrScheduleNotFound
	ErrSupervisorNotFound                = legacy.ErrSupervisorNotFound
	ErrEnrollmentNotFound                = legacy.ErrEnrollmentNotFound
	ErrStudentNotFound                   = legacy.ErrStudentNotFound
	ErrGroupFull                         = legacy.ErrGroupFull
	ErrAlreadyEnrolled                   = legacy.ErrAlreadyEnrolled
	ErrStudentAlreadyEnrolled            = legacy.ErrStudentAlreadyEnrolled
	ErrNotEnrolled                       = legacy.ErrNotEnrolled
	ErrStudentIsAlumnus                  = legacy.ErrStudentIsAlumnus
	ErrStudentCareEnded                  = legacy.ErrStudentCareEnded
	ErrInvalidAttendanceStatus           = legacy.ErrInvalidAttendanceStatus
	ErrGroupClosed                       = legacy.ErrGroupClosed
	ErrStaffNotFound                     = legacy.ErrStaffNotFound
	ErrNotOwner                          = legacy.ErrNotOwner
	ErrOnlySupervisorRequiresReplacement = legacy.ErrOnlySupervisorRequiresReplacement
	ErrSystemActivityProtected           = legacy.ErrSystemActivityProtected
	ErrTimetableTemplateProtected        = legacy.ErrTimetableTemplateProtected
	ErrSystemCategoryProtected           = legacy.ErrSystemCategoryProtected
	ErrSystemCategoryNameReserved        = legacy.ErrSystemCategoryNameReserved
	ErrCategoryNameExists                = legacy.ErrCategoryNameExists
	ErrCategoryArchived                  = legacy.ErrCategoryArchived
)
