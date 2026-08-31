package simulate

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// randomFromSet Tests
// =============================================================================

func TestRandomFromSet_EmptySet(t *testing.T) {
	t.Parallel()

	assert.Equal(t, int64(0), randomFromSet(map[int64]bool{}))
}

func TestRandomFromSet_NilSet(t *testing.T) {
	t.Parallel()

	assert.Equal(t, int64(0), randomFromSet(nil))
}

func TestRandomFromSet_SingleElement(t *testing.T) {
	t.Parallel()

	set := map[int64]bool{42: true}
	assert.Equal(t, int64(42), randomFromSet(set))
}

func TestRandomFromSet_MultipleElements(t *testing.T) {
	t.Parallel()

	set := map[int64]bool{1: true, 2: true, 3: true}
	result := randomFromSet(set)
	assert.True(t, result >= 1 && result <= 3)
}

// =============================================================================
// findStudent Tests
// =============================================================================

func TestFindStudent_Found(t *testing.T) {
	t.Parallel()

	students := []SeedStudent{
		{ID: 1, FirstName: "Felix", LastName: "Schneider"},
		{ID: 2, FirstName: "Emma", LastName: "Meyer"},
	}
	s := findStudent(students, 2)
	assert.Equal(t, "Emma", s.FirstName)
	assert.Equal(t, "Meyer", s.LastName)
}

func TestFindStudent_NotFound(t *testing.T) {
	t.Parallel()

	students := []SeedStudent{
		{ID: 1, FirstName: "Felix", LastName: "Schneider"},
	}
	s := findStudent(students, 99)
	assert.Equal(t, int64(99), s.ID)
	assert.Equal(t, "Unknown", s.FirstName)
	assert.Equal(t, "Student", s.LastName)
}

func TestFindStudent_EmptySlice(t *testing.T) {
	t.Parallel()

	s := findStudent(nil, 1)
	assert.Equal(t, "Unknown", s.FirstName)
}

// =============================================================================
// roomNameByID Tests
// =============================================================================

func TestRoomNameByID_Found(t *testing.T) {
	t.Parallel()

	rooms := map[string]int64{"Sporthalle": 20, "Mensa": 30}
	assert.Equal(t, "Sporthalle", roomNameByID(rooms, 20))
}

func TestRoomNameByID_NotFound(t *testing.T) {
	t.Parallel()

	rooms := map[string]int64{"Sporthalle": 20}
	assert.Equal(t, "room-99", roomNameByID(rooms, 99))
}

func TestRoomNameByID_EmptyMap(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "room-1", roomNameByID(map[string]int64{}, 1))
}

// =============================================================================
// printLiveSummary Tests
// =============================================================================

func TestPrintLiveSummary_ZeroCounts(t *testing.T) {
	t.Parallel()

	// Should not panic
	printLiveSummary(&liveCounts{})
}

func TestPrintLiveSummary_WithCounts(t *testing.T) {
	t.Parallel()

	counts := &liveCounts{
		roomMoves:  10,
		unterwegs:  5,
		returns:    8,
		sickToggle: 3,
		errors:     1,
	}
	// Should not panic
	printLiveSummary(counts)
}

// =============================================================================
// bootstrapLiveState Tests
// =============================================================================

func TestBootstrapLiveState_Success(t *testing.T) {
	t.Parallel()

	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, r *simulationHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "jwt"})
		case "/api/active/visits":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"student_id": 1},
					{"student_id": 3},
				},
			})
		default:
			w.WriteHeader(simulationHTTPStatusOK)
			_, _ = fmt.Fprint(w, `{}`)
		}
	})
	defer srv.Close()

	client := newClient(srv.URL, false)
	client.BindAuth(simulationTestAuth{Kind: simulationTestAuthBearer, Label: "tenant", Token: "jwt"})

	ls := &liveState{
		checkedIn: make(map[int64]bool),
		unterwegs: make(map[int64]bool),
		sick:      make(map[int64]bool),
		rfidTags:  map[int64]string{1: "DE000001", 2: "DE000002", 3: "DE000003"},
		roomIDs:   []int64{10, 20},
	}

	students := []SeedStudent{
		{ID: 1, FirstName: "A", LastName: "B"},
		{ID: 2, FirstName: "C", LastName: "D"},
		{ID: 3, FirstName: "E", LastName: "F"},
	}

	err := bootstrapLiveState(client, ls, students)
	require.NoError(t, err)
	assert.True(t, ls.checkedIn[1])
	assert.False(t, ls.checkedIn[2]) // not in visits response
	assert.True(t, ls.checkedIn[3])
}

