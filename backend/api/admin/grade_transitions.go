package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	"github.com/uptrace/bun"
)

// Constants for error messages and time formatting
const (
	// timeFormatISO8601 carries FIXED-WIDTH microsecond precision (PostgreSQL's
	// own timestamp resolution) and is always applied to a UTC time.
	//
	// Both properties are load-bearing for the revert UI. It must offer exactly
	// the transition the backend considers latest (`applied_at DESC NULLS LAST,
	// id DESC` in the owner's latest-applied lock) or the admin gets a 409,
	// refetches, and is offered the same rejected target forever. At second
	// precision two applies in the same second serialized identically, hiding a
	// real ordering behind the id tiebreak; a variable-width fraction would be
	// worse still, since the client compares these strings lexically and ".5Z"
	// sorts before "Z" (#405 review).
	timeFormatISO8601        = "2006-01-02T15:04:05.000000Z"
	errMsgInvalidTransition  = "invalid transition ID"
	errMsgTransitionNotFound = "grade transition not found"
)

// GradeTransitionResource is the HTTP adapter of the grade transition
// workflow (#2711). It parses, authorizes per route, calls exactly one
// workflow command and renders the outcome; the workflow owns the lock
// order, the preview fingerprint and the owner commands.
type GradeTransitionResource struct {
	workflow *gradetransition.Workflow
	db       *bun.DB
}

// NewGradeTransitionResource creates a new grade transition resource
func NewGradeTransitionResource(workflow *gradetransition.Workflow, db *bun.DB) *GradeTransitionResource {
	return &GradeTransitionResource{
		workflow: workflow,
		db:       db,
	}
}

