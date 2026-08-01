package active

import (
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// newErrResponse builds a common.ErrResponse carrying this package's
// historical human-readable status classification (e.g. "Room Conflict")
// instead of api/common's literal "error". The wire bytes are pinned by
// wire_format_test.go — normalizing the status values to "error" is a
// separate, frontend-audited change, not part of the struct consolidation
// (issue #575 B1).
func newErrResponse(status int, statusText string, err error) *common.ErrResponse {
	return &common.ErrResponse{
		Err:            err,
		HTTPStatusCode: status,
		Status:         statusText,
		ErrorText:      err.Error(),
	}
}

// statusText adapts newErrResponse to the ErrorRule.Render shape.
func statusText(status int, text string) func(error) render.Renderer {
	return func(err error) render.Renderer { return newErrResponse(status, text, err) }
}

// errorRules maps active-service sentinels to HTTP status + the package's
// human-text status classification. Matched via errors.Is, so both bare
// sentinels and *activeSvc.ActiveError-wrapped ones classify identically.
//
// The ErrStudentAlreadyActive 409: issue #844 added a DB-level partial
// unique index on active.visits; the active service translates the
// resulting 23505 to ErrStudentAlreadyActive for ALL CreateVisit callers,
// so the admin POST /active/visits route must answer 409 like the IoT
// checkin path, not 400.
var errorRules = []common.ErrorRule{
	{Target: activeSvc.ErrActiveGroupNotFound, Render: statusText(http.StatusNotFound, "Active Group Not Found")},
	{Target: activeSvc.ErrVisitNotFound, Render: statusText(http.StatusNotFound, "Visit Not Found")},
	{Target: activeSvc.ErrGroupSupervisorNotFound, Render: statusText(http.StatusNotFound, "Group Supervisor Not Found")},
	{Target: activeSvc.ErrCombinedGroupNotFound, Render: statusText(http.StatusNotFound, "Combined Group Not Found")},
	{Target: activeSvc.ErrGroupMappingNotFound, Render: statusText(http.StatusNotFound, "Group Mapping Not Found")},
	{Target: activeSvc.ErrInvalidData, Render: statusText(http.StatusBadRequest, "Invalid Data")},
	{Target: activeSvc.ErrActiveGroupAlreadyEnded, Render: statusText(http.StatusBadRequest, "Active Group Already Ended")},
	{Target: activeSvc.ErrVisitAlreadyEnded, Render: statusText(http.StatusBadRequest, "Visit Already Ended")},
	{Target: activeSvc.ErrSupervisionAlreadyEnded, Render: statusText(http.StatusBadRequest, "Supervision Already Ended")},
	{Target: activeSvc.ErrCombinedGroupAlreadyEnded, Render: statusText(http.StatusBadRequest, "Combined Group Already Ended")},
	{Target: activeSvc.ErrGroupAlreadyInCombination, Render: statusText(http.StatusBadRequest, "Group Already In Combination")},
	{Target: activeSvc.ErrStudentAlreadyInGroup, Render: statusText(http.StatusBadRequest, "Student Already In Group")},
	{Target: activeSvc.ErrStudentAlreadyActive, Render: statusText(http.StatusConflict, "Student Already Has Active Visit")},
	{Target: activeSvc.ErrStudentsNotPresent, Render: statusText(http.StatusConflict, "Students Not Present")},
	{Target: activeSvc.ErrStudentMoveForbidden, Render: statusText(http.StatusForbidden, "Forbidden")},
	{Target: activeSvc.ErrStaffAlreadySupervising, Render: statusText(http.StatusBadRequest, "Staff Already Supervising This Group")},
	{Target: activeSvc.ErrCannotDeleteActiveGroup, Render: statusText(http.StatusBadRequest, "Cannot Delete Active Group With Active Visits")},
	{Target: activeSvc.ErrInvalidTimeRange, Render: statusText(http.StatusBadRequest, "Invalid Time Range")},
	{Target: activeSvc.ErrRoomConflict, Render: statusText(http.StatusConflict, "Room Conflict")},
	{Target: activeSvc.ErrStudentNotFound, Render: statusText(http.StatusNotFound, "Student Not Found")},
	{Target: activeSvc.ErrStaffNotFound, Render: statusText(http.StatusNotFound, "Staff Not Found")},
	// A graduated (alumnus) student is treated like an unknown/absent student
	// (404), matching the IoT check-in mapper — a stale web/timetable request or
	// a graduation race must not fall through to a 500 (#405).
	{Target: activeSvc.ErrStudentGraduated, Render: statusText(http.StatusNotFound, "Student Graduated")},
}

// ErrorRenderer returns a render.Renderer for the given error
var ErrorRenderer = common.RulesRenderer(errorRules, statusText(http.StatusInternalServerError, "Internal Server Error"))

// ErrorInvalidRequest returns an error response for invalid requests
func ErrorInvalidRequest(err error) render.Renderer {
	return newErrResponse(http.StatusBadRequest, "Invalid Request", err)
}

// ErrorInternalServer returns an error response for server errors
func ErrorInternalServer(err error) render.Renderer {
	return newErrResponse(http.StatusInternalServerError, "Internal Server Error", err)
}

// ErrorForbidden returns an error response for forbidden actions
func ErrorForbidden(err error) render.Renderer {
	return newErrResponse(http.StatusForbidden, "Forbidden", err)
}

// ErrorUnauthorized returns an error response for unauthorized actions
func ErrorUnauthorized(err error) render.Renderer {
	return newErrResponse(http.StatusUnauthorized, "Unauthorized", err)
}
