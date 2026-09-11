// Package substitutions serves the established /api/substitutions contract
// through the Workforce capability: the overview of running group handovers
// and supervisions, assigning a substitute, and ending an assignment.
//
// Authentication, transaction scoping, caller resolution and rendering are
// supplied by the composition root through Runtime.
package substitutions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// Middleware is the HTTP middleware shape the composition root supplies.
type Middleware = func(http.Handler) http.Handler

// Failure is a rejected request in the shape the shared error envelope
// renders: the HTTP status, the stable code, the caller-facing message and
// the underlying error for the log.
type Failure struct {
	Status  int
	Code    string
	Message string
	Err     error
}

// Runtime carries the HTTP-platform behavior this adapter must not own.
type Runtime struct {
	Protected func(chi.Router, func(chi.Router, Middleware))
	// Caller resolves the authenticated principal of the request for the
	// request's tenant; a mismatch yields ErrSubstitutionForbidden.
	Caller  func(context.Context) (workforce.SubstitutionCaller, error)
	Success func(http.ResponseWriter, *http.Request, int, any, string)
	Failure func(http.ResponseWriter, *http.Request, Failure)
}

type Resource struct {
	substitutions workforce.Substitutions
	runtime       Runtime
}

func NewResource(substitutions workforce.Substitutions, runtime Runtime) *Resource {
	return &Resource{substitutions: substitutions, runtime: runtime}
}

func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	rs.runtime.Protected(r, func(r chi.Router, withTx Middleware) {
		r.With(withTx).Get("/", rs.overview)
		r.With(withTx).Post("/", rs.assign)
		r.With(withTx).Post("/end", rs.end)
	})
	return r
}

// jsonID accepts a JSON number or a decimal string. A string is parsed with
// strconv, so digits beyond float64 precision survive, and anything that is
// not a plain integer is rejected instead of being coerced to zero.
type jsonID int64

func (id jsonID) Int64() int64 { return int64(id) }

func (id *jsonID) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, `"`) {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return errors.New("id must not be empty")
		}
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid id %q: %w", raw, err)
		}
		*id = jsonID(parsed)
		return nil
	}
	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid id %s: %w", trimmed, err)
	}
	*id = jsonID(parsed)
	return nil
}

// wireDate is a strict YYYY-MM-DD calendar day on the wire; anything else is
// a decode error, as it was when the contract carried the shared date type.
type wireDate string

func (d *wireDate) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*d = ""
		return nil
	}
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw == "" {
		*d = ""
		return nil
	}
	if _, err := parseDate(raw); err != nil {
		return err
	}
	*d = wireDate(raw)
	return nil
}

func parseDate(raw string) (string, error) {
	parsed, err := time.Parse(workforce.DateLayout, raw)
	if err != nil || parsed.Format(workforce.DateLayout) != raw {
		return "", fmt.Errorf("invalid calendar date %q", raw)
	}
	return raw, nil
}

type assignmentRequest struct {
	Type          workforce.SubstitutionTargetType `json:"type"`
	GroupHandover *struct {
		GroupID       jsonID `json:"group_id"`
		TargetStaffID jsonID `json:"target_staff_id"`
		StartDate     string `json:"start_date,omitempty"`
		EndDate       string `json:"end_date,omitempty"`
	} `json:"group_handover"`
	AdditionalSupervision *struct {
		ActiveGroupID jsonID `json:"active_group_id"`
		TargetStaffID jsonID `json:"target_staff_id"`
	} `json:"additional_supervision"`
	ScheduleSubstitution *scheduleAssignmentRequest `json:"schedule_substitution"`
}

type scheduleAssignmentRequest struct {
	InstanceID           int64                                   `json:"instance_id"`
	UnderstaffedAck      *bool                                   `json:"understaffed_ack,omitempty"`
	UnderstaffedNote     *string                                 `json:"understaffed_note,omitempty"`
	Absences             []workforce.ScheduleAbsenceChange       `json:"absences,omitempty"`
	Substitutions        []workforce.ScheduleSubstitutionChange  `json:"substitutions,omitempty"`
	SubstitutionRemovals []workforce.ScheduleSubstitutionRemoval `json:"substitution_removals,omitempty"`
	Presences            []workforce.SchedulePresenceChange      `json:"presences,omitempty"`
	WholeDays            *struct {
		AbsentStaffID     int64      `json:"absent_staff_id"`
		SubstituteStaffID *int64     `json:"substitute_staff_id,omitempty"`
		Dates             []wireDate `json:"dates"`
		Reason            *string    `json:"reason,omitempty"`
	} `json:"whole_days,omitempty"`
}

type endRequest struct {
	Type workforce.SubstitutionTargetType `json:"type"`
	ID   jsonID                           `json:"id"`
}

