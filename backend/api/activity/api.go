package activity

import (
	"context"
	"errors"
	jwt2 "github.com/moto-nrw/project-phoenix/auth/jwt"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// Resource defines the activity group management resource
type Resource struct {
	Store     ActivityStore
	AuthStore AuthTokenStore
}

// ActivityStore defines database operations for activity group management
type ActivityStore interface {
	// Category operations
	CreateAgCategory(ctx context.Context, category *models2.AgCategory) error
	GetAgCategoryByID(ctx context.Context, id int64) (*models2.AgCategory, error)
	UpdateAgCategory(ctx context.Context, category *models2.AgCategory) error
	DeleteAgCategory(ctx context.Context, id int64) error
	ListAgCategories(ctx context.Context) ([]models2.AgCategory, error)

	// Activity Group operations
	CreateAg(ctx context.Context, ag *models2.Ag, studentIDs []int64, timeslots []*models2.AgTime) error
	GetAgByID(ctx context.Context, id int64) (*models2.Ag, error)
	UpdateAg(ctx context.Context, ag *models2.Ag) error
	DeleteAg(ctx context.Context, id int64) error
	ListAgs(ctx context.Context, filters map[string]interface{}) ([]models2.Ag, error)

	// Time slot operations
	CreateAgTime(ctx context.Context, agTime *models2.AgTime) error
	GetAgTimeByID(ctx context.Context, id int64) (*models2.AgTime, error)
	UpdateAgTime(ctx context.Context, agTime *models2.AgTime) error
	DeleteAgTime(ctx context.Context, id int64) error
	ListAgTimes(ctx context.Context, agID int64) ([]models2.AgTime, error)

	// Student enrollment operations
	EnrollStudent(ctx context.Context, agID, studentID int64) error
	UnenrollStudent(ctx context.Context, agID, studentID int64) error
	ListEnrolledStudents(ctx context.Context, agID int64) ([]models2.Student, error)
	ListStudentAgs(ctx context.Context, studentID int64) ([]models2.Ag, error)
}

// AuthTokenStore defines operations for the auth token store
type AuthTokenStore interface {
	GetToken(t string) (*jwt2.Token, error)
}

// NewResource creates a new activity group management resource
func NewResource(store ActivityStore, authStore AuthTokenStore) *Resource {
	return &Resource{
		Store:     store,
		AuthStore: authStore,
	}
}

// Router creates a router for activity group management
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()

	// JWT protected routes
	r.Group(func(r chi.Router) {
		r.Use(jwt2.Authenticator)

		// Category routes
		r.Route("/categories", func(r chi.Router) {
			r.Get("/", rs.listCategories)
			r.Post("/", rs.createCategory)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", rs.getCategory)
				r.Put("/", rs.updateCategory)
				r.Delete("/", rs.deleteCategory)
			})
		})

		// Activity Group routes
		r.Route("/", func(r chi.Router) {
			r.Get("/", rs.listActivityGroups)
			r.Post("/", rs.createActivityGroup)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", rs.getActivityGroup)
				r.Put("/", rs.updateActivityGroup)
				r.Delete("/", rs.deleteActivityGroup)

				// Timeslot routes for an activity group
				r.Route("/times", func(r chi.Router) {
					r.Get("/", rs.listAgTimes)
					r.Post("/", rs.createAgTime)
					r.Route("/{timeId}", func(r chi.Router) {
						r.Put("/", rs.updateAgTime)
						r.Delete("/", rs.deleteAgTime)
					})
				})

				// Student enrollment routes
				r.Route("/students", func(r chi.Router) {
					r.Get("/", rs.listEnrolledStudents)
					r.Post("/{studentId}", rs.enrollStudent)
					r.Delete("/{studentId}", rs.unenrollStudent)
				})
			})
		})

		// Student AG routes
		r.Get("/student/{studentId}", rs.listStudentAgs)
	})

	return r
}

// ======== Request/Response Types ========

// CategoryRequest is the request payload for AgCategory data
type CategoryRequest struct {
	*models2.AgCategory
}

// Bind preprocesses a CategoryRequest
func (req *CategoryRequest) Bind(r *http.Request) error {
	return nil
}

