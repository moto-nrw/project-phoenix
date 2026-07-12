package active

import (
	"errors"
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

// ErrorRenderer returns a render.Renderer for the given error
func ErrorRenderer(err error) render.Renderer {
	// Default to internal server error
	renderer := newErrResponse(http.StatusInternalServerError, "Internal Server Error", err)

	// Handle specific error types
	switch {
	case errors.Is(err, activeSvc.ErrActiveGroupNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.Status = "Active Group Not Found"

	case errors.Is(err, activeSvc.ErrVisitNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.Status = "Visit Not Found"

	case errors.Is(err, activeSvc.ErrGroupSupervisorNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.Status = "Group Supervisor Not Found"

	case errors.Is(err, activeSvc.ErrCombinedGroupNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.Status = "Combined Group Not Found"

	case errors.Is(err, activeSvc.ErrGroupMappingNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.Status = "Group Mapping Not Found"

	case errors.Is(err, activeSvc.ErrInvalidData):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.Status = "Invalid Data"

	case errors.Is(err, activeSvc.ErrActiveGroupAlreadyEnded):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.Status = "Active Group Already Ended"

	case errors.Is(err, activeSvc.ErrVisitAlreadyEnded):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.Status = "Visit Already Ended"

	case errors.Is(err, activeSvc.ErrSupervisionAlreadyEnded):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.Status = "Supervision Already Ended"

	case errors.Is(err, activeSvc.ErrCombinedGroupAlreadyEnded):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.Status = "Combined Group Already Ended"

	case errors.Is(err, activeSvc.ErrGroupAlreadyInCombination):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.Status = "Group Already In Combination"

	case errors.Is(err, activeSvc.ErrStudentAlreadyInGroup):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.Status = "Student Already In Group"

	case errors.Is(err, activeSvc.ErrStudentAlreadyActive):
		// 409 Conflict — the request is well-formed but conflicts with
		// existing state (the student already has an open visit). Issue
		// #844 added a DB-level partial unique index on active.visits;
		// the active service now translates the resulting 23505 error
		// to ErrStudentAlreadyActive for ALL CreateVisit callers, not
		// just the IoT checkin flow. Without this 409 mapping, the
		// admin POST /active/visits route would surface concurrent
		// duplicate-creation as 400 Bad Request, contradicting both
		// the IoT path's 409 response and the semantic meaning of the
		// error.
		renderer.HTTPStatusCode = http.StatusConflict
		renderer.Status = "Student Already Has Active Visit"

	case errors.Is(err, activeSvc.ErrStudentsNotPresent):
		renderer.HTTPStatusCode = http.StatusConflict
		renderer.Status = "Students Not Present"

	case errors.Is(err, activeSvc.ErrStudentMoveForbidden):
		renderer.HTTPStatusCode = http.StatusForbidden
		renderer.Status = "Forbidden"

	case errors.Is(err, activeSvc.ErrStaffAlreadySupervising):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.Status = "Staff Already Supervising This Group"

	case errors.Is(err, activeSvc.ErrCannotDeleteActiveGroup):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.Status = "Cannot Delete Active Group With Active Visits"

	case errors.Is(err, activeSvc.ErrInvalidTimeRange):
		renderer.HTTPStatusCode = http.StatusBadRequest
		renderer.Status = "Invalid Time Range"

	case errors.Is(err, activeSvc.ErrRoomConflict):
		renderer.HTTPStatusCode = http.StatusConflict
		renderer.Status = "Room Conflict"

	case errors.Is(err, activeSvc.ErrStudentNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.Status = "Student Not Found"

	case errors.Is(err, activeSvc.ErrStaffNotFound):
		renderer.HTTPStatusCode = http.StatusNotFound
		renderer.Status = "Staff Not Found"
	}

	return renderer
}

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
