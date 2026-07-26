package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Constants for error messages and time formatting
const (
	// timeFormatISO8601 carries FIXED-WIDTH microsecond precision (PostgreSQL's
	// own timestamp resolution) and is always applied to a UTC time.
	//
	// Both properties are load-bearing for the revert UI. It must offer exactly
	// the transition the backend considers latest (`applied_at DESC NULLS LAST,
	// id DESC` in LockLatestApplied) or the admin gets a 409, refetches, and is
	// offered the same rejected target forever. At second precision two applies
	// in the same second serialized identically, hiding a real ordering behind
	// the id tiebreak; a variable-width fraction would be worse still, since the
	// client compares these strings lexically and ".5Z" sorts before "Z"
	// (#405 review).
	timeFormatISO8601       = "2006-01-02T15:04:05.000000Z"
	errMsgNoAccountID       = "no account ID in context"
	errMsgInvalidTransition = "invalid transition ID"
)

// GradeTransitionResource handles grade transition API endpoints
type GradeTransitionResource struct {
	service *educationService.GradeTransitionService
	db      *bun.DB
}

// NewGradeTransitionResource creates a new grade transition resource
func NewGradeTransitionResource(service *educationService.GradeTransitionService, db *bun.DB) *GradeTransitionResource {
	return &GradeTransitionResource{
		service: service,
		db:      db,
	}
}

// Router returns a configured router for grade transition endpoints
func (rs *GradeTransitionResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// All routes require authentication
	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		// Read operations
		r.With(authorize.RequiresPermission(permissions.GradeTransitionsRead), withTx).
			Get("/", rs.list)
		r.With(authorize.RequiresPermission(permissions.GradeTransitionsRead), withTx).
			Get("/classes", rs.getDistinctClasses)
		r.With(authorize.RequiresPermission(permissions.GradeTransitionsRead), withTx).
			Get("/suggest", rs.suggestMappings)

		// Create operations
		r.With(authorize.RequiresPermission(permissions.GradeTransitionsCreate), withTx).
			Post("/", rs.create)

		// Individual transition routes
		r.Route("/{id}", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GradeTransitionsRead), withTx).
				Get("/", rs.getByID)
			r.With(authorize.RequiresPermission(permissions.GradeTransitionsRead), withTx).
				Get("/preview", rs.preview)
			r.With(authorize.RequiresPermission(permissions.GradeTransitionsRead), withTx).
				Get("/history", rs.getHistory)

			r.With(authorize.RequiresPermission(permissions.GradeTransitionsUpdate), withTx).
				Put("/", rs.update)

			r.With(authorize.RequiresPermission(permissions.GradeTransitionsDelete), withTx).
				Delete("/", rs.delete)

			r.With(authorize.RequiresPermission(permissions.GradeTransitionsApply), withTx).
				Post("/apply", rs.apply)

			r.With(authorize.RequiresPermission(permissions.GradeTransitionsApply), withTx).
				Post("/revert", rs.revert)
		})
	})

	return r
}

// Request/Response types

// TransitionRequest represents a request to create or update a transition
type TransitionRequest struct {
	AcademicYear string           `json:"academic_year"`
	Notes        *string          `json:"notes,omitempty"`
	Mappings     []MappingRequest `json:"mappings,omitempty"`
}

// Bind performs basic binding validation for the transition request.
// Note: academic_year is validated in the specific handler (create requires it, update does not)
func (req *TransitionRequest) Bind(_ *http.Request) error {
	return nil
}

// ApplyRequest carries the fingerprint of the preview the admin confirmed. It is
// optional (a caller without a preview omits it), but when present the apply is
// refused with 409 preview_stale unless the current cohort still matches — the
// confirmation must bind to the children that were actually shown (#405 review).
type ApplyRequest struct {
	ExpectedFingerprint string `json:"expected_fingerprint,omitempty"`
}

// MappingRequest represents a class mapping in a request
type MappingRequest struct {
	FromClass string  `json:"from_class"`
	ToClass   *string `json:"to_class,omitempty"` // null = graduate
}

