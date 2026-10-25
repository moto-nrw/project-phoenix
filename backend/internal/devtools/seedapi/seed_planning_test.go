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

func TestSeedPlanningDemoStepCreatesRealPlanningFlows(t *testing.T) {
	t.Parallel()

	var paths []string
	var templates []map[string]any
	deviations := map[string]map[string]any{}
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/timetable/templates" {
			var template map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&template))
			templates = append(templates, template)
		}
		if strings.HasSuffix(r.URL.Path, "/deviations") {
			var deviation map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&deviation))
			deviations[r.URL.Path] = deviation
		}
		if r.URL.Path == "/api/timetable/instances" {
			assert.Equal(t, todaySeedDate().AddDays(1).String(), r.URL.Query().Get("from"), "deviations planned ahead leave today alone")
		}
		switch r.URL.Path {
		case "/api/timetable/planning-tracks":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":31}}`)
		case "/api/timetable/templates":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"template_id":41,"schedule_ids":[51,52],"instances_created":2}}`)
		case "/api/timetable/instances":
			// A Tuesday the third staff member leads alone, then three days
			// with the second one too.
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"instances":[{"id":61,"title":"Frühbetreuung","activity_group_id":41,"staff":[{"staff_id":28}]},{"id":62,"title":"Frühbetreuung","activity_group_id":41,"staff":[{"staff_id":27},{"staff_id":28}]},{"id":63,"title":"Frühbetreuung","activity_group_id":41,"staff":[{"staff_id":27},{"staff_id":28}]},{"id":64,"title":"Frühbetreuung","activity_group_id":41,"staff":[{"staff_id":27},{"staff_id":28}]}]}}`)
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
		10: 38, 11: 39, 12: 40, 13: 41, 14: 42, 15: 43,
	}
	fs.groupIDs = map[string]int64{"sternengruppe": 23}
	fs.roomIDs = map[string]int64{"OGS-Raum 1": 24, "Mensa": 44, "Aula": 46}
	fs.categoryIDs = map[string]int64{"Gruppenraum": 25}
	fs.staffIDs = map[string]int64{"Anna Müller": 26, "Katarzyna Schmidt": 27, "Sabine Weber": 28, "Svenja Fischer": 29, "Claudia Wagner": 45}

	err := (seedPlanningDemoStep{}).Run(t.Context(), &Runtime{FixedSeeder: fs, Client: fs.client})
	require.NoError(t, err)

	// The day Abholart (#3945) is set for the day the seeder runs; the path
	// names that day, which the comparison below must not depend on.
	for i, path := range paths {
		if strings.HasPrefix(path, "/api/students/21/departure-exceptions/") {
			paths[i] = "/api/students/21/departure-exceptions/{today}"
		}
		if strings.HasPrefix(path, "/api/students/32/departure-exceptions/") {
			paths[i] = "/api/students/32/departure-exceptions/{weekday}"
		}
	}

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
		"/api/students/38/arrival-schedules",
		"/api/students/39/arrival-schedules",
		"/api/students/40/arrival-schedules",
		"/api/students/41/arrival-schedules",
		"/api/students/42/arrival-schedules",
		"/api/students/43/arrival-schedules",
		"/api/students/21/arrival-exceptions",
		"/api/students/21/arrival-notes",
		"/api/students/22/pickup-exceptions",
		"/api/students/22/pickup-notes",
		"/api/students/22/pickup-notes",
		"/api/students/21/departure-exceptions/{today}",
		// #3946: one-day changes of the Gehzeit and the Abholart for today
		// mark two Kinderkarten.
		"/api/students/33/pickup-exceptions",
		"/api/students/32/departure-exceptions/{weekday}",
		"/api/timetable/periods/bootstrap",
		"/api/timetable/planning-tracks",
		"/api/timetable/templates",
		"/api/timetable/templates",
		"/api/timetable/templates",
		"/api/timetable/templates",
		"/api/timetable/templates",
		"/api/timetable/templates",
		"/api/timetable/instances",
		"/api/timetable/instances/64/deviations",
		"/api/timetable/instances/63",
		"/api/timetable/instances/62/deviations",
		"/api/timetable/templates",
		"/api/timetable/templates",
		"/api/students/35/pickup-exceptions",
		"/api/students/37/pickup-schedules",
	}, paths)
	require.Len(t, templates, 8)
	monday := seedDate{Time: mostRecentWeekday(todaySeedDate().UTCMidnight(), time.Monday)}.String()
	assert.Equal(t, "care", templates[0]["type"])
	assert.Equal(t, "gruppe", templates[0]["target_group_type"])
	assert.EqualValues(t, 23, templates[0]["education_group_id"])
	assert.EqualValues(t, 31, templates[0]["planning_track_id"])
	assert.Equal(t, "edge_hours", templates[0]["list_kind"])
	assert.NotEmpty(t, templates[0]["weekday_assignments"])
	// The Frühbetreuung is led by the two colleagues in the Frühdienst, and
	// Felix (21) and Noah (32) of the screenshots come after school.
	assert.Equal(t, []any{float64(27), float64(28)}, templates[0]["staff_ids"])
	assert.NotContains(t, templates[0]["student_ids"], float64(21))
	assert.NotContains(t, templates[0]["student_ids"], float64(32))
	assert.Len(t, templates[0]["student_ids"], 12)
	assert.Equal(t, "jahrgang", templates[1]["target_group_type"])
	assert.Equal(t, "learning_time", templates[1]["list_kind"])
	assert.Equal(t, "klasse", templates[2]["target_group_type"])
	assert.Equal(t, "activity", templates[2]["list_kind"])
	// The week's blocks start on Monday, so the whole week shows them.
	for _, index := range []int{0, 1, 2, 4, 5} {
		assert.Equal(t, monday, templates[index]["start_date"], templates[index]["name"])
		assert.Equal(t, monday, templates[index]["materialize_from"], templates[index]["name"])
	}
	// #3594: holiday care runs in the Herbstferien only, closing days
	// included, and is not planned into this week.
	start, end := holidayCareDays(todaySeedDate())
	assert.Equal(t, "Ferienbetreuung", templates[3]["name"])
	assert.Equal(t, true, templates[3]["include_closing_days"])
	assert.Equal(t, start.String(), templates[3]["start_date"])
	assert.Equal(t, end.String(), templates[3]["end_date"])
	assert.Nil(t, templates[3]["materialize_from"])
	// #3822: duties without children; the Busaufsicht waits with the bus
	// children in the Aula and lies in the fourth staff member's Spätdienst.
	assert.Equal(t, "Essensausgabe", templates[4]["name"])
	assert.Equal(t, "duty", templates[4]["type"])
	assert.EqualValues(t, 44, templates[4]["room_id"])
	assert.EqualValues(t, 2, templates[4]["required_staff"])
	assert.Equal(t, "Busaufsicht", templates[5]["name"])
	assert.Equal(t, "duty", templates[5]["type"])
	assert.EqualValues(t, 46, templates[5]["room_id"], "a Dienst shown without a place reads as unplanned")
	assert.Nil(t, templates[5]["student_ids"])
	assert.Equal(t, []any{float64(29)}, templates[5]["staff_ids"])
	// #3261: an afternoon block with children and an office appointment
	// without children, which the later-pickup choice must leave out.
	assert.Equal(t, "Freies Spiel", templates[6]["name"])
	assert.Equal(t, "none", templates[6]["target_group_type"])
	assert.Equal(t, []any{float64(38), float64(39), float64(40), float64(41), float64(42), float64(43)}, templates[6]["student_ids"])
	assert.Equal(t, []any{float64(1), float64(2), float64(3), float64(4), float64(5)}, templates[6]["weekdays"])
	assert.Equal(t, "Teamsitzung", templates[7]["name"])
	assert.Nil(t, templates[7]["student_ids"])
	assert.Equal(t, []any{float64(3)}, templates[7]["weekdays"], "the team meets on Wednesdays only")
	// The coming days: the last Frühbetreuung is cancelled, and the second
	// staff member is at a Fortbildung on the first one he leads, covered by
	// the colleague in the Frühdienst for the same reason.
	assert.Equal(t, map[string]any{"cancel": true, "cancel_reason": "Teamfortbildung"}, deviations["/api/timetable/instances/64/deviations"])
	substitution := deviations["/api/timetable/instances/62/deviations"]["substitutions"].([]any)[0].(map[string]any)
	assert.EqualValues(t, 27, substitution["absent_staff_id"])
	assert.EqualValues(t, 45, substitution["substitute_staff_id"])
	assert.Equal(t, "Vertretung wegen Fortbildung", substitution["reason"])
	absence := deviations["/api/timetable/instances/62/deviations"]["absences"].([]any)[0].(map[string]any)
	assert.Equal(t, "Fortbildung", absence["reason"])
}

