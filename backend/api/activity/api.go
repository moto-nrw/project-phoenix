// Package activity provides the activity group management API
package activity

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/logging"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"github.com/sirupsen/logrus"
)

// Resource defines the activity group management resource
type Resource struct {
	Store     AgStore
	AuthStore AuthTokenStore
}

// AgStore defines database operations for activity group management
type AgStore interface {
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

	// Enrollment operations
	EnrollStudent(ctx context.Context, agID, studentID int64) error
	UnenrollStudent(ctx context.Context, agID, studentID int64) error
	ListEnrolledStudents(ctx context.Context, agID int64) ([]*models2.Student, error)
	ListStudentAgs(ctx context.Context, studentID int64) ([]models2.Ag, error)
}

// AuthTokenStore defines operations for the auth token store
type AuthTokenStore interface {
	GetToken(t string) (*jwt.Token, error)
}

// NewResource creates a new activity group management resource
func NewResource(store AgStore, authStore AuthTokenStore) *Resource {
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
		r.Use(jwt.Authenticator)

		// Category routes
		r.Route("/categories", func(r chi.Router) {
			r.Get("/", rs.listCategories)
			r.Post("/", rs.createCategory)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", rs.getCategory)
				r.Put("/", rs.updateCategory)
				r.Delete("/", rs.deleteCategory)
				r.Get("/ags", rs.getCategoryAgs)
			})
		})

		// Activity Group routes
		r.Route("/", func(r chi.Router) {
			r.Get("/", rs.listAgs)
			r.Post("/", rs.createAg)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", rs.getAg)
				r.Put("/", rs.updateAg)
				r.Delete("/", rs.deleteAg)
				r.Get("/students", rs.getAgStudents)
				r.Get("/times", rs.getAgTimes)
				r.Post("/times", rs.addAgTime)
				r.Delete("/times/{timeId}", rs.deleteAgTime)
				r.Post("/enroll/{studentId}", rs.enrollStudent)
				r.Delete("/enroll/{studentId}", rs.unenrollStudent)
			})
		})

		// Student-specific routes
		r.Route("/student", func(r chi.Router) {
			r.Get("/{id}/ags", rs.getStudentAgs)
			r.Get("/available", rs.getAvailableAgs)
		})

		// Public routes
		r.Route("/public", func(r chi.Router) {
			r.Get("/", rs.listPublicAgs)
			r.Get("/categories", rs.listPublicCategories)
		})
	})

	return r
}

// ======== Request/Response Models ========

// AgCategoryRequest is the request payload for AgCategory data
type AgCategoryRequest struct {
	*models2.AgCategory
}

// Bind preprocesses an AgCategoryRequest
func (req *AgCategoryRequest) Bind(r *http.Request) error {
	if req.AgCategory == nil {
		return errors.New("missing category data")
	}
	return nil
}

// AgRequest is the request payload for Ag data
type AgRequest struct {
	*models2.Ag
	StudentIDs []int64           `json:"student_ids,omitempty"`
	Timeslots  []*models2.AgTime `json:"timeslots,omitempty"`
}

// Bind preprocesses an AgRequest
func (req *AgRequest) Bind(r *http.Request) error {
	if req.Ag == nil {
		return errors.New("missing activity group data")
	}
	return nil
}

// AgTimeRequest is the request payload for AgTime data
type AgTimeRequest struct {
	*models2.AgTime
}

// Bind preprocesses an AgTimeRequest
func (req *AgTimeRequest) Bind(r *http.Request) error {
	if req.AgTime == nil {
		return errors.New("missing time slot data")
	}
	return nil
}

// EnrollmentRequest is the request payload for student enrollment/unenrollment
type EnrollmentRequest struct {
	AgID      int64 `json:"ag_id"`
	StudentID int64 `json:"student_id"`
}

// Bind preprocesses an EnrollmentRequest
func (req *EnrollmentRequest) Bind(r *http.Request) error {
	if req.AgID == 0 {
		return errors.New("activity group ID is required")
	}
	if req.StudentID == 0 {
		return errors.New("student ID is required")
	}
	return nil
}

// ======== Category Handlers ========