func TestBootstrapLiveState_NoRFID(t *testing.T) {
	t.Parallel()

	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, _ *simulationHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"student_id": 1}},
		})
	})
	defer srv.Close()

	client := newClient(srv.URL, false)
	client.BindAuth(simulationTestAuth{Kind: simulationTestAuthBearer, Label: "tenant", Token: "jwt"})

	ls := &liveState{
		checkedIn: make(map[int64]bool),
		unterwegs: make(map[int64]bool),
		sick:      make(map[int64]bool),
		rfidTags:  map[int64]string{}, // no RFID tags
	}

	err := bootstrapLiveState(client, ls, []SeedStudent{{ID: 1}})
	require.NoError(t, err)
	assert.Empty(t, ls.checkedIn) // skipped because no RFID
}

func TestBootstrapLiveState_ServerError(t *testing.T) {
	t.Parallel()

	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, _ *simulationHTTPRequest) {
		w.WriteHeader(simulationHTTPStatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error":"db fail"}`)
	})
	defer srv.Close()

	client := newClient(srv.URL, false)
	client.BindAuth(simulationTestAuth{Kind: simulationTestAuthBearer, Label: "tenant", Token: "jwt"})

	ls := &liveState{
		checkedIn: make(map[int64]bool),
		rfidTags:  map[int64]string{1: "DE000001"},
	}

	err := bootstrapLiveState(client, ls, []SeedStudent{{ID: 1}})
	assert.Error(t, err)
}

