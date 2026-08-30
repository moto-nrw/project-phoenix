package simulate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
	"github.com/stretchr/testify/require"
)

func TestRunFullDaySeedsStaffFeedTombstone(t *testing.T) {
	t.Parallel()

	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	today, err := time.ParseInLocation("2006-01-02", time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC).In(berlin).Format("2006-01-02"), berlin)
	require.NoError(t, err)
	periodStart := today.AddDate(0, 0, 14)
	for periodStart.Weekday() == time.Saturday || periodStart.Weekday() == time.Sunday {
		periodStart = periodStart.AddDate(0, 0, 1)
	}
	periodEnd := periodStart.AddDate(0, 0, 6)

	var createdStaffIDs []int64
	var createdDate string
	bootstrappedPeriods := false
	deletedInstance := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `"OK"`)
		case r.URL.Path == "/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "test-jwt"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/timetable/periods/bootstrap":
			bootstrappedPeriods = true
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"created": true,
					"periods": []map[string]any{{
						"start_date": periodStart.Format("2006-01-02"),
						"end_date":   periodEnd.Format("2006-01-02"),
						"is_active":  true,
					}},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/timetable/instances":
			require.True(t, bootstrappedPeriods, "calendar periods must be bootstrapped before creating the demo cancellation")
			var body struct {
				StaffIDs []int64 `json:"staff_ids"`
				Date     string  `json:"date"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			createdStaffIDs = body.StaffIDs
			createdDate = body.Date
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   map[string]any{"id": 77},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/timetable/instances/77":
			deletedInstance = true
			w.WriteHeader(http.StatusNoContent)
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   map[string]any{"id": 1, "active_group_id": 1},
			})
		}
	}))
	defer server.Close()

	statePath := filepath.Join(t.TempDir(), "state.json")
	state := &seedapi.SeedState{
		BaseURL:   server.URL,
		DevicePIN: "1234",
		Accounts: seedapi.SeedStateAccounts{
			Admin:    []seedapi.AccountCredentials{{Email: "admin@test.de", Password: "pass"}},
			Betreuer: []seedapi.AccountCredentials{{StaffID: 10, Name: "Julia Klein"}},
		},
		Devices:    map[string]seedapi.SeedDevice{"device": {APIKey: "key", Name: "Scanner"}},
		Rooms:      map[string]int64{"OGS-Raum 1": 1},
		Activities: map[string]int64{"Hausaufgaben": 50},
	}
	require.NoError(t, seedapi.WriteSeedState(state, statePath))

	require.NoError(t, RunFullDay(context.Background(), FullDayOptions{Client: newTestClientFactory, StatePath: statePath}))
	require.True(t, bootstrappedPeriods)
	require.Equal(t, []int64{10}, createdStaffIDs)
	createdDay, err := time.ParseInLocation("2006-01-02", createdDate, berlin)
	require.NoError(t, err)
	require.True(t, createdDay.After(today), "demo cancellation must be scheduled in the future")
	require.False(t, createdDay.Before(periodStart), "demo cancellation must be inside the active calendar period")
	require.False(t, createdDay.After(periodEnd), "demo cancellation must be inside the active calendar period")
	require.NotContains(t, []time.Weekday{time.Saturday, time.Sunday}, createdDay.Weekday())
	require.True(t, deletedInstance, "full-day simulation must delete the demo instance to retain a feed tombstone")
}
