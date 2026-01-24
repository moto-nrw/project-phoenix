package active

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
)

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
	// Parse request
	req := &VisitRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Create visit
	visit := &active.Visit{
		StudentID:     req.StudentID,
		ActiveGroupID: req.ActiveGroupID,
		EntryTime:     req.CheckInTime,
		ExitTime:      req.CheckOutTime,
	}

	// Create visit
	if err := rs.ActiveService.CreateVisit(r.Context(), visit); err != nil {
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
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidVisitID)))
		return
	}

	// Parse request
	req := &VisitRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get existing visit
	existing, err := rs.ActiveService.GetVisit(r.Context(), id)
	if err != nil {
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
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidVisitID)))
		return
	}

	// Delete visit
	if err := rs.ActiveService.DeleteVisit(r.Context(), id); err != nil {
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
