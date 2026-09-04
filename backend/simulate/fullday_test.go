package simulate

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// findRoomForActivity Tests
// =============================================================================

func TestFindRoomForActivity_KnownActivity(t *testing.T) {
	t.Parallel()

	rooms := map[string]int64{
		"OGS-Raum 1": 10,
		"OGS-Raum 3": 15,
		"Sporthalle": 20,
		"Schulhof":   30,
	}

	assert.Equal(t, int64(10), findRoomForActivity("Hausaufgaben", rooms))
	assert.Equal(t, int64(20), findRoomForActivity("Fußball", rooms))
	assert.Equal(t, int64(30), findRoomForActivity("Garten", rooms))
	assert.Equal(t, int64(15), findRoomForActivity("Freispiel", rooms))
}

func TestFindRoomForActivity_AllMappings(t *testing.T) {
	t.Parallel()

	rooms := map[string]int64{
		"OGS-Raum 1":    1,
		"OGS-Raum 2":    2,
		"OGS-Raum 3":    3,
		"Sporthalle":    4,
		"Kreativraum":   5,
		"Mensa":         6,
		"Schulhof":      7,
		"Leseecke":      8,
		"Musikraum":     9,
		"Bewegungsraum": 10,
	}

	tests := map[string]int64{
		"Hausaufgaben": 1,
		"Fußball":      4,
		"Basteln":      5,
		"Kochen":       6,
		"Lesen":        8,
		"Musik":        9,
		"Tanzen":       10,
		"Schach":       2,
		"Garten":       7,
		"Freispiel":    3,
	}

	for activity, expectedRoomID := range tests {
		t.Run(activity, func(t *testing.T) {
			assert.Equal(t, expectedRoomID, findRoomForActivity(activity, rooms))
		})
	}
}

func TestFindRoomForActivity_UnknownActivity(t *testing.T) {
	t.Parallel()

	rooms := map[string]int64{"OGS-Raum 1": 1}
	assert.Equal(t, int64(0), findRoomForActivity("Schwimmen", rooms))
}

func TestFindRoomForActivity_MissingRoom(t *testing.T) {
	t.Parallel()

	rooms := map[string]int64{} // Room "OGS-Raum 1" not in map
	assert.Equal(t, int64(0), findRoomForActivity("Hausaufgaben", rooms))
}

func TestFindRoomForActivity_EmptyRooms(t *testing.T) {
	t.Parallel()

	assert.Equal(t, int64(0), findRoomForActivity("Fußball", map[string]int64{}))
}

func TestFindRoomForActivity_EmptyActivity(t *testing.T) {
	t.Parallel()

	rooms := map[string]int64{"OGS-Raum 1": 1}
	assert.Equal(t, int64(0), findRoomForActivity("", rooms))
}

// =============================================================================
// sortedDeviceKeys Tests
// =============================================================================

func TestSortedDeviceKeys_SortsAlphabetically(t *testing.T) {
	t.Parallel()

	devices := map[string]SeedDevice{
		"demo-device-003": {APIKey: "k3"},
		"demo-device-001": {APIKey: "k1"},
		"demo-device-002": {APIKey: "k2"},
	}

	keys := sortedDeviceKeys(devices)
	assert.Equal(t, []string{"demo-device-001", "demo-device-002", "demo-device-003"}, keys)
}

func TestSortedDeviceKeys_Empty(t *testing.T) {
	t.Parallel()

	keys := sortedDeviceKeys(map[string]SeedDevice{})
	assert.Empty(t, keys)
}

func TestSortedDeviceKeys_Single(t *testing.T) {
	t.Parallel()

	devices := map[string]SeedDevice{
		"only-device": {APIKey: "k1"},
	}
	keys := sortedDeviceKeys(devices)
	assert.Equal(t, []string{"only-device"}, keys)
}

// =============================================================================
// sortedStringKeys Tests
// =============================================================================

