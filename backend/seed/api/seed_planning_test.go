package api

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedPlanningDemoStepCreatesRealPlanningFlows(t *testing.T) {
	t.Parallel()

	var paths []string
	var templates []map[string]any
	var staffDeviation map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/timetable/templates" {
			var template map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&template))
			templates = append(templates, template)
		}
		if r.URL.Path == "/api/timetable/instances/63/deviations" {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&staffDeviation))
		}
		switch r.URL.Path {
		case "/api/timetable/planning-tracks":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":31}}`)
		case "/api/timetable/templates":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"template_id":41,"schedule_ids":[51,52],"instances_created":2}}`)
		case "/api/timetable/instances":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"instances":[{"id":61,"title":"Frühbetreuung","activity_group_id":41},{"id":62,"title":"Frühbetreuung","activity_group_id":41},{"id":63,"title":"Frühbetreuung","activity_group_id":41}]}}`)
		default:
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		}
	})
	defer srv.Close()

	fs := NewFixedSeeder(newTestClient(srv.URL, false), false, "")
	fs.guardianIDs = map[string]int64{"Ada Beispiel": 11}
	fs.studentIDByIndex = map[int]int64{
		0: 21, 1: 22, 2: 30, 3: 31, 4: 32,
		5: 33, 6: 34, 7: 35, 8: 36, 9: 37,
	}
	fs.groupIDs = map[string]int64{"sternengruppe": 23}
	fs.roomIDs = map[string]int64{"OGS-Raum 1": 24}
	fs.categoryIDs = map[string]int64{"Gruppenraum": 25}
	fs.staffIDs = map[string]int64{"Anna Müller": 26, "Thomas Schmidt": 27, "Sabine Weber": 28, "Michael Fischer": 29}

	err := (seedPlanningDemoStep{}).Run(t.Context(), &Runtime{FixedSeeder: fs, Client: fs.client})
	require.NoError(t, err)

	assert.Equal(t, []string{
		"/api/guardians/11/phone-numbers",
		"/api/students/21/arrival-schedules",
		"/api/students/22/arrival-schedules",
		"/api/students/30/arrival-schedules",
		"/api/students/31/arrival-schedules",
		"/api/students/32/arrival-schedules",
		"/api/students/33/arrival-schedules",
		"/api/students/34/arrival-schedules",
		"/api/students/35/arrival-schedules",
		"/api/students/36/arrival-schedules",
		"/api/students/37/arrival-schedules",
		"/api/students/21/arrival-exceptions",
		"/api/students/21/arrival-notes",
		"/api/students/22/pickup-exceptions",
		"/api/students/22/pickup-notes",
		"/api/timetable/periods/bootstrap",
		"/api/timetable/planning-tracks",
		"/api/timetable/templates",
		"/api/timetable/templates",
		"/api/timetable/templates",
		"/api/timetable/instances",
		"/api/timetable/instances/61/deviations",
		"/api/timetable/instances/62",
		"/api/timetable/instances/63/deviations",
	}, paths)
	require.Len(t, templates, 3)
	assert.Equal(t, "care", templates[0]["type"])
	assert.Equal(t, "gruppe", templates[0]["target_group_type"])
	assert.EqualValues(t, 23, templates[0]["education_group_id"])
	assert.EqualValues(t, 31, templates[0]["planning_track_id"])
	assert.Equal(t, "edge_hours", templates[0]["list_kind"])
	assert.NotEmpty(t, templates[0]["weekday_assignments"])
	assert.Equal(t, "jahrgang", templates[1]["target_group_type"])
	assert.Equal(t, "learning_time", templates[1]["list_kind"])
	assert.Equal(t, "klasse", templates[2]["target_group_type"])
	assert.Equal(t, "activity", templates[2]["list_kind"])
	require.NotNil(t, staffDeviation)
	absentStaffID := staffDeviation["absences"].([]any)[0].(map[string]any)["staff_id"]
	tuesdayStaffIDs := templates[0]["weekday_assignments"].([]any)[0].(map[string]any)["staff_ids"].([]any)
	assert.Contains(t, tuesdayStaffIDs, absentStaffID)
}

func TestSeedPlanningDemoStepRequiresPlanningReferences(t *testing.T) {
	t.Parallel()

	fs := NewFixedSeeder(newTestClient("http://127.0.0.1", false), false, "")
	err := (seedPlanningDemoStep{}).Run(t.Context(), &Runtime{FixedSeeder: fs, Client: fs.client})
	require.ErrorContains(t, err, "planning demo")
}