func TestBootstrapLiveState_InvalidJSON(t *testing.T) {
	t.Parallel()

	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, _ *simulationHTTPRequest) {
		w.WriteHeader(simulationHTTPStatusOK)
		_, _ = fmt.Fprint(w, `not json`)
	})
	defer srv.Close()

	client := newClient(srv.URL, false)
	client.BindAuth(simulationTestAuth{Kind: simulationTestAuthBearer, Label: "tenant", Token: "jwt"})

	ls := &liveState{
		checkedIn: make(map[int64]bool),
		rfidTags:  map[int64]string{1: "DE000001"},
	}

	err := bootstrapLiveState(client, ls, []SeedStudent{{ID: 1}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse visits")
}

// =============================================================================
// liveRoomMove Tests
// =============================================================================

func TestLiveRoomMove_Success(t *testing.T) {
	t.Parallel()

	srv := liveAPIMock(t)
	defer srv.Close()

	client := newClient(srv.URL, false)
	client.BindAuth(simulationTestAuth{Kind: simulationTestAuthBearer, Label: "tenant", Token: "jwt"})

	state := minimalLiveState(srv.URL)
	ls := &liveState{
		checkedIn: map[int64]bool{1: true},
		unterwegs: make(map[int64]bool),
		sick:      make(map[int64]bool),
		rfidTags:  map[int64]string{1: "DE000001"},
		roomIDs:   []int64{10, 20},
	}

	err := liveRoomMove(client, ls, state, state.Devices["d1"], "12:00:00")
	assert.NoError(t, err)
	// Student should still be checked in (room move = checkout + checkin)
	assert.True(t, ls.checkedIn[1])
}

func TestLiveRoomMove_NoCheckedIn(t *testing.T) {
	t.Parallel()

	client := newClient("http://localhost:1", false)
	state := minimalLiveState("http://localhost:1")
	ls := &liveState{
		checkedIn: map[int64]bool{},
		rfidTags:  map[int64]string{},
		roomIDs:   []int64{10},
	}

	err := liveRoomMove(client, ls, state, SeedDevice{}, "12:00:00")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no checked-in students")
}

// =============================================================================
// liveGoUnterwegs Tests
// =============================================================================

func TestLiveGoUnterwegs_Success(t *testing.T) {
	t.Parallel()

	srv := liveAPIMock(t)
	defer srv.Close()

	client := newClient(srv.URL, false)
	client.BindAuth(simulationTestAuth{Kind: simulationTestAuthBearer, Label: "tenant", Token: "jwt"})

	state := minimalLiveState(srv.URL)
	ls := &liveState{
		checkedIn: map[int64]bool{1: true},
		unterwegs: make(map[int64]bool),
		sick:      make(map[int64]bool),
		rfidTags:  map[int64]string{1: "DE000001"},
		roomIDs:   []int64{10},
	}

	err := liveGoUnterwegs(client, ls, state, state.Devices["d1"], "12:00:00")
	assert.NoError(t, err)
	assert.False(t, ls.checkedIn[1])
	assert.True(t, ls.unterwegs[1])
}

func TestLiveGoUnterwegs_NoCheckedIn(t *testing.T) {
	t.Parallel()

	client := newClient("http://localhost:1", false)
	state := minimalLiveState("http://localhost:1")
	ls := &liveState{
		checkedIn: map[int64]bool{},
		rfidTags:  map[int64]string{},
	}

	err := liveGoUnterwegs(client, ls, state, SeedDevice{}, "12:00:00")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no checked-in students")
}

// =============================================================================
// liveReturnFromUnterwegs Tests
// =============================================================================

func TestLiveReturnFromUnterwegs_Success(t *testing.T) {
	t.Parallel()

	srv := liveAPIMock(t)
	defer srv.Close()

	client := newClient(srv.URL, false)
	client.BindAuth(simulationTestAuth{Kind: simulationTestAuthBearer, Label: "tenant", Token: "jwt"})

	state := minimalLiveState(srv.URL)
	ls := &liveState{
		checkedIn: make(map[int64]bool),
		unterwegs: map[int64]bool{1: true},
		sick:      make(map[int64]bool),
		rfidTags:  map[int64]string{1: "DE000001"},
		roomIDs:   []int64{10},
	}

	err := liveReturnFromUnterwegs(client, ls, state, state.Devices["d1"], "12:00:00")
	assert.NoError(t, err)
	assert.True(t, ls.checkedIn[1])
	assert.False(t, ls.unterwegs[1])
}

func TestLiveReturnFromUnterwegs_NobodyUnterwegs_FallsBackToRoomMove(t *testing.T) {
	t.Parallel()

	srv := liveAPIMock(t)
	defer srv.Close()

	client := newClient(srv.URL, false)
	client.BindAuth(simulationTestAuth{Kind: simulationTestAuthBearer, Label: "tenant", Token: "jwt"})

	state := minimalLiveState(srv.URL)
	ls := &liveState{
		checkedIn: map[int64]bool{1: true},
		unterwegs: map[int64]bool{},
		sick:      make(map[int64]bool),
		rfidTags:  map[int64]string{1: "DE000001"},
		roomIDs:   []int64{10},
	}

	// Falls back to room move when nobody is unterwegs
	err := liveReturnFromUnterwegs(client, ls, state, state.Devices["d1"], "12:00:00")
	assert.NoError(t, err)
}

// =============================================================================
// liveToggleSick Tests
// =============================================================================

func TestLiveToggleSick_MarkSick(t *testing.T) {
	t.Parallel()

	srv := liveAPIMock(t)
	defer srv.Close()

	client := newClient(srv.URL, false)
	client.BindAuth(simulationTestAuth{Kind: simulationTestAuthBearer, Label: "tenant", Token: "jwt"})

	state := minimalLiveState(srv.URL)
	ls := &liveState{
		checkedIn: make(map[int64]bool),
		unterwegs: make(map[int64]bool),
		sick:      make(map[int64]bool),
		rfidTags:  map[int64]string{1: "DE000001"},
	}

	err := liveToggleSick(client, ls, state, "12:00:00")
	assert.NoError(t, err)
	// Student should be toggled to sick (was not sick)
	assert.True(t, ls.sick[state.Students[0].ID])
}

func TestLiveToggleSick_MarkHealthy(t *testing.T) {
	t.Parallel()

	srv := liveAPIMock(t)
	defer srv.Close()

	client := newClient(srv.URL, false)
	client.BindAuth(simulationTestAuth{Kind: simulationTestAuthBearer, Label: "tenant", Token: "jwt"})

	state := minimalLiveState(srv.URL)
	ls := &liveState{
		sick: map[int64]bool{1: true}, // already sick
	}

	err := liveToggleSick(client, ls, state, "12:00:00")
	assert.NoError(t, err)
	assert.False(t, ls.sick[1]) // toggled back to healthy
}

func TestLiveToggleSick_NoStudents(t *testing.T) {
	t.Parallel()

	client := newClient("http://localhost:1", false)
	state := &SeedState{Students: nil}
	ls := &liveState{sick: make(map[int64]bool)}

	err := liveToggleSick(client, ls, state, "12:00:00")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no students")
}

func TestLiveToggleSick_ServerError(t *testing.T) {
	t.Parallel()

	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, _ *simulationHTTPRequest) {
		w.WriteHeader(simulationHTTPStatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error":"fail"}`)
	})
	defer srv.Close()

	client := newClient(srv.URL, false)
	client.BindAuth(simulationTestAuth{Kind: simulationTestAuthBearer, Label: "tenant", Token: "jwt"})

	state := minimalLiveState(srv.URL)
	ls := &liveState{sick: make(map[int64]bool)}

	err := liveToggleSick(client, ls, state, "12:00:00")
	assert.Error(t, err)
}

