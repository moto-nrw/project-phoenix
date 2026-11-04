package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Exercise both request producers, not a copy of their date formulas. The
// transport rejects a second pending request for the same child and day.
func TestSeedParentPickupRequestsDoNotCollide(t *testing.T) {
	t.Parallel()
	for offset := range 7 {
		today := mustParseSeedDate(t, "2026-10-26").AddDays(offset)
		t.Run(today.Weekday().String(), func(t *testing.T) {
			t.Parallel()
			pending := map[string]map[string]any{}
			requests := map[string]string{}
			nextID := 100
			srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
				w.Header().Set("Content-Type", "application/json")
				path := r.URL.Path
				switch {
				case strings.HasSuffix(path, "/care-exception"):
					var body map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					date := body["date"].(string)
					key := path + "/" + date
					if pending[key] != nil {
						w.WriteHeader(409)
						_, _ = fmt.Fprint(w, `{"error":"care.care_request_already_pending"}`)
						return
					}
					day := mustParseSeedDate(t, date)
					assert.NotEqual(t, time.Saturday, day.Weekday())
					assert.NotEqual(t, time.Sunday, day.Weekday())
					pending[key] = body
					nextID++
					id := fmt.Sprint(nextID)
					requests[id] = key
					_, _ = fmt.Fprintf(w, `{"data":{"id":%q}}`, id)
				case strings.HasSuffix(path, "/pickup-permission-requests"):
					_, _ = fmt.Fprint(w, `{"data":[{"id":"95","guardian_profile_id":"93","status":"pending"}]}`)
				case path == "/api/students/change-requests":
					items := []map[string]any{{"request_type": "care_schedule", "expected_version": "v95", "data": map[string]any{"id": "95"}}}
					for id := range requests {
						items = append(items, map[string]any{"request_type": "care_schedule", "expected_version": "v" + id, "data": map[string]any{"id": id}})
					}
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"items": items}}))
				case strings.HasSuffix(path, "/decide"):
					id := strings.TrimSuffix(strings.TrimPrefix(path, "/api/students/care-schedule-change-requests/"), "/decide")
					delete(pending, requests[id])
					_, _ = fmt.Fprint(w, `{"data":null}`)
				case strings.HasSuffix(path, "/pickup-signature-statement"):
					_, _ = fmt.Fprint(w, `{"data":{"statement":"Test declaration"}}`)
				default:
					_, _ = fmt.Fprint(w, `{"data":null}`)
				}
			})
			defer srv.Close()
			parents := []ParentCredentials{
				{Email: "parent@example.test", StudentIDs: []int64{44}},
				{Email: "other@example.test", StudentIDs: []int64{44}, GuardianID: 93},
			}
			rt := &Runtime{Client: newTestClient(srv.URL, false), Parents: parents, FixedSeeder: &FixedSeeder{}}
			actions, err := (parentEnrollmentSeedStep{}).seedParentPortalActions(rt, map[string]AuthRef{}, parents, today)
			require.NoError(t, err)
			assert.Len(t, actions, 4)
			require.NoError(t, seedParentPickupPermissions(rt, AuthRef{}, 44, 93, today))
			require.Len(t, pending, 2, "the approved collector leaves both original requests open")
			base := "/parent/me/children/44/care-exception/"
			pickup := pending[base+seedSchoolDayAfter(today, 3).String()]
			alone := pending[base+seedSchoolDayAfter(today, 4).String()]
			require.NotNil(t, pickup)
			require.NotNil(t, alone)
			assert.Equal(t, "14:30", pickup["pickup_time"])
			assert.Equal(t, "alone", alone["departure_mode"])
			assert.NotEmpty(t, alone["signature"])
			if today.Weekday() == time.Wednesday {
				assert.Contains(t, pending, base+"2026-11-02")
				assert.Contains(t, pending, base+"2026-11-03")
			}
		})
	}
}
