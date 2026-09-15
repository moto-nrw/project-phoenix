package simulate

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// stringField Tests
// =============================================================================

func TestStringField_StringValue(t *testing.T) {
	t.Parallel()

	m := map[string]any{"name": "Test Room"}
	assert.Equal(t, "Test Room", stringField(m, "name"))
}

func TestStringField_MissingKey(t *testing.T) {
	t.Parallel()

	m := map[string]any{"name": "Test"}
	assert.Equal(t, "", stringField(m, "missing"))
}

func TestStringField_NilValue(t *testing.T) {
	t.Parallel()

	m := map[string]any{"name": nil}
	assert.Equal(t, "", stringField(m, "name"))
}

func TestStringField_NumericValue(t *testing.T) {
	t.Parallel()

	m := map[string]any{"id": float64(42)}
	assert.Equal(t, "42", stringField(m, "id"))
}

func TestStringField_BoolValue(t *testing.T) {
	t.Parallel()

	m := map[string]any{"active": true}
	assert.Equal(t, "true", stringField(m, "active"))
}

func TestStringField_EmptyString(t *testing.T) {
	t.Parallel()

	m := map[string]any{"name": ""}
	assert.Equal(t, "", stringField(m, "name"))
}

func TestStringField_EmptyMap(t *testing.T) {
	t.Parallel()

	m := map[string]any{}
	assert.Equal(t, "", stringField(m, "anything"))
}

// =============================================================================
// printActiveGroups Tests (output verification)
// =============================================================================

func TestPrintActiveGroups_EmptyGroups(t *testing.T) {
	t.Parallel()

	// Should not panic with empty data
	data := `{"status":"success","data":[]}`
	require.NoError(t, printActiveGroups([]byte(data)))
}

func TestPrintActiveGroups_WithGroups(t *testing.T) {
	t.Parallel()

	data := `{"status":"success","data":[{"id":1,"activity_name":"Fußball","room_name":"Sporthalle","supervisor_names":"Julia Klein"}]}`
	// Should not panic
	require.NoError(t, printActiveGroups([]byte(data)))
}

func TestPrintActiveGroups_InvalidJSON(t *testing.T) {
	t.Parallel()

	// Should not panic with invalid JSON
	require.Error(t, printActiveGroups([]byte("not json")))
}

func TestPrintActiveGroups_DirectArray(t *testing.T) {
	t.Parallel()

	data := `[{"id":1,"name":"Fußball","room_name":"Sporthalle"}]`
	// Should handle direct array (no envelope)
	require.NoError(t, printActiveGroups([]byte(data)))
}

func TestPrintActiveGroups_FallbackToNameField(t *testing.T) {
	t.Parallel()

	data := `{"status":"success","data":[{"id":1,"name":"Fußball"}]}`
	// When activity_name is missing, should use "name"
	require.NoError(t, printActiveGroups([]byte(data)))
}

// =============================================================================
// printActiveVisits Tests
// =============================================================================

func TestPrintActiveVisits_EmptyVisits(t *testing.T) {
	t.Parallel()

	data := `{"status":"success","data":[]}`
	require.NoError(t, printActiveVisits([]byte(data)))
}

func TestPrintActiveVisits_WithVisits(t *testing.T) {
	t.Parallel()

	data := `{"status":"success","data":[{"student_id":1,"room_name":"OGS-Raum 1"},{"student_id":2,"room_name":"OGS-Raum 1"},{"student_id":3,"room_name":"Sporthalle"}]}`
	require.NoError(t, printActiveVisits([]byte(data)))
}

func TestPrintActiveVisits_InvalidJSON(t *testing.T) {
	t.Parallel()

	require.Error(t, printActiveVisits([]byte("not json")))
}

func TestPrintActiveVisits_UnknownRoom(t *testing.T) {
	t.Parallel()

	data := `{"status":"success","data":[{"student_id":1}]}`
	// room_name missing → should use "(unknown)"
	require.NoError(t, printActiveVisits([]byte(data)))
}