func TestHolidayCareDaysAreTheHerbstferienWithinTheSchoolYear(t *testing.T) {
	t.Parallel()
	at := func(value string) seedDate {
		day, err := time.Parse(seedDateLayout, value)
		require.NoError(t, err)
		return seedDate{Time: day}
	}
	days := func(value string) [2]string {
		first, last := holidayCareDays(at(value))
		return [2]string{first.String(), last.String()}
	}
	assert.Equal(t, [2]string{"2026-10-12", "2026-10-23"}, days("2026-09-23"))
	assert.Equal(t, [2]string{"2027-07-19", "2027-07-30"}, days("2027-07-01"))
	assert.Equal(t, [2]string{"2027-07-31", "2027-07-31"}, days("2027-07-14"), "Ferien after the school year shrink to its last day")
	assert.Equal(t, [2]string{"2027-08-16", "2027-08-27"}, days("2027-08-01"))
}

// The arrival plans follow the Frühbetreuung: its twelve children arrive
// before school, the children of the screenshots after it.
func TestEarlyCareStudentsLeaveOutTheScreenshotChildren(t *testing.T) {
	t.Parallel()
	var early []int
	for index := range 20 {
		if isEarlyCareStudent(index) {
			early = append(early, index)
		}
	}
	assert.Equal(t, []int{1, 2, 3, 5, 6, 7, 8, 9, 10, 11, 12, 13}, early)
}

func TestSeedPlanningDemoStepRequiresPlanningReferences(t *testing.T) {
	t.Parallel()

	fs := NewFixedSeeder(newTestClient("http://127.0.0.1", false), false, "")
	err := (seedPlanningDemoStep{}).Run(t.Context(), &Runtime{FixedSeeder: fs, Client: fs.client})
	require.ErrorContains(t, err, "planning demo")
}
