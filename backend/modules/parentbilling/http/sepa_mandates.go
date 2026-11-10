package parentbillinghttp

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// SEPAMandateResource serves the SEPA direct debit of the families (#4695,
// ADR 0171): the creditor of the Träger at /traeger/sepa-creditor and the
// mandates at /traeger/sepa-mandates. Every route sits behind the Träger
// session, needs the Träger right "Abrechnung" and runs in the Träger
// transaction of the session's Träger. No route answers an IBAN: a mandate
// leaves with its mask only.
type SEPAMandateResource struct {
	mandates       capability.SEPAMandates
	requireBilling common.Middleware
}

// NewSEPAMandateResource builds the resource. requireBilling admits only a
// session with the Träger right "Abrechnung".
func NewSEPAMandateResource(mandates capability.SEPAMandates, requireBilling common.Middleware) *SEPAMandateResource {
	if mandates == nil || requireBilling == nil {
		panic("parent billing http: the SEPA mandates and the right check are required")
	}
	return &SEPAMandateResource{mandates: mandates, requireBilling: requireBilling}
}

// CreditorRouter returns the routes mounted at /traeger/sepa-creditor.
func (rs *SEPAMandateResource) CreditorRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		billing := r.With(rs.requireBilling, withTx)
		billing.Get("/", rs.creditor)
		billing.Put("/", rs.saveCreditor)
	})
	return r
}

// MandateRouter returns the routes mounted at /traeger/sepa-mandates.
func (rs *SEPAMandateResource) MandateRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		billing := r.With(rs.requireBilling, withTx)
		billing.Get("/", rs.listMandates)
		billing.Post("/", rs.createMandate)
		billing.Get("/imports", rs.importLog)
		billing.Post("/imports", rs.applyImport)
		billing.Post("/imports/preview", rs.previewImport)
		billing.Put("/{id}", rs.changeMandate)
		billing.Post("/{id}/revoke", rs.revokeMandate)
		billing.Get("/{id}/history", rs.history)
		billing.Get("/{id}/collections", rs.collections)
	})
	return r
}

// SEPACreditorRequest is the body of PUT /traeger/sepa-creditor.
type SEPACreditorRequest struct {
	CreditorID          string `json:"creditor_id"`
	PrenotificationDays int    `json:"prenotification_days"`
	MissingMandate      string `json:"missing_mandate"`
}

// Bind accepts the body; the Owner validates the creditor.
func (req *SEPACreditorRequest) Bind(*http.Request) error { return nil }

// SEPAMandateRequest is the body of POST /traeger/sepa-mandates: the
// mandate of the family of a contract of the Träger.
type SEPAMandateRequest struct {
	ContractID    int64         `json:"contract_id"`
	Reference     string        `json:"reference"`
	SignedOn      calendar.Date `json:"signed_on"`
	Kind          string        `json:"kind"`
	Sequence      string        `json:"sequence"`
	AccountHolder string        `json:"account_holder"`
	IBAN          string        `json:"iban"`
	BIC           string        `json:"bic"`
}

// Bind accepts the body; the Owner validates the mandate.
func (req *SEPAMandateRequest) Bind(*http.Request) error { return nil }

// SEPAMandateChangeRequest is the body of PUT /traeger/sepa-mandates/{id};
// an empty IBAN keeps the stored one.
type SEPAMandateChangeRequest struct {
	AccountHolder string `json:"account_holder"`
	IBAN          string `json:"iban"`
	BIC           string `json:"bic"`
	Sequence      string `json:"sequence"`
}

// Bind accepts the body; the Owner validates the change.
func (req *SEPAMandateChangeRequest) Bind(*http.Request) error { return nil }

// SEPAMandateRevokeRequest is the body of POST
// /traeger/sepa-mandates/{id}/revoke: the day the revocation arrived.
type SEPAMandateRevokeRequest struct {
	RevokedOn calendar.Date `json:"revoked_on"`
}

// Bind accepts the body; the Owner validates the date.
func (req *SEPAMandateRevokeRequest) Bind(*http.Request) error { return nil }

func (rs *SEPAMandateResource) creditor(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	creditor, err := rs.mandates.SEPACreditor(r.Context(), organizationID)
	if err != nil {
		renderMandateError(w, r, err, common.CodeParentBillingSepaCreditorInvalid)
		return
	}
	common.Respond(w, r, http.StatusOK, creditor, "SEPA creditor retrieved successfully")
}

func (rs *SEPAMandateResource) saveCreditor(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &SEPACreditorRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingSepaCreditorInvalid))
		return
	}
	creditor, err := rs.mandates.SaveSEPACreditor(r.Context(), reviewer, capability.SEPACreditor{CreditorID: req.CreditorID,
		PrenotificationDays: req.PrenotificationDays, MissingMandate: req.MissingMandate})
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderMandateError(w, r, err, common.CodeParentBillingSepaCreditorInvalid)
		return
	}
	common.Respond(w, r, http.StatusOK, creditor, "SEPA creditor saved successfully")
}