func TestSortedStringKeys_SortsAlphabetically(t *testing.T) {
	t.Parallel()

	m := map[string]int64{
		"Fußball":      1,
		"Basteln":      2,
		"Hausaufgaben": 3,
	}

	keys := sortedStringKeys(m)
	assert.Equal(t, []string{"Basteln", "Fußball", "Hausaufgaben"}, keys)
}

func TestSortedStringKeys_Empty(t *testing.T) {
	t.Parallel()

	keys := sortedStringKeys(map[string]int64{})
	assert.Empty(t, keys)
}

func TestSortedStringKeys_Single(t *testing.T) {
	t.Parallel()

	m := map[string]int64{"only": 1}
	keys := sortedStringKeys(m)
	assert.Equal(t, []string{"only"}, keys)
}

// =============================================================================
// RunFullDay Integration Tests (httptest)
// =============================================================================

func TestRunFullDay_Success(t *testing.T) {
	t.Parallel()

	srv := simulationAPIMock(t)
	defer srv.Close()

	// Write a seed state file pointing to the mock server
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	state := &SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Accounts: SeedStateAccounts{
			Admin: []AccountCredentials{
				{Email: "admin@test.de", Password: "pass1"},
			},
			Betreuer: []AccountCredentials{
				{Email: "betreuer1@test.de", Password: "pass2", StaffID: 10, TeacherID: 100, Name: "Julia Klein"},
			},
		},
		Devices: map[string]SeedDevice{
			"demo-device-001": {APIKey: "key-001", Name: "Scanner 1"},
		},
		Students: []SeedStudent{
			{ID: 1, FirstName: "Felix", LastName: "Schneider", GroupKey: "sternengruppe", Class: "1a"},
			{ID: 2, FirstName: "Emma", LastName: "Meyer", GroupKey: "sternengruppe", Class: "1a"},
		},
		Rooms: map[string]int64{
			"OGS-Raum 1": 1,
			"Sporthalle": 2,
		},
		Activities: map[string]int64{
			"Hausaufgaben": 50,
		},
		Groups: map[string]int64{
			"sternengruppe": 400,
		},
	}

	err := WriteSeedState(state, statePath)
	require.NoError(t, err)

	opts := FullDayOptions{Client: newTestClientFactory,
		StatePath: statePath,
		Close:     false,
		Verbose:   false,
	}

	err = RunFullDay(context.Background(), opts)
	assert.NoError(t, err)
}

func TestRunFullDay_WithClose(t *testing.T) {
	t.Parallel()

	srv := simulationAPIMock(t)
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	state := &SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Accounts: SeedStateAccounts{
			Admin:    []AccountCredentials{{Email: "admin@test.de", Password: "p"}},
			Betreuer: []AccountCredentials{{StaffID: 10, Name: "B1"}},
		},
		Devices:    map[string]SeedDevice{"d1": {APIKey: "k1", Name: "S1"}},
		Students:   []SeedStudent{{ID: 1, FirstName: "F", LastName: "L"}},
		Rooms:      map[string]int64{"OGS-Raum 1": 1},
		Activities: map[string]int64{"Hausaufgaben": 50},
	}
	require.NoError(t, WriteSeedState(state, statePath))

	err := RunFullDay(context.Background(), FullDayOptions{Client: newTestClientFactory,
		StatePath: statePath,
		Close:     true,
		Verbose:   true,
	})
	assert.NoError(t, err)
}

func TestRunFullDay_NoAdminAccounts(t *testing.T) {
	t.Parallel()

	srv := simulationAPIMock(t)
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	state := &SeedState{
		BaseURL:  srv.URL,
		Accounts: SeedStateAccounts{},
	}
	require.NoError(t, WriteSeedState(state, statePath))

	err := RunFullDay(context.Background(), FullDayOptions{Client: newTestClientFactory, StatePath: statePath})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no admin accounts")
}

