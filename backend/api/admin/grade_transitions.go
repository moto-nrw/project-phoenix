package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
)

// GradeTransitionResource handles grade transition API endpoints
type GradeTransitionResource struct {
	service educationService.GradeTransitionService
}

// NewGradeTransitionResource creates a new grade transition resource
func NewGradeTransitionResource(service educationService.GradeTransitionService) *GradeTransitionResource {
	return &GradeTransitionResource{
		service: service,
	}
}

// Router returns a configured router for grade transition endpoints
func (rs *GradeTransitionResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// All routes require authentication
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Read operations
		r.With(authorize.RequiresPermission(permissions.GradeTransitionsRead)).
			Get("/", rs.list)
		r.With(authorize.RequiresPermission(permissions.GradeTransitionsRead)).
			Get("/classes", rs.getDistinctClasses)
		r.With(authorize.RequiresPermission(permissions.GradeTransitionsRead)).
			Get("/suggest", rs.suggestMappings)

		// Create operations
		r.With(authorize.RequiresPermission(permissions.GradeTransitionsCreate)).
			Post("/", rs.create)

		// Individual transition routes
		r.Route("/{id}", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GradeTransitionsRead)).
				Get("/", rs.getByID)
			r.With(authorize.RequiresPermission(permissions.GradeTransitionsRead)).
				Get("/preview", rs.preview)
			r.With(authorize.RequiresPermission(permissions.GradeTransitionsRead)).
				Get("/history", rs.getHistory)

			r.With(authorize.RequiresPermission(permissions.GradeTransitionsUpdate)).
				Put("/", rs.update)

			r.With(authorize.RequiresPermission(permissions.GradeTransitionsDelete)).
				Delete("/", rs.delete)

			r.With(authorize.RequiresPermission(permissions.GradeTransitionsApply)).
				Post("/apply", rs.apply)

			r.With(authorize.RequiresPermission(permissions.GradeTransitionsApply)).
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

// Bind validates the transition request
func (req *TransitionRequest) Bind(_ *http.Request) error {
	if req.AcademicYear == "" {
		return errors.New("academic_year is required")
	}
	return nil
}

// MappingRequest represents a class mapping in a request
type MappingRequest struct {
	FromClass string  `json:"from_class"`
	ToClass   *string `json:"to_class,omitempty"` // null = graduate
}

// TransitionResponse represents a transition in API responses
type TransitionResponse struct {
	ID           int64                    `json:"id"`
	AcademicYear string                   `json:"academic_year"`
	Status       string                   `json:"status"`
	AppliedAt    *string                  `json:"applied_at,omitempty"`
	AppliedBy    *int64                   `json:"applied_by,omitempty"`
	RevertedAt   *string                  `json:"reverted_at,omitempty"`
	RevertedBy   *int64                   `json:"reverted_by,omitempty"`
	CreatedAt    string                   `json:"created_at"`
	CreatedBy    int64                    `json:"created_by"`
	Notes        *string                  `json:"notes,omitempty"`
	Mappings     []MappingResponse        `json:"mappings,omitempty"`
	CanModify    bool                     `json:"can_modify"`
	CanApply     bool                     `json:"can_apply"`
	CanRevert    bool                     `json:"can_revert"`
}

// MappingResponse represents a mapping in API responses
type MappingResponse struct {
	ID        int64   `json:"id"`
	FromClass string  `json:"from_class"`
	ToClass   *string `json:"to_class,omitempty"`
	Action    string  `json:"action"` // "promote" or "graduate"
}

// toTransitionResponse converts a model to a response
func toTransitionResponse(t *education.GradeTransition) TransitionResponse {
	resp := TransitionResponse{
		ID:           t.ID,
		AcademicYear: t.AcademicYear,
		Status:       t.Status,
		CreatedAt:    t.CreatedAt.Format("2006-01-02T15:04:05Z"),
		CreatedBy:    t.CreatedBy,
		Notes:        t.Notes,
		CanModify:    t.CanModify(),
		CanApply:     t.CanApply(),
		CanRevert:    t.CanRevert(),
	}

	if t.AppliedAt != nil {
		formatted := t.AppliedAt.Format("2006-01-02T15:04:05Z")
		resp.AppliedAt = &formatted
	}
	if t.AppliedBy != nil {
		resp.AppliedBy = t.AppliedBy
	}
	if t.RevertedAt != nil {
		formatted := t.RevertedAt.Format("2006-01-02T15:04:05Z")
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

	// Get account ID from JWT
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("no account ID in context")))
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

	transition, err := rs.service.Create(r.Context(), createReq)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, toTransitionResponse(transition), "Grade transition created successfully")
}

// getByID returns a single grade transition
func (rs *GradeTransitionResource) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid transition ID")))
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid transition ID")))
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

	// Convert mappings if provided
	if len(req.Mappings) > 0 {
		for _, m := range req.Mappings {
			updateReq.Mappings = append(updateReq.Mappings, educationService.MappingRequest{
				FromClass: m.FromClass,
				ToClass:   m.ToClass,
			})
		}
	}

	transition, err := rs.service.Update(r.Context(), id, updateReq)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, toTransitionResponse(transition), "Grade transition updated successfully")
}

// delete deletes a grade transition
func (rs *GradeTransitionResource) delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid transition ID")))
		return
	}

	if err := rs.service.Delete(r.Context(), id); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Grade transition deleted successfully")
}

// preview returns a preview of what will happen when the transition is applied
func (rs *GradeTransitionResource) preview(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid transition ID")))
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid transition ID")))
		return
	}

	// Get account ID from JWT
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("no account ID in context")))
		return
	}
	accountID := int64(claims.ID)

	result, err := rs.service.Apply(r.Context(), id, accountID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Grade transition applied successfully")
}

// revert undoes an applied grade transition
func (rs *GradeTransitionResource) revert(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid transition ID")))
		return
	}

	// Get account ID from JWT
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("no account ID in context")))
		return
	}
	accountID := int64(claims.ID)

	result, err := rs.service.Revert(r.Context(), id, accountID)
	if err != nil {
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid transition ID")))
		return
	}

	history, err := rs.service.GetHistory(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, history, "Transition history retrieved successfully")
}