func (rs *Resource) overview(w http.ResponseWriter, r *http.Request) {
	query := workforce.SubstitutionOverviewQuery{IncludeTargets: true}
	if raw := r.URL.Query().Get("group_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			rs.runtime.Failure(w, r, invalidRequest("Die Gruppe ist ungültig.", "invalid_target"))
			return
		}
		query.GroupID = id
	}
	if raw := r.URL.Query().Get("active_group_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			rs.runtime.Failure(w, r, invalidRequest("Die Gruppe ist ungültig.", "invalid_target"))
			return
		}
		query.ActiveGroupID = id
	}
	if raw := r.URL.Query().Get("date"); raw != "" {
		date, err := parseDate(raw)
		if err != nil {
			rs.runtime.Failure(w, r, invalidRequest("Das Datum ist ungültig.", "invalid_period"))
			return
		}
		query.On = date
	}
	if !rs.parseScheduleRange(w, r, &query) {
		return
	}
	caller, err := rs.runtime.Caller(r.Context())
	if err != nil {
		rs.renderModuleError(w, r, err)
		return
	}
	result, err := rs.substitutions.Overview(r.Context(), caller, query)
	if err != nil {
		rs.renderModuleError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, result, "Vertretungen geladen")
}

func (rs *Resource) assign(w http.ResponseWriter, r *http.Request) {
	request, err := decodeAssignment(r.Body)
	if err != nil {
		rs.runtime.Failure(w, r, invalidRequest("Die Anfrage ist ungültig.", "invalid_target"))
		return
	}
	assignment, err := request.toAssignment()
	if err != nil {
		rs.renderModuleError(w, r, err)
		return
	}
	caller, err := rs.runtime.Caller(r.Context())
	if err != nil {
		rs.renderModuleError(w, r, err)
		return
	}
	created, err := rs.substitutions.Assign(r.Context(), caller, assignment)
	if err != nil {
		rs.renderModuleError(w, r, err)
		return
	}
	if created.ScheduleSubstitution != nil {
		rs.runtime.Success(w, r, http.StatusCreated, created.ScheduleSubstitution, "Vertretung gespeichert")
		return
	}
	if request.Type == workforce.TargetAdditionalSupervision {
		rs.runtime.Success(w, r, http.StatusCreated, created, "Betreuer hinzugefügt")
		return
	}
	rs.runtime.Success(w, r, http.StatusCreated, created, "Gruppe übergeben")
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

func (rs *Resource) end(w http.ResponseWriter, r *http.Request) {
	var request endRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		rs.runtime.Failure(w, r, invalidRequest("Die Anfrage ist ungültig.", "invalid_target"))
		return
	}
	caller, err := rs.runtime.Caller(r.Context())
	if err != nil {
		rs.renderModuleError(w, r, err)
		return
	}
	if err := rs.substitutions.End(r.Context(), caller, workforce.SubstitutionEndRequest{Type: request.Type, ID: request.ID.Int64()}); err != nil {
		rs.renderModuleError(w, r, err)
		return
	}
	message := "Gruppenübergabe beendet"
	if request.Type == workforce.TargetScheduleSubstitution {
		message = "Vertretung beendet"
	}
	rs.runtime.Success(w, r, http.StatusOK, map[string]bool{"ended": true}, message)
}

func (rs *Resource) parseScheduleRange(w http.ResponseWriter, r *http.Request, query *workforce.SubstitutionOverviewQuery) bool {
	fromRaw, toRaw := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if fromRaw == "" && toRaw == "" {
		return true
	}
	from, fromErr := parseDate(fromRaw)
	to, toErr := parseDate(toRaw)
	if fromErr != nil || toErr != nil {
		rs.runtime.Failure(w, r, invalidRequest("Der Zeitraum ist ungültig.", "invalid_period"))
		return false
	}
	query.ScheduleFrom, query.ScheduleTo, query.IncludeScheduleTargets = from, to, true
	return true
}

func (request assignmentRequest) toAssignment() (workforce.SubstitutionAssignment, error) {
	assignment := workforce.SubstitutionAssignment{Type: request.Type}
	switch request.Type {
	case workforce.TargetGroupHandover:
		if request.GroupHandover == nil {
			return assignment, invalidAssignmentRequest("Die Anfrage ist ungültig.", workforce.ErrSubstitutionInvalidTarget, "invalid_target")
		}
		start, err := optionalDate(request.GroupHandover.StartDate)
		if err != nil {
			return assignment, invalidAssignmentRequest("Das Startdatum ist ungültig.", workforce.ErrSubstitutionInvalidPeriod, "invalid_period")
		}
		end, err := optionalDate(request.GroupHandover.EndDate)
		if err != nil {
			return assignment, invalidAssignmentRequest("Das Enddatum ist ungültig.", workforce.ErrSubstitutionInvalidPeriod, "invalid_period")
		}
		assignment.GroupHandover = &workforce.GroupHandoverAssignment{
			GroupID: request.GroupHandover.GroupID.Int64(), TargetStaffID: request.GroupHandover.TargetStaffID.Int64(),
			StartDate: start, EndDate: end,
		}
	case workforce.TargetScheduleSubstitution:
		return request.toScheduleAssignment(assignment)
	case workforce.TargetAdditionalSupervision:
		if request.AdditionalSupervision == nil {
			return assignment, invalidAssignmentRequest("Die Anfrage ist ungültig.", workforce.ErrSubstitutionInvalidTarget, "invalid_target")
		}
		assignment.AdditionalSupervision = &workforce.AdditionalSupervisionAssignment{
			ActiveGroupID: request.AdditionalSupervision.ActiveGroupID.Int64(), TargetStaffID: request.AdditionalSupervision.TargetStaffID.Int64(),
		}
	default:
		return assignment, invalidAssignmentRequest("Die Anfrage ist ungültig.", workforce.ErrSubstitutionInvalidTarget, "invalid_target")
	}
	return assignment, nil
}

