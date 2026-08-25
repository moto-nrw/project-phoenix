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
	// Beide rendern denselben deutschen Satz: "nicht deine Unterhaltung" und
	// "gibt es nicht" muessen von aussen ununterscheidbar bleiben, sonst lassen
	// sich Thread-IDs durchprobieren. Der Text geht wortwoertlich an den Browser,
	// also darf hier nie der englische Go-Sentinel stehen.
	{Target: service.ErrThreadNotFound, Render: func(_ error) render.Renderer {
		return common.ErrorNotFoundMessage("Diese Unterhaltung gibt es nicht.")
	}},
	{Target: service.ErrNotParticipant, Render: func(_ error) render.Renderer {
		return common.ErrorNotFoundMessage("Diese Unterhaltung gibt es nicht.")
	}},
	// Stable code, not just prose: the inbox has to tell "your school switched
	// this off" apart from "loading failed". Matching the German sentence would
	// break the moment the wording is polished, and the page would fall back to
	// a red error plus a compose button that dead-ends in this very 403.
	{Target: service.ErrMessagingDisabled, Render: func(_ error) render.Renderer {
		return common.ErrorForbiddenMessageWithCode(
			"Der Team-Chat ist für diese Schule nicht eingeschaltet.",
			ErrCodeStaffMessagingDisabled,
		)
	}},
	{Target: service.ErrRecipientNotAvailable, Render: func(_ error) render.Renderer {
		// Deckt beide Faelle wahrheitsgemaess ab: jemand hat die Schule verlassen,
		// ODER das Konto gehoert keiner Mitarbeiterin (etwa einem Elternteil).
		// "gehoert nicht mehr zu dieser Schule" waere im zweiten Fall schlicht
		// falsch - Sorgeberechtigte gehoeren dazu, sind nur keine Kolleginnen.
		return common.ErrorConflictMessage("Diese Person können Sie nicht anschreiben.")
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
	{Target: service.ErrNoActor, Render: func(_ error) render.Renderer {
		return common.ErrorForbiddenMessage("Bitte melden Sie sich erneut an.")
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
