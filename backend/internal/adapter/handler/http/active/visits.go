package active

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
)

// formatInt64 converts int64 to string (avoids strconv.FormatInt verbosity)
func formatInt64(i int64) string {
	return strconv.FormatInt(i, 10)
}

// ===== Visit Response Types =====

// VisitResponse represents a visit API response
type VisitResponse struct {
	ID              int64      `json:"id"`
	StudentID       int64      `json:"student_id"`
	ActiveGroupID   int64      `json:"active_group_id"`
	CheckInTime     time.Time  `json:"check_in_time"`
	CheckOutTime    *time.Time `json:"check_out_time,omitempty"`
	IsActive        bool       `json:"is_active"`
	Notes           string     `json:"notes,omitempty"`
	StudentName     string     `json:"student_name,omitempty"`
	ActiveGroupName string     `json:"active_group_name,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// VisitWithDisplayDataResponse represents a visit with student display data (optimized for bulk fetch)
type VisitWithDisplayDataResponse struct {
	ID            int64      `json:"id"`
	StudentID     int64      `json:"student_id"`
	ActiveGroupID int64      `json:"active_group_id"`
	CheckInTime   time.Time  `json:"check_in_time"`
	CheckOutTime  *time.Time `json:"check_out_time,omitempty"`
	IsActive      bool       `json:"is_active"`
	StudentName   string     `json:"student_name"`
	SchoolClass   string     `json:"school_class"`
	GroupName     string     `json:"group_name,omitempty"` // Student's OGS group
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// ===== Visit Request Types =====

// VisitRequest represents a visit creation/update request
type VisitRequest struct {
	StudentID     int64      `json:"student_id"`
	ActiveGroupID int64      `json:"active_group_id"`
	CheckInTime   time.Time  `json:"check_in_time"`
	CheckOutTime  *time.Time `json:"check_out_time,omitempty"`
	Notes         string     `json:"notes,omitempty"`
}

// Bind validates the visit request
func (req *VisitRequest) Bind(_ *http.Request) error {
	if req.StudentID <= 0 {
		return errors.New("student ID is required")
	}
	if req.ActiveGroupID <= 0 {
		return errors.New(errMsgActiveGroupIDRequired)
	}
	if req.CheckInTime.IsZero() {
		return errors.New("check-in time is required")
	}
	return nil
}

// ===== Visit Conversion Functions =====

// newVisitResponse converts a visit model to a response object
func newVisitResponse(visit *active.Visit) VisitResponse {
	response := VisitResponse{
		ID:            visit.ID,
		StudentID:     visit.StudentID,
		ActiveGroupID: visit.ActiveGroupID,
		CheckInTime:   visit.EntryTime,
		CheckOutTime:  visit.ExitTime,
		IsActive:      visit.IsActive(),
		CreatedAt:     visit.CreatedAt,
		UpdatedAt:     visit.UpdatedAt,
	}

	// Add related information if available
	if visit.Student != nil && visit.Student.Person != nil {
		response.StudentName = visit.Student.Person.GetFullName()
	}
	if visit.ActiveGroup != nil {
		response.ActiveGroupName = displayGroupPrefix + formatInt64(visit.ActiveGroup.GroupID)
	}

	return response
}

// ===== Visit Handlers =====

// listVisits handles listing all visits
func (rs *Resource) listVisits(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	queryOptions := base.NewQueryOptions()

	// Set table alias to match repository implementation
	queryOptions.Filter.WithTableAlias("visit")

	// Get active status filter
	activeStr := r.URL.Query().Get("active")
	if activeStr != "" {
		isActive := activeStr == "true" || activeStr == "1"
		if isActive {
			// For active visits, exit_time should be NULL
			queryOptions.Filter.IsNull("exit_time")
		} else {
			// For inactive visits, exit_time should NOT be NULL
			queryOptions.Filter.IsNotNull("exit_time")
		}
	}

	// Get visits
	visits, err := rs.ActiveService.ListVisits(r.Context(), queryOptions)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	responses := make([]VisitResponse, 0, len(visits))
	for _, visit := range visits {
		responses = append(responses, newVisitResponse(visit))
	}

	common.Respond(w, r, http.StatusOK, responses, "Visits retrieved successfully")
}

// getVisit handles getting a visit by ID
func (rs *Resource) getVisit(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidVisitID)))
		return
	}

	// Get visit
	visit, err := rs.ActiveService.GetVisit(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Prepare response
	response := newVisitResponse(visit)

	common.Respond(w, r, http.StatusOK, response, "Visit retrieved successfully")
}

// getStudentVisits handles getting visits for a student
func (rs *Resource) getStudentVisits(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidStudentID)))
		return
	}

	// Get visits for student
	visits, err := rs.ActiveService.FindVisitsByStudentID(r.Context(), studentID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]VisitResponse, 0, len(visits))
	for _, visit := range visits {
		responses = append(responses, newVisitResponse(visit))
	}

	common.Respond(w, r, http.StatusOK, responses, "Student visits retrieved successfully")
}

// getStudentCurrentVisit handles getting the current active visit for a student
func (rs *Resource) getStudentCurrentVisit(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidStudentID)))
		return
	}

	// Get current visit for student
	visit, err := rs.ActiveService.GetStudentCurrentVisit(r.Context(), studentID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Check if student has an active visit
	if visit == nil {
		common.Respond(w, r, http.StatusOK, nil, "Student has no active visit")
		return
	}

	// Prepare response
	response := newVisitResponse(visit)

	common.Respond(w, r, http.StatusOK, response, "Student current visit retrieved successfully")
}

// getVisitsByGroup handles getting visits for an active group
func (rs *Resource) getVisitsByGroup(w http.ResponseWriter, r *http.Request) {
	// Parse group ID from URL
	groupID, err := common.ParseIDParam(r, "groupId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidGroupID)))
		return
	}

	// Get visits for active group
	visits, err := rs.ActiveService.FindVisitsByActiveGroupID(r.Context(), groupID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]VisitResponse, 0, len(visits))
	for _, visit := range visits {
		responses = append(responses, newVisitResponse(visit))
	}

	common.Respond(w, r, http.StatusOK, responses, "Group visits retrieved successfully")
}

// createVisit handles creating a new visit
func (rs *Resource) createVisit(w http.ResponseWriter, r *http.Request) {
	event := middleware.GetWideEvent(r.Context())
	event.Action = "check_in"

	// Parse request
	req := &VisitRequest{}
	if err := render.Bind(r, req); err != nil {
		event.ErrorType = "InvalidRequest"
		event.ErrorMessage = err.Error()
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}
	event.StudentID = formatInt64(req.StudentID)
	event.GroupID = formatInt64(req.ActiveGroupID)

	// Create visit
	visit := &active.Visit{
		StudentID:     req.StudentID,
		ActiveGroupID: req.ActiveGroupID,
		EntryTime:     req.CheckInTime,
		ExitTime:      req.CheckOutTime,
	}

	// Create visit
	if err := rs.ActiveService.CreateVisit(r.Context(), visit); err != nil {
		event.ErrorType = "CreateVisitError"
		event.ErrorMessage = err.Error()
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the created visit
	createdVisit, err := rs.ActiveService.GetVisit(r.Context(), visit.ID)
	if err != nil {
		// Still return success but with the basic visit info
		response := newVisitResponse(visit)
		common.Respond(w, r, http.StatusCreated, response, "Visit created successfully")
		return
	}

	// Return the visit with all details
	response := newVisitResponse(createdVisit)
	common.Respond(w, r, http.StatusCreated, response, "Visit created successfully")
}

// updateVisit handles updating a visit
func (rs *Resource) updateVisit(w http.ResponseWriter, r *http.Request) {
	event := middleware.GetWideEvent(r.Context())
	event.Action = "visit_update"

	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		event.ErrorType = "InvalidRequest"
		event.ErrorMessage = err.Error()
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidVisitID)))
		return
	}

	// Parse request
	req := &VisitRequest{}
	if err := render.Bind(r, req); err != nil {
		event.ErrorType = "InvalidRequest"
		event.ErrorMessage = err.Error()
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}
	event.StudentID = formatInt64(req.StudentID)
	event.GroupID = formatInt64(req.ActiveGroupID)

	// Get existing visit
	existing, err := rs.ActiveService.GetVisit(r.Context(), id)
	if err != nil {
		event.ErrorType = "VisitLookupError"
		event.ErrorMessage = err.Error()
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Update fields
	existing.StudentID = req.StudentID
	existing.ActiveGroupID = req.ActiveGroupID
	existing.EntryTime = req.CheckInTime
	existing.ExitTime = req.CheckOutTime

	// Update visit
	if err := rs.ActiveService.UpdateVisit(r.Context(), existing); err != nil {
		event.ErrorType = "UpdateVisitError"
		event.ErrorMessage = err.Error()
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated visit
	updatedVisit, err := rs.ActiveService.GetVisit(r.Context(), id)
	if err != nil {
		// Still return success but with the basic visit info
		response := newVisitResponse(existing)
		common.Respond(w, r, http.StatusOK, response, "Visit updated successfully")
		return
	}

	// Return the updated visit with all details
	response := newVisitResponse(updatedVisit)
	common.Respond(w, r, http.StatusOK, response, "Visit updated successfully")
}

// deleteVisit handles deleting a visit
func (rs *Resource) deleteVisit(w http.ResponseWriter, r *http.Request) {
	event := middleware.GetWideEvent(r.Context())
	event.Action = "visit_delete"

	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		event.ErrorType = "InvalidRequest"
		event.ErrorMessage = err.Error()
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidVisitID)))
		return
	}

	// Delete visit
	if err := rs.ActiveService.DeleteVisit(r.Context(), id); err != nil {
		event.ErrorType = "DeleteVisitError"
		event.ErrorMessage = err.Error()
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Visit deleted successfully")
}

// endVisit handles ending a visit
func (rs *Resource) endVisit(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidVisitID)))
		return
	}

	// End visit
	if err := rs.ActiveService.EndVisit(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated visit
	updatedVisit, err := rs.ActiveService.GetVisit(r.Context(), id)
	if err != nil {
		common.Respond(w, r, http.StatusOK, nil, "Visit ended successfully")
		return
	}

	// Return the updated visit
	response := newVisitResponse(updatedVisit)
	common.Respond(w, r, http.StatusOK, response, "Visit ended successfully")
}