// Router returns a configured router for grade transition endpoints
func (rs *GradeTransitionResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// All routes require authentication
	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		// Read operations
		r.With(common.RequiresPermission(permissions.GradeTransitionsRead), withTx).
			Get("/", rs.list)
		r.With(common.RequiresPermission(permissions.GradeTransitionsRead), withTx).
			Get("/classes", rs.getDistinctClasses)
		r.With(common.RequiresPermission(permissions.GradeTransitionsRead), withTx).
			Get("/suggest", rs.suggestMappings)

		// Create operations
		r.With(common.RequiresPermission(permissions.GradeTransitionsCreate), withTx).
			Post("/", rs.create)

		// Individual transition routes
		r.Route("/{id}", func(r chi.Router) {
			r.With(common.RequiresPermission(permissions.GradeTransitionsRead), withTx).
				Get("/", rs.getByID)
			r.With(common.RequiresPermission(permissions.GradeTransitionsRead), withTx).
				Get("/preview", rs.preview)
			r.With(common.RequiresPermission(permissions.GradeTransitionsRead), withTx).
				Get("/history", rs.getHistory)

			r.With(common.RequiresPermission(permissions.GradeTransitionsUpdate), withTx).
				Put("/", rs.update)

			r.With(common.RequiresPermission(permissions.GradeTransitionsDelete), withTx).
				Delete("/", rs.delete)

			r.With(common.RequiresPermission(permissions.GradeTransitionsApply), withTx).
				Post("/apply", rs.apply)

			r.With(common.RequiresPermission(permissions.GradeTransitionsApply), withTx).
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

// PreviewResponse is the wire shape of a transition preview.
type PreviewResponse struct {
	TransitionID    int64                    `json:"transition_id,string"`
	AcademicYear    string                   `json:"academic_year"`
	TotalStudents   int                      `json:"total_students"`
	ToPromote       int                      `json:"to_promote"`
	ToGraduate      int                      `json:"to_graduate"`
	ByMapping       []MappingPreviewResponse `json:"by_mapping"`
	UnmappedClasses []UnmappedClassResponse  `json:"unmapped_classes"`
	Warnings        []string                 `json:"warnings"`
	Fingerprint     string                   `json:"fingerprint"`
}

// MappingPreviewResponse shows the impact of a single mapping.
type MappingPreviewResponse struct {
	FromClass    string  `json:"from_class"`
	ToClass      *string `json:"to_class,omitempty"`
	StudentCount int     `json:"student_count"`
	Action       string  `json:"action"` // "promote" or "graduate"
}

// UnmappedClassResponse shows a class not included in the transition.
type UnmappedClassResponse struct {
	ClassName    string `json:"class_name"`
	StudentCount int    `json:"student_count"`
}

// ResultResponse is the wire shape of an apply or revert outcome.
type ResultResponse struct {
	TransitionID      int64    `json:"transition_id,string"`
	Status            string   `json:"status"`
	StudentsPromoted  int      `json:"students_promoted"`
	StudentsGraduated int      `json:"students_graduated"`
	CanRevert         bool     `json:"can_revert"`
	Warnings          []string `json:"warnings"`
}

// SuggestedMappingResponse is one auto-suggested class mapping.
type SuggestedMappingResponse struct {
	FromClass    string  `json:"from_class"`
	ToClass      *string `json:"to_class,omitempty"`
	StudentCount int     `json:"student_count"`
	IsGraduating bool    `json:"is_graduating"`
	// Ambiguous marks a class whose name does not match the grade pattern, so
	// the graduation guess is not confident. The editor must not preselect
	// Abgang for these.
	Ambiguous bool `json:"ambiguous,omitempty"`
}

// HistoryResponse represents one grade transition history row in API responses.
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

func toHistoryResponses(rows []gradetransition.HistoryEntry) []HistoryResponse {
	out := make([]HistoryResponse, 0, len(rows))
	for _, h := range rows {
		out = append(out, HistoryResponse{
			StudentState: h.StudentState,
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

func mappingAction(mapping schoolstructure.TransitionMapping) string {
	if mapping.IsGraduating() {
		return schoolstructure.TransitionActionGraduated
	}
	return schoolstructure.TransitionActionPromoted
}

// toTransitionResponse converts an owner transition to its wire shape.
func toTransitionResponse(t schoolstructure.Transition) TransitionResponse {
	resp := TransitionResponse{
		ID:           t.ID,
		AcademicYear: t.AcademicYear,
		Status:       t.Status,
		CreatedAt:    t.CreatedAt.UTC().Format(timeFormatISO8601),
		CreatedBy:    t.CreatedBy,
		Notes:        t.Notes,
		CanModify:    t.IsDraft(),
		CanApply:     t.CanApply(),
		CanRevert:    t.IsApplied(),
		AppliedBy:    t.AppliedBy,
		RevertedBy:   t.RevertedBy,
	}
	if t.AppliedAt != nil {
		formatted := t.AppliedAt.UTC().Format(timeFormatISO8601)
		resp.AppliedAt = &formatted
	}
	if t.RevertedAt != nil {
		formatted := t.RevertedAt.UTC().Format(timeFormatISO8601)
		resp.RevertedAt = &formatted
	}
	if len(t.Mappings) > 0 {
		resp.Mappings = make([]MappingResponse, 0, len(t.Mappings))
		for _, m := range t.Mappings {
			resp.Mappings = append(resp.Mappings, MappingResponse{
				ID:        m.ID,
				FromClass: m.FromClass,
				ToClass:   m.ToClass,
				Action:    mappingAction(m),
			})
		}
	}
	return resp
}

func toPreviewResponse(preview gradetransition.Preview) PreviewResponse {
	resp := PreviewResponse{
		TransitionID: preview.TransitionID, AcademicYear: preview.AcademicYear,
		TotalStudents: preview.TotalStudents, ToPromote: preview.ToPromote, ToGraduate: preview.ToGraduate,
		ByMapping:       make([]MappingPreviewResponse, 0, len(preview.ByMapping)),
		UnmappedClasses: make([]UnmappedClassResponse, 0, len(preview.UnmappedClasses)),
		Warnings:        preview.Warnings,
		Fingerprint:     preview.Fingerprint,
	}
	if resp.Warnings == nil {
		resp.Warnings = []string{}
	}
	for _, m := range preview.ByMapping {
		resp.ByMapping = append(resp.ByMapping, MappingPreviewResponse{
			FromClass: m.FromClass, ToClass: m.ToClass, StudentCount: m.StudentCount, Action: previewAction(m.Action),
		})
	}
	for _, u := range preview.UnmappedClasses {
		resp.UnmappedClasses = append(resp.UnmappedClasses, UnmappedClassResponse{ClassName: u.ClassName, StudentCount: u.StudentCount})
	}
	return resp
}

// previewAction renders the mapping action in the verb form the UI expects.
func previewAction(action string) string {
	if action == schoolstructure.TransitionActionGraduated {
		return "graduate"
	}
	return "promote"
}

func toResultResponse(result gradetransition.Result) ResultResponse {
	warnings := result.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return ResultResponse{
		TransitionID: result.TransitionID, Status: result.Status,
		StudentsPromoted: result.StudentsPromoted, StudentsGraduated: result.StudentsGraduated,
		CanRevert: result.CanRevert, Warnings: warnings,
	}
}

func toMappings(requests []MappingRequest) []gradetransition.Mapping {
	if requests == nil {
		return nil
	}
	mappings := make([]gradetransition.Mapping, 0, len(requests))
	for _, m := range requests {
		mappings = append(mappings, gradetransition.Mapping{FromClass: m.FromClass, ToClass: m.ToClass})
	}
	return mappings
}

// Handlers

// list returns all grade transitions
func (rs *GradeTransitionResource) list(w http.ResponseWriter, r *http.Request) {
	page, pageSize := common.ParsePagination(r)
	filter := gradetransition.ListFilter{
		Status:       r.URL.Query().Get("status"),
		AcademicYear: r.URL.Query().Get("academic_year"),
	}

	// Keyset mode: `after_id` windows by strictly increasing id instead of page
	// offsets. Offset pages re-slice the whole result on every request, so a row
	// created or deleted between two requests shifts the slicing and a survivor
	// can fall between pages. An id cursor never re-slices what was already
	// read: rows created during the iteration sort behind the cursor and appear
	// in a later window, deletions cannot move unread rows across the cursor
	// (#405 review).
	if rawAfterID := r.URL.Query().Get("after_id"); rawAfterID != "" {
		afterID, err := strconv.ParseInt(rawAfterID, 10, 64)
		if err != nil || afterID < 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid after_id")))
			return
		}
		filter.AfterID = afterID
		// A cursor request is always the first offset window of its remainder.
		page = 1
	}
	filter.Page, filter.PageSize = page, pageSize

	transitions, total, err := rs.workflow.List(r.Context(), filter)
	if err != nil {
		rs.renderReadError(w, r, err)
		return
	}

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

	transition, err := rs.workflow.Create(r.Context(), gradetransition.Draft{
		AcademicYear: req.AcademicYear,
		Notes:        req.Notes,
		Mappings:     toMappings(req.Mappings),
	})
	if err != nil {
		tenant.MarkRollback(r.Context())
		rs.renderDraftMutationError(w, r, err)
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

	transition, err := rs.workflow.Get(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New(errMsgTransitionNotFound)))
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

	patch := gradetransition.DraftPatch{Notes: req.Notes}
	if req.AcademicYear != "" {
		patch.AcademicYear = &req.AcademicYear
	}
	// Mappings: nil = not provided, empty = clear all.
	if req.Mappings != nil {
		patch.Mappings = toMappings(req.Mappings)
		if patch.Mappings == nil {
			patch.Mappings = []gradetransition.Mapping{}
		}
	}

	transition, err := rs.workflow.Update(r.Context(), id, patch)
	if err != nil {
		tenant.MarkRollback(r.Context())
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

	if err := rs.workflow.Delete(r.Context(), id); err != nil {
		tenant.MarkRollback(r.Context())
		rs.renderDraftMutationError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Grade transition deleted successfully")
}

// renderReadError classifies the outcomes of a read: an unauthorized
// principal is a 403, a missing transition a 404, everything else a fault.
func (rs *GradeTransitionResource) renderReadError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, gradetransition.ErrUnauthorized):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, gradetransition.ErrTransitionNotFound):
		common.RenderError(w, r, common.ErrorNotFound(errors.New(errMsgTransitionNotFound)))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// renderDraftMutationError classifies the expected outcomes of editing or
// deleting a loaded draft that the server state has since moved past: the
// draft was applied or reverted by another admin (409, so the UI refreshes the
// stale editor), it was deleted entirely (404), or the submitted data is
// invalid (400, so the admin corrects it). Everything else is a real server
// fault and stays 500 (#405 review).
func (rs *GradeTransitionResource) renderDraftMutationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, gradetransition.ErrUnauthorized):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, gradetransition.ErrTransitionNotDraft):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "not_draft"))
	case errors.Is(err, gradetransition.ErrTransitionNotFound):
		common.RenderError(w, r, common.ErrorNotFound(errors.New(errMsgTransitionNotFound)))
	case errors.Is(err, gradetransition.ErrInvalidTransitionData):
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

	preview, err := rs.workflow.Preview(r.Context(), id)
	if err != nil {
		// A missing or foreign-tenant id is the same normal outcome the detail
		// and history endpoints classify as 404 — not a server fault (#405
		// review).
		rs.renderReadError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, toPreviewResponse(preview), "Transition preview generated successfully")
}

// apply executes the grade transition
func (rs *GradeTransitionResource) apply(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", errMsgInvalidTransition)
	if !ok {
		return
	}

	// The body is optional: an absent or EMPTY one (io.EOF) means "no preview to
	// check". Anything else must be rejected. A truncated or malformed payload
	// used to be swallowed, leaving ExpectedFingerprint empty — which the
	// workflow reads as "caller opted out of the stale-preview check", so a
	// destructive transition ran unguarded on a cohort that may have changed
	// since the admin confirmed it (#405 review).
	var req ApplyRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("invalid request body: %w", err)))
			return
		}
	}

	result, err := rs.workflow.Apply(r.Context(), id, req.ExpectedFingerprint)
	if err != nil {
		// The route middleware owns the ambient transaction and commits on
		// every non-5xx response, so the 4xx paths request the rollback here.
		tenant.MarkRollback(r.Context())
		rs.renderApplyError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, toResultResponse(result), "Grade transition applied successfully")
}

