package simulate

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
)

// appCheckInCount is how many late children the team checks in through the
// app while a terminal is offline (#6953): all but the last go into a
// supervised room, the last into the released Sporthalle.
const appCheckInCount = 3

type appCheckInAction struct{}

func (appCheckInAction) Name() string { return "check children in through the app" }

// Run lets the last children of the day come while a terminal is offline.
// The team checks them in on the app straight into a room with a running
// supervision and into the released Sporthalle, attendance and room stay in
// one step each, so no child shows up as "Unterwegs" in between.
func (appCheckInAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 4a: A terminal is offline, the team checks children in on the app...")
	if !rt.Arrivals.planned {
		rt.Arrivals = planArrivals(rt.Children)
	}
	sessionID, roomID, err := supervisedSessionOutside(rt, rt.State.Rooms["Sporthalle"])
	if err != nil {
		return err
	}
	if sessionID == 0 {
		fmt.Println("  Skipped: no room has a running supervision")
		return nil
	}
	childIDs := rt.Arrivals.takeLast(appCheckInCount)
	if len(childIDs) == 0 {
		fmt.Println("  Skipped: no child arrives later today")
		return nil
	}

	toRoom := childIDs
	var toYard []int64
	if yard := rt.State.Rooms["Sporthalle"]; yard != 0 && len(childIDs) > 1 {
		toRoom, toYard = childIDs[:len(childIDs)-1], childIDs[len(childIDs)-1:]
	}
	if _, err := rt.Client.Post("/api/active/visits/check-in-to-group", map[string]any{
		"student_ids":            toRoom,
		"target_active_group_id": sessionID,
	}); err != nil {
		return fmt.Errorf("check children in to room %d through the app: %w", roomID, err)
	}
	rt.recordAppCheckIns(toRoom, roomID)
	if len(toYard) > 0 {
		yard := rt.State.Rooms["Sporthalle"]
		if _, err := rt.Client.Post("/api/active/visits/check-in-to-room", map[string]any{
			"student_ids":    toYard,
			"target_room_id": yard,
		}); err != nil {
			return fmt.Errorf("check a child in to the released Sporthalle through the app: %w", err)
		}
		rt.recordAppCheckIns(toYard, yard)
	}
	fmt.Printf("  %d children checked in on the app, straight into their room\n", rt.Counts.AppCheckIns)
	return nil
}

// recordAppCheckIns notes the children as present in the room, like a kiosk
// check-in would.
func (rt *Runtime) recordAppCheckIns(childIDs []int64, roomID int64) {
	for _, id := range childIDs {
		rt.markPresent(id)
		rt.Rooms.record(id, roomID)
		rt.Counts.StudentsCheckedIn++
		rt.Counts.AppCheckIns++
	}
}

// takeLast removes the n latest arrivals from the plan and returns their
// children, latest first.
func (p *arrivalPlan) takeLast(n int) []int64 {
	if n > len(p.pending) {
		n = len(p.pending)
	}
	taken := p.pending[len(p.pending)-n:]
	p.pending = p.pending[:len(p.pending)-n]
	ids := make([]int64, 0, n)
	for i := len(taken) - 1; i >= 0; i-- {
		ids = append(ids, taken[i].childID)
	}
	return ids
}

// supervisedSessionOutside returns a running kiosk session with supervision
// in a room other than skipRoomID, as the app's target list offers it.
func supervisedSessionOutside(rt *Runtime, skipRoomID int64) (int64, int64, error) {
	raw, err := rt.Client.Get("/api/active/groups?active=true")
	if err != nil {
		return 0, 0, fmt.Errorf("list running sessions: %w", err)
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, 0, fmt.Errorf("decode running sessions: %w", err)
	}
	var sessions []struct {
		ID              int64 `json:"id"`
		RoomID          int64 `json:"room_id"`
		SupervisorCount int   `json:"supervisor_count"`
	}
	// A server without running sessions may answer without a list.
	if err := json.Unmarshal(envelope.Data, &sessions); err != nil {
		return 0, 0, nil
	}
	perRoom := map[int64]int{}
	for _, session := range sessions {
		perRoom[session.RoomID]++
	}
	for _, session := range sessions {
		if session.RoomID == skipRoomID || session.SupervisorCount == 0 || perRoom[session.RoomID] != 1 {
			continue
		}
		if !slices.Contains(rt.Rooms.sessionRooms, session.RoomID) {
			continue
		}
		return session.ID, session.RoomID, nil
	}
	return 0, 0, nil
}
