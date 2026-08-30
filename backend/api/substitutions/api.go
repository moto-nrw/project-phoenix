package substitutions

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	substitution "github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Resource struct {
	Service substitution.SubstitutionModule
	db      *bun.DB
}

func NewResource(service substitution.SubstitutionModule, db *bun.DB) *Resource {
	return &Resource{Service: service, db: db}
}

func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {
		r.With(withTx).Get("/", rs.overview)
		r.With(withTx).Post("/", rs.assign)
		r.With(withTx).Post("/end", rs.end)
	})
	return r
}

type assignmentRequest struct {
	Type          substitution.TargetType `json:"type"`
	GroupHandover *struct {
		GroupID       int64  `json:"group_id"`
		TargetStaffID int64  `json:"target_staff_id"`
		StartDate     string `json:"start_date,omitempty"`
		EndDate       string `json:"end_date,omitempty"`
	} `json:"group_handover"`
	AdditionalSupervision *struct {
		ActiveGroupID int64 `json:"active_group_id"`
		TargetStaffID int64 `json:"target_staff_id"`
	} `json:"additional_supervision"`
}

type endRequest struct {
	Type substitution.TargetType `json:"type"`
	ID   int64                   `json:"id"`
}

func (rs *Resource) overview(w http.ResponseWriter, r *http.Request) {
	query := substitution.OverviewQuery{IncludeTargets: true}
	if raw := r.URL.Query().Get("group_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequestMessageWithCode("Die Gruppe ist ungültig.", "invalid_target"))
			return
		}
		query.GroupID = id
	}
	if raw := r.URL.Query().Get("active_group_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequestMessageWithCode("Die Betreuung ist ungültig.", "invalid_target"))
			return
		}
		query.ActiveGroupID = id
	}
	if raw := r.URL.Query().Get("date"); raw != "" {
		date, err := timezone.ParseDate(raw)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequestMessageWithCode("Das Datum ist ungültig.", "invalid_period"))
			return
		}
		query.On = &date
	}
	caller, err := callerFromContext(r.Context())
	if err != nil {
		renderModuleError(w, r, err)
		return
	}
	result, err := rs.Service.Overview(r.Context(), caller, query)
	if err != nil {
		renderModuleError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, result, "Gruppenübergaben geladen")
}

func (rs *Resource) assign(w http.ResponseWriter, r *http.Request) {
	request, err := decodeAssignment(r.Body)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestMessageWithCode("Die Anfrage ist ungültig.", "invalid_target"))
		return
	}
	assignment, err := request.assignment()
	if err != nil {
		renderModuleError(w, r, err)
		return
	}
	caller, err := callerFromContext(r.Context())
	if err != nil {
		renderModuleError(w, r, err)
		return
	}
	created, err := rs.Service.Assign(r.Context(), caller, assignment)
	if err != nil {
		renderModuleError(w, r, err)
		return
	}
	message := "Gruppe übergeben"
	if request.Type == substitution.TargetAdditionalSupervision {
		message = "Betreuer hinzugefügt"
	}
	common.Respond(w, r, http.StatusCreated, created, message)
}

func decodeAssignment(body io.Reader) (assignmentRequest, error) {
	var request assignmentRequest
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return assignmentRequest{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return assignmentRequest{}, errors.New("request body must contain one JSON object")
	}
	return request, nil
}

func (request assignmentRequest) assignment() (substitution.Assignment, error) {
	switch request.Type {
	case substitution.TargetGroupHandover:
		if request.GroupHandover == nil || request.AdditionalSupervision != nil {
			return substitution.Assignment{}, substitution.ErrInvalidTarget
		}
		start, err := optionalDate(request.GroupHandover.StartDate)
		if err != nil {
			return substitution.Assignment{}, substitution.ErrInvalidPeriod
		}
		end, err := optionalDate(request.GroupHandover.EndDate)
		if err != nil {
			return substitution.Assignment{}, substitution.ErrInvalidPeriod
		}
		return substitution.Assignment{Type: request.Type, GroupHandover: &substitution.GroupHandoverAssignment{
			GroupID: request.GroupHandover.GroupID, TargetStaffID: request.GroupHandover.TargetStaffID,
			StartDate: start, EndDate: end,
		}}, nil
	case substitution.TargetAdditionalSupervision:
		if request.AdditionalSupervision == nil || request.GroupHandover != nil {
			return substitution.Assignment{}, substitution.ErrInvalidTarget
		}
		return substitution.Assignment{Type: request.Type, AdditionalSupervision: &substitution.AdditionalSupervisionAssignment{
			ActiveGroupID: request.AdditionalSupervision.ActiveGroupID,
			TargetStaffID: request.AdditionalSupervision.TargetStaffID,
		}}, nil
	default:
		return substitution.Assignment{}, substitution.ErrInvalidTarget
	}
}

