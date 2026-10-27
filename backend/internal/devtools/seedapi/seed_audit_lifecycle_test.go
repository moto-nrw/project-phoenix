package api

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedAttendanceCorrectionUsesCompletedInstanceFlow(t *testing.T) {
	t.Parallel()

	var paths []string
	var correction map[string]any
	var instance map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/timetable/instances" {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&instance))
		}
		if r.URL.Path == "/api/timetable/instances/71/students/21/correction" {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&correction))
		}
		if r.URL.Path == "/api/timetable/instances" {
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"71"}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
	})
	defer srv.Close()

	fs := NewFixedSeeder(newTestClient(srv.URL, false), false, "")
	fs.studentIDByIndex = map[int]int64{0: 21}
	fs.staffIDs = map[string]int64{"Anna Müller": 26}
	fs.roomIDs = map[string]int64{"OGS-Raum 1": 24}

	err := seedAttendanceCorrection(&Runtime{FixedSeeder: fs, Client: fs.client})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"/api/timetable/instances",
		"/api/timetable/instances/71/start",
		"/api/timetable/instances/71/complete",
		"/api/timetable/instances/71/students/21/correction",
	}, paths)
	assert.Equal(t, "Nachträglich ergänzt.", correction["note"])
	assert.Equal(t, "Dokumentation vervollständigt", correction["reason"])
	date, err := time.Parse(seedDateLayout, instance["date"].(string))
	require.NoError(t, err)
	assert.NotEqual(t, time.Saturday, date.Weekday())
	assert.NotEqual(t, time.Sunday, date.Weekday())
	assert.Equal(t, previousSchoolDay(todaySeedDate()).String(), instance["date"], "the finished block lies before today")
	assert.Equal(t, "Hausaufgabenhilfe", instance["title"])
}

// The documented block is the school day before today: today's Tagesplan
// shows no finished block before the day began.
func TestPreviousSchoolDaySkipsTheWeekend(t *testing.T) {
	t.Parallel()
	at := func(value string) seedDate {
		day, err := parseSeedDate(value)
		require.NoError(t, err)
		return day
	}
	assert.Equal(t, "2026-10-19", previousSchoolDay(at("2026-10-20")).String(), "Tuesday: Monday")
	assert.Equal(t, "2026-10-16", previousSchoolDay(at("2026-10-19")).String(), "Monday: Friday")
	assert.Equal(t, "2026-10-23", previousSchoolDay(at("2026-10-25")).String(), "Sunday: Friday")
	assert.Equal(t, "2027-08-02", previousSchoolDay(at("2027-08-02")).String(), "the new school year starts without a day before it")
}
