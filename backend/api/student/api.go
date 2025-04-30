// Package student provides the student management API
package student

import (
	"context"
	"errors"
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

// UserStore defines operations for updating CustomUser records
type UserStore interface {
	GetCustomUserByID(ctx context.Context, id int64) (*models2.CustomUser, error)
	UpdateCustomUser(ctx context.Context, user *models2.CustomUser) error
}

// Resource defines the student management resource
type Resource struct {
	Store     StudentStore
	UserStore UserStore
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
	GetToken(t string) (*jwt.Token, error)
}

// NewResource creates a new student management resource
func NewResource(store StudentStore, userStore UserStore, authStore AuthTokenStore) *Resource {
	return &Resource{
		Store:     store,
		UserStore: userStore,
		AuthStore: authStore,
	}
}

// Router creates a router for student management
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()

	// JWT protected routes
	r.Group(func(r chi.Router) {
		r.Use(jwt.Authenticator)

		// Student routes
		r.Route("/", func(r chi.Router) {
			r.Get("/", rs.listStudents)
			r.Post("/", rs.createStudent)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", rs.getStudent)
				r.Put("/", rs.updateStudent)
				r.Delete("/", rs.deleteStudent)
				r.Get("/visits", rs.getStudentVisits)
				r.Get("/status", rs.getStudentStatus)
			})
		})

		// Special operations
		r.Post("/register-in-room", rs.registerStudentInRoom)
		r.Post("/unregister-from-room", rs.unregisterStudentFromRoom)
		r.Post("/update-location", rs.updateStudentLocation)
		r.Post("/give-feedback", rs.giveFeedback)

		// Combined group visits
		r.Get("/combined-group/{id}/visits", rs.getCombinedGroupVisits)

		// Room visits
		r.Get("/room/{id}/visits", rs.getRoomVisits)

		// Public API - usable without auth
		r.Route("/public", func(r chi.Router) {
			r.Get("/summary", rs.getStudentsSummary)
		})
	})

	return r
}

// ======== Student Handlers ========

// listStudents returns a list of all students
func (rs *Resource) listStudents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.GetLogEntry(r)

	// Parse query parameters for filtering
	filters := make(map[string]interface{})

	if groupIDStr := r.URL.Query().Get("group_id"); groupIDStr != "" {
		groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err == nil {
			filters["group_id"] = groupID
		} else {
			logger.WithError(err).Warn("Invalid group_id parameter")
		}
	}

	if searchTerm := r.URL.Query().Get("search"); searchTerm != "" {
		filters["search"] = searchTerm
	}

	if inHouseStr := r.URL.Query().Get("in_house"); inHouseStr != "" {
		inHouse := inHouseStr == "true"
		filters["in_house"] = inHouse
	}

	if wcStr := r.URL.Query().Get("wc"); wcStr != "" {
		wc := wcStr == "true"
		filters["wc"] = wc
	}

	if schoolYardStr := r.URL.Query().Get("school_yard"); schoolYardStr != "" {
		schoolYard := schoolYardStr == "true"
		filters["school_yard"] = schoolYard
	}

	students, err := rs.Store.ListStudents(ctx, filters)
	if err != nil {
		logger.WithError(err).Error("Failed to list students")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, students)
}

// createStudent creates a new student
func (rs *Resource) createStudent(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	data := &StudentRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid student creation request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Validate the student data
	if err := ValidateStudent(data.Student); err != nil {
		logger.WithError(err).Warn("Student validation failed")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	if err := rs.Store.CreateStudent(ctx, data.Student); err != nil {
		logger.WithError(err).Error("Failed to create student")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the newly created student to include all relations
	student, err := rs.Store.GetStudentByID(ctx, data.Student.ID)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve newly created student")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"student_id": student.ID,
		"name":       student.CustomUser.FirstName + " " + student.CustomUser.SecondName,
	}).Info("Student created successfully")

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, student)
}

// getStudent returns a specific student
func (rs *Resource) getStudent(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid student ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	student, err := rs.Store.GetStudentByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get student by ID")
		render.Render(w, r, ErrNotFound())
		return
	}

	render.JSON(w, r, student)
}