func (request assignmentRequest) toScheduleAssignment(assignment workforce.SubstitutionAssignment) (workforce.SubstitutionAssignment, error) {
	if request.ScheduleSubstitution == nil {
		return assignment, invalidAssignmentRequest("Die Anfrage ist ungültig.", workforce.ErrSubstitutionInvalidTarget, "invalid_target")
	}
	wire := request.ScheduleSubstitution
	value := &workforce.ScheduleSubstitutionAssignment{
		InstanceID: wire.InstanceID, UnderstaffedAck: wire.UnderstaffedAck, UnderstaffedNote: wire.UnderstaffedNote,
		Absences: wire.Absences, Substitutions: wire.Substitutions,
		SubstitutionRemovals: wire.SubstitutionRemovals, Presences: wire.Presences,
	}
	if wire.WholeDays != nil {
		dates := make([]string, 0, len(wire.WholeDays.Dates))
		for _, date := range wire.WholeDays.Dates {
			dates = append(dates, string(date))
		}
		value.WholeDays = &workforce.ScheduleWholeDayAssignment{
			AbsentStaffID: wire.WholeDays.AbsentStaffID, SubstituteStaffID: wire.WholeDays.SubstituteStaffID,
			Dates: dates, Reason: wire.WholeDays.Reason,
		}
	}
	assignment.ScheduleSubstitution = value
	return assignment, nil
}

func invalidAssignmentRequest(message string, target error, code string) error {
	return &workforce.SubstitutionOperationError{Target: target, Code: code, Message: message}
}

func invalidRequest(message, code string) Failure {
	return Failure{Status: http.StatusBadRequest, Code: code, Message: message, Err: errors.New(message)}
}

func optionalDate(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	return parseDate(raw)
}

// renderModuleError maps a capability error to the stable status, code and
// message this contract has always exposed; an operation error may override
// code and message, an unknown error stays hidden behind the internal text.
func (rs *Resource) renderModuleError(w http.ResponseWriter, r *http.Request, err error) {
	rs.runtime.Failure(w, r, classify(err))
}

func classify(err error) Failure {
	spec := internalModuleError
	if operation, ok := errors.AsType[*workforce.SubstitutionOperationError](err); ok {
		spec = operationErrorSpec(operation)
	} else {
		for _, candidate := range moduleErrorSpecs {
			if errors.Is(err, candidate.target) {
				spec = candidate
				break
			}
		}
	}
	return Failure{Status: spec.status, Code: spec.code, Message: spec.message, Err: err}
}

type moduleErrorSpec struct {
	target  error
	status  int
	code    string
	message string
}

var moduleErrorSpecs = []moduleErrorSpec{
	{target: workforce.ErrSubstitutionNotFound, status: http.StatusNotFound, code: "not_found", message: "Gruppenübergabe nicht gefunden."},
	{target: workforce.ErrSubstitutionForbidden, status: http.StatusForbidden, code: "forbidden", message: "Diese Aktion ist nicht erlaubt."},
	{target: workforce.ErrSubstitutionInvalidTarget, status: http.StatusBadRequest, code: "invalid_target", message: "Die ausgewählte Gruppe oder Fachkraft ist ungültig."},
	{target: workforce.ErrSubstitutionInvalidPeriod, status: http.StatusBadRequest, code: "invalid_period", message: "Der Zeitraum ist ungültig."},
	{target: workforce.ErrSubstitutionNotRunning, status: http.StatusConflict, code: "not_running", message: "Die Gruppenübergabe ist nicht mehr aktiv."},
	{target: workforce.ErrSubstitutionAlreadyAssigned, status: http.StatusConflict, code: "already_assigned", message: "Diese Gruppenübergabe besteht bereits."},
	{target: workforce.ErrSubstitutionConflict, status: http.StatusConflict, code: "conflict", message: "Die Änderung steht im Konflikt mit der aktuellen Planung."},
	{target: workforce.ErrSubstitutionSelfAssignment, status: http.StatusBadRequest, code: "self_assignment", message: "Sie können sich nicht selbst hinzufügen."},
}

func operationErrorSpec(operation *workforce.SubstitutionOperationError) moduleErrorSpec {
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
