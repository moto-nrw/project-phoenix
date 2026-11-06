// Package carecontracthttp serves the Betreuungsvertrag in the Träger-Portal
// (#4678, ADR 0153). Every route requires the right "Verträge verwalten".
// The writes of the contracts run in the Träger transaction. The reads of
// the contracts write into the access log of each school, and the
// candidates, the creation and the templates read the schools; all of them
// open their units themselves, so their routes carry no transaction.
package carecontracthttp

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Resource serves /traeger/contracts and /traeger/contract-templates.
type Resource struct {
	contracts       carecontract.Contracts
	intake          carecontract.Intake
	paper           carecontract.PaperSignatures
	requireContract common.Middleware
}

// NewResource binds the routes to the owner and the right check.
func NewResource(contracts carecontract.Contracts, intake carecontract.Intake, paper carecontract.PaperSignatures, requireContract common.Middleware) *Resource {
	if contracts == nil || intake == nil || paper == nil || requireContract == nil {
		panic("care contract http: the contracts, the intake, the paper signature and the right check are required")
	}
	return &Resource{contracts: contracts, intake: intake, paper: paper, requireContract: requireContract}
}

// ContractRouter returns the routes mounted at /traeger/contracts.
func (rs *Resource) ContractRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		open := r.With(rs.requireContract)
		open.Get("/candidates", rs.candidates)
		open.Post("/", rs.create)
		open.Get("/", rs.list)
		open.Get("/{id}", rs.detail)
		open.Get("/{id}/document", rs.document)
		inTx := r.With(rs.requireContract, withTx)
		inTx.Post("/send", rs.send)
		inTx.Delete("/{id}", rs.discard)
		// The paper contract (#4679, signing.go).
		inTx.Post("/{id}/paper-signature", rs.recordPaper)
		inTx.Get("/{id}/paper-signature", rs.paperDocument)
	})
	return r
}

// TemplateRouter returns the routes mounted at /traeger/contract-templates.
func (rs *Resource) TemplateRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, _ common.Middleware) {
		open := r.With(rs.requireContract)
		open.Get("/", rs.templates)
		open.Put("/{schoolId}/{offeringId}", rs.saveTemplate)
	})
	return r
}

func reviewerOf(w http.ResponseWriter, r *http.Request) (carecontract.Reviewer, bool) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 || principal.AccountID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return carecontract.Reviewer{}, false
	}
	return carecontract.Reviewer{OrganizationID: principal.OrganizationID(), AccountID: principal.AccountID()}, true
}

func contractKey(w http.ResponseWriter, r *http.Request) (carecontract.Reviewer, int64, bool) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return carecontract.Reviewer{}, 0, false
	}
	id, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return carecontract.Reviewer{}, 0, false
	}
	return reviewer, id, true
}

// optionalID reads an optional positive ID from the query.
func optionalID(r *http.Request, name string) (int64, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return id, nil
}

func (rs *Resource) list(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	schoolID, err := optionalID(r, "school_id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	query := r.URL.Query()
	offset, limit := 0, 0
	for name, target := range map[string]*int{"offset": &offset, "limit": &limit} {
		if raw := query.Get(name); raw != "" {
			value, parseErr := strconv.Atoi(raw)
			if parseErr != nil || value < 0 {
				common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("invalid %s", name)))
				return
			}
			*target = value
		}
	}
	list, err := rs.contracts.ListContracts(r.Context(), reviewer, carecontract.ContractFilter{
		Status: query.Get("status"), SchoolID: schoolID, Search: query.Get("search"), Offset: offset, Limit: limit,
	})
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, list, "Care contracts retrieved successfully")
}

func (rs *Resource) candidates(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	schoolID, err := optionalID(r, "school_id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	candidates, err := rs.intake.Candidates(r.Context(), reviewer, schoolID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, candidates, "Contract candidates retrieved successfully")
}

// CreateRequest names the application children and offerings to create a
// draft for.
type CreateRequest struct {
	Contracts []carecontract.CandidateKey `json:"contracts"`
}

// Bind accepts the body; the owner checks the keys.
func (req *CreateRequest) Bind(*http.Request) error { return nil }

func (rs *Resource) create(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &CreateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	result, err := rs.intake.CreateContracts(r.Context(), reviewer, req.Contracts)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, result, "Care contracts created successfully")
}

// SendRequest names the contracts to send.
type SendRequest struct {
	ContractIDs    []int64 `json:"contract_ids"`
	RecipientEmail *string `json:"recipient_email"`
}