// updateStudent updates a specific student
func (rs *Resource) updateStudent(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid student ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &StudentRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid student update request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Validate the student data
	if err := ValidateStudent(data.Student); err != nil {
		logger.WithError(err).Warn("Student validation failed")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	student, err := rs.Store.GetStudentByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get student by ID for update")
		render.Render(w, r, ErrNotFound())
		return
	}

	// Update student fields except ID, CreatedAt and relationships that should be managed separately
	student.SchoolClass = data.SchoolClass
	student.Bus = data.Bus
	student.NameLG = data.NameLG
	student.ContactLG = data.ContactLG
	student.InHouse = data.InHouse
	student.WC = data.WC
	student.SchoolYard = data.SchoolYard
	student.GroupID = data.GroupID

	// Check if we need to update the CustomUser data (name fields)
	if (data.FirstName != "" || data.SecondName != "") && student.CustomUserID > 0 {
		customUser, err := rs.UserStore.GetCustomUserByID(ctx, student.CustomUserID)
		if err != nil {
			logger.WithError(err).Error("Failed to get CustomUser for student")
			render.Render(w, r, ErrInternalServerError(err))
			return
		}

		if data.FirstName != "" {
			customUser.FirstName = data.FirstName
		}

		if data.SecondName != "" {
			customUser.SecondName = data.SecondName
		}

		if err := rs.UserStore.UpdateCustomUser(ctx, customUser); err != nil {
			logger.WithError(err).Error("Failed to update CustomUser name")
			render.Render(w, r, ErrInternalServerError(err))
			return
		}

		logger.WithField("custom_user_id", student.CustomUserID).Info("CustomUser name updated successfully")
	}

	if err := rs.Store.UpdateStudent(ctx, student); err != nil {
		logger.WithError(err).Error("Failed to update student")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the updated student with all relations
	student, err = rs.Store.GetStudentByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve updated student")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("student_id", id).Info("Student updated successfully")
	render.JSON(w, r, student)
}

// deleteStudent deletes a specific student
func (rs *Resource) deleteStudent(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid student ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.DeleteStudent(ctx, id); err != nil {
		logger.WithError(err).Error("Failed to delete student")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("student_id", id).Info("Student deleted successfully")
	render.NoContent(w, r)
}

// getStudentVisits returns visits for a student
func (rs *Resource) getStudentVisits(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid student ID format")
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
		} else {
			logger.WithError(err).Warn("Invalid date format")
		}
	}

	visits, err := rs.Store.GetStudentVisits(ctx, id, date)
	if err != nil {
		logger.WithError(err).Error("Failed to get student visits")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, visits)
}

// getStudentStatus returns the current status of a student
func (rs *Resource) getStudentStatus(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid student ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	student, err := rs.Store.GetStudentByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get student by ID for status")
		render.Render(w, r, ErrNotFound())
		return
	}

	status := GetStudentCurrentStatus(student)

	response := map[string]interface{}{
		"student_id":  student.ID,
		"name":        student.CustomUser.FirstName + " " + student.CustomUser.SecondName,
		"status":      status,
		"in_house":    student.InHouse,
		"wc":          student.WC,
		"school_yard": student.SchoolYard,
		"bus":         student.Bus,
	}

	render.JSON(w, r, response)
}

