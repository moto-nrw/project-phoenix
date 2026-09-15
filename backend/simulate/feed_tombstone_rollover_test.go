package simulate

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRunFullDayRollsTheSchoolYearForTombstone pins the school-year rollover
// behaviour of the demo cancellation step (#3173): when no active calendar
// period still contains a future weekday, the simulation creates or activates
// a school year through the regular periods API instead of failing.
func TestRunFullDayRollsTheSchoolYearForTombstone(t *testing.T) {
	t.Parallel()

	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)

	schoolYear2026 := map[string]any{
		"id": 1, "name": "Schuljahr 2026/2027", "period_type": "school_year",
		"start_date": "2026-08-01", "end_date": "2027-07-31",
		"week_cycle_length": 1, "is_active": true,
	}
	inactiveSchoolYear2027 := map[string]any{
		"id": 9, "name": "Schuljahr 2027/2028", "period_type": "school_year",
		"start_date": "2027-08-01", "end_date": "2028-07-31",
		"week_cycle_length": 1, "is_active": false,
	}

	tests := []struct {
		name string
		// now is the simulation clock; periods is what the bootstrap
		// endpoint returns; conflictOnCreate rejects the first create with
		// 409, the way a hand-edited calendar does.
		now              time.Time
		periods          []map[string]any
		conflictOnCreate bool
		expectedCreate   map[string]any
		expectedActivate int64
		expectedDate     string
	}{
		{
			// Last weekday of the 2026/27 school year: only Sat 2027-07-31 is
			// left inside the active period.
			name:    "last weekday of the school year",
			now:     time.Date(2027, 7, 30, 12, 0, 0, 0, time.UTC),
			periods: []map[string]any{schoolYear2026},
			expectedCreate: map[string]any{
				"name": "Schuljahr 2027/2028", "start_date": "2027-08-01", "end_date": "2028-07-31",
			},
			expectedDate: "2027-08-02",
		},
		{
			// Existing tenant after the school-year change: bootstrap returns
			// the stale 2026/27 period unchanged.
			name:    "after the school year change",
			now:     time.Date(2027, 8, 2, 12, 0, 0, 0, time.UTC),
			periods: []map[string]any{schoolYear2026},
			expectedCreate: map[string]any{
				"name": "Schuljahr 2027/2028", "start_date": "2027-08-01", "end_date": "2028-07-31",
			},
			expectedDate: "2027-08-03",
		},
		{
			// The school year exists but an administrator deactivated it.
			name:             "next school year only needs activating",
			now:              time.Date(2027, 8, 2, 12, 0, 0, 0, time.UTC),
			periods:          []map[string]any{schoolYear2026, inactiveSchoolYear2027},
			expectedActivate: 9,
			expectedDate:     "2027-08-03",
		},
		{
			// A hand-edited calendar holds the name or an overlapping active
			// period: the step moves on to the year after instead of failing.
			name:             "create conflict falls through to the next year",
			now:              time.Date(2027, 8, 2, 12, 0, 0, 0, time.UTC),
			periods:          []map[string]any{schoolYear2026},
			conflictOnCreate: true,
			expectedCreate: map[string]any{
				"name": "Schuljahr 2028/2029", "start_date": "2028-08-01", "end_date": "2029-07-31",
			},
			expectedDate: "2028-08-01",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var createdPeriods []map[string]any
			var activatedPeriodIDs []string
			var createdDate string
			deletedInstance := false
			server := newSimulationHTTPTestServer(func(w simulationHTTPResponseWriter, r *simulationHTTPRequest) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.URL.Path == "/health":
					w.WriteHeader(simulationHTTPStatusOK)
					_, _ = fmt.Fprint(w, `"OK"`)
				case r.URL.Path == "/auth/login":
					_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "test-jwt"})
				case r.Method == simulationHTTPMethodPost && r.URL.Path == "/api/timetable/periods/bootstrap":
					_ = json.NewEncoder(w).Encode(map[string]any{
						"status": "success",
						"data":   map[string]any{"created": false, "periods": tc.periods},
					})
				case r.Method == simulationHTTPMethodPost && r.URL.Path == "/api/timetable/periods":
					var body map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					createdPeriods = append(createdPeriods, body)
					if tc.conflictOnCreate && len(createdPeriods) == 1 {
						w.WriteHeader(simulationHTTPStatusConflict)
						_ = json.NewEncoder(w).Encode(map[string]string{"error": "Kalenderzeitraum existiert bereits"})
						return
					}
					w.WriteHeader(simulationHTTPStatusCreated)
					_ = json.NewEncoder(w).Encode(map[string]any{
						"status": "success",
						"data":   map[string]any{"id": 5},
					})
				case r.Method == simulationHTTPMethodPut && strings.HasPrefix(r.URL.Path, "/api/timetable/periods/"):
					var body map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					require.Equal(t, true, body["is_active"], "activation must not change anything but is_active")
					require.Equal(t, "Schuljahr 2027/2028", body["name"])
					require.Equal(t, "school_year", body["period_type"])
					require.Equal(t, "2027-08-01", body["start_date"])
					require.Equal(t, "2028-07-31", body["end_date"])
					activatedPeriodIDs = append(activatedPeriodIDs, strings.TrimPrefix(r.URL.Path, "/api/timetable/periods/"))
					_ = json.NewEncoder(w).Encode(map[string]any{
						"status": "success",
						"data":   map[string]any{"id": 9},
					})
				case r.Method == simulationHTTPMethodPost && r.URL.Path == "/api/timetable/instances":
					var body struct {
						Date string `json:"date"`
					}
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					createdDate = body.Date
					_ = json.NewEncoder(w).Encode(map[string]any{
						"status": "success",
						"data":   map[string]any{"id": 77},
					})
				case r.Method == simulationHTTPMethodDelete && r.URL.Path == "/api/timetable/instances/77":
					deletedInstance = true
					w.WriteHeader(simulationHTTPStatusNoContent)
				case r.Method == simulationHTTPMethodPost && r.URL.Path == "/api/iot/checkin":
					var body map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					if body["student_rfid"] == "DEMO-UNREGISTERED-TAG" {
						w.WriteHeader(simulationHTTPStatusNotFound)
						_ = json.NewEncoder(w).Encode(map[string]string{"error": "unknown tag", "code": "rfid_tag_not_found"})
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"id": 1}})
				default:
					_ = json.NewEncoder(w).Encode(map[string]any{
						"status": "success",
						"data":   map[string]any{"id": 1, "active_group_id": 1},
					})
				}
			})
			defer server.Close()

			statePath := filepath.Join(t.TempDir(), "state.json")
			state := &SeedState{
				BaseURL:   server.URL,
				DevicePIN: "1234",
				Accounts: SeedStateAccounts{
					Admin:    []AccountCredentials{{Email: "admin@test.de", Password: "pass"}},
					Betreuer: []AccountCredentials{{StaffID: 10, Name: "Julia Klein"}},
				},
				Devices:    map[string]SeedDevice{"device": {APIKey: "key", Name: "Scanner"}},
				Students:   []SeedStudent{{ID: 1, FirstName: "Felix", LastName: "Schneider"}},
				Rooms:      map[string]int64{"OGS-Raum 1": 1},
				Activities: map[string]int64{"Hausaufgaben": 50},
			}
			require.NoError(t, WriteSeedState(state, statePath))

			require.NoError(t, RunFullDay(context.Background(), FullDayOptions{
				Client:    newTestClientFactory,
				StatePath: statePath,
				Now:       func() time.Time { return tc.now },
			}))

			if tc.expectedCreate == nil {
				require.Empty(t, createdPeriods, "simulation must not create a period it can activate instead")
			} else {
				require.NotEmpty(t, createdPeriods, "simulation must create the missing school year through the periods API")
				created := createdPeriods[len(createdPeriods)-1]
				require.Equal(t, tc.expectedCreate["name"], created["name"])
				require.Equal(t, tc.expectedCreate["start_date"], created["start_date"])
				require.Equal(t, tc.expectedCreate["end_date"], created["end_date"])
				require.Equal(t, "school_year", created["period_type"])
				require.Equal(t, true, created["is_active"])
			}
			if tc.expectedActivate == 0 {
				require.Empty(t, activatedPeriodIDs)
			} else {
				require.Equal(t, []string{fmt.Sprintf("%d", tc.expectedActivate)}, activatedPeriodIDs)
			}

			require.Equal(t, tc.expectedDate, createdDate)
			createdDay, err := time.ParseInLocation("2006-01-02", createdDate, berlin)
			require.NoError(t, err)
			require.True(t, createdDay.After(tc.now.In(berlin)), "demo cancellation must be scheduled in the future")
			require.NotContains(t, []time.Weekday{time.Saturday, time.Sunday}, createdDay.Weekday())
			require.True(t, deletedInstance, "full-day simulation must delete the demo instance to retain a feed tombstone")
		})
	}
}