func TestRunFullDay_InvalidStatePath(t *testing.T) {
	t.Parallel()

	err := RunFullDay(context.Background(), FullDayOptions{Client: newTestClientFactory, StatePath: "/nonexistent/state.json"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "load seed state")
}

func TestRunFullDay_NoDevices(t *testing.T) {
	t.Parallel()

	srv := simulationAPIMock(t)
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	state := &SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Accounts: SeedStateAccounts{
			Admin:    []AccountCredentials{{Email: "a@t.de", Password: "p"}},
			Betreuer: []AccountCredentials{{StaffID: 10, Name: "B1"}},
		},
		Devices:    map[string]SeedDevice{},
		Students:   []SeedStudent{{ID: 1, FirstName: "F", LastName: "L"}},
		Rooms:      map[string]int64{"OGS-Raum 1": 1},
		Activities: map[string]int64{"Hausaufgaben": 50},
	}
	require.NoError(t, WriteSeedState(state, statePath))

	err := RunFullDay(context.Background(), FullDayOptions{Client: newTestClientFactory, StatePath: statePath})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no devices")
}

// simulationAPIMock creates a mock API server for simulation tests.
func simulationAPIMock(t *testing.T, failedPaths ...string) *simulationHTTPTestServer {
	return simulationAPIMockWithOptions(t, "rfid_tag_not_found", 0, failedPaths...)
}

func simulationAPIMockWithUnknownCode(t *testing.T, unknownCode string, failedPaths ...string) *simulationHTTPTestServer {
	return simulationAPIMockWithOptions(t, unknownCode, 0, failedPaths...)
}

func simulationAPIMockWithCheckinLimit(t *testing.T, checkinLimit int) *simulationHTTPTestServer {
	return simulationAPIMockWithOptions(t, "rfid_tag_not_found", checkinLimit)
}

func simulationAPIMockWithOptions(t *testing.T, unknownCode string, checkinLimit int, failedPaths ...string) *simulationHTTPTestServer {
	t.Helper()
	checkedInRFIDs := make(map[string]bool)
	return newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, r *simulationHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		if slices.Contains(failedPaths, r.URL.Path) {
			w.WriteHeader(simulationHTTPStatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "injected failure", "code": "injected_failure"})
			return
		}

		switch r.URL.Path {
		case "/health":
			w.WriteHeader(simulationHTTPStatusOK)
			_, _ = fmt.Fprint(w, `"OK"`)

		case "/auth/login":
			w.WriteHeader(simulationHTTPStatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "test-jwt"})

		case "/api/active/groups":
			w.WriteHeader(simulationHTTPStatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []any{},
			})

		case "/api/active/visits":
			w.WriteHeader(simulationHTTPStatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []any{},
			})

		case "/api/iot/checkin":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			if body["student_rfid"] == "DEMO-UNREGISTERED-TAG" {
				w.WriteHeader(404)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "unknown tag", "code": unknownCode})
				return
			}
			rfidTag, _ := body["student_rfid"].(string)
			action, _ := body["action"].(string)
			if checkinLimit > 0 && action == "checkout" {
				if !checkedInRFIDs[rfidTag] {
					w.WriteHeader(409)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "Student is not checked in", "code": "STUDENT_NOT_CHECKED_IN"})
					return
				}
				delete(checkedInRFIDs, rfidTag)
			}
			if checkinLimit > 0 && action == "checkin" {
				if len(checkedInRFIDs) >= checkinLimit {
					w.WriteHeader(409)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "Room capacity exceeded", "code": "ROOM_CAPACITY_EXCEEDED"})
					return
				}
				checkedInRFIDs[rfidTag] = true
			}
			w.WriteHeader(simulationHTTPStatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"id": 1}})

		case "/api/timetable/periods/bootstrap":
			w.WriteHeader(simulationHTTPStatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"created": false,
					"periods": []map[string]any{{
						"start_date": "2000-01-01",
						"end_date":   "2100-12-31",
						"is_active":  true,
					}},
				},
			})

		default:
			w.WriteHeader(simulationHTTPStatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   map[string]any{"id": 1, "active_group_id": 1},
			})
		}
	})
}