// getStudentsSummary returns a summary of all students and their statuses
func (rs *Resource) getStudentsSummary(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	students, err := rs.Store.ListStudents(ctx, nil)
	if err != nil {
		logger.WithError(err).Error("Failed to list students for summary")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	summary := map[string]interface{}{
		"total_students":    len(students),
		"in_house_count":    0,
		"wc_count":          0,
		"school_yard_count": 0,
		"not_present_count": 0,
	}

	for _, student := range students {
		if student.InHouse {
			summary["in_house_count"] = summary["in_house_count"].(int) + 1
			if student.WC {
				summary["wc_count"] = summary["wc_count"].(int) + 1
			}
		} else if student.SchoolYard {
			summary["school_yard_count"] = summary["school_yard_count"].(int) + 1
		} else {
			summary["not_present_count"] = summary["not_present_count"].(int) + 1
		}
	}

	render.JSON(w, r, summary)
}

// ======== Special Operations ========

// RoomOccupancyStore defines operations for getting room occupancy by device ID
type RoomOccupancyStore interface {
	GetRoomOccupancyByDeviceID(ctx context.Context, deviceID string) (*models2.RoomOccupancyDetail, error)
}

// registerStudentInRoom registers a student in a room
func (rs *Resource) registerStudentInRoom(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	data := &RoomRegistrationRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid room registration request")
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
		logger.WithError(err).Error("Failed to create student visit")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Update in_house status
	locations := map[string]bool{
		"in_house": true,
	}
	if err := rs.Store.UpdateStudentLocation(ctx, data.StudentID, locations); err != nil {
		logger.WithError(err).Error("Failed to update student location")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"student_id": data.StudentID,
		"room_id":    roomID,
		"visit_id":   visit.ID,
	}).Info("Student registered in room successfully")

	render.JSON(w, r, visit)
}

// unregisterStudentFromRoom unregisters a student from a room
func (rs *Resource) unregisterStudentFromRoom(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	data := &RoomRegistrationRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid room unregistration request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()

	// Update in_house status to false
	locations := map[string]bool{
		"in_house": false,
	}
	if err := rs.Store.UpdateStudentLocation(ctx, data.StudentID, locations); err != nil {
		logger.WithError(err).Error("Failed to update student location")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("student_id", data.StudentID).Info("Student unregistered from room")

	// Return success message
	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "Student unregistered from room",
	})
}

// getRoomVisits returns visits for a room
func (rs *Resource) getRoomVisits(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid room ID format")
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
		} else {
			logger.WithError(err).Warn("Invalid date format")
		}
	}

	// Parse active parameter
	active := false
	if activeStr := r.URL.Query().Get("active"); activeStr == "true" {
		active = true
	}

	visits, err := rs.Store.GetRoomVisits(ctx, id, date, active)
	if err != nil {
		logger.WithError(err).Error("Failed to get room visits")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, visits)
}

// getCombinedGroupVisits returns visits for a combined group
func (rs *Resource) getCombinedGroupVisits(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid combined group ID format")
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
		} else {
			logger.WithError(err).Warn("Invalid date format")
		}
	}

	// Parse active parameter
	active := false
	if activeStr := r.URL.Query().Get("active"); activeStr == "true" {
		active = true
	}

	visits, err := rs.Store.GetCombinedGroupVisits(ctx, id, date, active)
	if err != nil {
		logger.WithError(err).Error("Failed to get combined group visits")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, visits)
}

// giveFeedback records feedback from a student
func (rs *Resource) giveFeedback(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	data := &FeedbackRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid feedback request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	feedback, err := rs.Store.CreateFeedback(ctx, data.StudentID, data.FeedbackValue, data.MensaFeedback)
	if err != nil {
		logger.WithError(err).Error("Failed to create feedback")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"student_id":     data.StudentID,
		"feedback_id":    feedback.ID,
		"mensa_feedback": data.MensaFeedback,
	}).Info("Student feedback recorded successfully")

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, feedback)
}

// updateStudentLocation updates a student's location flags
func (rs *Resource) updateStudentLocation(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	data := &LocationUpdateRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid location update request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	if err := rs.Store.UpdateStudentLocation(ctx, data.StudentID, data.Locations); err != nil {
		logger.WithError(err).Error("Failed to update student location")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"student_id": data.StudentID,
		"locations":  data.Locations,
	}).Info("Student location updated successfully")

	// Return success message
	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "Student location updated",
	})
}

// StudentRequest represents request payload for student data
type StudentRequest struct {
	*models2.Student
	FirstName  string `json:"first_name,omitempty"`
	SecondName string `json:"second_name,omitempty"`
}

// Bind preprocesses a StudentRequest
func (sr *StudentRequest) Bind(r *http.Request) error {
	// Simple validation - more detailed validation in ValidateStudent
	if sr.Student == nil {
		return errors.New("missing student data")
	}
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
