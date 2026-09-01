package students

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Mark done (#2267, story 15). A request whose whole scope lies before today
// changes nothing if approved, and rejecting it would tell the family their
// wish was refused. "Als erledigt markieren" is the third verdict: the request
// leaves the queue, nothing is applied, the family sees a neutral note.
//
// Only the types with an effective scope can reach it. A weekly care plan and
// a Stammdaten change apply from the decision onwards, so they are never past
// and answer 409 request_not_past — the same answer a future-dated request of
// any type gets.

// parentRequestMarkDoner is the port each domain service satisfies. It is
// declared here rather than added to the four domain interfaces so existing
// test doubles keep compiling; the concrete services are asserted at request
// time, exactly like the enrollment coordinator ports.
type parentRequestMarkDoner interface {
	MarkDone(ctx context.Context, requestID int64, expectedVersion, reason string, reviewedBy int64) error
}

type markRequestDoneBody struct {
	ExpectedVersion string `json:"expected_version"`
	Reason          string `json:"reason"`
}

// markDoneTargets maps the URL kind to the service that owns it. Kinds absent
// from the map have no effective scope and can never be past.
func (rs *Resource) markDoneTarget(kind string) (parentRequestMarkDoner, bool) {
	var candidate any
	switch kind {
	case requestTypeExcused:
		candidate = rs.ExcusedRequestService
	case requestTypeCareSchedule:
		candidate = rs.CareRequestService
	case requestTypeOffering:
		candidate = rs.OfferingChangeService
	case requestTypeMasterData:
		// Stammdaten changes apply from the decision onwards.
		return nil, true
	default:
		return nil, false
	}
	service, ok := candidate.(parentRequestMarkDoner)
	if !ok || service == nil {
		return nil, true
	}
	return service, true
}

// markParentRequestDone closes a past request without applying it.
func (rs *Resource) markParentRequestDone(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	service, known := rs.markDoneTarget(kind)
	if !known {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("unknown request kind")))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request id")
	if !ok {
		return
	}
	var body markRequestDoneBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if service == nil {
		// The type has no effective scope, so it is never past. Answering the
		// same code a future request gets keeps the client's branch single.
		renderError(w, r, parentRequestLifecycleErrorRenderer(userService.ErrParentRequestNotPast))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if err := service.MarkDone(
		r.Context(), requestID, body.ExpectedVersion, body.Reason, int64(claims.ID),
	); err != nil {
		tenant.MarkRollback(r.Context())
		renderError(w, r, parentRequestLifecycleErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"status": "done"}, "Request marked done")
}

// parentRequestLifecycleErrorRenderer carries the not-found and not-pending
// sentinels of all four domains next to the shared lifecycle codes, so one
// route answers consistently whatever kind it was called with.
var parentRequestLifecycleErrorRenderer = common.RulesRenderer(parentRequestRules(
	common.ErrorRule{Match: isParentRequestMissing, Render: common.ErrorNotFound},
	common.ErrorRule{Match: isParentRequestNotDecided, Render: func(error) render.Renderer {
		return common.ErrorConflictMessageWithCode(
			"Diese Anfrage ist noch nicht entschieden. Es gibt nichts zu korrigieren.",
			codeRequestNotDecided,
		)
	}},
	common.ErrorRule{Match: isParentRequestNotPending, Render: conflictWithCode("change_request_not_pending")},
	common.ErrorRule{Match: isParentRequestForbidden, Render: common.ErrorForbidden},
), common.ErrorInternalServer)

// Correct a decision (#2267, stories 21-23). Staff decide fast, and a wrong
// decision used to be unrecoverable: the request was terminal and the family
// had to file a new one. Correcting keeps the old decision in the ledger and
// brings the child's record in line with the new verdict.
//
// Not every type can be corrected in both directions. Turning an approval into
// a rejection means undoing what the approval wrote, which needs a pre-state
// the type actually kept. Where there is none, the route answers
// `correction_unsupported` with a message naming the reason instead of
// pretending it reverted something.

// parentRequestCorrecter is the port a correctable domain satisfies.
type parentRequestCorrecter interface {
	Correct(ctx context.Context, requestID int64, approve bool, expectedVersion, reason string, reviewedBy int64) error
}

type correctRequestBody struct {
	Approve         *bool  `json:"approve"`
	Reason          string `json:"reason"`
	ExpectedVersion string `json:"expected_version"`
}

// correctionUnsupportedReasons names, per kind, why a decision cannot be
// reverted. The message reaches the reviewer verbatim, so it says what to do
// instead rather than only that it failed.
var correctionUnsupportedReasons = map[string]string{
	requestTypeOffering: "Angebote speichern keinen Stand von vor der Entscheidung. " +
		"Bitte ändern Sie die Buchung des Kindes direkt.",
}

func (rs *Resource) correctTarget(kind string) (parentRequestCorrecter, bool) {
	var candidate any
	switch kind {
	case requestTypeExcused:
		candidate = rs.ExcusedRequestService
	case requestTypeMasterData:
		candidate = rs.MasterDataReviewService
	case requestTypeCareSchedule:
		// Pickup-change requests live in this queue too, and those CAN be
		// corrected. The service decides per request kind: a weekly plan
		// answers correction_unsupported from inside.
		candidate = rs.CareRequestService
	case requestTypeOffering:
		return nil, true
	default:
		return nil, false
	}
	service, ok := candidate.(parentRequestCorrecter)
	if !ok || service == nil {
		return nil, true
	}
	return service, true
}

// correctParentRequestDecision rewrites a decision staff already took.
func (rs *Resource) correctParentRequestDecision(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	service, known := rs.correctTarget(kind)
	if !known {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("unknown request kind")))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request id")
	if !ok {
		return
	}
	var body correctRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if body.Approve == nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("approve is required")))
		return
	}
	if service == nil {
		message := correctionUnsupportedReasons[kind]
		if message == "" {
			message = "Diese Entscheidung kann nicht korrigiert werden."
		}
		renderError(w, r, common.ErrorConflictMessageWithCode(message, codeCorrectionUnsupp))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if err := service.Correct(
		r.Context(), requestID, *body.Approve, body.ExpectedVersion, body.Reason, int64(claims.ID),
	); err != nil {
		tenant.MarkRollback(r.Context())
		renderError(w, r, parentRequestLifecycleErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]bool{"approved": *body.Approve}, "Decision corrected")
}
