package students

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// ErrInvalidRequest is returned by request Bind validation.
var ErrInvalidRequest = errors.New("invalid request")

// Sentinel errors returned from the pickup/arrival exception write transactions
// so the closure can signal a precise HTTP status instead of every in-tx
// failure collapsing into a 500.
var (
	// ErrStaffProfileRequired means a guardian-authored exception can only be
	// changed or removed by a user who has a staff profile: reclaiming the day
	// for staff stamps the editing staff as author (created_by → users.staff),
	// which an admin-only account without a staff record cannot satisfy.
	ErrStaffProfileRequired = errors.New("a staff profile is required to change a parent-set time")
	// ErrExceptionNotFound means the targeted exception no longer exists (it was
	// removed between the ownership pre-check and the locked re-read).
	ErrExceptionNotFound = errors.New("schedule exception not found")
	// ErrExceptionWrongStudent means the exception does not belong to the student
	// named in the path.
	ErrExceptionWrongStudent = errors.New("schedule exception does not belong to this student")
	// ErrExceptionDayConflict means a staff-authored exception already existed for
	// the day when a create came in. The staff client only POSTs when its loaded
	// view shows no exception, so this is a concurrent staff edit — refuse rather
	// than silently overwrite a colleague's change. (A guardian-authored row is
	// reclaimed inline instead; only the staff-on-staff race lands here.) The
	// client reloads and the retry goes through the update path.
	ErrExceptionDayConflict = errors.New("schedule exception for this day was just changed")
)

// ErrorInvalidRequest returns a 400 Bad Request error response
func ErrorInvalidRequest(err error) render.Renderer {
	return common.ErrorInvalidRequest(err)
}

// ErrorInternalServer returns a 500 Internal Server Error response
func ErrorInternalServer(err error) render.Renderer {
	return common.ErrorInternalServer(err)
}

// ErrorNotFound returns a 404 Not Found error response
func ErrorNotFound(err error) render.Renderer {
	return common.ErrorNotFound(err)
}

// ErrorUnauthorized returns a 401 Unauthorized error response
func ErrorUnauthorized(err error) render.Renderer {
	return common.ErrorUnauthorized(err)
}

// ErrorForbidden returns a 403 Forbidden error response
func ErrorForbidden(err error) render.Renderer {
	return common.ErrorForbidden(err)
}

// ErrorConflictWithCode returns a 409 Conflict with a stable error code.
func ErrorConflictWithCode(err error, code string) render.Renderer {
	return common.ErrorConflictWithCode(err, code)
}

// ErrorForbiddenWithCode returns a 403 Forbidden with a stable error code.
func ErrorForbiddenWithCode(err error, code string) render.Renderer {
	return common.ErrorForbiddenWithCode(err, code)
}

// renderExceptionWriteError maps the sentinel errors a pickup/arrival exception
// write transaction can return to precise HTTP statuses. Anything unrecognized
// stays a 500 so genuine infrastructure failures are not masked as 4xx.
func renderExceptionWriteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrStaffProfileRequired):
		renderError(w, r, ErrorForbiddenWithCode(err, "staff_profile_required"))
	case errors.Is(err, ErrExceptionNotFound):
		renderError(w, r, ErrorNotFound(err))
	case errors.Is(err, ErrExceptionWrongStudent):
		renderError(w, r, ErrorForbidden(err))
	case errors.Is(err, ErrExceptionDayConflict):
		renderError(w, r, ErrorConflictWithCode(err, "care_exception_raced"))
	default:
		renderError(w, r, ErrorInternalServer(err))
	}
}

// ErrCodeSickExcusedConflict is returned when a status-flag toggle would
// leave both sick and excused set at the same time. The frontend uses this
// code to prompt the user to switch from one state to the other.
const ErrCodeSickExcusedConflict = "SICK_EXCUSED_CONFLICT"
