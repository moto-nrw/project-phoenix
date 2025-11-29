package substitutions

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/base"
	modelEducation "github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/services/education"
)

type Resource struct {
	Service education.Service
}

func NewResource(educationService education.Service) *Resource {
	return &Resource{
		Service: educationService,
	}
}

// SubstitutionResponse represents a substitution in API responses
type SubstitutionResponse struct {
	ID                int64      `json:"id"`
	GroupID           int64      `json:"group_id"`
	Group             *GroupInfo `json:"group,omitempty"`
	RegularStaffID    *int64     `json:"regular_staff_id,omitempty"`
	RegularStaff      *StaffInfo `json:"regular_staff,omitempty"`
	SubstituteStaffID int64      `json:"substitute_staff_id"`
	SubstituteStaff   *StaffInfo `json:"substitute_staff,omitempty"`
	StartDate         string     `json:"start_date"` // YYYY-MM-DD format
	EndDate           string     `json:"end_date"`   // YYYY-MM-DD format
	Reason            string     `json:"reason,omitempty"`
	Duration          int        `json:"duration_days"`
	IsActive          bool       `json:"is_active"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// GroupInfo represents basic group information in substitution responses
type GroupInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// StaffInfo represents basic staff information in substitution responses
type StaffInfo struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	FullName  string `json:"full_name"`
}

// newSubstitutionResponse converts a substitution model to a response object
func newSubstitutionResponse(sub *modelEducation.GroupSubstitution) SubstitutionResponse {
	response := SubstitutionResponse{
		ID:                sub.ID,
		GroupID:           sub.GroupID,
		RegularStaffID:    sub.RegularStaffID,
		SubstituteStaffID: sub.SubstituteStaffID,
		StartDate:         sub.StartDate.Format("2006-01-02"),
		EndDate:           sub.EndDate.Format("2006-01-02"),
		Reason:            sub.Reason,
		Duration:          sub.Duration(),
		IsActive:          sub.IsCurrentlyActive(),
		CreatedAt:         sub.CreatedAt,
		UpdatedAt:         sub.UpdatedAt,
	}

	// Add group details if available
	if sub.Group != nil {
		response.Group = &GroupInfo{
			ID:   sub.Group.ID,
			Name: sub.Group.Name,
		}
	}

	// Add regular staff details if available (only if RegularStaffID is set)
	if sub.RegularStaffID != nil && sub.RegularStaff != nil {
		response.RegularStaff = &StaffInfo{
			ID: sub.RegularStaff.ID,
		}
		if sub.RegularStaff.Person != nil {
			response.RegularStaff.FirstName = sub.RegularStaff.Person.FirstName
			response.RegularStaff.LastName = sub.RegularStaff.Person.LastName
			response.RegularStaff.FullName = sub.RegularStaff.Person.GetFullName()
		}
	}

	// Add substitute staff details if available
	if sub.SubstituteStaff != nil {
		response.SubstituteStaff = &StaffInfo{
			ID: sub.SubstituteStaff.ID,
		}
		if sub.SubstituteStaff.Person != nil {
			response.SubstituteStaff.FirstName = sub.SubstituteStaff.Person.FirstName
			response.SubstituteStaff.LastName = sub.SubstituteStaff.Person.LastName
			response.SubstituteStaff.FullName = sub.SubstituteStaff.Person.GetFullName()
		}
	}

	return response
}

// createSubstitutionRequest represents a request to create a substitution
type createSubstitutionRequest struct {
	GroupID           int64  `json:"group_id"`
	RegularStaffID    *int64 `json:"regular_staff_id,omitempty"`
	SubstituteStaffID int64  `json:"substitute_staff_id"`
	StartDate         string `json:"start_date"` // YYYY-MM-DD format
	EndDate           string `json:"end_date"`   // YYYY-MM-DD format
	Reason            string `json:"reason,omitempty"`
}

func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Read operations require substitutions:read permission
		r.With(authorize.RequiresPermission(permissions.SubstitutionsRead)).Get("/", rs.list)
		r.With(authorize.RequiresPermission(permissions.SubstitutionsRead)).Get("/active", rs.listActive)
		r.With(authorize.RequiresPermission(permissions.SubstitutionsRead)).Get("/{id}", rs.get)

		// Write operations require substitutions:create/update/delete permissions
		r.With(authorize.RequiresPermission(permissions.SubstitutionsCreate)).Post("/", rs.create)
		r.With(authorize.RequiresPermission(permissions.SubstitutionsUpdate)).Put("/{id}", rs.update)
		r.With(authorize.RequiresPermission(permissions.SubstitutionsDelete)).Delete("/{id}", rs.delete)
	})

	return r
}

// list handles GET /api/substitutions
func (rs *Resource) list(w http.ResponseWriter, r *http.Request) {
	options := base.NewQueryOptions()

	// Apply pagination
	page, pageSize := common.ParsePagination(r)
	options.WithPagination(page, pageSize)

	substitutions, err := rs.Service.ListSubstitutions(r.Context(), options)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// Transform to response DTOs - this ensures we always return an array, never null
	responses := make([]SubstitutionResponse, 0, len(substitutions))
	for _, sub := range substitutions {
		responses = append(responses, newSubstitutionResponse(sub))
	}

	common.RespondWithPagination(w, r, http.StatusOK, responses, page, pageSize, len(responses), "Substitutions retrieved successfully")
}

// listActive handles GET /api/substitutions/active
func (rs *Resource) listActive(w http.ResponseWriter, r *http.Request) {
	// Get date parameter (defaults to today)
	dateStr := r.URL.Query().Get("date")
	var date time.Time
	if dateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			common.RespondWithError(w, r, http.StatusBadRequest, ErrInvalidSubstitutionData.Error())
			return
		}
		date = parsedDate
	} else {
		date = time.Now()
	}

	substitutions, err := rs.Service.GetActiveSubstitutions(r.Context(), date)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// Transform to response DTOs - this ensures we always return an array, never null
	responses := make([]SubstitutionResponse, 0, len(substitutions))
	for _, sub := range substitutions {
		responses = append(responses, newSubstitutionResponse(sub))
	}

	common.Respond(w, r, http.StatusOK, responses, "Active substitutions retrieved successfully")
}

// create handles POST /api/substitutions
func (rs *Resource) create(w http.ResponseWriter, r *http.Request) {
	var req createSubstitutionRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, ErrInvalidSubstitutionData.Error())
		return
	}

	// Validate required fields
	if req.GroupID == 0 || req.SubstituteStaffID == 0 {
		common.RespondWithError(w, r, http.StatusBadRequest, ErrInvalidSubstitutionData.Error())
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid start date format. Expected YYYY-MM-DD")
		return
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid end date format. Expected YYYY-MM-DD")
		return
	}

	// Validate date range
	if startDate.After(endDate) {
		common.RespondWithError(w, r, http.StatusBadRequest, ErrSubstitutionDateRange.Error())
		return
	}

	// Validate no backdating - start date must be today or in the future
	today := time.Now().Truncate(24 * time.Hour)
	if startDate.Before(today) {
		common.RespondWithError(w, r, http.StatusBadRequest, ErrSubstitutionBackdated.Error())
		return
	}

	// Create domain model
	substitution := &modelEducation.GroupSubstitution{
		GroupID:           req.GroupID,
		RegularStaffID:    req.RegularStaffID,
		SubstituteStaffID: req.SubstituteStaffID,
		StartDate:         startDate,
		EndDate:           endDate,
		Reason:            req.Reason,
	}

	// Note: We intentionally allow staff members to substitute multiple groups simultaneously.
	// We also allow groups to have multiple substitutes at the same time.
	// This enables flexible team-based supervision of groups.

	// Create the substitution
	if err := rs.Service.CreateSubstitution(r.Context(), substitution); err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to response DTO
	response := newSubstitutionResponse(substitution)
	common.Respond(w, r, http.StatusCreated, response, "Substitution created successfully")
}

// get handles GET /api/substitutions/{id}
func (rs *Resource) get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, ErrInvalidSubstitutionData.Error())
		return
	}

	substitution, err := rs.Service.GetSubstitution(r.Context(), id)
	if err != nil {
		if err.Error() == "substitution not found" {
			common.RespondWithError(w, r, http.StatusNotFound, ErrSubstitutionNotFound.Error())
			return
		}
		common.RespondWithError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to response DTO
	response := newSubstitutionResponse(substitution)
	common.Respond(w, r, http.StatusOK, response, "Substitution retrieved successfully")
}

// update handles PUT /api/substitutions/{id}
func (rs *Resource) update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, ErrInvalidSubstitutionData.Error())
		return
	}

	var substitution modelEducation.GroupSubstitution
	if err := json.NewDecoder(r.Body).Decode(&substitution); err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, ErrInvalidSubstitutionData.Error())
		return
	}

	// Set the ID from the URL
	substitution.ID = id

	// Validate date range
	if substitution.StartDate.After(substitution.EndDate) {
		common.RespondWithError(w, r, http.StatusBadRequest, ErrSubstitutionDateRange.Error())
		return
	}

	// Validate no backdating - start date must be today or in the future
	today := time.Now().Truncate(24 * time.Hour)
	if substitution.StartDate.Before(today) {
		common.RespondWithError(w, r, http.StatusBadRequest, ErrSubstitutionBackdated.Error())
		return
	}

	// Check for conflicts if staff member changed
	existing, err := rs.Service.GetSubstitution(r.Context(), id)
	if err != nil {
		if err.Error() == "substitution not found" {
			common.RespondWithError(w, r, http.StatusNotFound, ErrSubstitutionNotFound.Error())
			return
		}
		common.RespondWithError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	if existing.SubstituteStaffID != substitution.SubstituteStaffID {
		conflicts, err := rs.Service.CheckSubstitutionConflicts(
			r.Context(),
			substitution.SubstituteStaffID,
			substitution.StartDate,
			substitution.EndDate,
		)
		if err != nil {
			common.RespondWithError(w, r, http.StatusInternalServerError, err.Error())
			return
		}

		// Filter out the current substitution from conflicts
		var realConflicts []*modelEducation.GroupSubstitution
		for _, conflict := range conflicts {
			if conflict.ID != id {
				realConflicts = append(realConflicts, conflict)
			}
		}

		if len(realConflicts) > 0 {
			common.RespondWithError(w, r, http.StatusConflict, ErrStaffAlreadySubstituting.Error())
			return
		}
	}

	// Update the substitution
	if err := rs.Service.UpdateSubstitution(r.Context(), &substitution); err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// Get the updated substitution with all relations
	updated, err := rs.Service.GetSubstitution(r.Context(), id)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to response DTO
	response := newSubstitutionResponse(updated)
	common.Respond(w, r, http.StatusOK, response, "Substitution updated successfully")
}

// delete handles DELETE /api/substitutions/{id}
func (rs *Resource) delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, ErrInvalidSubstitutionData.Error())
		return
	}

	// Check if substitution exists
	_, err = rs.Service.GetSubstitution(r.Context(), id)
	if err != nil {
		if err.Error() == "substitution not found" {
			common.RespondWithError(w, r, http.StatusNotFound, ErrSubstitutionNotFound.Error())
			return
		}
		common.RespondWithError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// Delete the substitution
	if err := rs.Service.DeleteSubstitution(r.Context(), id); err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	common.RespondNoContent(w, r)
}
