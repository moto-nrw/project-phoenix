package api

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The catalog page (#6891) needs a template in use, an unused one and one the
// school no longer offers; the full-time template keeps the custom Soll.
func TestSeedWorkTimeTemplatesCreatesArchivesAndAssigns(t *testing.T) {
	t.Parallel()

	var calls []string
	var assignedModels []any
	nextID := 40
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" && r.URL.Path == "/api/work-time-models" {
			nextID++
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"id":%d}}`, nextID)
			return
		}
		if r.Method == "PUT" {
			assignedModels = append(assignedModels, body["model_id"])
		}
		_, _ = fmt.Fprint(w, `{"status":"success","data":{}}`)
	})
	defer srv.Close()

	staff := []StaffCredentials{
		{Name: "Extern", Email: "x@example.test", Position: "Extern"},
		{Name: seedStaffWithoutTarget, Email: "p@example.test", Position: "Pädagogische Fachkraft"},
		{Name: "A", Email: "a@example.test"}, {Name: "B", Email: "b@example.test"},
		{Name: "C", Email: "c@example.test"}, {Name: "D", Email: "d@example.test"},
	}
	ids := map[string]int64{"x@example.test": 1, "p@example.test": 2, "a@example.test": 3, "b@example.test": 4, "c@example.test": 5, "d@example.test": 6}
	require.NoError(t, seedWorkTimeTemplatesViaAPI(
		&Runtime{Client: newTestClient(srv.URL, false), TenantAuth: AuthRef{Token: "admin"}},
		staff, ids, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)))

	assert.Equal(t, []string{
		"POST /api/work-time-models", "POST /api/work-time-models", "POST /api/work-time-models",
		"POST /api/work-time-models/43/archive",
		"PUT /api/staff/3/schedule", "PUT /api/staff/4/schedule", "PUT /api/staff/5/schedule",
	}, calls, "external staff and the person without a Soll keep their state")
	assert.Equal(t, []any{float64(41), float64(41), float64(41)}, assignedModels, "the full-time template is the one in use")
	assert.Equal(t, timeTrackingDailyTargetMin, seedWorkTimeTemplates()[0].weeks[0][0], "assigning it keeps the custom Soll")
}
