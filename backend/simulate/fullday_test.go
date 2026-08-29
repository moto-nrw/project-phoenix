package simulate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
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

	devices := map[string]seedapi.SeedDevice{
		"demo-device-003": {APIKey: "k3"},
		"demo-device-001": {APIKey: "k1"},
		"demo-device-002": {APIKey: "k2"},
	}

	keys := sortedDeviceKeys(devices)
	assert.Equal(t, []string{"demo-device-001", "demo-device-002", "demo-device-003"}, keys)
}

func TestSortedDeviceKeys_Empty(t *testing.T) {
	t.Parallel()

	keys := sortedDeviceKeys(map[string]seedapi.SeedDevice{})
	assert.Empty(t, keys)
}

func TestSortedDeviceKeys_Single(t *testing.T) {
	t.Parallel()

	devices := map[string]seedapi.SeedDevice{
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

	state := &seedapi.SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Accounts: seedapi.SeedStateAccounts{
			Admin: []seedapi.AccountCredentials{
				{Email: "admin@test.de", Password: "pass1"},
			},
			Betreuer: []seedapi.AccountCredentials{
				{Email: "betreuer1@test.de", Password: "pass2", StaffID: 10, TeacherID: 100, Name: "Julia Klein"},
			},
		},
		Devices: map[string]seedapi.SeedDevice{
			"demo-device-001": {APIKey: "key-001", Name: "Scanner 1"},
		},
		Students: []seedapi.SeedStudent{
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

	err := seedapi.WriteSeedState(state, statePath)
	require.NoError(t, err)

	opts := FullDayOptions{
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

	state := &seedapi.SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Accounts: seedapi.SeedStateAccounts{
			Admin:    []seedapi.AccountCredentials{{Email: "admin@test.de", Password: "p"}},
			Betreuer: []seedapi.AccountCredentials{{StaffID: 10, Name: "B1"}},
		},
		Devices:    map[string]seedapi.SeedDevice{"d1": {APIKey: "k1", Name: "S1"}},
		Students:   []seedapi.SeedStudent{{ID: 1, FirstName: "F", LastName: "L"}},
		Rooms:      map[string]int64{"OGS-Raum 1": 1},
		Activities: map[string]int64{"Hausaufgaben": 50},
	}
	require.NoError(t, seedapi.WriteSeedState(state, statePath))

	err := RunFullDay(context.Background(), FullDayOptions{
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

	state := &seedapi.SeedState{
		BaseURL:  srv.URL,
		Accounts: seedapi.SeedStateAccounts{},
	}
	require.NoError(t, seedapi.WriteSeedState(state, statePath))

	err := RunFullDay(context.Background(), FullDayOptions{StatePath: statePath})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no admin accounts")
}

func TestRunFullDay_InvalidStatePath(t *testing.T) {
	t.Parallel()

	err := RunFullDay(context.Background(), FullDayOptions{StatePath: "/nonexistent/state.json"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "load seed state")
}

func TestRunFullDay_NoDevices(t *testing.T) {
	t.Parallel()

	srv := simulationAPIMock(t)
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	state := &seedapi.SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Accounts: seedapi.SeedStateAccounts{
			Admin:    []seedapi.AccountCredentials{{Email: "a@t.de", Password: "p"}},
			Betreuer: []seedapi.AccountCredentials{{StaffID: 10, Name: "B1"}},
		},
		Devices:    map[string]seedapi.SeedDevice{},
		Students:   []seedapi.SeedStudent{{ID: 1, FirstName: "F", LastName: "L"}},
		Rooms:      map[string]int64{"OGS-Raum 1": 1},
		Activities: map[string]int64{"Hausaufgaben": 50},
	}
	require.NoError(t, seedapi.WriteSeedState(state, statePath))

	err := RunFullDay(context.Background(), FullDayOptions{StatePath: statePath})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no devices")
}

// simulationAPIMock creates a mock API server for simulation tests.
func simulationAPIMock(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `"OK"`)

		case "/auth/login":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "test-jwt"})

		case "/api/active/groups":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []any{},
			})

		case "/api/active/visits":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []any{},
			})

		case "/api/timetable/periods/bootstrap":
			w.WriteHeader(http.StatusOK)
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
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   map[string]any{"id": 1, "active_group_id": 1},
			})
		}
	}))
}

func TestRunFullDay_ManyStudents(t *testing.T) {
	t.Parallel()

	srv := simulationAPIMock(t)
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	// Create 95+ students to exercise all phases including sick marks and mid-day checkouts
	students := make([]seedapi.SeedStudent, 100)
	for i := range students {
		students[i] = seedapi.SeedStudent{
			ID:        int64(i + 1),
			FirstName: fmt.Sprintf("Student%d", i),
			LastName:  fmt.Sprintf("Last%d", i),
			GroupKey:  "sternengruppe",
			Class:     "1a",
		}
	}

	state := &seedapi.SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Accounts: seedapi.SeedStateAccounts{
			Admin: []seedapi.AccountCredentials{
				{Email: "admin@test.de", Password: "pass"},
			},
			Betreuer: []seedapi.AccountCredentials{
				{StaffID: 10, Name: "B1"},
				{StaffID: 20, Name: "B2"},
			},
		},
		Devices: map[string]seedapi.SeedDevice{
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
	require.NoError(t, seedapi.WriteSeedState(state, statePath))

	err := RunFullDay(context.Background(), FullDayOptions{
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

	state := &seedapi.SeedState{
		BaseURL: "http://localhost:1",
		Accounts: seedapi.SeedStateAccounts{
			Admin: []seedapi.AccountCredentials{{Email: "a@t.de", Password: "p"}},
		},
	}
	require.NoError(t, seedapi.WriteSeedState(state, statePath))

	err := RunFullDay(context.Background(), FullDayOptions{StatePath: statePath})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server health check")
}

func TestRunFullDay_LoginFails(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"error":"bad"}`)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	state := &seedapi.SeedState{
		BaseURL:  srv.URL,
		Accounts: seedapi.SeedStateAccounts{Admin: []seedapi.AccountCredentials{{Email: "a@t.de", Password: "p"}}},
	}
	require.NoError(t, seedapi.WriteSeedState(state, statePath))

	err := RunFullDay(context.Background(), FullDayOptions{StatePath: statePath})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "admin login")
}

// =============================================================================
// FullDayOptions Tests
// =============================================================================

func TestFullDayOptions_Defaults(t *testing.T) {
	t.Parallel()

	opts := FullDayOptions{
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

	opts := FullDayOptions{
		StatePath: "/tmp/state.json",
		Close:     true,
		Verbose:   true,
	}

	assert.True(t, opts.Close)
	assert.True(t, opts.Verbose)
}
