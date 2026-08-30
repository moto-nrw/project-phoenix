package substitutions

import (
	"context"
	"encoding/json"
	"errors"
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
	ScheduleSubstitution *scheduleAssignmentRequest `json:"schedule_substitution"`
}

type scheduleAssignmentRequest struct {
	InstanceID           int64                                      `json:"instance_id"`
	UnderstaffedAck      *bool                                      `json:"understaffed_ack,omitempty"`
	UnderstaffedNote     *string                                    `json:"understaffed_note,omitempty"`
	Absences             []substitution.ScheduleAbsenceChange       `json:"absences,omitempty"`
	Substitutions        []substitution.ScheduleSubstitutionChange  `json:"substitutions,omitempty"`
	SubstitutionRemovals []substitution.ScheduleSubstitutionRemoval `json:"substitution_removals,omitempty"`
	Presences            []substitution.SchedulePresenceChange      `json:"presences,omitempty"`
	WholeDays            *struct {
		AbsentStaffID     int64           `json:"absent_staff_id"`
		SubstituteStaffID *int64          `json:"substitute_staff_id,omitempty"`
		Dates             []timezone.Date `json:"dates"`
		Reason            *string         `json:"reason,omitempty"`
	} `json:"whole_days,omitempty"`
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
	if raw := r.URL.Query().Get("date"); raw != "" {
		date, err := timezone.ParseDate(raw)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequestMessageWithCode("Das Datum ist ungültig.", "invalid_period"))
			return
		}
		query.On = &date
	}
	if !parseScheduleRange(w, r, &query) {
		return
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
	common.Respond(w, r, http.StatusOK, result, "Vertretungen geladen")
}

func (rs *Resource) assign(w http.ResponseWriter, r *http.Request) {
	var request assignmentRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestMessageWithCode("Die Anfrage ist ungültig.", "invalid_target"))
		return
	}
	assignment, err := request.toAssignment()
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
	if created.ScheduleSubstitution != nil {
		common.Respond(w, r, http.StatusCreated, created.ScheduleSubstitution, "Vertretung gespeichert")
		return
	}
	common.Respond(w, r, http.StatusCreated, created, "Gruppe übergeben")
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
	message := "Gruppenübergabe beendet"
	if request.Type == substitution.TargetScheduleSubstitution {
		message = "Vertretung beendet"
	}
	common.Respond(w, r, http.StatusOK, map[string]bool{"ended": true}, message)
}

func parseScheduleRange(w http.ResponseWriter, r *http.Request, query *substitution.OverviewQuery) bool {
	fromRaw, toRaw := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if fromRaw == "" && toRaw == "" {
		return true
	}
	from, fromErr := timezone.ParseDate(fromRaw)
	to, toErr := timezone.ParseDate(toRaw)
	if fromErr != nil || toErr != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestMessageWithCode("Der Zeitraum ist ungültig.", "invalid_period"))
		return false
	}
	query.ScheduleFrom, query.ScheduleTo, query.IncludeScheduleTargets = &from, &to, true
	return true
}

func (request assignmentRequest) toAssignment() (substitution.Assignment, error) {
	assignment := substitution.Assignment{Type: request.Type}
	switch request.Type {
	case substitution.TargetGroupHandover:
		if request.GroupHandover == nil {
			return assignment, invalidAssignmentRequest("Die Anfrage ist ungültig.", substitution.ErrInvalidTarget, "invalid_target")
		}
		start, err := optionalDate(request.GroupHandover.StartDate)
		if err != nil {
			return assignment, invalidAssignmentRequest("Das Startdatum ist ungültig.", substitution.ErrInvalidPeriod, "invalid_period")
		}
		end, err := optionalDate(request.GroupHandover.EndDate)
		if err != nil {
			return assignment, invalidAssignmentRequest("Das Enddatum ist ungültig.", substitution.ErrInvalidPeriod, "invalid_period")
		}
		assignment.GroupHandover = &substitution.GroupHandoverAssignment{
			GroupID: request.GroupHandover.GroupID, TargetStaffID: request.GroupHandover.TargetStaffID,
			StartDate: start, EndDate: end,
		}
	case substitution.TargetScheduleSubstitution:
		return request.toScheduleAssignment(assignment)
	default:
		return assignment, invalidAssignmentRequest("Die Anfrage ist ungültig.", substitution.ErrInvalidTarget, "invalid_target")
	}
	return assignment, nil
}

