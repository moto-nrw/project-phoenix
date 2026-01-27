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
	Reason        string  `json:"reason"`
	CreatedBy     int64   `json:"created_by"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

// PickupDataResponse represents combined pickup data
type PickupDataResponse struct {
	Schedules  []PickupScheduleResponse  `json:"schedules"`
	Exceptions []PickupExceptionResponse `json:"exceptions"`
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
	Reason        string  `json:"reason"`
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
	for i, s := range r.Schedules {
		if s.Weekday < schedule.WeekdayMonday || s.Weekday > schedule.WeekdayFriday {
			return fmt.Errorf("schedule %d: weekday must be between 1 (Monday) and 5 (Friday)", i)
		}
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
	if r.Reason == "" {
		return errors.New("reason is required")
	}
	if len(r.Reason) > 255 {
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

// getStudentPickupSchedules handles GET /students/{id}/pickup-schedules
func (rs *Resource) getStudentPickupSchedules(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// Check if user has full access (admin or supervises this student's group)
	hasFullAccess := rs.checkStudentFullAccess(r, student)
	if !hasFullAccess {
		renderError(w, r, ErrorForbidden(errors.New("full access required to view pickup schedules")))
		return
	}

	data, err := rs.PickupScheduleService.GetStudentPickupData(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	response := PickupDataResponse{
		Schedules:  make([]PickupScheduleResponse, 0, len(data.Schedules)),
		Exceptions: make([]PickupExceptionResponse, 0, len(data.Exceptions)),
	}

	for _, s := range data.Schedules {
		response.Schedules = append(response.Schedules, mapScheduleToResponse(s))
	}
	for _, e := range data.Exceptions {
		response.Exceptions = append(response.Exceptions, mapExceptionToResponse(e))
	}

	common.Respond(w, r, http.StatusOK, response, "Pickup schedules retrieved successfully")
}

// updateStudentPickupSchedules handles PUT /students/{id}/pickup-schedules
func (rs *Resource) updateStudentPickupSchedules(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	hasFullAccess := rs.checkStudentFullAccess(r, student)
	if !hasFullAccess {
		renderError(w, r, ErrorForbidden(errors.New("full access required to update pickup schedules")))
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

	response := PickupDataResponse{
		Schedules:  make([]PickupScheduleResponse, 0, len(data.Schedules)),
		Exceptions: make([]PickupExceptionResponse, 0, len(data.Exceptions)),
	}
	for _, s := range data.Schedules {
		response.Schedules = append(response.Schedules, mapScheduleToResponse(s))
	}
	for _, e := range data.Exceptions {
		response.Exceptions = append(response.Exceptions, mapExceptionToResponse(e))
	}

	common.Respond(w, r, http.StatusOK, response, "Pickup schedules updated successfully")
}

// createStudentPickupException handles POST /students/{id}/pickup-exceptions
func (rs *Resource) createStudentPickupException(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	hasFullAccess := rs.checkStudentFullAccess(r, student)
	if !hasFullAccess {
		renderError(w, r, ErrorForbidden(errors.New("full access required to create pickup exceptions")))
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
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	hasFullAccess := rs.checkStudentFullAccess(r, student)
	if !hasFullAccess {
		renderError(w, r, ErrorForbidden(errors.New("full access required to update pickup exceptions")))
		return
	}

	exceptionIDStr := chi.URLParam(r, "exceptionId")
	exceptionID, err := strconv.ParseInt(exceptionIDStr, 10, 64)
	if err != nil {
		renderError(w, r, ErrorInvalidRequest(errors.New("invalid exception ID")))
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
		Reason:        req.Reason,
		CreatedBy:     existingException.CreatedBy, // Preserve original creator
	}
	exception.ID = exceptionID
	exception.CreatedAt = existingException.CreatedAt // Preserve original creation timestamp

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
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	hasFullAccess := rs.checkStudentFullAccess(r, student)
	if !hasFullAccess {
		renderError(w, r, ErrorForbidden(errors.New("full access required to delete pickup exceptions")))
		return
	}

	exceptionIDStr := chi.URLParam(r, "exceptionId")
	exceptionID, err := strconv.ParseInt(exceptionIDStr, 10, 64)
	if err != nil {
		renderError(w, r, ErrorInvalidRequest(errors.New("invalid exception ID")))
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

// BulkPickupTimeResponse represents pickup time data for a single student
type BulkPickupTimeResponse struct {
	StudentID   int64   `json:"student_id"`
	Date        string  `json:"date"`
	WeekdayName string  `json:"weekday_name"`
	PickupTime  *string `json:"pickup_time,omitempty"` // HH:MM format or null
	IsException bool    `json:"is_exception"`
	Reason      string  `json:"reason,omitempty"`
	Notes       string  `json:"notes,omitempty"`
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
			Reason:      ept.Reason,
			Notes:       ept.Notes,
		}
		if ept.PickupTime != nil {
			formatted := ept.PickupTime.Format("15:04")
			resp.PickupTime = &formatted
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
