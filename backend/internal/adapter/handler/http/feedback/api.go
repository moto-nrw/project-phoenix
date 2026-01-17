package feedback

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/feedback"
	feedbackSvc "github.com/moto-nrw/project-phoenix/internal/core/service/feedback"
)

// Constants for date formats (S1192 - avoid duplicate string literals)
const (
	dateFormatYMD = "2006-01-02"
)

// Resource defines the feedback API resource
type Resource struct {
	FeedbackService feedbackSvc.EntryReadWriter
}

// NewResource creates a new feedback resource
func NewResource(feedbackService feedbackSvc.EntryReadWriter) *Resource {
	return &Resource{
		FeedbackService: feedbackService,
	}
}

// Router returns a configured router for feedback endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Read operations require feedback:read permission
		r.With(authorize.RequiresPermission(permissions.FeedbackRead)).Get("/", rs.listFeedback)
		r.With(authorize.RequiresPermission(permissions.FeedbackRead)).Get("/{id}", rs.getFeedback)
		r.With(authorize.RequiresPermission(permissions.FeedbackRead)).Get("/student/{id}", rs.getStudentFeedback)
		r.With(authorize.RequiresPermission(permissions.FeedbackRead)).Get("/date/{date}", rs.getDateFeedback)
		r.With(authorize.RequiresPermission(permissions.FeedbackRead)).Get("/mensa", rs.getMensaFeedback)
		r.With(authorize.RequiresPermission(permissions.FeedbackRead)).Get("/date-range", rs.getDateRangeFeedback)

		// Write operations require specific permissions
		r.With(authorize.RequiresPermission(permissions.FeedbackCreate)).Post("/", rs.createFeedback)
		r.With(authorize.RequiresPermission(permissions.FeedbackCreate)).Post("/batch", rs.createBatchFeedback)
		r.With(authorize.RequiresPermission(permissions.FeedbackDelete)).Delete("/{id}", rs.deleteFeedback)
	})

	return r
}

