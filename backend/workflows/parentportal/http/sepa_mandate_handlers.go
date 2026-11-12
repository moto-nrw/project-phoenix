package parent

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	parentService "github.com/moto-ogs/project-phoenix/workflows/parentportal"
)

// The SEPA mandate of the family on a care contract of the inbox (#4696):
// the parents read it, read the text they confirm, give it and revoke it.
// The account comes from the claims; the service checks the relationship
// right, the address and the Träger's switch. The writes share the auth
// limiter: they take bank data. No answer or log line carries the IBAN.

const sepaMandatePath = "/me/documents/{organizationId}/contracts/{contractId}/sepa-mandate"

// mountSEPAMandateRoutes mounts the mandate routes; the writes run behind
// the auth limiter when one is configured.
func (rs *Resource) mountSEPAMandateRoutes(r chi.Router, authRateLimiter func(http.Handler) http.Handler) {
	r.Get(sepaMandatePath, rs.sepaMandate)
	writes := r
	if authRateLimiter != nil {
		writes = r.With(authRateLimiter)
	}
	writes.Post(sepaMandatePath+"/preview", rs.previewSEPAMandate)
	writes.Post(sepaMandatePath, rs.grantSEPAMandate)
	writes.Post(sepaMandatePath+"/revoke", rs.revokeSEPAMandate)
}

// sepaMandate answers the family's mandate on the contract.
func (rs *Resource) sepaMandate(w http.ResponseWriter, r *http.Request) {
	accountID, organizationID, contractID, ok := contractNoticeKey(w, r)
	if !ok {
		return
	}
	view, err := rs.ParentService.SEPAMandate(r.Context(), accountID, organizationID, contractID)
	if err != nil {
		renderSEPAMandateError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, view, "SEPA mandate retrieved successfully")
}

type sepaMandateRequest struct {
	parentService.SEPAMandateInput
}

func (req *sepaMandateRequest) Bind(*http.Request) error { return nil }

// bindSEPAMandate reads the body; a body that is no mandate is a 400.
func bindSEPAMandate(w http.ResponseWriter, r *http.Request) (parentService.SEPAMandateInput, bool) {
	req := &sepaMandateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(errors.New("the body is no SEPA mandate"),
			common.CodeParentBillingSepaMandateInvalid))
		return parentService.SEPAMandateInput{}, false
	}
	return req.SEPAMandateInput, true
}

// previewSEPAMandate answers the text the parents confirm next.
func (rs *Resource) previewSEPAMandate(w http.ResponseWriter, r *http.Request) {
	accountID, organizationID, contractID, ok := contractNoticeKey(w, r)
	if !ok {
		return
	}
	input, ok := bindSEPAMandate(w, r)
	if !ok {
		return
	}
	text, err := rs.ParentService.PreviewSEPAMandate(r.Context(), accountID, organizationID, contractID, input)
	if err != nil {
		renderSEPAMandateError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, text, "SEPA mandate text rendered successfully")
}

// grantSEPAMandate stores the mandate the parents confirmed.
func (rs *Resource) grantSEPAMandate(w http.ResponseWriter, r *http.Request) {
	accountID, organizationID, contractID, ok := contractNoticeKey(w, r)
	if !ok {
		return
	}
	input, ok := bindSEPAMandate(w, r)
	if !ok {
		return
	}
	view, err := rs.ParentService.GrantSEPAMandate(r.Context(), accountID, organizationID, contractID, input)
	if err != nil {
		renderSEPAMandateError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, view, "SEPA mandate granted successfully")
}

// revokeSEPAMandate ends the family's mandate today.
func (rs *Resource) revokeSEPAMandate(w http.ResponseWriter, r *http.Request) {
	accountID, organizationID, contractID, ok := contractNoticeKey(w, r)
	if !ok {
		return
	}
	view, err := rs.ParentService.RevokeSEPAMandate(r.Context(), accountID, organizationID, contractID)
	if err != nil {
		renderSEPAMandateError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, view, "SEPA mandate revoked successfully")
}

// sepaMandateErrors classify the refusals of the flow; anything else falls
// to the inbox's classification.
var sepaMandateErrors = []struct {
	err    error
	render func(error) render.Renderer
}{
	{parentService.ErrSEPAMandateInvalid, func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingSepaMandateInvalid)
	}},
	{parentService.ErrOnlineMandateDisabled, func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, common.CodeParentBillingOnlineMandateDisabled)
	}},
	{parentService.ErrSEPAMandateInForce, func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeParentBillingSepaMandateInForce)
	}},
	{parentService.ErrSEPAMandateTextChanged, func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeParentBillingSepaMandateTextChanged)
	}},
	{parentService.ErrSEPACreditorMissing, func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeParentBillingSepaCreditorMissing)
	}},
	{parentService.ErrSEPAMandateNotFound, func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, common.CodeParentBillingSepaMandateNotFound)
	}},
}

// renderSEPAMandateError answers an invalid mandate with 400, a Träger that
// takes no online mandate or a missing relationship right with 403, a
// contract outside the inbox or a family without mandate with 404 and a
// conflict with the current state with 409.
func renderSEPAMandateError(w http.ResponseWriter, r *http.Request, err error) {
	for _, rule := range sepaMandateErrors {
		if errors.Is(err, rule.err) {
			common.RenderError(w, r, rule.render(err))
			return
		}
	}
	renderBillingDocumentError(w, r, err)
}