// TransitionResponse represents a transition in API responses.
//
// Every int64 identifier is serialized as a JSON STRING (`,string`). The sole
// consumer is a browser, where JSON numbers are IEEE-754 doubles: an id beyond
// 2^53 is silently rounded on parse, so two distinct rows can collapse into one
// key and an apply/revert can address a transition the admin never selected.
// Sending the digits as text keeps the value exact end to end, and the frontend
// already models every backend id as a string (#405 review).
type TransitionResponse struct {
	ID           int64             `json:"id,string"`
	AcademicYear string            `json:"academic_year"`
	Status       string            `json:"status"`
	AppliedAt    *string           `json:"applied_at,omitempty"`
	AppliedBy    *int64            `json:"applied_by,string,omitempty"`
	RevertedAt   *string           `json:"reverted_at,omitempty"`
	RevertedBy   *int64            `json:"reverted_by,string,omitempty"`
	CreatedAt    string            `json:"created_at"`
	CreatedBy    int64             `json:"created_by,string"`
	Notes        *string           `json:"notes,omitempty"`
	Mappings     []MappingResponse `json:"mappings,omitempty"`
	CanModify    bool              `json:"can_modify"`
	CanApply     bool              `json:"can_apply"`
	CanRevert    bool              `json:"can_revert"`
}

// MappingResponse represents a mapping in API responses. Ids are strings for the
// reason given on TransitionResponse.
type MappingResponse struct {
	ID        int64   `json:"id,string"`
	FromClass string  `json:"from_class"`
	ToClass   *string `json:"to_class,omitempty"`
	Action    string  `json:"action"` // "promote" or "graduate"
}

// HistoryResponse represents one grade transition history row in API responses.
//
// It exists so the ledger's ids cross the wire as strings like every other id in
// this API; rendering the model directly would emit them as JSON numbers (the
// json tags on models/education.GradeTransitionHistory and base.Model are shared
// with persistence and are not this endpoint's to change).
//
// The ledger's rfid_tag column is deliberately NOT part of this shape: this
// route requires only grade_transitions:read, which does not imply the right to
// read children's RFID identifiers, and the UI has no use for the value — it
// exists solely so a revert can re-link the released tags server-side.
type HistoryResponse struct {
	ID           int64   `json:"id,string"`
	TransitionID int64   `json:"transition_id,string"`
	StudentID    int64   `json:"student_id,string"`
	PersonName   string  `json:"person_name"`
	FromClass    string  `json:"from_class"`
	ToClass      *string `json:"to_class,omitempty"`
	Action       string  `json:"action"`
	FromStatus   *string `json:"from_status,omitempty"`
	CreatedAt    string  `json:"created_at"`
	// StudentState is the child's state TODAY ("alumnus" | "restored" |
	// "purged"), not what the ledger recorded. The ledger is append-only, so a
	// graduated child who has since been reverted or hard-deleted still reads as
	// "graduated" in Action — only this field tells the Abgänge view which
	// actions are still possible.
	StudentState string `json:"student_state"`
}

// toHistoryResponses converts the ledger rows to their API shape. states maps a
// student id to its current lifecycle state; an id missing from it is reported
// as purged, which is what an absent student row means.
func toHistoryResponses(rows []*education.GradeTransitionHistory, states map[int64]string) []HistoryResponse {
	out := make([]HistoryResponse, 0, len(rows))
	for _, h := range rows {
		state, ok := states[h.StudentID]
		if !ok {
			state = educationService.GraduateStatePurged
		}
		out = append(out, HistoryResponse{
			StudentState: state,
			ID:           h.ID,
			TransitionID: h.TransitionID,
			StudentID:    h.StudentID,
			PersonName:   h.PersonName,
			FromClass:    h.FromClass,
			ToClass:      h.ToClass,
			Action:       h.Action,
			FromStatus:   h.FromStatus,
			CreatedAt:    h.CreatedAt.UTC().Format(timeFormatISO8601),
		})
	}
	return out
}

