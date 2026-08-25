package staffmessaging

import (
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	service "github.com/moto-nrw/project-phoenix/services/staffmessaging"
)

// ErrCodeStaffMessagingDisabled is the wire contract for "this school has the
// Team-Chat switched off". The frontend keys its read-only state on it.
const ErrCodeStaffMessagingDisabled = "staff_messaging_disabled"

// staffMessagingErrorRules maps service sentinels to HTTP responses
// declaratively (backend-conventions Rule 4: classification belongs in a rule
// table, not a hand-written switch).
//
// Note that ErrNotParticipant renders as 404, NOT 403: a 403 would confirm that
// a thread with that id exists, letting someone enumerate other people's
// conversations by probing ids. "Not yours" and "not there" must be
// indistinguishable from outside.
var staffMessagingErrorRules = []common.ErrorRule{
	{Target: service.ErrThreadNotFound, Render: func(err error) render.Renderer {
		return common.ErrorNotFound(err)
	}},
	{Target: service.ErrNotParticipant, Render: func(err error) render.Renderer {
		return common.ErrorNotFound(err)
	}},
	// Stable code, not just prose: the inbox has to tell "your school switched
	// this off" apart from "loading failed". Matching the German sentence would
	// break the moment the wording is polished, and the page would fall back to
	// a red error plus a compose button that dead-ends in this very 403.
	{Target: service.ErrMessagingDisabled, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, ErrCodeStaffMessagingDisabled)
	}},
	{Target: service.ErrRecipientNotAvailable, Render: func(_ error) render.Renderer {
		return common.ErrorConflictMessage("Diese Person gehört nicht mehr zu dieser Schule.")
	}},
	{Target: service.ErrSelfConversation, Render: func(_ error) render.Renderer {
		return common.ErrorInvalidRequestMessage("Sie können sich nicht selbst schreiben.")
	}},
	{Target: service.ErrEmptyMessage, Render: func(_ error) render.Renderer {
		return common.ErrorInvalidRequestMessage("Die Nachricht darf nicht leer sein.")
	}},
	{Target: service.ErrMessageTooLong, Render: func(_ error) render.Renderer {
		return common.ErrorInvalidRequestMessage("Die Nachricht ist zu lang.")
	}},
	{Target: service.ErrNoActor, Render: func(err error) render.Renderer {
		return common.ErrorForbidden(err)
	}},
}

var renderStaffMessagingErrorRenderer = common.RulesRenderer(
	staffMessagingErrorRules,
	func(err error) render.Renderer {
		return common.ErrorInternalServerWrap("staff messaging request failed", err)
	},
)

func renderStaffMessagingError(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, renderStaffMessagingErrorRenderer(err))
}

// currentAccountID returns the caller's account id, or 0 when absent. Used only
// to decorate the inbox preview with "Sie: "; authorization never depends on it.
func currentAccountID(r *http.Request) int64 {
	if id := jwt.ActorAccountIDFromCtx(r.Context()); id != nil {
		return *id
	}
	return 0
}