// Bind accepts the body; the owner checks the IDs.
func (req *SendRequest) Bind(*http.Request) error { return nil }

func (rs *Resource) send(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	req := &SendRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	var result carecontract.SendResult
	var err error
	if req.RecipientEmail == nil {
		result, err = rs.contracts.SendContracts(r.Context(), reviewer, req.ContractIDs)
	} else if len(req.ContractIDs) != 1 {
		err = &carecontract.ValidationError{Fields: map[string]string{"contract_ids": "single_required"}}
	} else {
		result, err = rs.contracts.ResendContract(r.Context(), reviewer, req.ContractIDs[0], *req.RecipientEmail)
	}
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusAccepted, result, "Care contracts queued for mail")
}

func (rs *Resource) detail(w http.ResponseWriter, r *http.Request) {
	reviewer, id, ok := contractKey(w, r)
	if !ok {
		return
	}
	detail, err := rs.contracts.Contract(r.Context(), reviewer, id)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Care contract retrieved successfully")
}

func (rs *Resource) document(w http.ResponseWriter, r *http.Request) {
	reviewer, id, ok := contractKey(w, r)
	if !ok {
		return
	}
	file, err := rs.contracts.ContractDocument(r.Context(), reviewer, id)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderError(w, r, err)
		return
	}
	WriteFile(w, file, "inline")
}

func (rs *Resource) discard(w http.ResponseWriter, r *http.Request) {
	reviewer, id, ok := contractKey(w, r)
	if !ok {
		return
	}
	if err := rs.contracts.DiscardContract(r.Context(), reviewer, id); err != nil {
		tenant.MarkRollback(r.Context())
		renderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (rs *Resource) templates(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	templates, err := rs.intake.Templates(r.Context(), reviewer)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, templates, "Contract templates retrieved successfully")
}

// TemplateRequest is the body of a template.
type TemplateRequest struct {
	carecontract.TemplateInput
}

// Bind accepts the body; the owner validates the template.
func (req *TemplateRequest) Bind(*http.Request) error { return nil }

func (rs *Resource) saveTemplate(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	schoolID, err := common.ParseIDParam(r, "schoolId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	offeringID, err := common.ParseIDParam(r, "offeringId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	req := &TemplateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	saved, err := rs.intake.SaveTemplate(r.Context(), reviewer, schoolID, offeringID, req.TemplateInput)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, saved, "Contract template saved successfully")
}

// WriteFile writes a PDF that no cache keeps.
func WriteFile(w http.ResponseWriter, file carecontract.File, disposition string) {
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, file.Filename))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

var errorRules = []struct {
	err    error
	render func(error) render.Renderer
}{
	{carecontract.ErrContractNotFound, func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, common.CodeCareContractContractNotFound)
	}},
	{carecontract.ErrApplicationNotFound, func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, common.CodeCareContractApplicationNotFound)
	}},
	{carecontract.ErrOfferingNotFound, func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, common.CodeCareContractOfferingNotFound)
	}},
	{carecontract.ErrSchoolOutsideTraeger, func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, common.CodeCareContractSchoolOutsideTraeger)
	}},
	{carecontract.ErrContractNotDraft, func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeCareContractContractNotDraft)
	}},
	{carecontract.ErrLetterheadTooLong, func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeCareContractLetterheadTooLong)
	}},
}

// renderError answers an owner error with its registered code.
func renderError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *carecontract.ValidationError
	if errors.As(err, &invalid) {
		common.RenderError(w, r, validationError(invalid))
		return
	}
	for _, rule := range errorRules {
		if errors.Is(err, rule.err) {
			common.RenderError(w, r, rule.render(err))
			return
		}
	}
	common.RenderError(w, r, common.ErrorInternalServer(err))
}

// validationError marks every refused field. A template names its fields;
// a list or a batch names the input as a whole.
func validationError(invalid *carecontract.ValidationError) render.Renderer {
	code := common.CodeCareContractTemplateInvalid
	names := make([]string, 0, len(invalid.Fields))
	for name := range invalid.Fields {
		names = append(names, name)
		if name == "status" || name == "contracts" || name == "contract_ids" || name == "recipient_email" {
			code = common.CodeCareContractContractInputInvalid
		}
	}
	sort.Strings(names)
	fields := make([]common.FieldError, len(names))
	for i, name := range names {
		fields[i] = common.FieldError{Field: name, Reason: invalid.Fields[name]}
	}
	response := common.ErrorValidation(invalid.Error(), fields).(*common.ErrResponse)
	response.Code = code
	return response
}
