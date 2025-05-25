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
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/education"
	modelEducation "github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/base"
)

type Resource struct {
	Service education.Service
}

func NewResource(factory *services.Factory) *Resource {
	return &Resource{
		Service: factory.Education,
	}
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

		// Read operations only require groups:read permission
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", rs.list)
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/active", rs.listActive)
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}", rs.get)
		
		// Write operations require groups:create/update/delete permissions
		r.With(authorize.RequiresPermission(permissions.GroupsCreate)).Post("/", rs.create)
		r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Put("/{id}", rs.update)
		r.With(authorize.RequiresPermission(permissions.GroupsDelete)).Delete("/{id}", rs.delete)
	})

	return r
}

// list handles GET /api/substitutions
func (rs *Resource) list(w http.ResponseWriter, r *http.Request) {
	options := base.NewQueryOptions()
	
	// Apply pagination
	page := 1
	pageSize := 50
	
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	
	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}
	
	options.WithPagination(page, pageSize)

	substitutions, err := rs.Service.ListSubstitutions(r.Context(), options)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	common.RespondWithPagination(w, r, http.StatusOK, substitutions, page, pageSize, len(substitutions), "Substitutions retrieved successfully")
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

	common.Respond(w, r, http.StatusOK, substitutions, "Active substitutions retrieved successfully")
}

// create handles POST /api/substitutions
func (rs *Resource) create(w http.ResponseWriter, r *http.Request) {
	var substitution modelEducation.GroupSubstitution
	
	if err := json.NewDecoder(r.Body).Decode(&substitution); err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, ErrInvalidSubstitutionData.Error())
		return
	}

	// Validate required fields
	if substitution.GroupID == 0 || substitution.SubstituteStaffID == 0 {
		common.RespondWithError(w, r, http.StatusBadRequest, ErrInvalidSubstitutionData.Error())
		return
	}

	// Validate date range
	if substitution.StartDate.After(substitution.EndDate) {
		common.RespondWithError(w, r, http.StatusBadRequest, ErrSubstitutionDateRange.Error())
		return
	}

	// Check for conflicts
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
	
	if len(conflicts) > 0 {
		common.RespondWithError(w, r, http.StatusConflict, ErrStaffAlreadySubstituting.Error())
		return
	}

	// Check if group already has a substitute for this period
	activeSubstitutions, err := rs.Service.GetActiveGroupSubstitutions(
		r.Context(), 
		substitution.GroupID, 
		substitution.StartDate,
	)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	
	if len(activeSubstitutions) > 0 {
		common.RespondWithError(w, r, http.StatusConflict, ErrGroupAlreadyHasSubstitute.Error())
		return
	}

	// Create the substitution
	if err := rs.Service.CreateSubstitution(r.Context(), &substitution); err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	common.Respond(w, r, http.StatusCreated, substitution, "Substitution created successfully")
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

	common.Respond(w, r, http.StatusOK, substitution, "Substitution retrieved successfully")
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

	common.Respond(w, r, http.StatusOK, substitution, "Substitution updated successfully")
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