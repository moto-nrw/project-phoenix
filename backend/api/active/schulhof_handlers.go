package active

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
)

// SchulhofResource handles Schulhof-specific API endpoints.
type SchulhofResource struct {
	schulhofService    facilities.SchulhofService
	userContextService usercontext.UserContextService
}

// NewSchulhofResource creates a new Schulhof resource.
func NewSchulhofResource(schulhofService facilities.SchulhofService, userContextService usercontext.UserContextService) *SchulhofResource {
	return &SchulhofResource{
		schulhofService:    schulhofService,
		userContextService: userContextService,
	}
}

// SchulhofStatusResponse represents the API response for Schulhof status.
type SchulhofStatusResponse struct {
	Exists            bool                     `json:"exists"`
	RoomID            *int64                   `json:"room_id,omitempty"`
	RoomName          string                   `json:"room_name"`
	ActivityGroupID   *int64                   `json:"activity_group_id,omitempty"`
	ActiveGroupID     *int64                   `json:"active_group_id,omitempty"`
	IsUserSupervising bool                     `json:"is_user_supervising"`
	SupervisionID     *int64                   `json:"supervision_id,omitempty"`
	SupervisorCount   int                      `json:"supervisor_count"`
	StudentCount      int                      `json:"student_count"`
	Supervisors       []SupervisorInfoResponse `json:"supervisors"`
}

// SupervisorInfoResponse represents supervisor information in API responses.
type SupervisorInfoResponse struct {
	ID            int64  `json:"id"`
	StaffID       int64  `json:"staff_id"`
	Name          string `json:"name"`
	IsCurrentUser bool   `json:"is_current_user"`
}

// ToggleSupervisionRequest represents the request body for toggling supervision.
type ToggleSupervisionRequest struct {
	Action string `json:"action"` // "start" or "stop"
}

// ToggleSupervisionResponse represents the API response for toggle supervision.
type ToggleSupervisionResponse struct {
	Action        string `json:"action"` // "started" or "stopped"
	SupervisionID *int64 `json:"supervision_id,omitempty"`
	ActiveGroupID int64  `json:"active_group_id"`
}

// getSchulhofStatus handles GET /api/active/schulhof/status
func (rs *SchulhofResource) getSchulhofStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get current staff from user context
	staff, err := rs.userContextService.GetCurrentStaff(ctx)
	if err != nil || staff == nil {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("user must be a staff member")))
		return
	}

	// Get Schulhof status
	status, err := rs.schulhofService.GetSchulhofStatus(ctx, staff.ID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("failed to get Schulhof status")))
		return
	}

	// Convert to API response
	resp := SchulhofStatusResponse{
		Exists:            status.Exists,
		RoomID:            status.RoomID,
		RoomName:          status.RoomName,
		ActivityGroupID:   status.ActivityGroupID,
		ActiveGroupID:     status.ActiveGroupID,
		IsUserSupervising: status.IsUserSupervising,
		SupervisionID:     status.SupervisionID,
		SupervisorCount:   status.SupervisorCount,
		StudentCount:      status.StudentCount,
		Supervisors:       make([]SupervisorInfoResponse, 0, len(status.Supervisors)),
	}

	for _, sup := range status.Supervisors {
		resp.Supervisors = append(resp.Supervisors, SupervisorInfoResponse{
			ID:            sup.ID,
			StaffID:       sup.StaffID,
			Name:          sup.Name,
			IsCurrentUser: sup.IsCurrentUser,
		})
	}

	common.Respond(w, r, http.StatusOK, resp, "")
}

// toggleSchulhofSupervision handles POST /api/active/schulhof/supervise
func (rs *SchulhofResource) toggleSchulhofSupervision(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get current staff from user context
	staff, err := rs.userContextService.GetCurrentStaff(ctx)
	if err != nil || staff == nil {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("user must be a staff member")))
		return
	}

	// Parse request body
	var req ToggleSupervisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}

	// Validate action
	if req.Action != "start" && req.Action != "stop" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("action must be 'start' or 'stop'")))
		return
	}

	// Toggle supervision
	result, err := rs.schulhofService.ToggleSupervision(ctx, staff.ID, req.Action)
	if err != nil {
		// Check for specific error messages
		errMsg := err.Error()
		if errMsg == "user is not currently supervising the Schulhof" {
			common.RenderError(w, r, common.ErrorConflict(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Return success response
	resp := ToggleSupervisionResponse{
		Action:        result.Action,
		SupervisionID: result.SupervisionID,
		ActiveGroupID: result.ActiveGroupID,
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, common.Response{
		Status:  "success",
		Data:    resp,
		Message: "Schulhof supervision " + result.Action + " successfully",
	})
}