// =============================================================================
// liveSchulhofRotate Tests
// =============================================================================

func TestLiveSchulhofRotate_FullCycle(t *testing.T) {
	t.Parallel()

	srv := liveAPIMock(t)
	defer srv.Close()

	client := newClient(srv.URL, false)
	client.BindAuth(simulationTestAuth{Kind: simulationTestAuthBearer, Label: "tenant", Token: "jwt"})

	state := minimalLiveState(srv.URL)
	ls := &liveState{
		checkedIn:      map[int64]bool{1: true},
		unterwegs:      make(map[int64]bool),
		sick:           make(map[int64]bool),
		rfidTags:       map[int64]string{1: "DE000001"},
		roomIDs:        []int64{10, 20},
		rotation:       map[int64]*rotationState{1: {phase: phaseHeimatraum, agHopTarget: 1}},
		schulhofRoomID: 99,
	}

	// Heimatraum → AG
	err := liveSchulhofRotate(client, ls, state, state.Devices["d1"], "12:00:00")
	assert.NoError(t, err)
	rot := ls.rotation[1]
	assert.Equal(t, phaseAG, rot.phase)
	assert.Equal(t, 1, rot.agHops)
	assert.True(t, rot.cooldownUntil.After(time.Now()))

	// AG (target reached) → Schulhof
	rot.cooldownUntil = time.Time{}
	err = liveSchulhofRotate(client, ls, state, state.Devices["d1"], "12:00:01")
	assert.NoError(t, err)
	assert.Equal(t, phaseSchulhof, rot.phase)

	// Schulhof → Heimatraum, hop counters reset
	rot.cooldownUntil = time.Time{}
	err = liveSchulhofRotate(client, ls, state, state.Devices["d1"], "12:00:02")
	assert.NoError(t, err)
	assert.Equal(t, phaseHeimatraum, rot.phase)
	assert.Equal(t, 0, rot.agHops)
	assert.GreaterOrEqual(t, rot.agHopTarget, 1)
	assert.LessOrEqual(t, rot.agHopTarget, 2)
	assert.True(t, ls.checkedIn[1])
}

func TestLiveSchulhofRotate_CooldownSkips(t *testing.T) {
	t.Parallel()

	client := newClient("http://localhost:1", false)
	state := minimalLiveState("http://localhost:1")
	ls := &liveState{
		checkedIn: map[int64]bool{1: true},
		rfidTags:  map[int64]string{1: "DE000001"},
		roomIDs:   []int64{10},
		rotation: map[int64]*rotationState{
			1: {phase: phaseAG, cooldownUntil: time.Now().Add(time.Minute)},
		},
	}

	err := liveSchulhofRotate(client, ls, state, SeedDevice{}, "12:00:00")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no rotation-eligible students")
}