// listCategories returns a list of all activity group categories
func (rs *Resource) listCategories(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	categories, err := rs.Store.ListAgCategories(ctx)
	if err != nil {
		logger.WithError(err).Error("Failed to list activity group categories")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("count", len(categories)).Info("Listed activity group categories")
	render.JSON(w, r, categories)
}

// createCategory creates a new activity group category
func (rs *Resource) createCategory(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	data := &AgCategoryRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid category creation request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Validate the category data
	if err := ValidateAgCategory(data.AgCategory); err != nil {
		logger.WithError(err).Warn("Category validation failed")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	if err := rs.Store.CreateAgCategory(ctx, data.AgCategory); err != nil {
		logger.WithError(err).Error("Failed to create activity group category")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"category_id": data.ID,
		"name":        data.Name,
	}).Info("Activity group category created successfully")

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, data.AgCategory)
}

// getCategory returns a specific activity group category
func (rs *Resource) getCategory(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid category ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	category, err := rs.Store.GetAgCategoryByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get category by ID")
		render.Render(w, r, ErrNotFound())
		return
	}

	render.JSON(w, r, category)
}

// updateCategory updates a specific activity group category
func (rs *Resource) updateCategory(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid category ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &AgCategoryRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid category update request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Validate the category data
	if err := ValidateAgCategory(data.AgCategory); err != nil {
		logger.WithError(err).Warn("Category validation failed")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Ensure the ID in the URL matches the ID in the request body
	data.ID = id

	ctx := r.Context()
	if err := rs.Store.UpdateAgCategory(ctx, data.AgCategory); err != nil {
		logger.WithError(err).Error("Failed to update activity group category")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the updated category
	updatedCategory, err := rs.Store.GetAgCategoryByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve updated category")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("category_id", id).Info("Activity group category updated successfully")
	render.JSON(w, r, updatedCategory)
}

// deleteCategory deletes a specific activity group category
func (rs *Resource) deleteCategory(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid category ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.DeleteAgCategory(ctx, id); err != nil {
		logger.WithError(err).Error("Failed to delete activity group category")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("category_id", id).Info("Activity group category deleted successfully")
	render.NoContent(w, r)
}

// getCategoryAgs returns all activity groups in a specific category
func (rs *Resource) getCategoryAgs(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid category ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()

	// Get all AGs with filter for this category
	filters := map[string]interface{}{
		"category_id": id,
	}

	ags, err := rs.Store.ListAgs(ctx, filters)
	if err != nil {
		logger.WithError(err).Error("Failed to list activity groups for category")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"category_id": id,
		"count":       len(ags),
	}).Info("Listed activity groups for category")

	render.JSON(w, r, ags)
}

// ======== Activity Group Handlers ========

// listAgs returns a list of all activity groups with optional filtering
func (rs *Resource) listAgs(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	// Parse query parameters for filtering
	filters := make(map[string]interface{})

	if categoryIDStr := r.URL.Query().Get("category_id"); categoryIDStr != "" {
		categoryID, err := strconv.ParseInt(categoryIDStr, 10, 64)
		if err == nil {
			filters["category_id"] = categoryID
		} else {
			logger.WithError(err).Warn("Invalid category_id parameter")
		}
	}

	if supervisorIDStr := r.URL.Query().Get("supervisor_id"); supervisorIDStr != "" {
		supervisorID, err := strconv.ParseInt(supervisorIDStr, 10, 64)
		if err == nil {
			filters["supervisor_id"] = supervisorID
		} else {
			logger.WithError(err).Warn("Invalid supervisor_id parameter")
		}
	}

	if isOpenStr := r.URL.Query().Get("is_open"); isOpenStr != "" {
		isOpen := isOpenStr == "true"
		filters["is_open"] = isOpen
	}

	if activeStr := r.URL.Query().Get("active"); activeStr != "" {
		active := activeStr == "true"
		filters["active"] = active
	}

	if searchTerm := r.URL.Query().Get("search"); searchTerm != "" {
		filters["search"] = searchTerm
	}

	ags, err := rs.Store.ListAgs(ctx, filters)
	if err != nil {
		logger.WithError(err).Error("Failed to list activity groups")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("count", len(ags)).Info("Listed activity groups")
	render.JSON(w, r, ags)
}

// createAg creates a new activity group
func (rs *Resource) createAg(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	data := &AgRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid activity group creation request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Validate the activity group data
	if err := ValidateAg(data.Ag); err != nil {
		logger.WithError(err).Warn("Activity group validation failed")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Validate timeslots if provided
	if data.Timeslots != nil {
		for i, timeslot := range data.Timeslots {
			if err := ValidateAgTime(timeslot); err != nil {
				logger.WithError(err).Warnf("Timeslot validation failed at index %d", i)
				render.Render(w, r, ErrInvalidRequest(fmt.Errorf("invalid timeslot at index %d: %w", i, err)))
				return
			}
		}
	}

	ctx := r.Context()
	if err := rs.Store.CreateAg(ctx, data.Ag, data.StudentIDs, data.Timeslots); err != nil {
		logger.WithError(err).Error("Failed to create activity group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the created activity group with all its relations
	ag, err := rs.Store.GetAgByID(ctx, data.Ag.ID)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve newly created activity group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"ag_id": ag.ID,
		"name":  ag.Name,
	}).Info("Activity group created successfully")

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, ag)
}

// getAg returns a specific activity group
func (rs *Resource) getAg(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid activity group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	ag, err := rs.Store.GetAgByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get activity group by ID")
		render.Render(w, r, ErrNotFound())
		return
	}

	render.JSON(w, r, ag)
}

// updateAg updates a specific activity group
func (rs *Resource) updateAg(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid activity group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &AgRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid activity group update request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Validate the activity group data
	if err := ValidateAg(data.Ag); err != nil {
		logger.WithError(err).Warn("Activity group validation failed")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Ensure the ID in the URL matches the ID in the request body
	data.ID = id

	ctx := r.Context()
	if err := rs.Store.UpdateAg(ctx, data.Ag); err != nil {
		logger.WithError(err).Error("Failed to update activity group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the updated activity group with all its relations
	updatedAg, err := rs.Store.GetAgByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve updated activity group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("ag_id", id).Info("Activity group updated successfully")
	render.JSON(w, r, updatedAg)
}

// deleteAg deletes a specific activity group
func (rs *Resource) deleteAg(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid activity group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.DeleteAg(ctx, id); err != nil {
		logger.WithError(err).Error("Failed to delete activity group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("ag_id", id).Info("Activity group deleted successfully")
	render.NoContent(w, r)
}

// getAgStudents returns all students enrolled in a specific activity group
func (rs *Resource) getAgStudents(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid activity group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	students, err := rs.Store.ListEnrolledStudents(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to list enrolled students")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"ag_id": id,
		"count": len(students),
	}).Info("Listed enrolled students for activity group")

	render.JSON(w, r, students)
}

// getAgTimes returns all time slots for a specific activity group
func (rs *Resource) getAgTimes(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid activity group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	times, err := rs.Store.ListAgTimes(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to list time slots")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"ag_id": id,
		"count": len(times),
	}).Info("Listed time slots for activity group")

	render.JSON(w, r, times)
}

// addAgTime adds a new time slot to a specific activity group
func (rs *Resource) addAgTime(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid activity group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &AgTimeRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid time slot creation request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Validate the time slot data
	if err := ValidateAgTime(data.AgTime); err != nil {
		logger.WithError(err).Warn("Time slot validation failed")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Set the activity group ID
	data.AgID = id

	ctx := r.Context()
	if err := rs.Store.CreateAgTime(ctx, data.AgTime); err != nil {
		logger.WithError(err).Error("Failed to create time slot")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the created time slot
	timeSlot, err := rs.Store.GetAgTimeByID(ctx, data.AgTime.ID)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve newly created time slot")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"ag_id":   id,
		"time_id": timeSlot.ID,
		"weekday": timeSlot.Weekday,
	}).Info("Time slot added to activity group successfully")

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, timeSlot)
}

// deleteAgTime deletes a specific time slot from an activity group
func (rs *Resource) deleteAgTime(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	agIDStr := chi.URLParam(r, "id")
	agID, err := strconv.ParseInt(agIDStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid activity group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid activity group ID format")))
		return
	}

	timeIDStr := chi.URLParam(r, "timeId")
	timeID, err := strconv.ParseInt(timeIDStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid time slot ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid time slot ID format")))
		return
	}

	// Verify the time slot belongs to this activity group
	ctx := r.Context()
	timeSlot, err := rs.Store.GetAgTimeByID(ctx, timeID)
	if err != nil {
		logger.WithError(err).Error("Failed to get time slot by ID")
		render.Render(w, r, ErrNotFound())
		return
	}

	if timeSlot.AgID != agID {
		logger.Warn("Time slot does not belong to the specified activity group")
		render.Render(w, r, ErrForbidden)
		return
	}

	if err := rs.Store.DeleteAgTime(ctx, timeID); err != nil {
		logger.WithError(err).Error("Failed to delete time slot")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"ag_id":   agID,
		"time_id": timeID,
	}).Info("Time slot deleted successfully")

	render.NoContent(w, r)
}

// enrollStudent enrolls a student in a specific activity group
func (rs *Resource) enrollStudent(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	agIDStr := chi.URLParam(r, "id")
	agID, err := strconv.ParseInt(agIDStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid activity group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid activity group ID format")))
		return
	}

	studentIDStr := chi.URLParam(r, "studentId")
	studentID, err := strconv.ParseInt(studentIDStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid student ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid student ID format")))
		return
	}

	ctx := r.Context()

	// Verify the activity group exists
	ag, err := rs.Store.GetAgByID(ctx, agID)
	if err != nil {
		logger.WithError(err).Error("Failed to get activity group by ID")
		render.Render(w, r, ErrNotFound())
		return
	}

	// Check if the activity group has available space
	if !HasAvailableSpace(ag) {
		logger.WithField("ag_id", agID).Warn("Activity group is full")
		render.Render(w, r, ErrConflict(errors.New("activity group is full")))
		return
	}

	if err := rs.Store.EnrollStudent(ctx, agID, studentID); err != nil {
		logger.WithError(err).Error("Failed to enroll student")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"ag_id":      agID,
		"student_id": studentID,
	}).Info("Student enrolled in activity group successfully")

	// Return success response
	render.JSON(w, r, map[string]interface{}{
		"success":     true,
		"ag_id":       agID,
		"student_id":  studentID,
		"enrolled_at": time.Now(),
	})
}

// unenrollStudent removes a student from a specific activity group
func (rs *Resource) unenrollStudent(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	agIDStr := chi.URLParam(r, "id")
	agID, err := strconv.ParseInt(agIDStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid activity group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid activity group ID format")))
		return
	}

	studentIDStr := chi.URLParam(r, "studentId")
	studentID, err := strconv.ParseInt(studentIDStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid student ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid student ID format")))
		return
	}

	ctx := r.Context()

	// Verify the activity group exists
	if _, err := rs.Store.GetAgByID(ctx, agID); err != nil {
		logger.WithError(err).Error("Failed to get activity group by ID")
		render.Render(w, r, ErrNotFound())
		return
	}

	if err := rs.Store.UnenrollStudent(ctx, agID, studentID); err != nil {
		logger.WithError(err).Error("Failed to unenroll student")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"ag_id":      agID,
		"student_id": studentID,
	}).Info("Student unenrolled from activity group successfully")

	render.NoContent(w, r)
}

// ======== Student-Specific Handlers ========

// getStudentAgs returns all activity groups a student is enrolled in
func (rs *Resource) getStudentAgs(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid student ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid student ID format")))
		return
	}

	ctx := r.Context()
	ags, err := rs.Store.ListStudentAgs(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to list student's activity groups")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"student_id": id,
		"count":      len(ags),
	}).Info("Listed student's activity groups")

	render.JSON(w, r, ags)
}