// toTransitionResponse converts a model to a response
func toTransitionResponse(t *education.GradeTransition) TransitionResponse {
	resp := TransitionResponse{
		ID:           t.ID,
		AcademicYear: t.AcademicYear,
		Status:       t.Status,
		CreatedAt:    t.CreatedAt.UTC().Format(timeFormatISO8601),
		CreatedBy:    t.CreatedBy,
		Notes:        t.Notes,
		CanModify:    t.CanModify(),
		CanApply:     t.CanApply(),
		CanRevert:    t.CanRevert(),
	}

	if t.AppliedAt != nil {
		formatted := t.AppliedAt.UTC().Format(timeFormatISO8601)
		resp.AppliedAt = &formatted
	}
	if t.AppliedBy != nil {
		resp.AppliedBy = t.AppliedBy
	}
	if t.RevertedAt != nil {
		formatted := t.RevertedAt.UTC().Format(timeFormatISO8601)
		resp.RevertedAt = &formatted
	}
	if t.RevertedBy != nil {
		resp.RevertedBy = t.RevertedBy
	}

	// Convert mappings
	if len(t.Mappings) > 0 {
		resp.Mappings = make([]MappingResponse, 0, len(t.Mappings))
		for _, m := range t.Mappings {
			resp.Mappings = append(resp.Mappings, MappingResponse{
				ID:        m.ID,
				FromClass: m.FromClass,
				ToClass:   m.ToClass,
				Action:    m.GetAction(),
			})
		}
	}

	return resp
}

// Handlers

// list returns all grade transitions
func (rs *GradeTransitionResource) list(w http.ResponseWriter, r *http.Request) {
	options := base.NewQueryOptions()

	// Parse pagination
	page, pageSize := common.ParsePagination(r)
	options.WithPagination(page, pageSize)

	// Parse filters
	filter := base.NewFilter()
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Equal("status", status)
	}
	if academicYear := r.URL.Query().Get("academic_year"); academicYear != "" {
		filter.Equal("academic_year", academicYear)
	}
	options.Filter = filter

	transitions, total, err := rs.service.List(r.Context(), options)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response format
	responses := make([]TransitionResponse, 0, len(transitions))
	for _, t := range transitions {
		responses = append(responses, toTransitionResponse(t))
	}

	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, "Grade transitions retrieved successfully")
}

// create creates a new grade transition
func (rs *GradeTransitionResource) create(w http.ResponseWriter, r *http.Request) {
	req := &TransitionRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Validate required field for create
	if req.AcademicYear == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("academic_year is required")))
		return
	}

	// Get account ID from JWT
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New(errMsgNoAccountID)))
		return
	}
	accountID := int64(claims.ID)

	// Convert request to service request
	createReq := educationService.CreateTransitionRequest{
		AcademicYear: req.AcademicYear,
		Notes:        req.Notes,
		CreatedBy:    accountID,
	}

	// Convert mappings
	for _, m := range req.Mappings {
		createReq.Mappings = append(createReq.Mappings, educationService.MappingRequest{
			FromClass: m.FromClass,
			ToClass:   m.ToClass,
		})
	}

	tenantID := tenant.FromContext(r.Context())
	var transition *education.GradeTransition
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		transition, txErr = rs.service.Create(ctx, createReq)
		return txErr
	}); err != nil {
		if errors.Is(err, educationService.ErrInvalidTransitionData) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, toTransitionResponse(transition), "Grade transition created successfully")
}

// getByID returns a single grade transition
func (rs *GradeTransitionResource) getByID(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", errMsgInvalidTransition)
	if !ok {
		return
	}

	transition, err := rs.service.GetByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("grade transition not found")))
		return
	}

	common.Respond(w, r, http.StatusOK, toTransitionResponse(transition), "Grade transition retrieved successfully")
}

// update updates a grade transition
func (rs *GradeTransitionResource) update(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", errMsgInvalidTransition)
	if !ok {
		return
	}

	req := &TransitionRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Convert request to service request
	updateReq := educationService.UpdateTransitionRequest{
		Notes: req.Notes,
	}
	if req.AcademicYear != "" {
		updateReq.AcademicYear = &req.AcademicYear
	}

	// Convert mappings if provided (nil = not provided, empty = clear all)
	if req.Mappings != nil {
		updateReq.Mappings = make([]educationService.MappingRequest, 0, len(req.Mappings))
		for _, m := range req.Mappings {
			updateReq.Mappings = append(updateReq.Mappings, educationService.MappingRequest{
				FromClass: m.FromClass,
				ToClass:   m.ToClass,
			})
		}
	}

	tenantID := tenant.FromContext(r.Context())
	var transition *education.GradeTransition
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		transition, txErr = rs.service.Update(ctx, id, updateReq)
		return txErr
	}); err != nil {
		rs.renderDraftMutationError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, toTransitionResponse(transition), "Grade transition updated successfully")
}

// delete deletes a grade transition
func (rs *GradeTransitionResource) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", errMsgInvalidTransition)
	if !ok {
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.service.Delete(ctx, id)
	}); err != nil {
		rs.renderDraftMutationError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Grade transition deleted successfully")
}