func TestRunFullDay_RejectsUnexpectedUnknownRFIDCode(t *testing.T) {
	t.Parallel()

	srv := simulationAPIMockWithUnknownCode(t, "different_not_found")
	defer srv.Close()
	statePath := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, WriteSeedState(&SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Bootstrap: SeedStateBootstrap{TenantSlug: "demo-school"},
		Accounts: SeedStateAccounts{
			Admin:    []AccountCredentials{{Email: "admin@test.de", Password: "pass"}},
			Betreuer: []AccountCredentials{{StaffID: 10, Name: "Mara Muster"}},
		},
		Devices:    map[string]SeedDevice{"demo-device-001": {APIKey: "key", Name: "Scanner"}},
		Students:   []SeedStudent{{ID: 1, FirstName: "Felix", LastName: "Schneider"}},
		Activities: map[string]int64{"Hausaufgaben": 50},
		Rooms:      map[string]int64{"OGS-Raum 1": 1},
	}, statePath))

	err := RunFullDay(context.Background(), FullDayOptions{Client: newTestClientFactory, StatePath: statePath})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "record attendance and checkins")
	assert.Contains(t, err.Error(), "expected 404 (rfid_tag_not_found)")
	assert.Contains(t, err.Error(), "different_not_found")
}

func TestRunFullDay_FailsWholeRunWhenActivityStartFails(t *testing.T) {
	t.Parallel()

	srv := simulationAPIMock(t, "/api/iot/session/start")
	defer srv.Close()

	statePath := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, WriteSeedState(&SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Bootstrap: SeedStateBootstrap{TenantSlug: "demo-school"},
		Accounts: SeedStateAccounts{
			Admin:    []AccountCredentials{{Email: "admin@test.de", Password: "pass"}},
			Betreuer: []AccountCredentials{{StaffID: 10, Name: "Mara Muster"}},
		},
		Devices:    map[string]SeedDevice{"demo-device-001": {APIKey: "key", Name: "Scanner"}},
		Students:   []SeedStudent{{ID: 1, FirstName: "Felix", LastName: "Schneider"}},
		Activities: map[string]int64{"Hausaufgaben": 50},
		Rooms:      map[string]int64{"OGS-Raum 1": 1},
	}, statePath))

	err := RunFullDay(context.Background(), FullDayOptions{Client: newTestClientFactory, StatePath: statePath})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `demo school profile "demo-school"`)
	assert.Contains(t, err.Error(), "start sessions")
	assert.Contains(t, err.Error(), "POST /api/iot/session/start")
	assert.Contains(t, err.Error(), "500")
	assert.Contains(t, err.Error(), "injected_failure")
}

func TestRunFullDay_EndsStartedSessionsWhenLaterStartFails(t *testing.T) {
	t.Parallel()

	starts, ends := 0, 0
	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, r *simulationHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(simulationHTTPStatusOK)
			_, _ = fmt.Fprint(w, `"OK"`)
		case "/auth/login":
			w.WriteHeader(simulationHTTPStatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "test-jwt"})
		case "/api/iot/session/start":
			starts++
			if starts == 2 {
				w.WriteHeader(simulationHTTPStatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "injected failure"})
				return
			}
			w.WriteHeader(simulationHTTPStatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success"})
		case "/api/iot/session/end":
			ends++
			w.WriteHeader(simulationHTTPStatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success"})
		default:
			w.WriteHeader(simulationHTTPStatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success"})
		}
	})
	defer srv.Close()

	statePath := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, WriteSeedState(&SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Bootstrap: SeedStateBootstrap{TenantSlug: "demo-school"},
		Accounts: SeedStateAccounts{
			Admin: []AccountCredentials{{Email: "admin@test.de", Password: "pass"}},
			Betreuer: []AccountCredentials{
				{StaffID: 10, Name: "Mara Muster"},
				{StaffID: 11, Name: "Nora Muster"},
			},
		},
		Devices: map[string]SeedDevice{
			"demo-device-001": {APIKey: "key-1", Name: "Scanner 1"},
			"demo-device-002": {APIKey: "key-2", Name: "Scanner 2"},
		},
		Students: []SeedStudent{{ID: 1, FirstName: "Felix", LastName: "Schneider"}},
		Activities: map[string]int64{
			"Basteln":      50,
			"Hausaufgaben": 51,
		},
		Rooms: map[string]int64{"OGS-Raum 1": 1},
	}, statePath))

	err := RunFullDay(context.Background(), FullDayOptions{Client: newTestClientFactory, StatePath: statePath})
	require.Error(t, err)
	assert.Equal(t, 2, starts)
	assert.Equal(t, 1, ends)
}

