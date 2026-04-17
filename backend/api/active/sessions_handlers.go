package active

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/active"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// StartSessionRequest is the payload for POST /api/active/sessions/start.
// Mirrors the IoT version so the frontend can share request shape.
type StartSessionRequest struct {
	ActivityID    int64   `json:"activity_id"`
	RoomID        *int64  `json:"room_id,omitempty"`
	SupervisorIDs []int64 `json:"supervisor_ids"`
	Force         bool    `json:"force,omitempty"`
}

// Bind validates the start-session payload.
func (req *StartSessionRequest) Bind(_ *http.Request) error {
	if req.ActivityID <= 0 {
		return errors.New("activity_id is required")
	}
	if len(req.SupervisorIDs) == 0 {
		return errors.New("at least one supervisor is required")
	}
	if req.RoomID != nil && *req.RoomID <= 0 {
		return errors.New("room_id must be positive when provided")
	}
	return nil
}

// SessionSupervisorResponse is the per-supervisor payload returned on start.
type SessionSupervisorResponse struct {
	StaffID int64  `json:"staff_id"`
	Role    string `json:"role,omitempty"`
}

// SessionStartConflictResponse matches the IoT session conflict envelope so the
// frontend can share the confirmation modal UX between web and kiosk flows.
type SessionStartConflictResponse struct {
	HasConflict      bool   `json:"has_conflict"`
	ConflictMessage  string `json:"conflict_message,omitempty"`
	CanOverride      bool   `json:"can_override"`
	ConflictingRoom  *int64 `json:"conflicting_room_id,omitempty"`
	ConflictingGroup *int64 `json:"conflicting_active_group_id,omitempty"`
}

// StartSessionResponse is returned on success.
type StartSessionResponse struct {
	ActiveGroupID int64                       `json:"active_group_id"`
	ActivityID    int64                       `json:"activity_id"`
	RoomID        int64                       `json:"room_id"`
	StartTime     time.Time                   `json:"start_time"`
	StartedBy     int64                       `json:"started_by"`
	Supervisors   []SessionSupervisorResponse `json:"supervisors"`
	Status        string                      `json:"status"`
	Message       string                      `json:"message"`
}

// startSession handles POST /api/active/sessions/start — web-originated (JWT)
// counterpart to the IoT /api/iot/session/start endpoint.
func (rs *Resource) startSession(w http.ResponseWriter, r *http.Request) {
	req := &StartSessionRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	staffID, err := rs.resolveStaffIDFromJWT(r)
	if err != nil {
		rs.getLogger().WarnContext(r.Context(), "session start: staff resolve failed",
			slog.String("error", err.Error()),
		)
		common.RenderError(w, r, ErrorForbidden(errors.New("only staff accounts may start activity sessions")))
		return
	}

	var group *active.Group
	if req.Force {
		group, err = rs.ActiveService.ForceStartWebActivitySession(r.Context(), req.ActivityID, staffID, req.SupervisorIDs, req.RoomID)
	} else {
		group, err = rs.ActiveService.StartWebActivitySession(r.Context(), req.ActivityID, staffID, req.SupervisorIDs, req.RoomID)
	}
	if err != nil {
		if errors.Is(err, activeSvc.ErrSessionConflict) {
			rs.respondWithConflict(w, r, req.ActivityID)
			return
		}
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	rs.respondWithStartedSession(w, r, group, staffID)
}

// respondWithConflict loads the conflicting session details and returns a 409.
func (rs *Resource) respondWithConflict(w http.ResponseWriter, r *http.Request, activityID int64) {
	info, err := rs.ActiveService.CheckWebActivityConflict(r.Context(), activityID)
	if err != nil || info == nil || !info.HasConflict {
		common.RenderError(w, r, ErrorRenderer(activeSvc.ErrSessionConflict))
		return
	}

	resp := SessionStartConflictResponse{
		HasConflict:     info.HasConflict,
		ConflictMessage: info.ConflictMessage,
		CanOverride:     info.CanOverride,
	}
	if g := info.ConflictingGroup; g != nil {
		groupID := g.ID
		resp.ConflictingGroup = &groupID
		roomID := g.RoomID
		resp.ConflictingRoom = &roomID
	}
	common.Respond(w, r, http.StatusConflict, resp, "Activity already running — pass force=true to override")
}

// respondWithStartedSession builds the success payload and responds.
func (rs *Resource) respondWithStartedSession(w http.ResponseWriter, r *http.Request, group *active.Group, staffID int64) {
	supervisors, err := rs.ActiveService.FindSupervisorsByActiveGroupID(r.Context(), group.ID)
	if err != nil {
		rs.getLogger().WarnContext(r.Context(), "session start: supervisor lookup failed",
			slog.Int64("active_group_id", group.ID),
			slog.String("error", err.Error()),
		)
	}

	supervisorPayload := make([]SessionSupervisorResponse, 0, len(supervisors))
	for _, s := range supervisors {
		if s == nil || s.EndDate != nil {
			continue
		}
		supervisorPayload = append(supervisorPayload, SessionSupervisorResponse{
			StaffID: s.StaffID,
			Role:    s.Role,
		})
	}

	resp := StartSessionResponse{
		ActiveGroupID: group.ID,
		ActivityID:    group.GroupID,
		RoomID:        group.RoomID,
		StartTime:     group.StartTime,
		StartedBy:     staffID,
		Supervisors:   supervisorPayload,
		Status:        "started",
		Message:       "Activity session started",
	}
	common.Respond(w, r, http.StatusCreated, resp, "Activity session started successfully")
}

// resolveStaffIDFromJWT resolves the staff record for the JWT-authenticated
// account. Returns an error if the account is not linked to a staff member.
func (rs *Resource) resolveStaffIDFromJWT(r *http.Request) (int64, error) {
	ctx := r.Context()
	claims, ok := ctx.Value(jwt.CtxClaims).(jwt.AppClaims)
	if !ok {
		return 0, errors.New("missing JWT claims")
	}

	accountID := int64(claims.ID)
	if accountID <= 0 {
		return 0, errors.New("invalid account id")
	}

	person, err := rs.PersonService.FindByAccountID(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if person == nil {
		return 0, errors.New("no person linked to account")
	}

	staff, err := rs.PersonService.StaffRepository().FindByPersonID(ctx, person.ID)
	if err != nil {
		return 0, err
	}
	if staff == nil {
		return 0, errors.New("account is not a staff member")
	}
	return staff.ID, nil
}