func (request assignmentRequest) toScheduleAssignment(assignment substitution.Assignment) (substitution.Assignment, error) {
	if request.ScheduleSubstitution == nil {
		return assignment, invalidAssignmentRequest("Die Anfrage ist ungültig.", substitution.ErrInvalidTarget, "invalid_target")
	}
	wire := request.ScheduleSubstitution
	value := &substitution.ScheduleSubstitutionAssignment{
		InstanceID: wire.InstanceID, UnderstaffedAck: wire.UnderstaffedAck, UnderstaffedNote: wire.UnderstaffedNote,
		Absences: wire.Absences, Substitutions: wire.Substitutions,
		SubstitutionRemovals: wire.SubstitutionRemovals, Presences: wire.Presences,
	}
	if wire.WholeDays != nil {
		value.WholeDays = &substitution.ScheduleWholeDayAssignment{
			AbsentStaffID: wire.WholeDays.AbsentStaffID, SubstituteStaffID: wire.WholeDays.SubstituteStaffID,
			Dates: wire.WholeDays.Dates, Reason: wire.WholeDays.Reason,
		}
	}
	assignment.ScheduleSubstitution = value
	return assignment, nil
}

func invalidAssignmentRequest(message string, target error, code string) error {
	return &substitution.OperationError{Target: target, Code: code, Message: message}
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
		Roles: principal.Roles(), Admin: principal.HasAdminScope(), HasPermission: principal.HasPermission,
	}, nil
}

func renderModuleError(w http.ResponseWriter, r *http.Request, err error) {
	var operation *substitution.OperationError
	if errors.As(err, &operation) {
		common.RenderError(w, r, moduleErrorResponse(operationErrorSpec(operation))(err))
		return
	}
	common.RenderError(w, r, moduleErrorRenderer(err))
}

type moduleErrorSpec struct {
	target  error
	status  int
	code    string
	message string
}

var moduleErrorSpecs = []moduleErrorSpec{
	{target: substitution.ErrNotFound, status: http.StatusNotFound, code: "not_found", message: "Gruppenübergabe nicht gefunden."},
	{target: substitution.ErrForbidden, status: http.StatusForbidden, code: "forbidden", message: "Diese Aktion ist nicht erlaubt."},
	{target: substitution.ErrInvalidTarget, status: http.StatusBadRequest, code: "invalid_target", message: "Die ausgewählte Gruppe oder Fachkraft ist ungültig."},
	{target: substitution.ErrInvalidPeriod, status: http.StatusBadRequest, code: "invalid_period", message: "Der Zeitraum ist ungültig."},
	{target: substitution.ErrNotRunning, status: http.StatusConflict, code: "not_running", message: "Die Gruppenübergabe ist nicht mehr aktiv."},
	{target: substitution.ErrAlreadyAssigned, status: http.StatusConflict, code: "already_assigned", message: "Diese Gruppenübergabe besteht bereits."},
	{target: substitution.ErrConflict, status: http.StatusConflict, code: "conflict", message: "Die Änderung steht im Konflikt mit der aktuellen Planung."},
}

func operationErrorSpec(operation *substitution.OperationError) moduleErrorSpec {
	for _, spec := range moduleErrorSpecs {
		if errors.Is(operation.Target, spec.target) {
			if operation.Code != "" {
				spec.code = operation.Code
			}
			if operation.Message != "" {
				spec.message = operation.Message
			}
			return spec
		}
	}
	return internalModuleError
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
