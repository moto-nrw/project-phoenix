// Package student provides the student management API
package student

import (
	"context"
	"errors"
	jwt2 "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/logging"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/sirupsen/logrus"
)

// Resource defines the student management resource
type Resource struct {
	Store     StudentStore
	AuthStore AuthTokenStore
}

// StudentStore defines database operations for student management
type StudentStore interface {
	GetStudentByID(ctx context.Context, id int64) (*models2.Student, error)
	GetStudentByCustomUserID(ctx context.Context, customUserID int64) (*models2.Student, error)
	CreateStudent(ctx context.Context, student *models2.Student) error
	UpdateStudent(ctx context.Context, student *models2.Student) error
	DeleteStudent(ctx context.Context, id int64) error
	ListStudents(ctx context.Context, filters map[string]interface{}) ([]models2.Student, error)
	UpdateStudentLocation(ctx context.Context, id int64, locations map[string]bool) error
	CreateStudentVisit(ctx context.Context, studentID, roomID, timespanID int64) (*models2.Visit, error)
	GetStudentVisits(ctx context.Context, studentID int64, date *time.Time) ([]models2.Visit, error)
	GetRoomVisits(ctx context.Context, roomID int64, date *time.Time, active bool) ([]models2.Visit, error)
	GetCombinedGroupVisits(ctx context.Context, combinedGroupID int64, date *time.Time, active bool) ([]models2.Visit, error)
	GetStudentAsList(ctx context.Context, id int64) (*models2.StudentList, error)
	CreateFeedback(ctx context.Context, studentID int64, feedbackValue string, mensaFeedback bool) (*models2.Feedback, error)
}

// AuthTokenStore defines operations for the auth token store
type AuthTokenStore interface {
	GetToken(t string) (*jwt2.Token, error)
}

// NewResource creates a new student management resource
func NewResource(store StudentStore, authStore AuthTokenStore) *Resource {
	return &Resource{
		Store:     store,
		AuthStore: authStore,
	}
}

// Router creates a router for student management
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()

	// JWT protected routes
	r.Group(func(r chi.Router) {
		r.Use(jwt2.Authenticator)

		// Student routes
		r.Route("/", func(r chi.Router) {
			r.Get("/", rs.listStudents)
			r.Post("/", rs.createStudent)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", rs.getStudent)
				r.Put("/", rs.updateStudent)
				r.Delete("/", rs.deleteStudent)
				r.Get("/visits", rs.getStudentVisits)
			})
		})

		// Special operations
		r.Post("/register-in-room", rs.registerStudentInRoom)
		r.Post("/unregister-from-room", rs.unregisterStudentFromRoom)
		r.Post("/update-location", rs.updateStudentLocation)
		r.Post("/give-feedback", rs.giveFeedback)

		// Combined group visits
		r.Get("/combined-group/{id}/visits", rs.getCombinedGroupVisits)
	})

	return r
}

// ======== Student Handlers ========

// listStudents returns a list of all students
func (rs *Resource) listStudents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters for filtering
	filters := make(map[string]interface{})

	if groupIDStr := r.URL.Query().Get("group_id"); groupIDStr != "" {
		groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err == nil {
			filters["group_id"] = groupID
		}
	}

	if searchTerm := r.URL.Query().Get("search"); searchTerm != "" {
		filters["search"] = searchTerm
	}

	if inHouseStr := r.URL.Query().Get("in_house"); inHouseStr != "" {
		inHouse := inHouseStr == "true"
		filters["in_house"] = inHouse
	}

	students, err := rs.Store.ListStudents(ctx, filters)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, students)
}

// createStudent creates a new student
func (rs *Resource) createStudent(w http.ResponseWriter, r *http.Request) {
	data := &StudentRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	if err := rs.Store.CreateStudent(ctx, data.Student); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the newly created student to include all relations
	student, err := rs.Store.GetStudentByID(ctx, data.Student.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, student)
}

// getStudent returns a specific student
func (rs *Resource) getStudent(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	student, err := rs.Store.GetStudentByID(ctx, id)
	if err != nil {
		render.Render(w, r, ErrNotFound())
		return
	}

	render.JSON(w, r, student)
}

// updateStudent updates a specific student
func (rs *Resource) updateStudent(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &StudentRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	student, err := rs.Store.GetStudentByID(ctx, id)
	if err != nil {
		render.Render(w, r, ErrNotFound())
		return
	}

	// Update student fields except ID, CreatedAt and relationships
	student.SchoolClass = data.SchoolClass
	student.Bus = data.Bus
	student.NameLG = data.NameLG
	student.ContactLG = data.ContactLG
	student.InHouse = data.InHouse
	student.WC = data.WC
	student.SchoolYard = data.SchoolYard
	student.GroupID = data.GroupID

	if err := rs.Store.UpdateStudent(ctx, student); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the updated student with all relations
	student, err = rs.Store.GetStudentByID(ctx, id)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, student)
}

// deleteStudent deletes a specific student
func (rs *Resource) deleteStudent(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.DeleteStudent(ctx, id); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.NoContent(w, r)
}

// ======== Special Operations ========

// RoomOccupancyStore defines operations for getting room occupancy by device ID
type RoomOccupancyStore interface {
	GetRoomOccupancyByDeviceID(ctx context.Context, deviceID string) (*models2.RoomOccupancyDetail, error)
}

