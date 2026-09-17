package simulate

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordedCall struct {
	Path string
	Body map[string]any
}

// openRoomBlockRuntime is a full-day runtime after the sessions started: two
// rooms in turn, the Sporthalle second, and five checked-in children.
func openRoomBlockRuntime(t *testing.T, now time.Time) (*Runtime, func() []recordedCall) {
	t.Helper()

	var mu sync.Mutex
	var calls []recordedCall
	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, r *simulationHTTPRequest) {
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		calls = append(calls, recordedCall{Path: r.URL.Path, Body: body})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(simulationHTTPStatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"id": "77"}})
	})
	t.Cleanup(srv.Close)

	state := &SeedState{
		BaseURL: srv.URL,
		Accounts: SeedStateAccounts{
			Betreuer: []AccountCredentials{{StaffID: 10, Name: "Julia Klein"}},
		},
		Students: []SeedStudent{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}, {ID: 6}, {ID: 7}, {ID: 8}, {ID: 9}, {ID: 10}},
		Rooms:    map[string]int64{"OGS-Raum 1": 1, "Sporthalle": 2},
	}
	rt := newRuntime(state, newClient(srv.URL, false), FullDayOptions{Now: func() time.Time { return now }})
	rt.ActiveRoomIDs = []int64{1, 2}
	return rt, func() []recordedCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]recordedCall(nil), calls...)
	}
}

// The demo runs a planned block beside the Sporthalle's independent stays, so
// the released room shows a block section on every dev machine (#3281).
func TestOpenRoomBlockAction_StartsAPlannedBlockInTheReleasedRoom(t *testing.T) {
	t.Parallel()

	wednesday := time.Date(2026, time.September, 16, 9, 0, 0, 0, time.UTC)
	rt, calls := openRoomBlockRuntime(t, wednesday)

	require.NoError(t, independentRoomStaysAction{}.Run(context.Background(), rt))
	require.NoError(t, openRoomBlockAction{}.Run(context.Background(), rt))

	got := calls()
	require.Len(t, got, 5)
	assert.Equal(t, "/api/active/visits/move-to-room", got[0].Path)
	assert.Equal(t, []any{1.0, 3.0, 5.0}, got[0].Body["student_ids"],
		"the independent stays take the first children outside the Sporthalle")

	assert.Equal(t, "/api/timetable/instances", got[1].Path)
	assert.Equal(t, map[string]any{
		"date":        "2026-09-16",
		"start_time":  "14:00",
		"end_time":    "15:30",
		"title":       "Turnen",
		"room_id":     2.0,
		"staff_ids":   []any{10.0},
		"student_ids": []any{7.0, 9.0},
	}, got[1].Body, "the block takes the next children, planned on the first caregiver")
	assert.Equal(t, "/api/timetable/instances/77/start", got[2].Path)
	assert.Equal(t, "/api/timetable/operations/instances/77/students/7/check-in", got[3].Path)
	assert.Equal(t, "/api/timetable/operations/instances/77/students/9/check-in", got[4].Path)
	assert.Equal(t, int64(77), rt.OpenRoomBlockInstanceID)
	assert.Equal(t, 2, rt.Counts.OpenRoomBlockChildren)

	require.NoError(t, completeOpenRoomBlock(rt))
	got = calls()
	assert.Equal(t, "/api/timetable/instances/77/complete", got[len(got)-1].Path)
	assert.Zero(t, rt.OpenRoomBlockInstanceID)
	require.NoError(t, completeOpenRoomBlock(rt), "a finished block is not completed twice")
	assert.Len(t, calls(), len(got))
}

func TestOpenRoomBlockAction_SkipsDaysWithoutCare(t *testing.T) {
	t.Parallel()

	// Saturday in Berlin, Friday evening in UTC.
	saturday := time.Date(2026, time.September, 18, 22, 30, 0, 0, time.UTC)
	rt, calls := openRoomBlockRuntime(t, saturday)

	require.NoError(t, openRoomBlockAction{}.Run(context.Background(), rt))

	assert.Empty(t, calls())
	assert.Zero(t, rt.OpenRoomBlockInstanceID)
}

func TestOpenRoomBlockAction_SkipsProfilesWithoutSporthalle(t *testing.T) {
	t.Parallel()

	rt, calls := openRoomBlockRuntime(t, time.Date(2026, time.September, 16, 9, 0, 0, 0, time.UTC))
	delete(rt.State.Rooms, "Sporthalle")

	require.NoError(t, openRoomBlockAction{}.Run(context.Background(), rt))

	assert.Empty(t, calls())
}
