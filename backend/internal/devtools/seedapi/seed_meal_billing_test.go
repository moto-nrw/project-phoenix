package api

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedMealBillingReleasesPopulatedMonth(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ today, previous string }{
		{"2026-11-09", "2026-10-01"},
		{"2027-01-01", "2026-12-01"},
		{"2026-03-31", "2026-02-01"},
		{"2028-03-01", "2028-02-01"},
	} {
		t.Run(tc.today, func(t *testing.T) {
			today, err := calendar.ParseDate(tc.today)
			require.NoError(t, err)
			previous, err := calendar.ParseDate(tc.previous)
			require.NoError(t, err)
			testSeedMealBillingMonth(t, today, previous)
		})
	}
}

func testSeedMealBillingMonth(t *testing.T, today, previous calendar.Date) {
	meals := map[PastMealDay]bool{}
	var releases int
	var present []PastPresenceDay
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/traeger/care-meal-tariffs/":
			_, _ = fmt.Fprint(w, `{"data":{"schools":[],"tariffs":[]}}`)
		case "/api/time-tracking/holidays", "/api/timetable/closing-days/", "/api/timetable/periods/":
			_, _ = fmt.Fprint(w, `{"data":[]}`)
		case "/api/students":
			_, _ = fmt.Fprint(w, `{"data":[{"id":22}],"pagination":{"total_pages":1}}`)
		case "/api/iot/":
			_, _ = fmt.Fprintf(w, `{"data":[{"id":9,"device_id":%q,"device_type":"terminal"}]}`, DemoDevices[0].DeviceID)
		case "/api/meal-billing-months/":
			status := "open"
			if releases > 0 {
				status = "released"
			}
			_, _ = fmt.Fprintf(w, `{"data":[{"month":"2099-01-01","status":"open"},{"month":%q,"status":%q}]}`, previous.String(), status)
		case "/api/meal-billing-months/" + previous.String():
			count := 0
			if len(meals) > 0 {
				count = 1
			}
			_, _ = fmt.Fprintf(w, `{"data":{"child_count":%d,"meal_count":%d,"fingerprint":"counts"}}`, count, len(meals))
		case "/api/meal-billing-months/" + previous.String() + "/release":
			var body struct {
				Children int `json:"child_count"`
				Meals    int `json:"meal_count"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, 1, body.Children)
			assert.Positive(t, body.Meals, "register past meals before releasing the month")
			assert.Equal(t, len(present), body.Meals)
			releases++
			_, _ = fmt.Fprint(w, `{"data":{}}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	defer srv.Close()
	rt := &Runtime{Client: newTestClient(srv.URL, false), Bootstrap: &bootstrapSeedState{SchoolID: 42, AdminEmail: "admin@example.test"}, FixedSeeder: &FixedSeeder{seedDay: seedDateAt(today.BerlinMidnight())}, Values: map[string]any{seedTraegerSessionKey: seedTraegerSessionRef{email: "admin@example.test"}}}
	recordRegularLunchDays(rt, 22, []int{1, 2, 3, 4, 5})
	step := seedMealBillingStep{seeder: &Seeder{options: SeedOptions{
		PresenceHistory: func(_ context.Context, school int64, days []PastPresenceDay) (int, error) {
			assert.EqualValues(t, 42, school)
			present = days
			for _, day := range days {
				assert.False(t, day.Date.Before(previous))
				assert.True(t, day.Date.Before(today))
			}
			return len(days), nil
		},
		MealHistory: func(_ context.Context, _ int64, days []PastMealDay) error {
			for _, day := range days {
				meals[day] = true
			}
			return nil
		},
	}}}
	require.NoError(t, step.Run(context.Background(), rt))
	count := len(meals)
	require.Positive(t, count)
	require.NoError(t, step.Run(context.Background(), rt))
	assert.Len(t, meals, count)
	assert.Equal(t, 1, releases)
}