// registerStudentInRoom registers a student in a room
func (rs *Resource) registerStudentInRoom(w http.ResponseWriter, r *http.Request) {
	data := &RoomRegistrationRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()

	// Get the active room occupancy for the device ID
	log := logging.GetLogEntry(r)
	log.WithFields(logrus.Fields{
		"student_id": data.StudentID,
		"device_id":  data.DeviceID,
	}).Info("Registering student in room")

	// In a production environment, we would:
	// 1. Lookup the device ID to find the room
	// 2. Get the roomID and timespanID from the RoomOccupancy

	// For now, we'll use fallback values if the lookup fails
	roomID := int64(1)     // Default fallback
	timespanID := int64(1) // Default fallback

	// Try to get the actual room occupancy info if we have a RoomOccupancyStore
	if roomStore, ok := rs.Store.(RoomOccupancyStore); ok {
		occupancy, err := roomStore.GetRoomOccupancyByDeviceID(ctx, data.DeviceID)
		if err == nil && occupancy != nil {
			// Extract roomID from the detail
			roomID = occupancy.RoomID
			timespanID = occupancy.TimespanID

			log.WithFields(logrus.Fields{
				"room_id":     roomID,
				"timespan_id": timespanID,
			}).Info("Found room occupancy for device")
		} else {
			log.WithError(err).Warn("Could not find room occupancy for device, using fallback values")
		}
	}

	// Create a visit record
	visit, err := rs.Store.CreateStudentVisit(ctx, data.StudentID, roomID, timespanID)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Update in_house status
	locations := map[string]bool{
		"in_house": true,
	}
	if err := rs.Store.UpdateStudentLocation(ctx, data.StudentID, locations); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, visit)
}

// unregisterStudentFromRoom unregisters a student from a room
func (rs *Resource) unregisterStudentFromRoom(w http.ResponseWriter, r *http.Request) {
	data := &RoomRegistrationRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()

	// Update in_house status to false
	locations := map[string]bool{
		"in_house": false,
	}
	if err := rs.Store.UpdateStudentLocation(ctx, data.StudentID, locations); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Return success message
	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "Student unregistered from room",
	})
}

// getStudentVisits returns visits for a student
func (rs *Resource) getStudentVisits(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()

	// Parse date parameter
	var date *time.Time
	if dateStr := r.URL.Query().Get("date"); dateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", dateStr)
		if err == nil {
			date = &parsedDate
		}
	}

	visits, err := rs.Store.GetStudentVisits(ctx, id, date)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, visits)
}

// getCombinedGroupVisits returns visits for a combined group
func (rs *Resource) getCombinedGroupVisits(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()

	// Parse date parameter
	var date *time.Time
	if dateStr := r.URL.Query().Get("date"); dateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", dateStr)
		if err == nil {
			date = &parsedDate
		}
	}

	// Parse active parameter
	active := false
	if activeStr := r.URL.Query().Get("active"); activeStr == "true" {
		active = true
	}

	visits, err := rs.Store.GetCombinedGroupVisits(ctx, id, date, active)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, visits)
}

// giveFeedback records feedback from a student
func (rs *Resource) giveFeedback(w http.ResponseWriter, r *http.Request) {
	data := &FeedbackRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	feedback, err := rs.Store.CreateFeedback(ctx, data.StudentID, data.FeedbackValue, data.MensaFeedback)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, feedback)
}

// updateStudentLocation updates a student's location flags
func (rs *Resource) updateStudentLocation(w http.ResponseWriter, r *http.Request) {
	data := &LocationUpdateRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	if err := rs.Store.UpdateStudentLocation(ctx, data.StudentID, data.Locations); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Return success message
	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "Student location updated",
	})
}

// StudentRequest represents request payload for student data
type StudentRequest struct {
	*models2.Student
}

// Bind preprocesses a StudentRequest
func (sr *StudentRequest) Bind(r *http.Request) error {
	// Validation logic can be added here
	return nil
}

// FeedbackRequest represents request payload for feedback
type FeedbackRequest struct {
	StudentID     int64  `json:"student_id"`
	FeedbackValue string `json:"feedback_value"`
	MensaFeedback bool   `json:"mensa_feedback"`
}

// Bind preprocesses a FeedbackRequest
func (fr *FeedbackRequest) Bind(r *http.Request) error {
	if fr.StudentID == 0 {
		return errors.New("student_id is required")
	}
	if fr.FeedbackValue == "" {
		return errors.New("feedback_value is required")
	}
	return nil
}

// LocationUpdateRequest represents request payload for updating student location
type LocationUpdateRequest struct {
	StudentID int64           `json:"student_id"`
	Locations map[string]bool `json:"locations"`
}

// Bind preprocesses a LocationUpdateRequest
func (lu *LocationUpdateRequest) Bind(r *http.Request) error {
	if lu.StudentID == 0 {
		return errors.New("student_id is required")
	}
	if lu.Locations == nil {
		return errors.New("locations is required")
	}
	return nil
}

// RoomRegistrationRequest represents request payload for room registration
type RoomRegistrationRequest struct {
	StudentID int64  `json:"student_id"`
	DeviceID  string `json:"device_id"`
}

// Bind preprocesses a RoomRegistrationRequest
func (rr *RoomRegistrationRequest) Bind(r *http.Request) error {
	if rr.StudentID == 0 {
		return errors.New("student_id is required")
	}
	if rr.DeviceID == "" {
		return errors.New("device_id is required")
	}
	return nil
}