// getAvailableAgs returns all activity groups a student can enroll in
func (rs *Resource) getAvailableAgs(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	// Parse student ID from query parameter
	studentIDStr := r.URL.Query().Get("student_id")
	if studentIDStr == "" {
		logger.Warn("Missing student_id parameter")
		render.Render(w, r, ErrInvalidRequest(errors.New("student_id parameter is required")))
		return
	}

	studentID, err := strconv.ParseInt(studentIDStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid student_id parameter")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid student ID format")))
		return
	}

	// Get student's current AGs
	enrolledAgs, err := rs.Store.ListStudentAgs(ctx, studentID)
	if err != nil {
		logger.WithError(err).Error("Failed to list student's activity groups")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Create a set of AG IDs the student is already enrolled in
	enrolledAgIDs := make(map[int64]bool)
	for _, ag := range enrolledAgs {
		enrolledAgIDs[ag.ID] = true
	}

	// Get all AGs that are open and have available spaces
	filters := map[string]interface{}{
		"is_open": true,
		"active":  true,
	}

	allAgs, err := rs.Store.ListAgs(ctx, filters)
	if err != nil {
		logger.WithError(err).Error("Failed to list available activity groups")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Filter out AGs the student is already enrolled in and those that are full
	availableAgs := make([]models2.Ag, 0)
	for _, ag := range allAgs {
		// Skip if student is already enrolled
		if enrolledAgIDs[ag.ID] {
			continue
		}

		// Skip if AG is full
		if !HasAvailableSpace(&ag) {
			continue
		}

		availableAgs = append(availableAgs, ag)
	}

	logger.WithFields(logrus.Fields{
		"student_id": studentID,
		"count":      len(availableAgs),
	}).Info("Listed available activity groups for student")

	render.JSON(w, r, availableAgs)
}

// ======== Public Handlers ========

// listPublicAgs returns a public list of active activity groups
func (rs *Resource) listPublicAgs(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	// Only include active and open activity groups
	filters := map[string]interface{}{
		"is_open": true,
		"active":  true,
	}

	if categoryIDStr := r.URL.Query().Get("category_id"); categoryIDStr != "" {
		categoryID, err := strconv.ParseInt(categoryIDStr, 10, 64)
		if err == nil {
			filters["category_id"] = categoryID
		}
	}

	ags, err := rs.Store.ListAgs(ctx, filters)
	if err != nil {
		logger.WithError(err).Error("Failed to list public activity groups")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Create a limited view of the activity groups for public access
	type PublicAg struct {
		ID             int64  `json:"id"`
		Name           string `json:"name"`
		CategoryID     int64  `json:"category_id"`
		CategoryName   string `json:"category_name,omitempty"`
		MaxParticipant int    `json:"max_participant"`
		AvailableSpots int    `json:"available_spots"`
		Schedule       string `json:"schedule,omitempty"`
	}

	publicAgs := make([]PublicAg, 0, len(ags))
	for _, ag := range ags {
		categoryName := ""
		if ag.AgCategory != nil {
			categoryName = ag.AgCategory.Name
		}

		participantCount := 0
		if ag.Students != nil {
			participantCount = len(ag.Students)
		}

		availableSpots := ag.MaxParticipant - participantCount
		if availableSpots < 0 {
			availableSpots = 0
		}

		publicAg := PublicAg{
			ID:             ag.ID,
			Name:           ag.Name,
			CategoryID:     ag.AgCategoryID,
			CategoryName:   categoryName,
			MaxParticipant: ag.MaxParticipant,
			AvailableSpots: availableSpots,
			Schedule:       FormatTimeslots(&ag),
		}

		publicAgs = append(publicAgs, publicAg)
	}

	logger.WithField("count", len(publicAgs)).Info("Listed public activity groups")
	render.JSON(w, r, publicAgs)
}

// listPublicCategories returns a public list of activity group categories
func (rs *Resource) listPublicCategories(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	categories, err := rs.Store.ListAgCategories(ctx)
	if err != nil {
		logger.WithError(err).Error("Failed to list public activity group categories")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Create a limited view of the categories for public access
	type PublicCategory struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}

	publicCategories := make([]PublicCategory, len(categories))
	for i, category := range categories {
		publicCategories[i] = PublicCategory{
			ID:   category.ID,
			Name: category.Name,
		}
	}

	logger.WithField("count", len(publicCategories)).Info("Listed public activity group categories")
	render.JSON(w, r, publicCategories)
}
