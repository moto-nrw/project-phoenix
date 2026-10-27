package api

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Today has a real substitution case: the third staff member reports sick,
// the Leitung covers her Frühbetreuung and Essensausgabe with the colleague
// in the Frühdienst and leaves her afternoon block open, and the seventh
// takes a vacation day.
func TestSeedTodaysAbsencesCoversTheSickColleaguesMorning(t *testing.T) {
	t.Parallel()

	var paths []string
	var absences []map[string]any
	var deviation map[string]any
	var instancesQuery string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/staff/37/absences", "/api/staff/33/absences":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			absences = append(absences, body)
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":1}}`)
		case "/api/timetable/instances":
			instancesQuery = r.URL.RawQuery
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"instances":[`+
				`{"id":61,"title":"Frühbetreuung","staff":[{"staff_id":32},{"staff_id":33}]},`+
				`{"id":62,"title":"Essensausgabe","staff":[{"staff_id":33},{"staff_id":34}]},`+
				`{"id":63,"title":"AG Klasse 1a","staff":[{"staff_id":31},{"staff_id":33}]},`+
				`{"id":64,"title":"Busaufsicht","staff":[{"staff_id":34}]}]}}`)
		default:
			require.NoError(t, json.NewDecoder(r.Body).Decode(&deviation))
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		}
	})
	defer srv.Close()

	fs := NewFixedSeeder(newTestClient(srv.URL, false), false, "")
	for index, staff := range DemoStaff[:7] {
		fs.staffIDs[staff.FirstName+" "+staff.LastName] = int64(31 + index)
	}
	rt := &Runtime{Client: fs.client, FixedSeeder: fs, TenantAuth: AuthRef{Token: "admin"}}
	tuesday := seedDate{Time: time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC)}
	require.NoError(t, seedTodaysAbsences(rt, tuesday))

	assert.Equal(t, []string{
		"POST /api/staff/37/absences",
		"POST /api/staff/33/absences",
		"GET /api/timetable/instances",
		"POST /api/timetable/instances/61/deviations",
	}, paths)
	assert.Equal(t, "from=2026-10-20&to=2026-10-20", instancesQuery)
	require.Len(t, absences, 2)
	assert.Equal(t, "vacation", absences[0]["absence_type"])
	assert.Equal(t, "2026-10-20", absences[0]["date_start"])
	assert.Equal(t, "sick", absences[1]["absence_type"], "the sick report cancels her Dienst and marks her blocks (#1843)")
	assert.Equal(t, "2026-10-20", absences[1]["date_end"])
	substitution := deviation["substitutions"].([]any)[0].(map[string]any)
	assert.EqualValues(t, 33, substitution["absent_staff_id"])
	assert.EqualValues(t, 35, substitution["substitute_staff_id"])
	assert.Equal(t, "Vertretung wegen Krankheit", substitution["reason"])
	assert.Equal(t, []any{float64(61), float64(62)}, substitution["instance_ids"], "the afternoon block stays an open gap")
}

func TestSeedTodaysAbsencesSkipsTheWeekend(t *testing.T) {
	t.Parallel()

	srv := newSeedHTTPTestServer(func(_ seedHTTPResponseWriter, r *seedHTTPRequest) {
		t.Errorf("no request on a weekend, got %s", r.URL.Path)
	})
	defer srv.Close()
	fs := NewFixedSeeder(newTestClient(srv.URL, false), false, "")
	for index, staff := range DemoStaff[:7] {
		fs.staffIDs[staff.FirstName+" "+staff.LastName] = int64(31 + index)
	}
	saturday := seedDate{Time: time.Date(2026, time.October, 24, 0, 0, 0, 0, time.UTC)}
	require.NoError(t, seedTodaysAbsences(&Runtime{Client: fs.client, FixedSeeder: fs}, saturday))
}

func TestSeedTodaysAbsencesRequiresSevenStaff(t *testing.T) {
	t.Parallel()

	fs := NewFixedSeeder(newTestClient("http://127.0.0.1", false), false, "")
	tuesday := seedDate{Time: time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC)}
	require.ErrorContains(t, seedTodaysAbsences(&Runtime{Client: fs.client, FixedSeeder: fs}, tuesday), "need 7 staff members")
}

// A demo school seeds its day after its first tick, which has completed the
// blocks that are over. The Vertretungsplan refuses to rewrite those, so the
// Leitung covers only the block still to come.
func TestSeedTodaysAbsencesCoversOnlyBlocksNotOver(t *testing.T) {
	t.Parallel()

	var paths []string
	var deviation map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/staff/37/absences", "/api/staff/33/absences":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":1}}`)
		case "/api/timetable/instances":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"instances":[`+
				`{"id":61,"title":"Frühbetreuung","status":"completed","staff":[{"staff_id":32},{"staff_id":33}]},`+
				`{"id":62,"title":"Essensausgabe","status":"planned","staff":[{"staff_id":33},{"staff_id":34}]}]}}`)
		default:
			require.NoError(t, json.NewDecoder(r.Body).Decode(&deviation))
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
		}
	})
	defer srv.Close()

	fs := NewFixedSeeder(newTestClient(srv.URL, false), false, "")
	for index, staff := range DemoStaff[:7] {
		fs.staffIDs[staff.FirstName+" "+staff.LastName] = int64(31 + index)
	}
	tuesday := seedDate{Time: time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC)}
	require.NoError(t, seedTodaysAbsences(&Runtime{Client: fs.client, FixedSeeder: fs, TenantAuth: AuthRef{Token: "admin"}}, tuesday))

	assert.Equal(t, "POST /api/timetable/instances/62/deviations", paths[len(paths)-1])
	substitution := deviation["substitutions"].([]any)[0].(map[string]any)
	assert.Equal(t, []any{float64(62)}, substitution["instance_ids"], "the completed Frühbetreuung is not rewritten")
}

// After 13:00 both blocks are over: the sick report stays, no substitution.
func TestSeedTodaysAbsencesLeavesADayThatIsOver(t *testing.T) {
	t.Parallel()

	var paths []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/timetable/instances":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"instances":[`+
				`{"id":61,"title":"Frühbetreuung","status":"completed","staff":[{"staff_id":33}]},`+
				`{"id":62,"title":"Essensausgabe","status":"completed","staff":[{"staff_id":33}]}]}}`)
		default:
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":1}}`)
		}
	})
	defer srv.Close()

	fs := NewFixedSeeder(newTestClient(srv.URL, false), false, "")
	for index, staff := range DemoStaff[:7] {
		fs.staffIDs[staff.FirstName+" "+staff.LastName] = int64(31 + index)
	}
	tuesday := seedDate{Time: time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC)}
	require.NoError(t, seedTodaysAbsences(&Runtime{Client: fs.client, FixedSeeder: fs, TenantAuth: AuthRef{Token: "admin"}}, tuesday))

	assert.Equal(t, []string{
		"POST /api/staff/37/absences",
		"POST /api/staff/33/absences",
		"GET /api/timetable/instances",
	}, paths)
}
