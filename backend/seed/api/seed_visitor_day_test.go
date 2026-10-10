package api

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedVisitorDayStepFillsTheVisitorsDay(t *testing.T) {
	t.Parallel()

	var templates, appointments []map[string]any
	var person map[string]any
	var personPath string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		switch r.URL.Path {
		case "/api/timetable/templates":
			templates = append(templates, body)
		case "/api/calendar/appointments":
			appointments = append(appointments, body)
		default:
			personPath, person = r.URL.Path, body
		}
		_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
	})
	defer srv.Close()

	fs := NewFixedSeeder(newTestClient(srv.URL, false), false, "")
	fs.groupIDs = map[string]int64{"sternengruppe": 5}
	fs.roomIDs = map[string]int64{"OGS-Raum 2": 6, "Leseecke": 7, "Kreativraum": 8}
	fs.categoryIDs = map[string]int64{"Gruppenraum": 9, "Hausaufgaben": 10}
	for index := range 40 {
		fs.studentIDByIndex[index] = int64(100 + index)
	}
	visitor, colleague := DemoStaff[visitorStaffIndex], DemoStaff[birthdayColleagueIndex]
	fs.staffIDs = map[string]int64{
		visitor.FirstName + " " + visitor.LastName:     70,
		colleague.FirstName + " " + colleague.LastName: 71,
	}
	rt := &Runtime{Client: fs.client, FixedSeeder: fs, TenantAuth: AuthRef{Token: "admin"}}
	require.NoError(t, (seedVisitorDayStep{}).Run(t.Context(), rt))

	// With Mittagessen every weekday and "Lernzeit Jahrgang 1" on Monday,
	// Wednesday and Friday, the visitor has four blocks on each weekday.
	perWeekday := map[int]int{1: 2, 2: 1, 3: 2, 4: 1, 5: 2}
	require.Len(t, templates, 4)
	for _, template := range templates {
		assert.Equal(t, []any{float64(70)}, template["staff_ids"], template["name"])
		assert.EqualValues(t, 70, template["primary_staff_id"])
		assert.NotZero(t, template["room_id"], template["name"])
		assert.NotZero(t, template["category_id"], template["name"])
		for _, weekday := range template["weekdays"].([]any) {
			perWeekday[int(weekday.(float64))]++
		}
	}
	for weekday := 1; weekday <= 5; weekday++ {
		assert.Equal(t, 4, perWeekday[weekday], "weekday %d", weekday)
	}
	assert.EqualValues(t, 10, templates[0]["category_id"], "Lernzeit in the Hausaufgaben category")
	assert.EqualValues(t, 9, templates[1]["category_id"], "a missing category falls back to Gruppenraum")

	// The appointments fall into the Monday-to-Friday week of the seed day.
	monday := mostRecentWeekday(todaySeedDate().Time, time.Monday)
	require.Len(t, appointments, 3)
	for _, appointment := range appointments {
		day, err := time.Parse(seedDateLayout, appointment["start_date"].(string))
		require.NoError(t, err)
		assert.False(t, day.Before(monday), appointment["title"])
		assert.True(t, day.Before(monday.AddDate(0, 0, 5)), appointment["title"])
	}

	assert.Equal(t, "/api/staff/71/stammdaten/person", personPath)
	assert.Equal(t, colleague.FirstName, person["first_name"])
	assert.Equal(t, colleague.LastName, person["last_name"])
	assert.Equal(t, todaySeedDate().Format("01-02"), person["birthday"].(string)[5:])
}

func TestWithVisitorStaffAddsTheVisitorOnce(t *testing.T) {
	t.Parallel()

	fs := NewFixedSeeder(newTestClient("http://127.0.0.1", false), false, "")
	assert.Equal(t, []int64{1, 2}, withVisitorStaff(fs, []int64{1, 2}), "no visitor staff member, nothing added")

	visitor := DemoStaff[visitorStaffIndex]
	fs.staffIDs[visitor.FirstName+" "+visitor.LastName] = 9
	assert.Equal(t, []int64{1, 9}, withVisitorStaff(fs, []int64{1}))
	assert.Equal(t, []int64{9, 1}, withVisitorStaff(fs, []int64{9, 1}))
}