func TestLiveSchulhofRotate_ServerError(t *testing.T) {
	t.Parallel()

	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, _ *simulationHTTPRequest) {
		w.WriteHeader(simulationHTTPStatusInternalServerError)
	})
	defer srv.Close()

	client := newClient(srv.URL, false)
	state := minimalLiveState(srv.URL)
	ls := &liveState{
		checkedIn: map[int64]bool{1: true},
		unterwegs: make(map[int64]bool),
		rfidTags:  map[int64]string{1: "DE000001"},
		roomIDs:   []int64{10},
	}

	err := liveSchulhofRotate(client, ls, state, state.Devices["d1"], "12:00:00")
	assert.Error(t, err)
	// Failed checkout still stamps the cooldown
	assert.True(t, ls.rotation[1].cooldownUntil.After(time.Now()))
}

// =============================================================================
// liveAttendanceToggle Tests
// =============================================================================

func TestLiveAttendanceToggle_Success(t *testing.T) {
	t.Parallel()

	srv := liveAPIMock(t)
	defer srv.Close()

	client := newClient(srv.URL, false)
	client.BindAuth(simulationTestAuth{Kind: simulationTestAuthBearer, Label: "tenant", Token: "jwt"})

	state := minimalLiveState(srv.URL)
	ls := &liveState{
		rfidTags: map[int64]string{1: "DE000001"},
		interval: 10 * time.Second,
	}

	err := liveAttendanceToggle(client, ls, state, state.Devices["d1"], "12:00:00")
	assert.NoError(t, err)
	assert.False(t, ls.lastAttendance[1].IsZero())
}

func TestLiveAttendanceToggle_CooldownSkips(t *testing.T) {
	t.Parallel()

	client := newClient("http://localhost:1", false)
	state := minimalLiveState("http://localhost:1")
	ls := &liveState{
		rfidTags:       map[int64]string{1: "DE000001"},
		lastAttendance: map[int64]time.Time{1: time.Now()},
		interval:       10 * time.Second,
	}

	err := liveAttendanceToggle(client, ls, state, SeedDevice{}, "12:00:00")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no attendance-eligible students")
}

func TestLiveAttendanceToggle_ServerError(t *testing.T) {
	t.Parallel()

	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, _ *simulationHTTPRequest) {
		w.WriteHeader(simulationHTTPStatusInternalServerError)
	})
	defer srv.Close()

	client := newClient(srv.URL, false)
	state := minimalLiveState(srv.URL)
	ls := &liveState{
		rfidTags: map[int64]string{1: "DE000001"},
		interval: 10 * time.Second,
	}

	err := liveAttendanceToggle(client, ls, state, state.Devices["d1"], "12:00:00")
	assert.Error(t, err)
	// Failed toggle still stamps the cooldown so the student is not retried immediately
	assert.False(t, ls.lastAttendance[1].IsZero())
}

// =============================================================================
// liveSupervisorSwap Tests
// =============================================================================

func TestLiveSupervisorSwap_Success(t *testing.T) {
	t.Parallel()

	srv := liveAPIMock(t)
	defer srv.Close()

	client := newClient(srv.URL, false)
	client.BindAuth(simulationTestAuth{Kind: simulationTestAuthBearer, Label: "tenant", Token: "jwt"})

	state := minimalLiveState(srv.URL)
	ls := &liveState{staffIDs: []int64{7}}

	err := liveSupervisorSwap(client, ls, state, state.Devices["d1"], "12:00:00")
	assert.NoError(t, err)
	assert.EqualValues(t, 5, ls.sessionID)
}

func TestLiveSupervisorSwap_NoStaff(t *testing.T) {
	t.Parallel()

	client := newClient("http://localhost:1", false)
	state := minimalLiveState("http://localhost:1")
	ls := &liveState{}

	err := liveSupervisorSwap(client, ls, state, SeedDevice{}, "12:00:00")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no staff IDs")
}

