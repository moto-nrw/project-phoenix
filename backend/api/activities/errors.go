package activities

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/services/activities"
)

const errorCodeOnlySupervisorReplacementRequired = "ONLY_SUPERVISOR_REPLACEMENT_REQUIRED"

// ErrorRenderer renders an error to an HTTP response
func ErrorRenderer(err error) render.Renderer {
	// Check if the error is a specific activity service error
	if actErr, ok := err.(*activities.ActivityError); ok {
		// Map specific activity service errors to appropriate HTTP status codes
		switch {
		case errors.Is(actErr, activities.ErrCategoryNotFound):
			return common.ErrorNotFound(actErr)
		case errors.Is(actErr, activities.ErrGroupNotFound):
			return common.ErrorNotFound(actErr)
		case errors.Is(actErr, activities.ErrScheduleNotFound):
			return common.ErrorNotFound(actErr)
		case errors.Is(actErr, activities.ErrSupervisorNotFound):
			return common.ErrorNotFound(actErr)
		case errors.Is(actErr, activities.ErrEnrollmentNotFound):
			return common.ErrorNotFound(actErr)
		case errors.Is(actErr, activities.ErrGroupFull):
			return common.ErrorConflict(actErr)
		case errors.Is(actErr, activities.ErrAlreadyEnrolled):
			return common.ErrorConflict(actErr)
		case errors.Is(actErr, activities.ErrTimetableTemplateProtected):
			return common.ErrorConflict(actErr)
		case errors.Is(actErr, activities.ErrOnlySupervisorRequiresReplacement):
			return common.ErrorConflictWithCode(actErr, errorCodeOnlySupervisorReplacementRequired)
		case errors.Is(actErr, activities.ErrNotEnrolled):
			return common.ErrorNotFound(actErr)
		case errors.Is(actErr, activities.ErrSystemActivityProtected):
			return common.ErrorForbidden(actErr)
		case errors.Is(actErr, activities.ErrGroupClosed):
			return common.ErrorForbidden(actErr)
		case errors.Is(actErr, activities.ErrInvalidAttendanceStatus):
			return common.ErrorInvalidRequest(actErr)
		case errors.Is(actErr, activities.ErrStaffNotFound):
			return common.ErrorNotFound(actErr)
		default:
			return common.ErrorInternalServer(actErr)
		}
	}

	// For unknown errors, return a generic internal server error
	return common.ErrorInternalServer(err)
}