// FeedbackResponse represents a feedback entry API response
type FeedbackResponse struct {
	ID              int64     `json:"id"`
	Value           string    `json:"value"`
	Day             string    `json:"day"`  // Formatted as YYYY-MM-DD
	Time            string    `json:"time"` // Formatted as HH:MM:SS
	StudentID       int64     `json:"student_id"`
	IsMensaFeedback bool      `json:"is_mensa_feedback"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	// Include student details if available
	Student *StudentResponse `json:"student,omitempty"`
}

// StudentResponse represents a simplified student in feedback response
type StudentResponse struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// FeedbackRequest represents a feedback creation/update request
type FeedbackRequest struct {
	Value           string `json:"value"`
	Day             string `json:"day"`  // Expected format: YYYY-MM-DD
	Time            string `json:"time"` // Expected format: HH:MM:SS
	StudentID       int64  `json:"student_id"`
	IsMensaFeedback bool   `json:"is_mensa_feedback"`
}

// Bind validates the feedback request
func (req *FeedbackRequest) Bind(_ *http.Request) error {
	if req.Value == "" {
		return errors.New("feedback value is required")
	}
	if req.StudentID <= 0 {
		return errors.New("student ID is required")
	}
	if req.Day == "" {
		return errors.New("day is required")
	}
	if req.Time == "" {
		return errors.New("time is required")
	}

	// Validate date format
	_, err := time.Parse(dateFormatYMD, req.Day)
	if err != nil {
		return errors.New("day must be in YYYY-MM-DD format")
	}

	// Validate time format
	_, err = time.Parse("15:04:05", req.Time)
	if err != nil {
		return errors.New("time must be in HH:MM:SS format")
	}

	return nil
}

// BatchFeedbackRequest represents a batch of feedback entries to create
type BatchFeedbackRequest struct {
	Entries []FeedbackRequest `json:"entries"`
}

// Bind validates the batch feedback request
func (req *BatchFeedbackRequest) Bind(r *http.Request) error {
	if len(req.Entries) == 0 {
		return errors.New("at least one feedback entry is required")
	}

	// Validate each entry
	for i, entry := range req.Entries {
		if err := (&entry).Bind(r); err != nil {
			return errors.New("invalid entry at index " + strconv.Itoa(i) + ": " + err.Error())
		}
	}

	return nil
}

// newFeedbackResponse converts a feedback model to a response object
func newFeedbackResponse(entry *feedback.Entry) FeedbackResponse {
	response := FeedbackResponse{
		ID:              entry.ID,
		Value:           entry.Value,
		Day:             entry.GetFormattedDate(),
		Time:            entry.GetFormattedTime(),
		StudentID:       entry.StudentID,
		IsMensaFeedback: entry.IsMensaFeedback,
		CreatedAt:       entry.CreatedAt,
		UpdatedAt:       entry.UpdatedAt,
	}

	// Include student details if available
	if entry.Student != nil {
		response.Student = &StudentResponse{
			ID:        entry.Student.ID,
			FirstName: entry.Student.Person.FirstName,
			LastName:  entry.Student.Person.LastName,
		}
	}

	return response
}

// requestToModel converts a request to a model
func requestToModel(req *FeedbackRequest) (*feedback.Entry, error) {
	// Parse day
	day, err := time.Parse(dateFormatYMD, req.Day)
	if err != nil {
		return nil, errors.New("invalid day format, expected YYYY-MM-DD")
	}

	// Parse time
	timeValue, err := time.Parse("15:04:05", req.Time)
	if err != nil {
		return nil, errors.New("invalid time format, expected HH:MM:SS")
	}

	return &feedback.Entry{
		Value:           req.Value,
		Day:             day,
		Time:            timeValue,
		StudentID:       req.StudentID,
		IsMensaFeedback: req.IsMensaFeedback,
	}, nil
}

// listFeedback handles listing all feedback entries with optional filtering
func (rs *Resource) listFeedback(w http.ResponseWriter, r *http.Request) {
	// Get filter parameters
	studentIDStr := r.URL.Query().Get("student_id")
	dateStr := r.URL.Query().Get("date")
	isMensaStr := r.URL.Query().Get("is_mensa")

	// Create filters map
	filters := make(map[string]interface{})

	// Apply filters
	if studentIDStr != "" {
		studentID, err := strconv.ParseInt(studentIDStr, 10, 64)
		if err == nil && studentID > 0 {
			filters["student_id"] = studentID
		}
	}

	if dateStr != "" {
		date, err := time.Parse(dateFormatYMD, dateStr)
		if err == nil {
			filters["day"] = date
		}
	}

	if isMensaStr != "" {
		isMensa := isMensaStr == "true" || isMensaStr == "1"
		filters["is_mensa_feedback"] = isMensa
	}

	// Get feedback entries
	entries, err := rs.FeedbackService.ListEntries(r.Context(), filters)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	responses := make([]FeedbackResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, newFeedbackResponse(entry))
	}

	common.Respond(w, r, http.StatusOK, responses, "Feedback entries retrieved successfully")
}

// getFeedback handles getting a feedback entry by ID
func (rs *Resource) getFeedback(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid feedback ID")))
		return
	}

	// Get feedback entry
	entry, err := rs.FeedbackService.GetEntryByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Prepare response
	response := newFeedbackResponse(entry)

	common.Respond(w, r, http.StatusOK, response, "Feedback entry retrieved successfully")
}

// getStudentFeedback handles getting feedback entries for a specific student
func (rs *Resource) getStudentFeedback(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}

	// Get feedback entries for student
	entries, err := rs.FeedbackService.GetEntriesByStudent(r.Context(), studentID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]FeedbackResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, newFeedbackResponse(entry))
	}

	common.Respond(w, r, http.StatusOK, responses, "Student feedback entries retrieved successfully")
}

// getDateFeedback handles getting feedback entries for a specific date
func (rs *Resource) getDateFeedback(w http.ResponseWriter, r *http.Request) {
	// Parse date from URL
	dateStr := chi.URLParam(r, "date")
	day, err := time.Parse(dateFormatYMD, dateStr)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid date format, expected YYYY-MM-DD")))
		return
	}

	// Get feedback entries for date
	entries, err := rs.FeedbackService.GetEntriesByDay(r.Context(), day)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]FeedbackResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, newFeedbackResponse(entry))
	}

	common.Respond(w, r, http.StatusOK, responses, "Date feedback entries retrieved successfully")
}

// getMensaFeedback handles getting feedback entries for the cafeteria
func (rs *Resource) getMensaFeedback(w http.ResponseWriter, r *http.Request) {
	// Get query parameter
	isMensaStr := r.URL.Query().Get("is_mensa")
	isMensa := isMensaStr != "false" && isMensaStr != "0" // Default to true if not specified

	// Get mensa feedback entries
	entries, err := rs.FeedbackService.GetMensaFeedback(r.Context(), isMensa)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]FeedbackResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, newFeedbackResponse(entry))
	}

	common.Respond(w, r, http.StatusOK, responses, "Mensa feedback entries retrieved successfully")
}

// getDateRangeFeedback handles getting feedback entries within a date range
func (rs *Resource) getDateRangeFeedback(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")
	studentIDStr := r.URL.Query().Get("student_id")

	// Parse start date
	startDate, err := time.Parse(dateFormatYMD, startDateStr)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid start date format, expected YYYY-MM-DD")))
		return
	}

	// Parse end date
	endDate, err := time.Parse(dateFormatYMD, endDateStr)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid end date format, expected YYYY-MM-DD")))
		return
	}

	// Handle student ID if provided
	var entries []*feedback.Entry
	if studentIDStr != "" {
		studentID, err := strconv.ParseInt(studentIDStr, 10, 64)
		if err != nil {
			common.RenderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
			return
		}

		// Get feedback entries for student within date range
		entries, err = rs.FeedbackService.GetEntriesByStudentAndDateRange(r.Context(), studentID, startDate, endDate)
		if err != nil {
			common.RenderError(w, r, ErrorRenderer(err))
			return
		}
	} else {
		// Get feedback entries for all students within date range
		entries, err = rs.FeedbackService.GetEntriesByDateRange(r.Context(), startDate, endDate)
		if err != nil {
			common.RenderError(w, r, ErrorRenderer(err))
			return
		}
	}

	// Build response
	responses := make([]FeedbackResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, newFeedbackResponse(entry))
	}

	common.Respond(w, r, http.StatusOK, responses, "Date range feedback entries retrieved successfully")
}

// createFeedback handles creating a new feedback entry
func (rs *Resource) createFeedback(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &FeedbackRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Convert request to model
	entry, err := requestToModel(req)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Create feedback entry
	if err := rs.FeedbackService.CreateEntry(r.Context(), entry); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Prepare response
	response := newFeedbackResponse(entry)

	common.Respond(w, r, http.StatusCreated, response, "Feedback entry created successfully")
}

// createBatchFeedback handles creating multiple feedback entries in a batch
func (rs *Resource) createBatchFeedback(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &BatchFeedbackRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Convert requests to models
	entries := make([]*feedback.Entry, 0, len(req.Entries))
	for _, entryReq := range req.Entries {
		entry, err := requestToModel(&entryReq)
		if err != nil {
			common.RenderError(w, r, ErrorInvalidRequest(err))
			return
		}
		entries = append(entries, entry)
	}

	// Create feedback entries
	errorList, err := rs.FeedbackService.CreateEntries(r.Context(), entries)
	if err != nil {
		// If we have individual errors, include them in the response
		if len(errorList) > 0 {
			errorMessages := make([]string, 0, len(errorList))
			for _, e := range errorList {
				errorMessages = append(errorMessages, e.Error())
			}
			common.Respond(w, r, http.StatusPartialContent, map[string]interface{}{
				"errors": errorMessages,
			}, "Some feedback entries could not be created")
			return
		}

		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, map[string]interface{}{
		"count": len(entries),
	}, "Feedback entries created successfully")
}

// deleteFeedback handles deleting a feedback entry
func (rs *Resource) deleteFeedback(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid feedback ID")))
		return
	}

	// Delete feedback entry
	if err := rs.FeedbackService.DeleteEntry(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Feedback entry deleted successfully")
}

// =============================================================================
// EXPORTED HANDLERS FOR TESTING
// =============================================================================

// ListFeedbackHandler returns the listFeedback handler for testing.
func (rs *Resource) ListFeedbackHandler() http.HandlerFunc { return rs.listFeedback }

// GetFeedbackHandler returns the getFeedback handler for testing.
func (rs *Resource) GetFeedbackHandler() http.HandlerFunc { return rs.getFeedback }

// GetStudentFeedbackHandler returns the getStudentFeedback handler for testing.
func (rs *Resource) GetStudentFeedbackHandler() http.HandlerFunc { return rs.getStudentFeedback }

// GetDateFeedbackHandler returns the getDateFeedback handler for testing.
func (rs *Resource) GetDateFeedbackHandler() http.HandlerFunc { return rs.getDateFeedback }

// GetMensaFeedbackHandler returns the getMensaFeedback handler for testing.
func (rs *Resource) GetMensaFeedbackHandler() http.HandlerFunc { return rs.getMensaFeedback }

// GetDateRangeFeedbackHandler returns the getDateRangeFeedback handler for testing.
func (rs *Resource) GetDateRangeFeedbackHandler() http.HandlerFunc { return rs.getDateRangeFeedback }

// CreateFeedbackHandler returns the createFeedback handler for testing.
func (rs *Resource) CreateFeedbackHandler() http.HandlerFunc { return rs.createFeedback }

// CreateBatchFeedbackHandler returns the createBatchFeedback handler for testing.
func (rs *Resource) CreateBatchFeedbackHandler() http.HandlerFunc { return rs.createBatchFeedback }

// DeleteFeedbackHandler returns the deleteFeedback handler for testing.
func (rs *Resource) DeleteFeedbackHandler() http.HandlerFunc { return rs.deleteFeedback }
