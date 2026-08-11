package students

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// renderError writes an error response to the HTTP response writer.
// Delegates to common.RenderError which logs 5xx root causes to slog
// and captures them to Sentry.
func renderError(w http.ResponseWriter, r *http.Request, errorResponse render.Renderer) {
	common.RenderError(w, r, errorResponse)
}

// ErrInvalidRequest is returned by request Bind validation.
var ErrInvalidRequest = errors.New("invalid request")

// Sentinel errors returned from the pickup/arrival exception write flows so the
// API layer can signal a precise HTTP status instead of every failure collapsing
// into a 500. They alias the service-layer sentinels so renderExceptionWriteError
// maps both the schedule service's care-exception flows and the delete handlers'
// own in-tx signals through one switch.
var (
	// ErrStaffProfileRequired means a guardian-authored exception can only be
	// changed or removed by a user who has a staff profile: reclaiming the day
	// for staff stamps the editing staff as author (created_by → users.staff),
	// which an admin-only account without a staff record cannot satisfy.
	ErrStaffProfileRequired = scheduleService.ErrCareExceptionStaffProfileRequired
	// ErrExceptionNotFound means the targeted exception no longer exists (it was
	// removed between the ownership pre-check and the locked re-read).
	ErrExceptionNotFound = scheduleService.ErrCareExceptionNotFound
	// ErrExceptionWrongStudent means the exception does not belong to the student
	// named in the path.
	ErrExceptionWrongStudent = scheduleService.ErrCareExceptionWrongStudent
	// ErrExceptionDayConflict means a staff-authored exception already existed for
	// the day when a create came in. The staff client only POSTs when its loaded
	// view shows no exception, so this is a concurrent staff edit — refuse rather
	// than silently overwrite a colleague's change. (A guardian-authored row is
	// reclaimed inline instead; only the staff-on-staff race lands here.) The
	// client reloads and the retry goes through the update path.
	ErrExceptionDayConflict = scheduleService.ErrCareExceptionDayConflict
	// ErrExceptionContainsPartialAbsence requires the dedicated partial-absence
	// endpoint so slot provenance is restored atomically with the pickup row.
	ErrExceptionContainsPartialAbsence = scheduleService.ErrCareExceptionContainsPartialAbsence
)

var exceptionWriteErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: ErrStaffProfileRequired, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, "staff_profile_required")
	}},
	{Target: ErrExceptionNotFound, Render: common.ErrorNotFound},
	{Target: ErrExceptionWrongStudent, Render: common.ErrorForbidden},
	{Target: ErrExceptionDayConflict, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, "care_exception_raced")
	}},
	{Target: ErrExceptionContainsPartialAbsence, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, "partial_absence_requires_dedicated_action")
	}},
}, common.ErrorInternalServer)

// renderExceptionWriteError maps the sentinel errors a pickup/arrival exception
// write transaction to precise HTTP statuses through the shared declarative
// rule engine. Anything unrecognized stays a 500.
func renderExceptionWriteError(w http.ResponseWriter, r *http.Request, err error) {
	renderError(w, r, exceptionWriteErrorRenderer(err))
}

// ErrCodeSickExcusedConflict is returned when a status-flag toggle would
// leave both sick and excused set at the same time. The frontend uses this
// code to prompt the user to switch from one state to the other.
const ErrCodeSickExcusedConflict = "SICK_EXCUSED_CONFLICT"
