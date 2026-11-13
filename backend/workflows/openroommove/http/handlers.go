package openroommovehttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/workflows/openroommove"
)

// Resource serves the workflow while the host binds the existing presence authorization.
type Resource struct {
	OpenRoomMove openroommove.Command
	Authorize    func(http.ResponseWriter, *http.Request) (*studentpresence.StudentMoveAuthorization, bool)
	// CheckInActor resolves the staff member a check-in names and the move
	// rights the placement revalidates (#6953).
	CheckInActor        func(http.ResponseWriter, *http.Request) (int64, *studentpresence.StudentMoveAuthorization, bool)
	MarkRollback        func(context.Context)
	RenderPresenceError func(error) render.Renderer
	WebAttendance       func(context.Context) (bool, error)
}

func (rs *Resource) Register(r chi.Router, withTx common.Middleware) {
	r.With(common.RequireVisitUpdate(), withTx, common.RequireWebAttendanceEnabled(rs.WebAttendance)).Post("/visits/move-to-room", rs.moveStudentsToOpenRoom)
	r.With(common.RequireSchoolCheckin(), common.RequireVisitUpdate(), withTx, common.RequireWebAttendanceEnabled(rs.WebAttendance)).Post("/visits/check-in-to-room", rs.checkInStudentsToOpenRoom)
}

type moveStudentsToOpenRoomRequest struct {
	StudentIDs   []int64 `json:"student_ids"`
	TargetRoomID int64   `json:"target_room_id"`
}

// checkInStudentsToOpenRoomRequest adds keep_present: the check-in mode of
// „Alle Kinder“ leaves present children where they are (#6953).
type checkInStudentsToOpenRoomRequest struct {
	moveStudentsToOpenRoomRequest
	KeepPresent bool `json:"keep_present"`
}

// moveStudentsToOpenRoom records that the children now use a released room
// (#3066): an independent room stay, not participation in an activity there
// and no supervision for the caller. The caller keeps every source-side right
// of the ordinary move; the destination only has to be released.
func (rs *Resource) moveStudentsToOpenRoom(w http.ResponseWriter, r *http.Request) {
	var req moveStudentsToOpenRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if len(req.StudentIDs) == 0 || req.TargetRoomID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(openroommove.ErrInvalidMove))
		return
	}

	auth, ok := rs.Authorize(w, r)
	if !ok {
		return
	}

	result, err := rs.OpenRoomMove.MoveToOpenRoom(r.Context(), openroommove.Move{
		StudentIDs: req.StudentIDs,
		RoomID:     req.TargetRoomID,
		Actor: openroommove.Actor{
			StaffID:                      auth.StaffID,
			BypassResourceChecks:         auth.BypassResourceChecks,
			SchoolWideAttendanceEligible: auth.SchoolWideAttendanceEligible,
		},
	})
	if err != nil {
		rs.MarkRollback(r.Context())
		common.RenderError(w, r, rs.openRoomMoveErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, moveResponse(result), "Students moved successfully")
}

// checkInStudentsToOpenRoom checks children in to the school and places them
// in a released room in one unit (#6953). Children already present move with
// every source-side right of moveStudentsToOpenRoom.
func (rs *Resource) checkInStudentsToOpenRoom(w http.ResponseWriter, r *http.Request) {
	var req checkInStudentsToOpenRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if len(req.StudentIDs) == 0 || req.TargetRoomID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(openroommove.ErrInvalidMove))
		return
	}

	staffID, auth, ok := rs.CheckInActor(w, r)
	if !ok {
		return
	}

	result, err := rs.OpenRoomMove.CheckInToOpenRoom(r.Context(), openroommove.CheckIn{
		StudentIDs:  req.StudentIDs,
		RoomID:      req.TargetRoomID,
		CheckedInBy: staffID,
		KeepPresent: req.KeepPresent,
		Actor: openroommove.Actor{
			StaffID:                      auth.StaffID,
			BypassResourceChecks:         auth.BypassResourceChecks,
			SchoolWideAttendanceEligible: auth.SchoolWideAttendanceEligible,
		},
	})
	if err != nil {
		rs.MarkRollback(r.Context())
		common.RenderError(w, r, rs.openRoomMoveErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, moveResponse(result), "Students checked in successfully")
}

// moveResponse keeps the wire shape of the ordinary bulk move, so clients
// handle every result alike.
func moveResponse(result openroommove.Result) *studentpresence.StudentMoveResult {
	response := &studentpresence.StudentMoveResult{
		Moved:         nonNilIDs(result.Moved),
		Unchanged:     nonNilIDs(result.Unchanged),
		Skipped:       make([]studentpresence.StudentMoveSkipped, 0, len(result.Skipped)),
		ActiveGroupID: &result.RoomSessionID,
		RoomID:        &result.RoomID,
		CheckedIn:     result.CheckedIn,
	}
	for _, skipped := range result.Skipped {
		response.Skipped = append(response.Skipped, studentpresence.StudentMoveSkipped{StudentID: skipped.StudentID, Reason: skipped.Reason})
	}
	return response
}

// openRoomMoveErrorRenderer maps the workflow's own rejections and hands every
// Student Presence failure to the ordinary move mapping. A release removed
// after the caller loaded the room list is a conflict with current state, not
// a permission problem, so the client can explain the stale choice.
func (rs *Resource) openRoomMoveErrorRenderer(err error) render.Renderer {
	switch {
	case errors.Is(err, openroommove.ErrInvalidMove):
		return common.ErrorInvalidRequest(err)
	case errors.Is(err, openroommove.ErrRoomNotFound):
		return common.ErrorNotFound(err)
	case errors.Is(err, openroommove.ErrRoomNotReleased):
		return common.ErrorConflictWithCode(err, common.CodeRoomsNotReleased)
	}
	return rs.RenderPresenceError(err)
}

func nonNilIDs(ids []int64) []int64 {
	if ids == nil {
		return []int64{}
	}
	return ids
}