// renderApplyError classifies the expected outcomes of applying a transition.
// Graduating children still checked in is a client-recoverable safety
// condition (409, so the UI tells the admin to check them out first); a stale
// confirmed preview means the cohort changed underneath the admin (409, the UI
// reloads the preview and asks again); a transition another admin has since
// applied, reverted, or emptied is a normal stale-state conflict (409, the UI
// refreshes the list); a deleted transition is 404. Everything else is a real
// server fault and stays 500 (#405 review).
func (rs *GradeTransitionResource) renderApplyError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, gradetransition.ErrUnauthorized):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, gradetransition.ErrGraduatesCheckedIn):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "graduates_checked_in"))
	case errors.Is(err, gradetransition.ErrPreviewStale):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "preview_stale"))
	case errors.Is(err, gradetransition.ErrTransitionNotDraft):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "not_draft"))
	case errors.Is(err, gradetransition.ErrTransitionNotFound):
		common.RenderError(w, r, common.ErrorNotFound(errors.New(errMsgTransitionNotFound)))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// revert undoes an applied grade transition
func (rs *GradeTransitionResource) revert(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", errMsgInvalidTransition)
	if !ok {
		return
	}

	result, err := rs.workflow.Revert(r.Context(), id)
	if err != nil {
		tenant.MarkRollback(r.Context())
		rs.renderRevertError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, toResultResponse(result), "Grade transition reverted successfully")
}