func (rs *Resource) end(w http.ResponseWriter, r *http.Request) {
	var request endRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestMessageWithCode("Die Anfrage ist ungültig.", "invalid_target"))
		return
	}
	caller, err := callerFromContext(r.Context())
	if err != nil {
		renderModuleError(w, r, err)
		return
	}
	if err := rs.Service.End(r.Context(), caller, substitution.EndRequest{Type: request.Type, ID: request.ID}); err != nil {
		renderModuleError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]bool{"ended": true}, "Gruppenübergabe beendet")
}

func optionalDate(raw string) (*timezone.Date, error) {
	if raw == "" {
		return nil, nil
	}
	date, err := timezone.ParseDate(raw)
	if err != nil {
		return nil, err
	}
	return &date, nil
}

func callerFromContext(ctx context.Context) (substitution.SubstitutionCaller, error) {
	principal, err := permissions.PrincipalFromContext(ctx)
	if err != nil || principal.TenantID() != tenant.FromContext(ctx) {
		return substitution.SubstitutionCaller{}, substitution.ErrForbidden
	}
	return substitution.SubstitutionCaller{
		AccountID: principal.AccountID(), TenantID: principal.TenantID(), Scope: string(principal.Scope()),
		Roles: principal.Roles(), Admin: principal.HasAdminScope(),
	}, nil
}

func renderModuleError(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, moduleErrorRenderer(err))
}

type moduleErrorSpec struct {
	target  error
	status  int
	code    string
	message string
}

var moduleErrorSpecs = []moduleErrorSpec{
	{target: substitution.ErrNotFound, status: http.StatusNotFound, code: "not_found", message: "Die Auswahl ist nicht mehr verfügbar."},
	{target: substitution.ErrForbidden, status: http.StatusForbidden, code: "forbidden", message: "Diese Aktion ist nicht erlaubt."},
	{target: substitution.ErrInvalidTarget, status: http.StatusBadRequest, code: "invalid_target", message: "Die ausgewählte Gruppe, Betreuung oder Person ist ungültig."},
	{target: substitution.ErrInvalidPeriod, status: http.StatusBadRequest, code: "invalid_period", message: "Der Zeitraum ist ungültig."},
	{target: substitution.ErrNotRunning, status: http.StatusConflict, code: "not_running", message: "Die Auswahl ist nicht mehr gültig."},
	{target: substitution.ErrAlreadyAssigned, status: http.StatusConflict, code: "already_assigned", message: "Diese Person ist bereits eingetragen."},
	{target: substitution.ErrSelfAssignment, status: http.StatusBadRequest, code: "self_assignment", message: "Sie können sich nicht selbst hinzufügen."},
}

var internalModuleError = moduleErrorSpec{
	status: http.StatusInternalServerError, code: "internal", message: "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
}

func moduleErrorResponse(spec moduleErrorSpec) func(error) render.Renderer {
	return func(err error) render.Renderer {
		return &common.ErrResponse{
			Err: err, HTTPStatusCode: spec.status, Status: "error", ErrorText: spec.message, Code: spec.code,
		}
	}
}

func moduleErrorRules() []common.ErrorRule {
	rules := make([]common.ErrorRule, 0, len(moduleErrorSpecs))
	for _, spec := range moduleErrorSpecs {
		rules = append(rules, common.ErrorRule{Target: spec.target, Render: moduleErrorResponse(spec)})
	}
	return rules
}

var moduleErrorRenderer = common.RulesRenderer(moduleErrorRules(), moduleErrorResponse(internalModuleError))