func (rs *SEPAMandateResource) listMandates(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	var contractID int64
	if raw := r.URL.Query().Get("contract_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequestWithCode(errors.New("contract_id must be a positive number"),
				common.CodeParentBillingSepaMandateInvalid))
			return
		}
		contractID = parsed
	}
	mandates, err := rs.mandates.ListSEPAMandates(r.Context(), reviewer, contractID)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderMandateError(w, r, err, common.CodeParentBillingSepaMandateInvalid)
		return
	}
	common.Respond(w, r, http.StatusOK, mandates, "SEPA mandates retrieved successfully")
}

func (rs *SEPAMandateResource) createMandate(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &SEPAMandateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(errors.New("the body is no mandate"), common.CodeParentBillingSepaMandateInvalid))
		return
	}
	mandate, err := rs.mandates.CreateSEPAMandate(r.Context(), reviewer, capability.SEPAMandateInput{
		ContractID: req.ContractID, Reference: req.Reference, SignedOn: req.SignedOn, Kind: req.Kind, Sequence: req.Sequence,
		AccountHolder: req.AccountHolder, IBAN: req.IBAN, BIC: req.BIC, Source: capability.MandateFromOffice,
	})
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderMandateError(w, r, err, common.CodeParentBillingSepaMandateInvalid)
		return
	}
	common.Respond(w, r, http.StatusCreated, mandate, "SEPA mandate created successfully")
}

func (rs *SEPAMandateResource) changeMandate(w http.ResponseWriter, r *http.Request) {
	reviewer, id, ok := mandateKey(w, r)
	if !ok {
		return
	}
	req := &SEPAMandateChangeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(errors.New("the body is no mandate change"), common.CodeParentBillingSepaMandateInvalid))
		return
	}
	mandate, err := rs.mandates.ChangeSEPAMandate(r.Context(), reviewer, id, capability.SEPAMandateChange(*req))
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderMandateError(w, r, err, common.CodeParentBillingSepaMandateInvalid)
		return
	}
	common.Respond(w, r, http.StatusOK, mandate, "SEPA mandate changed successfully")
}

func (rs *SEPAMandateResource) revokeMandate(w http.ResponseWriter, r *http.Request) {
	reviewer, id, ok := mandateKey(w, r)
	if !ok {
		return
	}
	req := &SEPAMandateRevokeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingSepaMandateInvalid))
		return
	}
	mandate, err := rs.mandates.RevokeSEPAMandate(r.Context(), reviewer, id, req.RevokedOn)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderMandateError(w, r, err, common.CodeParentBillingSepaMandateInvalid)
		return
	}
	common.Respond(w, r, http.StatusOK, mandate, "SEPA mandate revoked successfully")
}

func (rs *SEPAMandateResource) history(w http.ResponseWriter, r *http.Request) {
	reviewer, id, ok := mandateKey(w, r)
	if !ok {
		return
	}
	events, err := rs.mandates.SEPAMandateHistory(r.Context(), reviewer, id)
	if err != nil {
		renderMandateError(w, r, err, common.CodeParentBillingSepaMandateInvalid)
		return
	}
	common.Respond(w, r, http.StatusOK, events, "SEPA mandate history retrieved successfully")
}

// collections answers the debits of a mandate (#4697): the collection
// history per invoice, without bank data.
func (rs *SEPAMandateResource) collections(w http.ResponseWriter, r *http.Request) {
	reviewer, id, ok := mandateKey(w, r)
	if !ok {
		return
	}
	debits, err := rs.mandates.SEPAMandateCollections(r.Context(), reviewer, id)
	if err != nil {
		renderMandateError(w, r, err, common.CodeParentBillingSepaMandateInvalid)
		return
	}
	common.Respond(w, r, http.StatusOK, debits, "SEPA mandate collections retrieved successfully")
}

func mandateKey(w http.ResponseWriter, r *http.Request) (capability.HolidayReviewer, int64, bool) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return capability.HolidayReviewer{}, 0, false
	}
	id, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingSepaMandateInvalid))
		return capability.HolidayReviewer{}, 0, false
	}
	return reviewer, id, true
}

// renderMandateError answers a refused creditor or mandate. A refused field
// names the field only, never the value: the request may carry an IBAN.
func renderMandateError(w http.ResponseWriter, r *http.Request, err error, invalidCode string) {
	var fields validation.Errors
	switch {
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, invalidCode))
	case errors.Is(err, capability.ErrSEPAMandateNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingSepaMandateNotFound))
	case errors.Is(err, capability.ErrFamilyUnknown):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingFamilyUnknown))
	case errors.Is(err, capability.ErrSEPAMandateInForce):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingSepaMandateInForce))
	case errors.Is(err, capability.ErrSEPAMandateReferenceTaken):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingSepaMandateReferenceTaken))
	case errors.Is(err, capability.ErrSEPACreditorMissing):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingSepaCreditorMissing))
	case errors.Is(err, capability.ErrSEPAMandateRevoked):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingSepaMandateRevoked))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
