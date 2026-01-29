package students

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// dateFormatISO is the standard date format (YYYY-MM-DD) used for pickup schedules.
const dateFormatISO = "2006-01-02"

// parseTimeOnly parses a time string (HH:MM) and returns a time.Time with a valid reference date.
// PostgreSQL TIME columns require a valid date, so we use 2000-01-01 as reference.
func parseTimeOnly(timeStr string) (time.Time, error) {
	return time.Parse("2006-01-02 15:04", "2000-01-01 "+timeStr)
}

// PickupScheduleResponse represents a pickup schedule in API responses
type PickupScheduleResponse struct {
	ID          int64   `json:"id"`
	StudentID   int64   `json:"student_id"`
	Weekday     int     `json:"weekday"`
	WeekdayName string  `json:"weekday_name"`
	PickupTime  string  `json:"pickup_time"` // HH:MM format
	Notes       *string `json:"notes,omitempty"`
	CreatedBy   int64   `json:"created_by"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// PickupExceptionResponse represents a pickup exception in API responses
type PickupExceptionResponse struct {
	ID            int64   `json:"id"`
	StudentID     int64   `json:"student_id"`
	ExceptionDate string  `json:"exception_date"` // YYYY-MM-DD format
	PickupTime    *string `json:"pickup_time,omitempty"`
	Reason        *string `json:"reason,omitempty"`
	CreatedBy     int64   `json:"created_by"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

// PickupNoteResponse represents a pickup note in API responses
type PickupNoteResponse struct {
	ID        int64  `json:"id"`
	StudentID int64  `json:"student_id"`
	NoteDate  string `json:"note_date"` // YYYY-MM-DD format
	Content   string `json:"content"`
	CreatedBy int64  `json:"created_by"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// PickupDataResponse represents combined pickup data
type PickupDataResponse struct {
	Schedules  []PickupScheduleResponse  `json:"schedules"`
	Exceptions []PickupExceptionResponse `json:"exceptions"`
	Notes      []PickupNoteResponse      `json:"notes"`
}

// PickupScheduleRequest represents a request to create/update a pickup schedule
type PickupScheduleRequest struct {
	Weekday    int     `json:"weekday"`
	PickupTime string  `json:"pickup_time"` // HH:MM format
	Notes      *string `json:"notes,omitempty"`
}

// BulkPickupScheduleRequest represents a request to update all weekly schedules
type BulkPickupScheduleRequest struct {
	Schedules []PickupScheduleRequest `json:"schedules"`
}

// PickupExceptionRequest represents a request to create/update a pickup exception
type PickupExceptionRequest struct {
	ExceptionDate string  `json:"exception_date"` // YYYY-MM-DD format
	PickupTime    *string `json:"pickup_time,omitempty"`
	Reason        *string `json:"reason,omitempty"`
}

// PickupNoteRequest represents a request to create/update a pickup note
type PickupNoteRequest struct {
	NoteDate string `json:"note_date"` // YYYY-MM-DD format
	Content  string `json:"content"`
}

// Bind implements render.Binder
func (r *PickupNoteRequest) Bind(_ *http.Request) error {
	if r.NoteDate == "" {
		return errors.New("note_date is required")
	}
	if _, err := time.Parse(dateFormatISO, r.NoteDate); err != nil {
		return errors.New("invalid note_date format, expected YYYY-MM-DD")
	}
	if r.Content == "" {
		return errors.New("content is required")
	}
	if len(r.Content) > 500 {
		return errors.New("content cannot exceed 500 characters")
	}
	return nil
}

// Bind implements render.Binder
func (r *PickupScheduleRequest) Bind(_ *http.Request) error {
	if r.Weekday < schedule.WeekdayMonday || r.Weekday > schedule.WeekdayFriday {
		return errors.New("weekday must be between 1 (Monday) and 5 (Friday)")
	}
	if r.PickupTime == "" {
		return errors.New("pickup_time is required")
	}
	if _, err := time.Parse("15:04", r.PickupTime); err != nil {
		return errors.New("invalid pickup_time format, expected HH:MM")
	}
	if r.Notes != nil && len(*r.Notes) > 500 {
		return errors.New("notes cannot exceed 500 characters")
	}
	return nil
}

// Bind implements render.Binder
func (r *BulkPickupScheduleRequest) Bind(_ *http.Request) error {
	if len(r.Schedules) == 0 {
		return errors.New("schedules array cannot be empty")
	}
	seenWeekdays := make(map[int]bool)
	for i, s := range r.Schedules {
		if s.Weekday < schedule.WeekdayMonday || s.Weekday > schedule.WeekdayFriday {
			return fmt.Errorf("schedule %d: weekday must be between 1 (Monday) and 5 (Friday)", i)
		}
		if seenWeekdays[s.Weekday] {
			return fmt.Errorf("schedule %d: duplicate weekday %d", i, s.Weekday)
		}
		seenWeekdays[s.Weekday] = true
		if s.PickupTime == "" {
			return fmt.Errorf("schedule %d: pickup_time is required", i)
		}
		if _, err := time.Parse("15:04", s.PickupTime); err != nil {
			return fmt.Errorf("schedule %d: invalid pickup_time format, expected HH:MM", i)
		}
		if s.Notes != nil && len(*s.Notes) > 500 {
			return fmt.Errorf("schedule %d: notes cannot exceed 500 characters", i)
		}
	}
	return nil
}

// Bind implements render.Binder
func (r *PickupExceptionRequest) Bind(_ *http.Request) error {
	if r.ExceptionDate == "" {
		return errors.New("exception_date is required")
	}
	if _, err := time.Parse(dateFormatISO, r.ExceptionDate); err != nil {
		return errors.New("invalid exception_date format, expected YYYY-MM-DD")
	}
	// pickup_time is optional (nil = absent/no pickup)
	// but if provided, it must be valid HH:MM format
	if r.PickupTime != nil && *r.PickupTime != "" {
		if _, err := time.Parse("15:04", *r.PickupTime); err != nil {
			return errors.New("invalid pickup_time format, expected HH:MM")
		}
	}
	if r.Reason != nil && len(*r.Reason) > 255 {
		return errors.New("reason cannot exceed 255 characters")
	}
	return nil
}

// mapScheduleToResponse converts a schedule model to API response
func mapScheduleToResponse(s *schedule.StudentPickupSchedule) PickupScheduleResponse {
	return PickupScheduleResponse{
		ID:          s.ID,
		StudentID:   s.StudentID,
		Weekday:     s.Weekday,
		WeekdayName: s.GetWeekdayName(),
		PickupTime:  s.PickupTime.Format("15:04"),
		Notes:       s.Notes,
		CreatedBy:   s.CreatedBy,
		CreatedAt:   s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   s.UpdatedAt.Format(time.RFC3339),
	}
}

// mapExceptionToResponse converts an exception model to API response
func mapExceptionToResponse(e *schedule.StudentPickupException) PickupExceptionResponse {
	resp := PickupExceptionResponse{
		ID:            e.ID,
		StudentID:     e.StudentID,
		ExceptionDate: e.ExceptionDate.Format(dateFormatISO),
		Reason:        e.Reason,
		CreatedBy:     e.CreatedBy,
		CreatedAt:     e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     e.UpdatedAt.Format(time.RFC3339),
	}
	if e.PickupTime != nil {
		formatted := e.PickupTime.Format("15:04")
		resp.PickupTime = &formatted
	}
	return resp
}

// mapNoteToResponse converts a note model to API response
func mapNoteToResponse(n *schedule.StudentPickupNote) PickupNoteResponse {
	return PickupNoteResponse{
		ID:        n.ID,
		StudentID: n.StudentID,
		NoteDate:  n.NoteDate.Format(dateFormatISO),
		Content:   n.Content,
		CreatedBy: n.CreatedBy,
		CreatedAt: n.CreatedAt.Format(time.RFC3339),
		UpdatedAt: n.UpdatedAt.Format(time.RFC3339),
	}
}

// getStaffIDFromJWT extracts the staff ID from JWT claims by looking up the person and staff
func (rs *Resource) getStaffIDFromJWT(r *http.Request) (int64, error) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		return 0, errors.New("no valid JWT claims found")
	}

	// Get person from account ID
	person, err := rs.PersonService.FindByAccountID(r.Context(), int64(claims.ID))
	if err != nil || person == nil {
		return 0, errors.New("person not found for account")
	}

	// Get staff from person ID
	staff, err := rs.PersonService.StaffRepository().FindByPersonID(r.Context(), person.ID)
	if err != nil || staff == nil {
		return 0, errors.New("user is not a staff member")
	}

	return staff.ID, nil
}

// requirePickupAccess parses the student from URL params and verifies full access.
// Returns the student on success or writes an error response and returns nil.
func (rs *Resource) requirePickupAccess(w http.ResponseWriter, r *http.Request, action string) *users.Student {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return nil
	}
	if !rs.checkStudentFullAccess(r, student) {
		renderError(w, r, ErrorForbidden(fmt.Errorf("full access required to %s", action)))
		return nil
	}
	return student
}

// parseEntityID extracts a numeric ID from a URL parameter.
// Returns 0 and writes an error response on failure.
func parseEntityID(w http.ResponseWriter, r *http.Request, param string, label string) (int64, bool) {
	idStr := chi.URLParam(r, param)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		renderError(w, r, ErrorInvalidRequest(fmt.Errorf("invalid %s ID", label)))
		return 0, false
	}
	return id, true
}

// getStudentPickupSchedules handles GET /students/{id}/pickup-schedules
func (rs *Resource) getStudentPickupSchedules(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupAccess(w, r, "view pickup schedules")
	if student == nil {
		return
	}

	data, err := rs.PickupScheduleService.GetStudentPickupData(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	response := buildPickupDataResponse(data)

	common.Respond(w, r, http.StatusOK, response, "Pickup schedules retrieved successfully")
}

// buildPickupDataResponse converts service pickup data to API response
func buildPickupDataResponse(data *scheduleService.StudentPickupData) PickupDataResponse {
	response := PickupDataResponse{
		Schedules:  make([]PickupScheduleResponse, 0, len(data.Schedules)),
		Exceptions: make([]PickupExceptionResponse, 0, len(data.Exceptions)),
		Notes:      make([]PickupNoteResponse, 0, len(data.Notes)),
	}

	for _, s := range data.Schedules {
		response.Schedules = append(response.Schedules, mapScheduleToResponse(s))
	}
	for _, e := range data.Exceptions {
		response.Exceptions = append(response.Exceptions, mapExceptionToResponse(e))
	}
	for _, n := range data.Notes {
		response.Notes = append(response.Notes, mapNoteToResponse(n))
	}

	return response
}

// updateStudentPickupSchedules handles PUT /students/{id}/pickup-schedules
func (rs *Resource) updateStudentPickupSchedules(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupAccess(w, r, "update pickup schedules")
	if student == nil {
		return
	}

	req := &BulkPickupScheduleRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get staff ID from JWT
	staffID, err := rs.getStaffIDFromJWT(r)
	if err != nil {
		renderError(w, r, ErrorForbidden(err))
		return
	}

	// Convert requests to schedule models
	schedules := make([]*schedule.StudentPickupSchedule, 0, len(req.Schedules))
	for _, s := range req.Schedules {
		pickupTime, _ := parseTimeOnly(s.PickupTime)
		schedules = append(schedules, &schedule.StudentPickupSchedule{
			StudentID:  student.ID,
			Weekday:    s.Weekday,
			PickupTime: pickupTime,
			Notes:      s.Notes,
			CreatedBy:  staffID,
		})
	}

	if err := rs.PickupScheduleService.UpsertBulkStudentPickupSchedules(r.Context(), student.ID, schedules); err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	// Fetch updated data
	data, err := rs.PickupScheduleService.GetStudentPickupData(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	response := buildPickupDataResponse(data)

	common.Respond(w, r, http.StatusOK, response, "Pickup schedules updated successfully")
}

// createStudentPickupException handles POST /students/{id}/pickup-exceptions
func (rs *Resource) createStudentPickupException(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupAccess(w, r, "create pickup exceptions")
	if student == nil {
		return
	}

	req := &PickupExceptionRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	staffID, err := rs.getStaffIDFromJWT(r)
	if err != nil {
		renderError(w, r, ErrorForbidden(err))
		return
	}

	exceptionDate, _ := time.Parse(dateFormatISO, req.ExceptionDate)
	exception := &schedule.StudentPickupException{
		StudentID:     student.ID,
		ExceptionDate: exceptionDate,
		Reason:        req.Reason,
		CreatedBy:     staffID,
	}

	if req.PickupTime != nil && *req.PickupTime != "" {
		pickupTime, _ := parseTimeOnly(*req.PickupTime)
		exception.PickupTime = &pickupTime
	}

	if err := rs.PickupScheduleService.CreateStudentPickupException(r.Context(), exception); err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, mapExceptionToResponse(exception), "Pickup exception created successfully")
}

// updateStudentPickupException handles PUT /students/{id}/pickup-exceptions/{exceptionId}
func (rs *Resource) updateStudentPickupException(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupAccess(w, r, "update pickup exceptions")
	if student == nil {
		return
	}

	exceptionID, ok := parseEntityID(w, r, "exceptionId", "exception")
	if !ok {
		return
	}

	// Verify exception exists and belongs to this student (ownership check)
	existingException, err := rs.PickupScheduleService.GetStudentPickupExceptionByID(r.Context(), exceptionID)
	if err != nil || existingException == nil {
		renderError(w, r, ErrorNotFound(errors.New("pickup exception not found")))
		return
	}
	if existingException.StudentID != student.ID {
		renderError(w, r, ErrorForbidden(errors.New("exception does not belong to this student")))
		return
	}

	req := &PickupExceptionRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	exceptionDate, _ := time.Parse(dateFormatISO, req.ExceptionDate)
	exception := &schedule.StudentPickupException{
		StudentID:     student.ID,
		ExceptionDate: exceptionDate,
		Reason:        existingException.Reason, // Preserve existing reason by default
		CreatedBy:     existingException.CreatedBy,
	}
	exception.ID = exceptionID
	exception.CreatedAt = existingException.CreatedAt // Preserve original creation timestamp

	if req.Reason != nil {
		exception.Reason = req.Reason
	}

	if req.PickupTime != nil && *req.PickupTime != "" {
		pickupTime, _ := parseTimeOnly(*req.PickupTime)
		exception.PickupTime = &pickupTime
	}

	if err := rs.PickupScheduleService.UpdateStudentPickupException(r.Context(), exception); err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, mapExceptionToResponse(exception), "Pickup exception updated successfully")
}

// deleteStudentPickupException handles DELETE /students/{id}/pickup-exceptions/{exceptionId}
func (rs *Resource) deleteStudentPickupException(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupAccess(w, r, "delete pickup exceptions")
	if student == nil {
		return
	}

	exceptionID, ok := parseEntityID(w, r, "exceptionId", "exception")
	if !ok {
		return
	}

	// Verify exception exists and belongs to this student (ownership check)
	existingException, err := rs.PickupScheduleService.GetStudentPickupExceptionByID(r.Context(), exceptionID)
	if err != nil || existingException == nil {
		renderError(w, r, ErrorNotFound(errors.New("pickup exception not found")))
		return
	}
	if existingException.StudentID != student.ID {
		renderError(w, r, ErrorForbidden(errors.New("exception does not belong to this student")))
		return
	}

	if err := rs.PickupScheduleService.DeleteStudentPickupException(r.Context(), exceptionID); err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Pickup exception deleted successfully")
}

// createStudentPickupNote handles POST /students/{id}/pickup-notes
func (rs *Resource) createStudentPickupNote(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupAccess(w, r, "create pickup notes")
	if student == nil {
		return
	}

	req := &PickupNoteRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	staffID, err := rs.getStaffIDFromJWT(r)
	if err != nil {
		renderError(w, r, ErrorForbidden(err))
		return
	}

	noteDate, _ := time.Parse(dateFormatISO, req.NoteDate)
	note := &schedule.StudentPickupNote{
		StudentID: student.ID,
		NoteDate:  noteDate,
		Content:   req.Content,
		CreatedBy: staffID,
	}

	if err := rs.PickupScheduleService.CreateStudentPickupNote(r.Context(), note); err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, mapNoteToResponse(note), "Pickup note created successfully")
}

// updateStudentPickupNote handles PUT /students/{id}/pickup-notes/{noteId}
func (rs *Resource) updateStudentPickupNote(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupAccess(w, r, "update pickup notes")
	if student == nil {
		return
	}

	noteID, ok := parseEntityID(w, r, "noteId", "note")
	if !ok {
		return
	}

	// Verify note exists and belongs to this student (ownership check)
	existingNote, err := rs.PickupScheduleService.GetStudentPickupNoteByID(r.Context(), noteID)
	if err != nil || existingNote == nil {
		renderError(w, r, ErrorNotFound(errors.New("pickup note not found")))
		return
	}
	if existingNote.StudentID != student.ID {
		renderError(w, r, ErrorForbidden(errors.New("note does not belong to this student")))
		return
	}

	req := &PickupNoteRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	noteDate, _ := time.Parse(dateFormatISO, req.NoteDate)
	note := &schedule.StudentPickupNote{
		StudentID: student.ID,
		NoteDate:  noteDate,
		Content:   req.Content,
		CreatedBy: existingNote.CreatedBy, // Preserve original creator
	}
	note.ID = noteID
	note.CreatedAt = existingNote.CreatedAt // Preserve original creation timestamp

	if err := rs.PickupScheduleService.UpdateStudentPickupNote(r.Context(), note); err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, mapNoteToResponse(note), "Pickup note updated successfully")
}

// deleteStudentPickupNote handles DELETE /students/{id}/pickup-notes/{noteId}
func (rs *Resource) deleteStudentPickupNote(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupAccess(w, r, "delete pickup notes")
	if student == nil {
		return
	}

	noteID, ok := parseEntityID(w, r, "noteId", "note")
	if !ok {
		return
	}

	// Verify note exists and belongs to this student (ownership check)
	existingNote, err := rs.PickupScheduleService.GetStudentPickupNoteByID(r.Context(), noteID)
	if err != nil || existingNote == nil {
		renderError(w, r, ErrorNotFound(errors.New("pickup note not found")))
		return
	}
	if existingNote.StudentID != student.ID {
		renderError(w, r, ErrorForbidden(errors.New("note does not belong to this student")))
		return
	}

	if err := rs.PickupScheduleService.DeleteStudentPickupNote(r.Context(), noteID); err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Pickup note deleted successfully")
}

// BulkPickupTimeRequest represents a request to get pickup times for multiple students
type BulkPickupTimeRequest struct {
	StudentIDs []int64 `json:"student_ids"`
	Date       *string `json:"date,omitempty"` // Optional date in YYYY-MM-DD format, defaults to today
}

// Bind implements render.Binder
func (r *BulkPickupTimeRequest) Bind(_ *http.Request) error {
	if len(r.StudentIDs) == 0 {
		return errors.New("student_ids array cannot be empty")
	}
	if len(r.StudentIDs) > 500 {
		return errors.New("student_ids array cannot exceed 500 items")
	}
	if r.Date != nil && *r.Date != "" {
		if _, err := time.Parse(dateFormatISO, *r.Date); err != nil {
			return errors.New("invalid date format, expected YYYY-MM-DD")
		}
	}
	return nil
}

// BulkDayNoteResponse represents a single day note in bulk pickup time responses
type BulkDayNoteResponse struct {
	ID      int64  `json:"id"`
	Content string `json:"content"`
}

// BulkPickupTimeResponse represents pickup time data for a single student
type BulkPickupTimeResponse struct {
	StudentID   int64                 `json:"student_id"`
	Date        string                `json:"date"`
	WeekdayName string                `json:"weekday_name"`
	PickupTime  *string               `json:"pickup_time,omitempty"` // HH:MM format or null
	IsException bool                  `json:"is_exception"`
	Notes       string                `json:"notes,omitempty"`
	DayNotes    []BulkDayNoteResponse `json:"day_notes,omitempty"`
}

// getBulkPickupTimes handles POST /students/pickup-times/bulk
// Returns effective pickup times for multiple students on a given date
func (rs *Resource) getBulkPickupTimes(w http.ResponseWriter, r *http.Request) {
	req := &BulkPickupTimeRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Filter student IDs to only those the user has access to
	authorizedIDs, err := rs.filterAuthorizedStudentIDs(r, req.StudentIDs)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	if len(authorizedIDs) == 0 {
		// No authorized students - return empty result
		common.Respond(w, r, http.StatusOK, []BulkPickupTimeResponse{}, "Bulk pickup times retrieved successfully")
		return
	}

	// Determine the date to query
	date := time.Now()
	if req.Date != nil && *req.Date != "" {
		parsedDate, _ := time.Parse(dateFormatISO, *req.Date) // Already validated in Bind
		date = parsedDate
	}

	// Use bulk service method (O(2) queries instead of O(N))
	pickupTimes, err := rs.PickupScheduleService.GetBulkEffectivePickupTimesForDate(r.Context(), authorizedIDs, date)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	// Convert to response format
	responses := make([]BulkPickupTimeResponse, 0, len(pickupTimes))
	for studentID, ept := range pickupTimes {
		resp := BulkPickupTimeResponse{
			StudentID:   studentID,
			Date:        ept.Date.Format(dateFormatISO),
			WeekdayName: ept.WeekdayName,
			IsException: ept.IsException,
			Notes:       ept.Notes,
		}
		if ept.PickupTime != nil {
			formatted := ept.PickupTime.Format("15:04")
			resp.PickupTime = &formatted
		}
		if len(ept.DayNotes) > 0 {
			resp.DayNotes = make([]BulkDayNoteResponse, 0, len(ept.DayNotes))
			for _, note := range ept.DayNotes {
				resp.DayNotes = append(resp.DayNotes, BulkDayNoteResponse{
					ID:      note.ID,
					Content: note.Content,
				})
			}
		}
		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Bulk pickup times retrieved successfully")
}

// Handler accessor methods for testing

// GetStudentPickupSchedulesHandler returns the handler for getting pickup schedules
func (rs *Resource) GetStudentPickupSchedulesHandler() http.HandlerFunc {
	return rs.getStudentPickupSchedules
}

// UpdateStudentPickupSchedulesHandler returns the handler for updating pickup schedules
func (rs *Resource) UpdateStudentPickupSchedulesHandler() http.HandlerFunc {
	return rs.updateStudentPickupSchedules
}

// CreateStudentPickupExceptionHandler returns the handler for creating pickup exceptions
func (rs *Resource) CreateStudentPickupExceptionHandler() http.HandlerFunc {
	return rs.createStudentPickupException
}

// UpdateStudentPickupExceptionHandler returns the handler for updating pickup exceptions
func (rs *Resource) UpdateStudentPickupExceptionHandler() http.HandlerFunc {
	return rs.updateStudentPickupException
}

// DeleteStudentPickupExceptionHandler returns the handler for deleting pickup exceptions
func (rs *Resource) DeleteStudentPickupExceptionHandler() http.HandlerFunc {
	return rs.deleteStudentPickupException
}

// GetBulkPickupTimesHandler returns the handler for getting bulk pickup times
func (rs *Resource) GetBulkPickupTimesHandler() http.HandlerFunc {
	return rs.getBulkPickupTimes
}

// CreateStudentPickupNoteHandler returns the handler for creating pickup notes
func (rs *Resource) CreateStudentPickupNoteHandler() http.HandlerFunc {
	return rs.createStudentPickupNote
}

// UpdateStudentPickupNoteHandler returns the handler for updating pickup notes
func (rs *Resource) UpdateStudentPickupNoteHandler() http.HandlerFunc {
	return rs.updateStudentPickupNote
}

// DeleteStudentPickupNoteHandler returns the handler for deleting pickup notes
func (rs *Resource) DeleteStudentPickupNoteHandler() http.HandlerFunc {
	return rs.deleteStudentPickupNote
}

// filterAuthorizedStudentIDs filters the requested student IDs to only those
// the current user has access to (admin sees all, others see only their groups' students)
func (rs *Resource) filterAuthorizedStudentIDs(r *http.Request, requestedIDs []int64) ([]int64, error) {
	userPermissions := jwt.PermissionsFromCtx(r.Context())

	// Admins have access to all students
	if hasAdminPermissions(userPermissions) {
		return requestedIDs, nil
	}

	// Get groups the user supervises
	educationGroups, err := rs.UserContextService.GetMyGroups(r.Context())
	if err != nil {
		return nil, err
	}

	if len(educationGroups) == 0 {
		return []int64{}, nil
	}

	// Extract group IDs
	groupIDs := make([]int64, 0, len(educationGroups))
	for _, group := range educationGroups {
		groupIDs = append(groupIDs, group.ID)
	}

	// Get all students in these groups
	students, err := rs.StudentRepo.FindByGroupIDs(r.Context(), groupIDs)
	if err != nil {
		return nil, err
	}

	// Build set of authorized student IDs for O(1) lookup
	authorizedSet := make(map[int64]struct{}, len(students))
	for _, student := range students {
		authorizedSet[student.ID] = struct{}{}
	}

	// Filter requested IDs to only authorized ones
	filtered := make([]int64, 0, len(requestedIDs))
	for _, id := range requestedIDs {
		if _, ok := authorizedSet[id]; ok {
			filtered = append(filtered, id)
		}
	}

	return filtered, nil
}