// ActivityGroupRequest is the request payload for Ag data
type ActivityGroupRequest struct {
	*models2.Ag
	StudentIDs []int64           `json:"student_ids,omitempty"`
	Times      []*models2.AgTime `json:"times,omitempty"`
}

// Bind preprocesses an ActivityGroupRequest
func (req *ActivityGroupRequest) Bind(r *http.Request) error {
	return nil
}

// TimeSlotRequest is the request payload for AgTime data
type TimeSlotRequest struct {
	*models2.AgTime
}

// Bind preprocesses a TimeSlotRequest
func (req *TimeSlotRequest) Bind(r *http.Request) error {
	return nil
}

// ======== Category Handlers ========

// listCategories returns a list of all activity group categories
func (rs *Resource) listCategories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	categories, err := rs.Store.ListAgCategories(ctx)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, categories)
}

// createCategory creates a new activity group category
func (rs *Resource) createCategory(w http.ResponseWriter, r *http.Request) {
	data := &CategoryRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	if err := rs.Store.CreateAgCategory(ctx, data.AgCategory); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, data.AgCategory)
}

// getCategory returns a specific activity group category
func (rs *Resource) getCategory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	category, err := rs.Store.GetAgCategoryByID(ctx, id)
	if err != nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	render.JSON(w, r, category)
}

// updateCategory updates a specific activity group category
func (rs *Resource) updateCategory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &CategoryRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	category, err := rs.Store.GetAgCategoryByID(ctx, id)
	if err != nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	// Update fields
	category.Name = data.Name

	if err := rs.Store.UpdateAgCategory(ctx, category); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, category)
}

// deleteCategory deletes a specific activity group category
func (rs *Resource) deleteCategory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.DeleteAgCategory(ctx, id); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.NoContent(w, r)
}

// ======== Activity Group Handlers ========

// listActivityGroups returns a list of all activity groups with optional filtering
func (rs *Resource) listActivityGroups(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters for filtering
	filters := make(map[string]interface{})

	if categoryIDStr := r.URL.Query().Get("category_id"); categoryIDStr != "" {
		categoryID, err := strconv.ParseInt(categoryIDStr, 10, 64)
		if err == nil {
			filters["category_id"] = categoryID
		}
	}

	if supervisorIDStr := r.URL.Query().Get("supervisor_id"); supervisorIDStr != "" {
		supervisorID, err := strconv.ParseInt(supervisorIDStr, 10, 64)
		if err == nil {
			filters["supervisor_id"] = supervisorID
		}
	}

	if isOpenStr := r.URL.Query().Get("is_open"); isOpenStr != "" {
		isOpen := isOpenStr == "true"
		filters["is_open"] = isOpen
	}

	if searchTerm := r.URL.Query().Get("search"); searchTerm != "" {
		filters["search"] = searchTerm
	}

	ags, err := rs.Store.ListAgs(ctx, filters)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, ags)
}

// createActivityGroup creates a new activity group
func (rs *Resource) createActivityGroup(w http.ResponseWriter, r *http.Request) {
	data := &ActivityGroupRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	if err := rs.Store.CreateAg(ctx, data.Ag, data.StudentIDs, data.Times); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the created activity group with all its relations
	ag, err := rs.Store.GetAgByID(ctx, data.Ag.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, ag)
}

// getActivityGroup returns a specific activity group
func (rs *Resource) getActivityGroup(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	ag, err := rs.Store.GetAgByID(ctx, id)
	if err != nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	render.JSON(w, r, ag)
}

// updateActivityGroup updates a specific activity group
func (rs *Resource) updateActivityGroup(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &ActivityGroupRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	ag, err := rs.Store.GetAgByID(ctx, id)
	if err != nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	// Update fields
	ag.Name = data.Name
	ag.MaxParticipant = data.MaxParticipant
	ag.IsOpenAg = data.IsOpenAg
	ag.SupervisorID = data.SupervisorID
	ag.AgCategoryID = data.AgCategoryID
	ag.DatespanID = data.DatespanID

	if err := rs.Store.UpdateAg(ctx, ag); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the updated activity group
	updatedAg, err := rs.Store.GetAgByID(ctx, ag.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, updatedAg)
}