// renderDraftMutationError classifies the expected outcomes of editing or
// deleting a loaded draft that the server state has since moved past: the
// draft was applied or reverted by another admin (409, so the UI refreshes the
// stale editor), it was deleted entirely (404), or the submitted data is
// invalid (400, so the admin corrects it). Everything else is a real server
// fault and stays 500 (#405 review).
func (rs *GradeTransitionResource) renderDraftMutationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, educationService.ErrTransitionNotDraft):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "not_draft"))
	case errors.Is(err, sql.ErrNoRows):
		common.RenderError(w, r, common.ErrorNotFound(errors.New("grade transition not found")))
	case errors.Is(err, educationService.ErrInvalidTransitionData):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// preview returns a preview of what will happen when the transition is applied
func (rs *GradeTransitionResource) preview(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", errMsgInvalidTransition)
	if !ok {
		return
	}

	preview, err := rs.service.Preview(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, preview, "Transition preview generated successfully")
}

// apply executes the grade transition
func (rs *GradeTransitionResource) apply(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", errMsgInvalidTransition)
	if !ok {
		return
	}

	// Get account ID from JWT
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New(errMsgNoAccountID)))
		return
	}
	accountID := int64(claims.ID)

	// The body is optional: an absent or EMPTY one (io.EOF) means "no preview to
	// check". Anything else must be rejected. A truncated or malformed payload
	// used to be swallowed, leaving ExpectedFingerprint empty — which ApplyChecked
	// reads as "caller opted out of the stale-preview check", so a destructive
	// transition ran unguarded on a cohort that may have changed since the admin
	// confirmed it (#405 review).
	var req ApplyRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("invalid request body: %w", err)))
			return
		}
	}

	tenantID := tenant.FromContext(r.Context())
	var result interface{}
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		result, txErr = rs.service.ApplyChecked(ctx, id, accountID, req.ExpectedFingerprint)
		return txErr
	}); err != nil {
		// Graduating children still checked in is a client-recoverable safety
		// condition, not a server fault: return 409 with a stable code so the UI
		// can tell the admin to check them out first, instead of a bare 500 (#405).
		if errors.Is(err, educationService.ErrGraduatesCheckedIn) {
			common.RenderError(w, r, common.ErrorConflictWithCode(err, "graduates_checked_in"))
			return
		}
		// The confirmed preview no longer describes the current data. Also
		// client-recoverable: the UI reloads the preview and asks again (#405).
		if errors.Is(err, educationService.ErrPreviewStale) {
			common.RenderError(w, r, common.ErrorConflictWithCode(err, "preview_stale"))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Grade transition applied successfully")
}

// revert undoes an applied grade transition
func (rs *GradeTransitionResource) revert(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", errMsgInvalidTransition)
	if !ok {
		return
	}

	// Get account ID from JWT
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New(errMsgNoAccountID)))
		return
	}
	accountID := int64(claims.ID)

	tenantID := tenant.FromContext(r.Context())
	var result interface{}
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		result, txErr = rs.service.Revert(ctx, id, accountID)
		return txErr
	}); err != nil {
		// Reverting anything but the latest applied transition is a client-
		// recoverable ordering conflict (the admin's list was stale), not a server
		// fault: return 409 with a stable code so the UI can refresh and revert the
		// newest first, instead of a bare 500 (#405).
		if errors.Is(err, educationService.ErrNotLatestApplied) {
			common.RenderError(w, r, common.ErrorConflictWithCode(err, "not_latest_transition"))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Grade transition reverted successfully")
}

// getDistinctClasses returns all distinct school class values
func (rs *GradeTransitionResource) getDistinctClasses(w http.ResponseWriter, r *http.Request) {
	classes, err := rs.service.GetDistinctClasses(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, classes, "Distinct classes retrieved successfully")
}

// suggestMappings returns auto-suggested class mappings
func (rs *GradeTransitionResource) suggestMappings(w http.ResponseWriter, r *http.Request) {
	suggestions, err := rs.service.SuggestMappings(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, suggestions, "Mapping suggestions generated successfully")
}

// getHistory returns the history records for a transition
func (rs *GradeTransitionResource) getHistory(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", errMsgInvalidTransition)
	if !ok {
		return
	}

	history, states, err := rs.service.GetHistoryWithStudentStates(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, toHistoryResponses(history, states), "Transition history retrieved successfully")
}