func TestLiveSupervisorSwap_NoActiveSession(t *testing.T) {
	t.Parallel()

	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, r *simulationHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"is_active": false},
		})
	})
	defer srv.Close()

	client := newClient(srv.URL, false)
	state := minimalLiveState(srv.URL)
	ls := &liveState{staffIDs: []int64{7}}

	err := liveSupervisorSwap(client, ls, state, state.Devices["d1"], "12:00:00")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
	assert.Equal(t, int64(0), ls.sessionID)
}

func TestLiveSupervisorSwap_PutErrorResetsSession(t *testing.T) {
	t.Parallel()

	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, r *simulationHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "PUT" {
			w.WriteHeader(simulationHTTPStatusConflict)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"active_group_id": 5, "is_active": true},
		})
	})
	defer srv.Close()

	client := newClient(srv.URL, false)
	state := minimalLiveState(srv.URL)
	ls := &liveState{staffIDs: []int64{7}, sessionID: 5}

	err := liveSupervisorSwap(client, ls, state, state.Devices["d1"], "12:00:00")
	assert.Error(t, err)
	// Session may have ended — the next attempt must re-discover it
	assert.Equal(t, int64(0), ls.sessionID)
}

// =============================================================================
// runLiveTick Tests
// =============================================================================

func TestRunLiveTick_IncrementsCounts(t *testing.T) {
	t.Parallel()

	srv := liveAPIMock(t)
	defer srv.Close()

	client := newClient(srv.URL, false)
	client.BindAuth(simulationTestAuth{Kind: simulationTestAuthBearer, Label: "tenant", Token: "jwt"})

	state := minimalLiveState(srv.URL)
	ls := &liveState{
		checkedIn: map[int64]bool{1: true},
		unterwegs: make(map[int64]bool),
		sick:      make(map[int64]bool),
		rfidTags:  map[int64]string{1: "DE000001"},
		roomIDs:   []int64{10, 20},
	}

	counts := &liveCounts{}
	// Run multiple ticks to exercise different random branches
	for i := 0; i < 20; i++ {
		// Reset state to ensure there's always a checked-in student
		ls.checkedIn[1] = true
		runLiveTick(client, ls, state, state.Devices["d1"], counts)
	}

	total := counts.roomMoves + counts.unterwegs + counts.returns + counts.sickToggle + counts.errors
	assert.Greater(t, total, 0)
}

// =============================================================================
// RunLive Integration Tests
// =============================================================================

