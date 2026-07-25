package activities

import (
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/services/activities"
)

const errorCodeOnlySupervisorReplacementRequired = "ONLY_SUPERVISOR_REPLACEMENT_REQUIRED"

func onlySupervisorConflict(err error) render.Renderer {
	return common.ErrorConflictWithCode(err, errorCodeOnlySupervisorReplacementRequired)
}

// errorRules map activity-service sentinels to HTTP responses. Matched via
// errors.Is against the full error and rendered with the wrapper text.
var errorRules = []common.ErrorRule{
	{Target: activities.ErrCategoryNotFound, Render: common.ErrorNotFound},
	{Target: activities.ErrGroupNotFound, Render: common.ErrorNotFound},
	{Target: activities.ErrScheduleNotFound, Render: common.ErrorNotFound},
	{Target: activities.ErrSupervisorNotFound, Render: common.ErrorNotFound},
	{Target: activities.ErrEnrollmentNotFound, Render: common.ErrorNotFound},
	{Target: activities.ErrGroupFull, Render: common.ErrorConflict},
	{Target: activities.ErrAlreadyEnrolled, Render: common.ErrorConflict},
	{Target: activities.ErrTimetableTemplateProtected, Render: common.ErrorConflict},
	{Target: activities.ErrOnlySupervisorRequiresReplacement, Render: onlySupervisorConflict},
	{Target: activities.ErrNotEnrolled, Render: common.ErrorNotFound},
	{Target: activities.ErrStudentIsAlumnus, Render: common.ErrorInvalidRequest},
	{Target: activities.ErrSystemActivityProtected, Render: common.ErrorForbidden},
	{Target: activities.ErrGroupClosed, Render: common.ErrorForbidden},
	{Target: activities.ErrInvalidAttendanceStatus, Render: common.ErrorInvalidRequest},
	{Target: activities.ErrStaffNotFound, Render: common.ErrorNotFound},
}

// ErrorRenderer renders an error to an HTTP response.
var ErrorRenderer = common.RulesRenderer(errorRules, common.ErrorInternalServer)
