package httpadapter

import (
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
)

func onlySupervisorConflict(err error) render.Renderer {
	return common.ErrorConflictWithCode(err, common.CodeTimetableOnlySupervisorReplacementRequired)
}

// Protected activities name the reason with their own code (#2517): the class
// text "fehlende Berechtigung" would send the user to the school for a row
// nobody may change.
func systemActivityForbidden(err error) render.Renderer {
	return common.ErrorForbiddenWithCode(err, common.CodeTimetableActivitySystemProtected)
}

func timetableTemplateConflict(err error) render.Renderer {
	return common.ErrorConflictWithCode(err, common.CodeTimetableActivityTemplateProtected)
}

// errorRules map activity-service sentinels to HTTP responses. Matched via
// errors.Is against the full error and rendered with the wrapper text.
var errorRules = []common.ErrorRule{
	{Target: activityContract.ErrCategoryNotFound, Render: common.ErrorNotFound},
	{Target: activityContract.ErrGroupNotFound, Render: common.ErrorNotFound},
	{Target: activityContract.ErrScheduleNotFound, Render: common.ErrorNotFound},
	{Target: activityContract.ErrSupervisorNotFound, Render: common.ErrorNotFound},
	{Target: activityContract.ErrStudentNotFound, Render: common.ErrorNotFound},
	{Target: activityContract.ErrGroupFull, Render: common.ErrorConflict},
	{Target: activityContract.ErrAlreadyEnrolled, Render: common.ErrorConflict},
	{Target: activityContract.ErrTimetableTemplateProtected, Render: timetableTemplateConflict},
	{Target: activityContract.ErrOnlySupervisorRequiresReplacement, Render: onlySupervisorConflict},
	{Target: activityContract.ErrSupervisorAlreadyAssigned, Render: common.ErrorConflict},
	{Target: activityContract.ErrPrimarySupervisorRequired, Render: common.ErrorConflict},
	{Target: activityContract.ErrNotEnrolled, Render: common.ErrorNotFound},
	{Target: activityContract.ErrStudentIsAlumnus, Render: common.ErrorInvalidRequest},
	{Target: activityContract.ErrStudentCareEnded, Render: common.ErrorInvalidRequest},
	{Target: activityContract.ErrSystemActivityProtected, Render: systemActivityForbidden},
	{Target: activityContract.ErrSystemCategoryProtected, Render: common.ErrorForbidden},
	{Target: activityContract.ErrSystemCategoryNameReserved, Render: common.ErrorConflict},
	{Target: activityContract.ErrCategoryNameExists, Render: common.ErrorConflict},
	{Target: activityContract.ErrCategoryArchived, Render: common.ErrorConflict},
	{Target: activityContract.ErrGroupClosed, Render: common.ErrorForbidden},
	{Target: activityContract.ErrInvalidAttendanceStatus, Render: common.ErrorInvalidRequest},
	{Target: activityContract.ErrStaffNotFound, Render: common.ErrorNotFound},
}

// ErrorRenderer renders an error to an HTTP response.
var ErrorRenderer = common.RulesRenderer(errorRules, common.ErrorInternalServer)

// categoryErrorRenderer is ErrorRenderer with the ActivityError wrapper peeled
// off, so a category conflict reaches the school as "Eine Kategorie mit diesem
// Namen existiert bereits" rather than "activity error during create category:
// …". Scoped to the category Stammdaten routes (#2131): the other activities
// endpoints keep their current wire strings, which existing consumers and
// tests pin.
var categoryErrorRenderer = common.UnwrapRenderer[*activityContract.ActivityError](errorRules, common.ErrorInternalServer)