func TestRunLive_InvalidStatePath(t *testing.T) {
	t.Parallel()

	err := RunLive(context.Background(), LiveOptions{Client: newTestClientFactory, StatePath: "/nonexistent/state.json"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "load seed state")
}

func TestRunLive_NoAdminAccounts(t *testing.T) {
	t.Parallel()

	srv := liveAPIMock(t)
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	state := &SeedState{
		BaseURL:  srv.URL,
		Accounts: SeedStateAccounts{},
	}
	require.NoError(t, WriteSeedState(state, statePath))

	err := RunLive(context.Background(), LiveOptions{Client: newTestClientFactory, StatePath: statePath, Interval: time.Second})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no admin accounts")
}

func TestRunLive_LoginFails(t *testing.T) {
	t.Parallel()

	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, r *simulationHTTPRequest) {
		switch r.URL.Path {
		case "/auth/login":
			w.WriteHeader(simulationHTTPStatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"error":"bad"}`)
		default:
			w.WriteHeader(simulationHTTPStatusOK)
		}
	})
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	state := &SeedState{
		BaseURL:  srv.URL,
		Accounts: SeedStateAccounts{Admin: []AccountCredentials{{Email: "a@t.de", Password: "p"}}},
	}
	require.NoError(t, WriteSeedState(state, statePath))

	err := RunLive(context.Background(), LiveOptions{Client: newTestClientFactory, StatePath: statePath, Interval: time.Second})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "admin login")
}

func TestRunLive_NoDevices(t *testing.T) {
	t.Parallel()

	srv := liveAPIMock(t)
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	state := &SeedState{
		BaseURL:   srv.URL,
		Accounts:  SeedStateAccounts{Admin: []AccountCredentials{{Email: "a@t.de", Password: "p"}}},
		Devices:   map[string]SeedDevice{},
		DevicePIN: "1234",
	}
	require.NoError(t, WriteSeedState(state, statePath))

	err := RunLive(context.Background(), LiveOptions{Client: newTestClientFactory, StatePath: statePath, Interval: time.Second})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no devices")
}

func TestRunLive_NoRooms(t *testing.T) {
	t.Parallel()

	srv := liveAPIMock(t)
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	state := &SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Accounts:  SeedStateAccounts{Admin: []AccountCredentials{{Email: "a@t.de", Password: "p"}}},
		Devices:   map[string]SeedDevice{"d1": {APIKey: "k1"}},
		Students:  []SeedStudent{{ID: 1, FirstName: "F", LastName: "L"}},
		Rooms:     map[string]int64{},
	}
	require.NoError(t, WriteSeedState(state, statePath))

	err := RunLive(context.Background(), LiveOptions{Client: newTestClientFactory, StatePath: statePath, Interval: time.Second})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no rooms")
}

func TestRunLive_RejectsNonPositiveInterval(t *testing.T) {
	t.Parallel()

	srv := liveAPIMock(t)
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	state := &SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Accounts:  SeedStateAccounts{Admin: []AccountCredentials{{Email: "a@t.de", Password: "p"}}},
		Devices:   map[string]SeedDevice{"d1": {APIKey: "k1"}},
		Students:  []SeedStudent{{ID: 1, FirstName: "F", LastName: "L"}},
		Rooms:     map[string]int64{"OGS-Raum 1": 10},
	}
	require.NoError(t, WriteSeedState(state, statePath))

	err := RunLive(context.Background(), LiveOptions{Client: newTestClientFactory,
		StatePath: statePath,
		Interval:  0,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interval must be greater than 0")
}

func TestRunLive_NoActiveVisits(t *testing.T) {
	t.Parallel()

	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, r *simulationHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "jwt"})
		case "/api/active/visits":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		default:
			w.WriteHeader(simulationHTTPStatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success"})
		}
	})
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	state := &SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Accounts:  SeedStateAccounts{Admin: []AccountCredentials{{Email: "a@t.de", Password: "p"}}},
		Devices:   map[string]SeedDevice{"d1": {APIKey: "k1"}},
		Students:  []SeedStudent{{ID: 1, FirstName: "F", LastName: "L"}},
		Rooms:     map[string]int64{"OGS-Raum 1": 10},
	}
	require.NoError(t, WriteSeedState(state, statePath))

	err := RunLive(context.Background(), LiveOptions{Client: newTestClientFactory,
		StatePath: statePath,
		Interval:  time.Second,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active visits found")
}

func TestRunLive_CancelledContext(t *testing.T) {
	t.Parallel()

	srv := liveAPIMock(t)
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	state := &SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Accounts:  SeedStateAccounts{Admin: []AccountCredentials{{Email: "a@t.de", Password: "p"}}},
		Devices:   map[string]SeedDevice{"d1": {APIKey: "k1"}},
		Students:  []SeedStudent{{ID: 1, FirstName: "F", LastName: "L"}},
		Rooms:     map[string]int64{"OGS-Raum 1": 10},
	}
	require.NoError(t, WriteSeedState(state, statePath))

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the loop exits on first select
	cancel()

	err := RunLive(ctx, LiveOptions{Client: newTestClientFactory,
		StatePath: statePath,
		Interval:  1 * time.Second,
		Verbose:   false,
	})
	assert.NoError(t, err) // graceful shutdown returns nil
}

func TestRunLive_RunsTicksBeforeCancel(t *testing.T) {
	t.Parallel()

	srv := liveAPIMock(t)
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	state := &SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Accounts:  SeedStateAccounts{Admin: []AccountCredentials{{Email: "a@t.de", Password: "p"}}},
		Devices:   map[string]SeedDevice{"d1": {APIKey: "k1"}},
		Students:  []SeedStudent{{ID: 1, FirstName: "Felix", LastName: "S"}},
		Rooms:     map[string]int64{"OGS-Raum 1": 10},
	}
	require.NoError(t, WriteSeedState(state, statePath))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := RunLive(ctx, LiveOptions{Client: newTestClientFactory,
		StatePath: statePath,
		Interval:  50 * time.Millisecond,
		Verbose:   true,
	})
	assert.NoError(t, err)
}

func TestRunLive_BootstrapFallback(t *testing.T) {
	t.Parallel()

	// Server that fails on /api/active/visits so bootstrap falls back
	callCount := 0
	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, r *simulationHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "jwt"})
		case "/api/active/visits":
			if callCount == 0 {
				callCount++
				w.WriteHeader(simulationHTTPStatusInternalServerError)
				_, _ = fmt.Fprint(w, `{"error":"db"}`)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		default:
			w.WriteHeader(simulationHTTPStatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success"})
		}
	})
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	state := &SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Accounts:  SeedStateAccounts{Admin: []AccountCredentials{{Email: "a@t.de", Password: "p"}}},
		Devices:   map[string]SeedDevice{"d1": {APIKey: "k1"}},
		Students:  []SeedStudent{{ID: 1, FirstName: "F", LastName: "L"}},
		Rooms:     map[string]int64{"OGS-Raum 1": 10},
	}
	require.NoError(t, WriteSeedState(state, statePath))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := RunLive(ctx, LiveOptions{Client: newTestClientFactory,
		StatePath: statePath,
		Interval:  1 * time.Second,
	})
	assert.NoError(t, err)
}

// =============================================================================
// LiveOptions Tests
// =============================================================================

func TestLiveOptions_Fields(t *testing.T) {
	t.Parallel()

	opts := LiveOptions{Client: newTestClientFactory,
		StatePath: "/tmp/state.json",
		Interval:  5 * time.Second,
		Verbose:   true,
	}
	assert.Equal(t, "/tmp/state.json", opts.StatePath)
	assert.Equal(t, 5*time.Second, opts.Interval)
	assert.True(t, opts.Verbose)
}

// =============================================================================
// liveCounts Tests
// =============================================================================

func TestLiveCounts_Defaults(t *testing.T) {
	t.Parallel()

	c := &liveCounts{}
	assert.Equal(t, 0, c.roomMoves)
	assert.Equal(t, 0, c.unterwegs)
	assert.Equal(t, 0, c.returns)
	assert.Equal(t, 0, c.sickToggle)
	assert.Equal(t, 0, c.errors)
}

// =============================================================================
// liveState Tests
// =============================================================================

func TestLiveState_Initialization(t *testing.T) {
	t.Parallel()

	ls := &liveState{
		checkedIn: make(map[int64]bool),
		unterwegs: make(map[int64]bool),
		sick:      make(map[int64]bool),
		rfidTags:  make(map[int64]string),
		roomIDs:   []int64{10, 20, 30},
	}
	assert.Empty(t, ls.checkedIn)
	assert.Empty(t, ls.unterwegs)
	assert.Empty(t, ls.sick)
	assert.Len(t, ls.roomIDs, 3)
}

// =============================================================================
// Helpers
// =============================================================================

func liveAPIMock(t *testing.T) *simulationHTTPTestServer {
	t.Helper()
	return newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, r *simulationHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "test-jwt"})
		case "/api/active/visits":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"student_id": 1}},
			})
		case "/api/iot/session/current":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"active_group_id": 5, "is_active": true},
			})
		default:
			w.WriteHeader(simulationHTTPStatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   map[string]any{"id": 1},
			})
		}
	})
}

func minimalLiveState(baseURL string) *SeedState {
	return &SeedState{
		BaseURL:   baseURL,
		DevicePIN: "1234",
		Accounts: SeedStateAccounts{
			Admin: []AccountCredentials{{Email: "a@t.de", Password: "p"}},
		},
		Devices:  map[string]SeedDevice{"d1": {APIKey: "k1", Name: "S1"}},
		Students: []SeedStudent{{ID: 1, FirstName: "Felix", LastName: "S"}},
		Rooms:    map[string]int64{"OGS-Raum 1": 10, "Sporthalle": 20},
	}
}
