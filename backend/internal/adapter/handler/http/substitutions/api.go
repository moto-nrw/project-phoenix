package substitutions

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	modelEducation "github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	"github.com/moto-nrw/project-phoenix/internal/core/service/education"
)

// Constants for date formats (S1192 - avoid duplicate string literals)
const dateFormatYMD = "2006-01-02"

// API-layer validation errors (not duplicated from service layer)
var (
	errInvalidData            = errors.New("invalid substitution data")
	errInvalidDateRange       = errors.New("invalid substitution date range")
	errBackdated              = errors.New("substitutions cannot be created or updated for past dates")
	errStaffAlreadySubstitute = errors.New("staff member is already substituting another group")
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
		StartDate:         sub.StartDate.Format(dateFormatYMD),
		EndDate:           sub.EndDate.Format(dateFormatYMD),
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
	tokenAuth := jwt.MustTokenAuth()

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

	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: page, PageSize: pageSize, Total: len(responses)}, "Substitutions retrieved successfully")
}

// listActive handles GET /api/substitutions/active
func (rs *Resource) listActive(w http.ResponseWriter, r *http.Request) {
	// Get date parameter (defaults to today)
	dateStr := r.URL.Query().Get("date")
	var date time.Time
	if dateStr != "" {
		parsedDate, err := time.Parse(dateFormatYMD, dateStr)
		if err != nil {
			common.RespondWithError(w, r, http.StatusBadRequest, errInvalidData.Error())
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

	if json.NewDecoder(r.Body).Decode(&req) != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, errInvalidData.Error())
		return
	}

	// Validate required fields
	if req.GroupID == 0 || req.SubstituteStaffID == 0 {
		common.RespondWithError(w, r, http.StatusBadRequest, errInvalidData.Error())
		return
	}

	// Parse dates
	startDate, err := time.Parse(dateFormatYMD, req.StartDate)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid start date format. Expected YYYY-MM-DD")
		return
	}

	endDate, err := time.Parse(dateFormatYMD, req.EndDate)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid end date format. Expected YYYY-MM-DD")
		return
	}

	// Validate date range
	if startDate.After(endDate) {
		common.RespondWithError(w, r, http.StatusBadRequest, errInvalidDateRange.Error())
		return
	}

	// Validate no backdating - start date must be today or in the future
	today := time.Now().Truncate(24 * time.Hour)
	if startDate.Before(today) {
		common.RespondWithError(w, r, http.StatusBadRequest, errBackdated.Error())
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
	id, err := common.ParseID(r)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, errInvalidData.Error())
		return
	}

	substitution, err := rs.Service.GetSubstitution(r.Context(), id)
	if err != nil {
		if errors.Is(err, education.ErrSubstitutionNotFound) {
			common.RespondWithError(w, r, http.StatusNotFound, education.ErrSubstitutionNotFound.Error())
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
	id, err := common.ParseID(r)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, errInvalidData.Error())
		return
	}

	var substitution modelEducation.GroupSubstitution
	if json.NewDecoder(r.Body).Decode(&substitution) != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, errInvalidData.Error())
		return
	}
	substitution.ID = id

	// Validate dates
	if errMsg := validateSubstitutionDates(&substitution); errMsg != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, errMsg.Error())
		return
	}

	// Check existing and validate conflicts
	existing, err := rs.Service.GetSubstitution(r.Context(), id)
	if err != nil {
		rs.handleGetSubstitutionError(w, r, err)
		return
	}

	if err := rs.checkStaffChangeConflicts(r.Context(), &substitution, existing); err != nil {
		common.RespondWithError(w, r, http.StatusConflict, err.Error())
		return
	}

	// Perform update
	if err := rs.Service.UpdateSubstitution(r.Context(), &substitution); err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	updated, err := rs.Service.GetSubstitution(r.Context(), id)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	common.Respond(w, r, http.StatusOK, newSubstitutionResponse(updated), "Substitution updated successfully")
}

// validateSubstitutionDates validates date range and no backdating
func validateSubstitutionDates(sub *modelEducation.GroupSubstitution) error {
	if sub.StartDate.After(sub.EndDate) {
		return errInvalidDateRange
	}

	today := time.Now().Truncate(24 * time.Hour)
	if sub.StartDate.Before(today) {
		return errBackdated
	}
	return nil
}

// handleGetSubstitutionError handles errors from GetSubstitution
func (rs *Resource) handleGetSubstitutionError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, education.ErrSubstitutionNotFound) {
		common.RespondWithError(w, r, http.StatusNotFound, education.ErrSubstitutionNotFound.Error())
		return
	}
	common.RespondWithError(w, r, http.StatusInternalServerError, err.Error())
}

// checkStaffChangeConflicts checks for conflicts when staff member changes
func (rs *Resource) checkStaffChangeConflicts(
	ctx context.Context,
	newSub *modelEducation.GroupSubstitution,
	existing *modelEducation.GroupSubstitution,
) error {
	if existing.SubstituteStaffID == newSub.SubstituteStaffID {
		return nil
	}

	conflicts, err := rs.Service.CheckSubstitutionConflicts(ctx, newSub.SubstituteStaffID, newSub.StartDate, newSub.EndDate)
	if err != nil {
		return err
	}

	if hasRealConflicts(conflicts, newSub.ID) {
		return errStaffAlreadySubstitute
	}
	return nil
}

// hasRealConflicts checks if there are conflicts excluding the current substitution
func hasRealConflicts(conflicts []*modelEducation.GroupSubstitution, excludeID int64) bool {
	for _, conflict := range conflicts {
		if conflict.ID != excludeID {
			return true
		}
	}
	return false
}

// delete handles DELETE /api/substitutions/{id}
func (rs *Resource) delete(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, errInvalidData.Error())
		return
	}

	// Check if substitution exists
	_, err = rs.Service.GetSubstitution(r.Context(), id)
	if err != nil {
		if errors.Is(err, education.ErrSubstitutionNotFound) {
			common.RespondWithError(w, r, http.StatusNotFound, education.ErrSubstitutionNotFound.Error())
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
