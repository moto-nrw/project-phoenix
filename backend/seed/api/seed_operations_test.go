package api

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedOperationsDemoStepCreatesOperationalPlanningData(t *testing.T) {
	t.Parallel()

	var paths []string
	var shift map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/staff-shifts" {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&shift))
		}
		switch r.URL.Path {
		case "/api/shift-types/defaults":
			_, _ = fmt.Fprint(w, `{"status":"success","data":[{"id":81,"name":"Betreuung"}]}`)
		default:
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":91}}`)
		}
	})
	defer srv.Close()

	fs := NewFixedSeeder(newTestClient(srv.URL, false), false, "")
	fs.staffIDs = map[string]int64{"Anna Müller": 17}
	rt := &Runtime{Client: fs.client, FixedSeeder: fs, TenantAuth: AuthRef{Token: "admin"}}
	require.NoError(t, (seedOperationsDemoStep{}).Run(t.Context(), rt))

	mealDate := todaySeedDate()
	if mealDate.Weekday() == time.Saturday || mealDate.Weekday() == time.Sunday {
		mealDate = seedDate{Time: nextWeekday(mealDate.UTCMidnight(), time.Monday)}
	}
	assert.Equal(t, []string{
		"/api/settings/values/operations.meal_plan_enabled",
		"/api/settings/values/operations.meal_registration_enabled",
		"/api/timetable/closing-days",
		"/api/meal-plan/" + mealDate.String(),
		"/api/shift-types/defaults",
		"/api/staff-shifts",
	}, paths)
	assert.EqualValues(t, 17, shift["staff_id"])
	assert.EqualValues(t, 81, shift["shift_type_id"])
}

func TestSeedMealPlanUsesTodayOrNextSchoolDay(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		day  int
		want string
	}{
		{4, "2026-09-04"},
		{5, "2026-09-07"},
		{6, "2026-09-07"},
		{7, "2026-09-07"},
	} {
		t.Run(fmt.Sprint(tt.day), func(t *testing.T) {
			t.Parallel()
			srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
				if r.URL.Path != "/api/meal-plan/"+tt.want {
					w.WriteHeader(400)
					_, _ = fmt.Fprint(w, "meal plan must use today or the next school day")
					return
				}
				assert.Equal(t, "PUT", r.Method)
				_, _ = fmt.Fprint(w, `{"status":"success"}`)
			})
			defer srv.Close()
			rt := &Runtime{Client: newTestClient(srv.URL, false)}
			today := seedDate{Time: time.Date(2026, time.September, tt.day, 0, 0, 0, 0, time.UTC)}
			require.NoError(t, seedMealPlan(rt, today))
		})
	}
}
