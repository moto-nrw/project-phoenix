package api

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedShiftSeriesStepPlansAroundTheFerien(t *testing.T) {
	t.Parallel()

	var paths []string
	var ferien map[string]any
	var series []map[string]any
	var shiftTypes []map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/timetable/periods/bootstrap":
			// A school year wide enough to hold any test day.
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"periods":[`+
				`{"id":5,"period_type":"holiday","start_date":"2000-01-01","end_date":"2999-12-31","is_active":true},`+
				`{"id":6,"period_type":"school_year","start_date":"2000-01-01","end_date":"2999-12-31","is_active":true}]}}`)
		case "/api/shift-types/defaults":
			_, _ = fmt.Fprint(w, `{"status":"success","data":[{"id":81,"name":"Betreuung / Zeit am Kind"},{"id":82,"name":"Pause"}]}`)
		case "/api/shift-types/":
			var payload map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			shiftTypes = append(shiftTypes, payload)
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"id":%d}}`, 90+len(shiftTypes))
		case "/api/timetable/periods":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&ferien))
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":7}}`)
		case "/api/staff-shifts/series":
			var payload map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			series = append(series, payload)
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"series_id":"9"}}`)
		}
	})
	defer srv.Close()

	fs := NewFixedSeeder(newTestClient(srv.URL, false), false, "")
	fs.staffIDs = map[string]int64{}
	for index, staff := range DemoStaff[:4] {
		fs.staffIDs[fmt.Sprintf("%s %s", staff.FirstName, staff.LastName)] = int64(31 + index)
	}
	rt := &Runtime{Client: fs.client, FixedSeeder: fs, TenantAuth: AuthRef{Token: "admin"}}
	require.NoError(t, (seedShiftSeriesStep{}).Run(t.Context(), rt))

	assert.Equal(t, []string{
		"/api/timetable/periods/bootstrap",
		"/api/shift-types/defaults",
		"/api/shift-types/",
		"/api/shift-types/",
		"/api/timetable/periods",
		"/api/staff-shifts/series",
		"/api/staff-shifts/series",
		"/api/staff-shifts/series",
	}, paths)
	// The school's own Früh- and Spätdienst join the default Schichtarten.
	require.Len(t, shiftTypes, 2)
	assert.Equal(t, "Frühdienst", shiftTypes[0]["name"])
	assert.Equal(t, "Spätdienst", shiftTypes[1]["name"])

	assert.Equal(t, "holiday", ferien["period_type"])
	assert.Equal(t, true, ferien["is_active"])
	start, err := time.Parse(seedDateLayout, ferien["start_date"].(string))
	require.NoError(t, err)
	assert.Equal(t, time.Monday, start.Weekday(), "the Ferien start on a Monday")
	assert.Equal(t, start.AddDate(0, 0, 11).Format(seedDateLayout), ferien["end_date"], "two weeks of Herbstferien, Monday to Friday")

	require.Len(t, series, 3)
	assert.EqualValues(t, 33, series[0]["staff_id"])
	assert.Equal(t, false, series[0]["include_school_breaks"], "no holiday care: the series leaves the Ferien out")
	assert.EqualValues(t, 91, series[0]["shift_type_id"], "the Frühbetreuung is led from the Frühdienst")
	assert.Equal(t, earlyDutyStart, series[0]["start_time"])
	assert.EqualValues(t, 34, series[1]["staff_id"])
	assert.Equal(t, true, series[1]["include_school_breaks"], "this person also works in the Ferien")
	assert.EqualValues(t, 92, series[1]["shift_type_id"], "the Spätdienst covers the Busaufsicht")
	assert.Equal(t, lateDutyEnd, series[1]["end_time"])
	// The second staff member leads the Frühbetreuung too. The first has
	// single shifts next week and is left out of the series.
	assert.EqualValues(t, 32, series[2]["staff_id"])
	assert.Equal(t, earlyDutyStart, series[2]["start_time"])
	assert.Equal(t, earlyDutyEnd, series[2]["end_time"])
	assert.EqualValues(t, regularDutyBreakMinutes, series[2]["break_minutes"])
	for _, payload := range series {
		assert.EqualValues(t, 6, payload["calendar_period_id"], "the school year, not the Ferien period")
		validFrom, err := time.Parse(seedDateLayout, payload["valid_from"].(string))
		require.NoError(t, err)
		validUntil, err := time.Parse(seedDateLayout, payload["valid_until"].(string))
		require.NoError(t, err)
		assert.True(t, validFrom.Before(start), "the series starts before the Ferien")
		assert.True(t, validUntil.After(start.AddDate(0, 0, 11)), "and runs past them")
	}
}

// Everybody else who records time works the regular Dienst (#3892), with the
// Schichtart the Dienstplan chip shows.
func TestDutySeriesPlansTheRegularDienstWithItsSchichtart(t *testing.T) {
	t.Parallel()

	fs := NewFixedSeeder(newTestClient("http://127.0.0.1", false), false, "")
	fs.staffIDs = map[string]int64{}
	for index, staff := range DemoStaff[:7] {
		fs.staffIDs[fmt.Sprintf("%s %s", staff.FirstName, staff.LastName)] = int64(31 + index)
	}
	series := dutySeries(fs, orderedSeedStaffIDs(fs), seedDutyTypes{care: 81, early: 91, late: 92})
	require.Len(t, series, 6)
	assert.EqualValues(t, 35, series[3]["staff_id"], "the stand-in of seedTodaysAbsencesStep works the Frühdienst")
	assert.EqualValues(t, 91, series[3]["shift_type_id"])
	for _, payload := range series[4:] {
		assert.Equal(t, regularDutyStart, payload["start_time"])
		assert.Equal(t, regularDutyEnd, payload["end_time"])
		assert.EqualValues(t, regularDutyBreakMinutes, payload["break_minutes"])
		assert.EqualValues(t, 81, payload["shift_type_id"])
	}
}

// The series start tomorrow, so the days of the current week up to today
// are planned with single shifts: the Dienstplan and the calendar open on the
// whole week, and the simulation clocks people in from the first minute
// (#3892).
func TestCurrentWeekShiftsFillTheWeekUpToToday(t *testing.T) {
	t.Parallel()

	types := seedDutyTypes{care: 81, early: 91, late: 92}
	series := []map[string]any{{
		"staff_id": int64(33), "start_time": earlyDutyStart, "end_time": earlyDutyEnd, "break_minutes": 30,
		"shift_type_id": int64(91), "notes": "Keine Ferienbetreuung", "include_school_breaks": false, "weekdays": []int{1},
	}}

	thursday := seedDate{Time: time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)}
	shifts := currentWeekShifts(thursday, []int64{31}, series, types)
	require.Len(t, shifts, 8, "Monday to Thursday, two people")
	assert.Equal(t, map[string]any{
		"staff_id": int64(31), "date": "2026-10-05",
		"start_time": lateDutyStart, "end_time": lateDutyEnd, "break_minutes": regularDutyBreakMinutes, "shift_type_id": int64(92),
	}, shifts[0], "the first staff member's Freies Spiel runs until 17:00")
	assert.Equal(t, map[string]any{
		"staff_id": int64(33), "date": "2026-10-05",
		"start_time": earlyDutyStart, "end_time": earlyDutyEnd, "break_minutes": 30, "shift_type_id": int64(91),
		"notes": "Keine Ferienbetreuung",
	}, shifts[1], "a series person works the series' hours, without series-only fields")
	assert.Equal(t, "2026-10-08", shifts[7]["date"])

	saturday := seedDate{Time: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)}
	assert.Len(t, currentWeekShifts(saturday, []int64{31}, series, types), 10, "a weekend fills the week that just ended")

	monday := seedDate{Time: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)}
	assert.Len(t, currentWeekShifts(monday, []int64{31}, series, types), 2, "on Monday only today")

	// The colleague on vacation today keeps the days before but not today.
	staffIDs := []int64{31, 32, 33, 34, 35, 36, 37}
	onVacation := append(series, map[string]any{"staff_id": int64(37), "start_time": regularDutyStart, "end_time": regularDutyEnd})
	planned := map[string]bool{}
	for _, shift := range currentWeekShifts(thursday, staffIDs, onVacation, types) {
		if shift["staff_id"] == int64(37) {
			planned[shift["date"].(string)] = true
		}
	}
	assert.Equal(t, map[string]bool{"2026-10-05": true, "2026-10-06": true, "2026-10-07": true}, planned)
}
