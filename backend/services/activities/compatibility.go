// Package activities retains the established service contract while the
// Timetable & Activities owner composes its implementation.
package activities

import timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"

type ActivityError = timetableCompose.ActivityError
type ActivityGroupWithOccupancy = timetableCompose.ActivityGroupWithOccupancy
type ActivityService = timetableCompose.ActivityService
type CategoryInput = timetableCompose.CategoryInput
type Service = timetableCompose.Service

var NewService = timetableCompose.NewService

var (
	ErrCategoryNotFound                  = timetableCompose.ErrCategoryNotFound
	ErrGroupNotFound                     = timetableCompose.ErrGroupNotFound
	ErrScheduleNotFound                  = timetableCompose.ErrScheduleNotFound
	ErrSupervisorNotFound                = timetableCompose.ErrSupervisorNotFound
	ErrEnrollmentNotFound                = timetableCompose.ErrEnrollmentNotFound
	ErrStudentNotFound                   = timetableCompose.ErrStudentNotFound
	ErrGroupFull                         = timetableCompose.ErrGroupFull
	ErrAlreadyEnrolled                   = timetableCompose.ErrAlreadyEnrolled
	ErrStudentAlreadyEnrolled            = timetableCompose.ErrStudentAlreadyEnrolled
	ErrNotEnrolled                       = timetableCompose.ErrNotEnrolled
	ErrStudentIsAlumnus                  = timetableCompose.ErrStudentIsAlumnus
	ErrStudentCareEnded                  = timetableCompose.ErrStudentCareEnded
	ErrInvalidAttendanceStatus           = timetableCompose.ErrInvalidAttendanceStatus
	ErrGroupClosed                       = timetableCompose.ErrGroupClosed
	ErrStaffNotFound                     = timetableCompose.ErrStaffNotFound
	ErrNotOwner                          = timetableCompose.ErrNotOwner
	ErrOnlySupervisorRequiresReplacement = timetableCompose.ErrOnlySupervisorRequiresReplacement
	ErrSystemActivityProtected           = timetableCompose.ErrSystemActivityProtected
	ErrTimetableTemplateProtected        = timetableCompose.ErrTimetableTemplateProtected
	ErrSystemCategoryProtected           = timetableCompose.ErrSystemCategoryProtected
	ErrSystemCategoryNameReserved        = timetableCompose.ErrSystemCategoryNameReserved
	ErrCategoryNameExists                = timetableCompose.ErrCategoryNameExists
	ErrCategoryArchived                  = timetableCompose.ErrCategoryArchived
)
