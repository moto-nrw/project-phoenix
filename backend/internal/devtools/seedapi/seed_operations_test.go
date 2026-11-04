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

func TestSeedOperationsDemoStepCreatesOperationalPlanningData(t *testing.T) {
	t.Parallel()

	var paths []string
	var shifts []map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/staff-shifts" {
			var shift map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&shift))
			shifts = append(shifts, shift)
		}
		switch r.URL.Path {
		case "/api/shift-types/defaults":
			_, _ = fmt.Fprint(w, `{"status":"success","data":[{"id":81,"name":"Betreuung"},{"id":82,"name":"Verfügungszeit"}]}`)
		default:
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":91}}`)
		}
	})
	defer srv.Close()

	fs := NewFixedSeeder(newTestClient(srv.URL, false), false, "")
	fs.staffIDs = map[string]int64{"Anna Müller": 17}
	rt := &Runtime{Client: fs.client, FixedSeeder: fs, TenantAuth: AuthRef{Token: "admin"}}
	require.NoError(t, (seedOperationsDemoStep{}).Run(t.Context(), rt))

	want := []string{
		"/api/settings/values/operations.meal_plan_enabled",
		"/api/settings/values/operations.meal_registration_enabled",
		"/api/timetable/closing-days",
	}
	for _, day := range demoMealDates(todaySeedDate()) {
		want = append(want, "/api/meal-plan/"+day)
	}
	want = append(want, "/api/shift-types/defaults", "/api/staff-shifts", "/api/staff-shifts")
	assert.Equal(t, want, paths)
	require.Len(t, shifts, 2)
	assert.EqualValues(t, 17, shifts[0]["staff_id"])
	assert.EqualValues(t, 81, shifts[0]["shift_type_id"])
	// The second Schichtart lands in the same week, the day after.
	assert.EqualValues(t, 17, shifts[1]["staff_id"])
	assert.EqualValues(t, 82, shifts[1]["shift_type_id"])
	first, err := time.Parse(seedDateLayout, shifts[0]["date"].(string))
	require.NoError(t, err)
	assert.Equal(t, first.AddDate(0, 0, 1).Format(seedDateLayout), shifts[1]["date"])
}

// demoMealDates are the ten weekdays seedMealPlan fills for today: Monday
// to Friday of today's week and of the week after it.
func demoMealDates(today seedDate) []string {
	monday := seedDate{Time: mostRecentWeekday(today.Time, time.Monday)}
	dates := make([]string, 0, 10)
	for _, offset := range []int{0, 1, 2, 3, 4, 7, 8, 9, 10, 11} {
		dates = append(dates, monday.AddDays(offset).String())
	}
	return dates
}

// The meal plan opens on the current week, also on a weekend (#3894): a
// Saturday seed fills the week that just ended and the coming one, never only
// the next Monday.
func TestSeedMealPlanFillsCurrentAndNextWeek(t *testing.T) {
	t.Parallel()
	week := []string{"2026-09-21", "2026-09-22", "2026-09-23", "2026-09-24", "2026-09-25",
		"2026-09-28", "2026-09-29", "2026-09-30", "2026-10-01", "2026-10-02"}
	for _, day := range []int{21, 23, 25, 26, 27} {
		t.Run(fmt.Sprint(day), func(t *testing.T) {
			t.Parallel()
			var paths []string
			srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
				assert.Equal(t, "PUT", r.Method)
				paths = append(paths, strings.TrimPrefix(r.URL.Path, "/api/meal-plan/"))
				_, _ = fmt.Fprint(w, `{"status":"success"}`)
			})
			defer srv.Close()
			rt := &Runtime{Client: newTestClient(srv.URL, false)}
			today := seedDate{Time: time.Date(2026, time.September, day, 0, 0, 0, 0, time.UTC)}
			require.NoError(t, seedMealPlan(rt, today))
			assert.Equal(t, week, paths)
		})
	}
}