// renderRevertError classifies the expected outcomes of reverting a
// transition. Targeting anything but the latest applied one is a client-
// recoverable ordering conflict (409, the UI refreshes and reverts the newest
// first); a transition that is still a draft or was already reverted by
// another admin is the same stale-list outcome (409, the UI reloads); a
// deleted transition is 404. Everything else is a real server fault and stays
// 500 (#405 review).
func (rs *GradeTransitionResource) renderRevertError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, gradetransition.ErrUnauthorized):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, gradetransition.ErrNotLatestApplied):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "not_latest_transition"))
	case errors.Is(err, gradetransition.ErrTransitionNotApplied):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "not_applied"))
	case errors.Is(err, gradetransition.ErrTransitionNotFound):
		common.RenderError(w, r, common.ErrorNotFound(errors.New(errMsgTransitionNotFound)))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// getDistinctClasses returns all distinct school class values
func (rs *GradeTransitionResource) getDistinctClasses(w http.ResponseWriter, r *http.Request) {
	classes, err := rs.workflow.ListClasses(r.Context())
	if err != nil {
		rs.renderReadError(w, r, err)
		return
	}
	if classes == nil {
		classes = []string{}
	}

	common.Respond(w, r, http.StatusOK, classes, "Distinct classes retrieved successfully")
}

// suggestMappings returns auto-suggested class mappings
func (rs *GradeTransitionResource) suggestMappings(w http.ResponseWriter, r *http.Request) {
	suggestions, err := rs.workflow.SuggestMappings(r.Context())
	if err != nil {
		rs.renderReadError(w, r, err)
		return
	}

	responses := make([]SuggestedMappingResponse, 0, len(suggestions))
	for _, s := range suggestions {
		responses = append(responses, SuggestedMappingResponse{
			FromClass: s.FromClass, ToClass: s.ToClass, StudentCount: s.StudentCount, IsGraduating: s.IsGraduating, Ambiguous: s.Ambiguous,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Mapping suggestions generated successfully")
}

// getHistory returns the history records for a transition
func (rs *GradeTransitionResource) getHistory(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", errMsgInvalidTransition)
	if !ok {
		return
	}

	history, err := rs.workflow.History(r.Context(), id)
	if err != nil {
		// A nonexistent transition must be a 404, not a 200 with an empty list:
		// the ledger query alone returns zero rows for both, so the workflow
		// resolves the transition first and surfaces not-found here (#405 review).
		rs.renderReadError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, toHistoryResponses(history), "Transition history retrieved successfully")
}