// deleteActivityGroup deletes a specific activity group
func (rs *Resource) deleteActivityGroup(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.DeleteAg(ctx, id); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.NoContent(w, r)
}

// ======== Timeslot Handlers ========

// listAgTimes returns a list of all timeslots for a specific activity group
func (rs *Resource) listAgTimes(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	times, err := rs.Store.ListAgTimes(ctx, id)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, times)
}

// createAgTime creates a new timeslot for a specific activity group
func (rs *Resource) createAgTime(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &TimeSlotRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Set the AG ID from the URL parameter
	data.AgTime.AgID = id

	ctx := r.Context()
	if err := rs.Store.CreateAgTime(ctx, data.AgTime); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, data.AgTime)
}

// updateAgTime updates a specific timeslot
func (rs *Resource) updateAgTime(w http.ResponseWriter, r *http.Request) {
	agIDStr := chi.URLParam(r, "id")
	agID, err := strconv.ParseInt(agIDStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid AG ID format")))
		return
	}

	timeIDStr := chi.URLParam(r, "timeId")
	timeID, err := strconv.ParseInt(timeIDStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid Time ID format")))
		return
	}

	data := &TimeSlotRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	agTime, err := rs.Store.GetAgTimeByID(ctx, timeID)
	if err != nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	// Ensure the time slot belongs to the specified AG
	if agTime.AgID != agID {
		render.Render(w, r, ErrNotFound)
		return
	}

	// Update fields
	agTime.Weekday = data.Weekday
	agTime.TimespanID = data.TimespanID

	if err := rs.Store.UpdateAgTime(ctx, agTime); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the updated time
	updatedTime, err := rs.Store.GetAgTimeByID(ctx, agTime.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, updatedTime)
}

// deleteAgTime deletes a specific timeslot
func (rs *Resource) deleteAgTime(w http.ResponseWriter, r *http.Request) {
	agIDStr := chi.URLParam(r, "id")
	agID, err := strconv.ParseInt(agIDStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid AG ID format")))
		return
	}

	timeIDStr := chi.URLParam(r, "timeId")
	timeID, err := strconv.ParseInt(timeIDStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid Time ID format")))
		return
	}

	ctx := r.Context()
	agTime, err := rs.Store.GetAgTimeByID(ctx, timeID)
	if err != nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	// Ensure the time slot belongs to the specified AG
	if agTime.AgID != agID {
		render.Render(w, r, ErrNotFound)
		return
	}

	if err := rs.Store.DeleteAgTime(ctx, timeID); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.NoContent(w, r)
}

// ======== Student Enrollment Handlers ========

// listEnrolledStudents returns a list of all students enrolled in a specific activity group
func (rs *Resource) listEnrolledStudents(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	students, err := rs.Store.ListEnrolledStudents(ctx, id)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, students)
}

// enrollStudent enrolls a student in a specific activity group
func (rs *Resource) enrollStudent(w http.ResponseWriter, r *http.Request) {
	agIDStr := chi.URLParam(r, "id")
	agID, err := strconv.ParseInt(agIDStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid AG ID format")))
		return
	}

	studentIDStr := chi.URLParam(r, "studentId")
	studentID, err := strconv.ParseInt(studentIDStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid Student ID format")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.EnrollStudent(ctx, agID, studentID); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]string{"message": "Student enrolled successfully"})
}

// unenrollStudent removes a student from a specific activity group
func (rs *Resource) unenrollStudent(w http.ResponseWriter, r *http.Request) {
	agIDStr := chi.URLParam(r, "id")
	agID, err := strconv.ParseInt(agIDStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid AG ID format")))
		return
	}

	studentIDStr := chi.URLParam(r, "studentId")
	studentID, err := strconv.ParseInt(studentIDStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid Student ID format")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.UnenrollStudent(ctx, agID, studentID); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.NoContent(w, r)
}

// listStudentAgs returns a list of all activity groups a student is enrolled in
func (rs *Resource) listStudentAgs(w http.ResponseWriter, r *http.Request) {
	studentIDStr := chi.URLParam(r, "studentId")
	studentID, err := strconv.ParseInt(studentIDStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid Student ID format")))
		return
	}

	ctx := r.Context()
	ags, err := rs.Store.ListStudentAgs(ctx, studentID)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, ags)
}
