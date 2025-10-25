package guardians

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// Resource defines the guardians API resource
type Resource struct {
	GuardianRepo        users.GuardianRepository
	StudentGuardianRepo users.StudentGuardianRepository
	StudentRepo         users.StudentRepository
}

// NewResource creates a new guardians resource
func NewResource(guardianRepo users.GuardianRepository, studentGuardianRepo users.StudentGuardianRepository, studentRepo users.StudentRepository) *Resource {
	return &Resource{
		GuardianRepo:        guardianRepo,
		StudentGuardianRepo: studentGuardianRepo,
		StudentRepo:         studentRepo,
	}
}

// Router returns a configured router for guardian endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Routes requiring users:read permission
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/", rs.list)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/search", rs.search)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}", rs.get)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}/students", rs.getGuardianStudents)

		// Routes requiring users:create permission
		r.With(authorize.RequiresPermission(permissions.UsersCreate)).Post("/", rs.create)

		// Routes requiring users:update permission
		r.With(authorize.RequiresPermission(permissions.UsersUpdate)).Put("/{id}", rs.update)

		// Routes requiring users:delete permission
		r.With(authorize.RequiresPermission(permissions.UsersDelete)).Delete("/{id}", rs.delete)
	})

	return r
}

// list handles GET /api/guardians
func (rs *Resource) list(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	options := base.NewQueryOptions()
	filter := base.NewFilter()

	// Apply filters from query params
	if active := r.URL.Query().Get("active"); active != "" {
		if activeBool, err := strconv.ParseBool(active); err == nil {
			filter.Equal("active", activeBool)
		}
	}
	if email := r.URL.Query().Get("email"); email != "" {
		filter.ILike("email", "%"+email+"%")
	}
	if phone := r.URL.Query().Get("phone"); phone != "" {
		filter.ILike("phone", "%"+phone+"%")
	}
	if country := r.URL.Query().Get("country"); country != "" {
		filter.Equal("country", country)
	}

	options.Filter = filter

	// Apply pagination
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage <= 0 {
		perPage = 50
	}
	if page > 0 {
		options.WithPagination(page, perPage)
	}

	// Apply sorting
	sortField := r.URL.Query().Get("sort")
	sortOrder := r.URL.Query().Get("order")
	if sortField != "" {
		sorting := base.NewSorting()
		if sortOrder == "desc" {
			sorting.AddField(sortField, base.SortDesc)
		} else {
			sorting.AddField(sortField, base.SortAsc)
		}
		options.WithSorting(sorting)
	}

	guardians, err := rs.GuardianRepo.ListWithOptions(ctx, options)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to retrieve guardians")
		return
	}

	// Get total count for pagination
	total, err := rs.GuardianRepo.CountWithOptions(ctx, options)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to count guardians")
		return
	}

	response := map[string]interface{}{
		"data":     guardians,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	}

	render.JSON(w, r, response)
}

// search handles GET /api/guardians/search
func (rs *Resource) search(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query().Get("q")
	if query == "" {
		common.RespondWithError(w, r, http.StatusBadRequest, "Search query is required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	guardians, err := rs.GuardianRepo.Search(ctx, query, limit)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to search guardians")
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"data": guardians,
	})
}

// get handles GET /api/guardians/{id}
func (rs *Resource) get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid guardian ID")
		return
	}

	guardian, err := rs.GuardianRepo.FindByID(ctx, id)
	if err != nil {
		common.RespondWithError(w, r, http.StatusNotFound, "Guardian not found")
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"data": guardian,
	})
}

// getGuardianStudents handles GET /api/guardians/{id}/students
func (rs *Resource) getGuardianStudents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid guardian ID")
		return
	}

	// Get student-guardian relationships
	relationships, err := rs.StudentGuardianRepo.FindByGuardianID(ctx, id)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to retrieve students")
		return
	}

	// Get student details
	var students []*users.Student
	for _, rel := range relationships {
		student, err := rs.StudentRepo.FindByID(ctx, rel.StudentID)
		if err != nil {
			continue
		}
		students = append(students, student)
	}

	render.JSON(w, r, map[string]interface{}{
		"data": students,
	})
}

// create handles POST /api/guardians
func (rs *Resource) create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var guardian users.Guardian
	if err := render.DecodeJSON(r.Body, &guardian); err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := rs.GuardianRepo.Create(ctx, &guardian); err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to create guardian")
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]interface{}{
		"data":    guardian,
		"message": "Guardian created successfully",
	})
}

// update handles PUT /api/guardians/{id}
func (rs *Resource) update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid guardian ID")
		return
	}

	// Get existing guardian
	guardian, err := rs.GuardianRepo.FindByID(ctx, id)
	if err != nil {
		common.RespondWithError(w, r, http.StatusNotFound, "Guardian not found")
		return
	}

	// Decode updates
	var updates users.Guardian
	if err := render.DecodeJSON(r.Body, &updates); err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Apply updates
	updates.ID = guardian.ID
	updates.CreatedAt = guardian.CreatedAt

	if err := rs.GuardianRepo.Update(ctx, &updates); err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to update guardian")
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"data":    updates,
		"message": "Guardian updated successfully",
	})
}

// delete handles DELETE /api/guardians/{id}
func (rs *Resource) delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid guardian ID")
		return
	}

	// Check if guardian has any students
	relationships, err := rs.StudentGuardianRepo.FindByGuardianID(ctx, id)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to check guardian relationships")
		return
	}

	if len(relationships) > 0 {
		common.RespondWithError(w, r, http.StatusBadRequest, "Cannot delete guardian with active student relationships")
		return
	}

	if err := rs.GuardianRepo.Delete(ctx, id); err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to delete guardian")
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"message": "Guardian deleted successfully",
	})
}