func TestPrintActiveVisits_DirectArray(t *testing.T) {
	t.Parallel()

	data := `[{"student_id":1,"room_name":"OGS-Raum 1"}]`
	require.NoError(t, printActiveVisits([]byte(data)))
}

// =============================================================================
// RunStatus Integration Tests
// =============================================================================

func statusAPIMock(t *testing.T) *simulationHTTPTestServer {
	t.Helper()
	return newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, r *simulationHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/health":
			w.WriteHeader(simulationHTTPStatusOK)
			_, _ = fmt.Fprint(w, `"OK"`)

		case "/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "jwt"})

		case "/api/active/groups":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": []map[string]any{
					{"id": 1, "activity_name": "Fußball", "room_name": "Sporthalle", "supervisor_names": "Julia Klein"},
				},
			})

		case "/api/active/visits":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": []map[string]any{
					{"student_id": 1, "room_name": "Sporthalle"},
					{"student_id": 2, "room_name": "Sporthalle"},
					{"student_id": 3, "room_name": "OGS-Raum 1"},
				},
			})

		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success"})
		}
	})
}

func TestRunStatus_Success(t *testing.T) {
	t.Parallel()

	srv := statusAPIMock(t)
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	state := &SeedState{
		BaseURL:   srv.URL,
		DevicePIN: "1234",
		Accounts: SeedStateAccounts{
			Admin:    []AccountCredentials{{Email: "admin@test.de", Password: "pass"}},
			Betreuer: []AccountCredentials{{Name: "B1"}},
		},
		Devices:    map[string]SeedDevice{"d1": {APIKey: "k1"}},
		Students:   []SeedStudent{{ID: 1}},
		Rooms:      map[string]int64{"OGS-Raum 1": 1},
		Activities: map[string]int64{"Fußball": 50},
		Groups:     map[string]int64{"sternengruppe": 400},
	}
	require.NoError(t, WriteSeedState(state, statePath))

	err := RunStatus(context.Background(), StatusOptions{Client: newTestClientFactory,
		StatePath: statePath,
		Verbose:   false,
	})
	assert.NoError(t, err)
}

func TestRunStatus_InvalidStatePath(t *testing.T) {
	t.Parallel()

	err := RunStatus(context.Background(), StatusOptions{Client: newTestClientFactory, StatePath: "/nonexistent/state.json"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "load seed state")
}

func TestRunStatus_NoAdminAccounts(t *testing.T) {
	t.Parallel()

	srv := statusAPIMock(t)
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	state := &SeedState{
		BaseURL:  srv.URL,
		Accounts: SeedStateAccounts{},
	}
	require.NoError(t, WriteSeedState(state, statePath))

	err := RunStatus(context.Background(), StatusOptions{Client: newTestClientFactory, StatePath: statePath})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no admin accounts")
}

func TestRunStatus_ServerDown(t *testing.T) {
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

	err := RunStatus(context.Background(), StatusOptions{Client: newTestClientFactory, StatePath: statePath})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server health check")
}

func TestRunStatus_GroupsFetchError(t *testing.T) {
	t.Parallel()

	// Server that returns errors on /api/active/groups
	srv := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, r *simulationHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/health":
			_, _ = fmt.Fprint(w, `"OK"`)
		case "/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "jwt"})
		case "/api/active/groups":
			w.WriteHeader(simulationHTTPStatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"error":"db error"}`)
		case "/api/active/visits":
			w.WriteHeader(simulationHTTPStatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"error":"db error"}`)
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

	// Smoke checks must fail when the profile cannot be read.
	err := RunStatus(context.Background(), StatusOptions{Client: newTestClientFactory, StatePath: statePath})
	assert.ErrorContains(t, err, "active groups")
}

// =============================================================================
// StatusOptions Tests
// =============================================================================

func TestStatusOptions_Fields(t *testing.T) {
	t.Parallel()

	opts := StatusOptions{Client: newTestClientFactory,
		StatePath: ".seed-state.json",
		Verbose:   true,
	}
	assert.Equal(t, ".seed-state.json", opts.StatePath)
	assert.True(t, opts.Verbose)
}