func TestRunFullDay_ManyStudents(t *testing.T) {
	t.Parallel()

	// The seeded room plan can admit 84 round-robin check-ins before its
	// smallest room reaches capacity.
	srv := simulationAPIMockWithCheckinLimit(t, 84)
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	// Create 95+ students to exercise all phases including sick marks and mid-day checkouts
	students := make([]SeedStudent, 100)
	for i := range students {
		students[i] = SeedStudent{
			ID:        int64(i + 1),
			FirstName: fmt.Sprintf("Student%d", i),
			LastName:  fmt.Sprintf("Last%d", i),
			GroupKey:  "sternengruppe",
			Class:     "1a",
		}
	}

	state := &SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Accounts: SeedStateAccounts{
			Admin: []AccountCredentials{
				{Email: "admin@test.de", Password: "pass"},
			},
			Betreuer: []AccountCredentials{
				{StaffID: 10, Name: "B1"},
				{StaffID: 20, Name: "B2"},
			},
		},
		Devices: map[string]SeedDevice{
			"d1": {APIKey: "k1", Name: "S1"},
			"d2": {APIKey: "k2", Name: "S2"},
		},
		Students: students,
		Rooms: map[string]int64{
			"OGS-Raum 1": 1,
			"Sporthalle": 2,
		},
		Activities: map[string]int64{
			"Hausaufgaben": 50,
			"Fußball":      51,
		},
	}
	require.NoError(t, WriteSeedState(state, statePath))

	err := RunFullDay(context.Background(), FullDayOptions{Client: newTestClientFactory,
		StatePath: statePath,
		Close:     true,
		Verbose:   true,
	})
	assert.NoError(t, err)
}

func TestRunFullDay_ServerHealthFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	state := &SeedState{
		BaseURL: "http://localhost:1",
		Accounts: SeedStateAccounts{
			Admin: []AccountCredentials{{Email: "a@t.de", Password: "p"}},
		},
	}
	require.NoError(t, WriteSeedState(state, statePath))

	err := RunFullDay(context.Background(), FullDayOptions{Client: newTestClientFactory, StatePath: statePath})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server health check")
}

func TestRunFullDay_LoginFails(t *testing.T) {
	t.Parallel()

	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, r *simulationHTTPRequest) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(simulationHTTPStatusOK)
		default:
			w.WriteHeader(simulationHTTPStatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"error":"bad"}`)
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

	err := RunFullDay(context.Background(), FullDayOptions{Client: newTestClientFactory, StatePath: statePath})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "admin login")
}

// =============================================================================
// FullDayOptions Tests
// =============================================================================

func TestFullDayOptions_Defaults(t *testing.T) {
	t.Parallel()

	opts := FullDayOptions{Client: newTestClientFactory,
		StatePath: ".seed-state.json",
		Close:     false,
		Verbose:   false,
	}

	assert.Equal(t, ".seed-state.json", opts.StatePath)
	assert.False(t, opts.Close)
	assert.False(t, opts.Verbose)
}

func TestFullDayOptions_WithClose(t *testing.T) {
	t.Parallel()

	opts := FullDayOptions{Client: newTestClientFactory,
		StatePath: "/tmp/state.json",
		Close:     true,
		Verbose:   true,
	}

	assert.True(t, opts.Close)
	assert.True(t, opts.Verbose)
}
