package api

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedShiftSeriesStepPlansAroundAFerienWeek(t *testing.T) {
	t.Parallel()

	var paths []string
	var ferien map[string]any
	var series []map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/timetable/periods/bootstrap":
			// A school year wide enough to hold any test day.
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"periods":[`+
				`{"id":5,"period_type":"holiday","start_date":"2000-01-01","end_date":"2999-12-31","is_active":true},`+
				`{"id":6,"period_type":"school_year","start_date":"2000-01-01","end_date":"2999-12-31","is_active":true}]}}`)
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
		"/api/timetable/periods",
		"/api/staff-shifts/series",
		"/api/staff-shifts/series",
	}, paths)

	assert.Equal(t, "holiday", ferien["period_type"])
	assert.Equal(t, true, ferien["is_active"])
	start, err := time.Parse(seedDateLayout, ferien["start_date"].(string))
	require.NoError(t, err)
	assert.Equal(t, time.Monday, start.Weekday(), "the Ferien start on a Monday")
	assert.Equal(t, start.AddDate(0, 0, 4).Format(seedDateLayout), ferien["end_date"])

	require.Len(t, series, 2)
	assert.EqualValues(t, 33, series[0]["staff_id"])
	assert.Equal(t, false, series[0]["include_school_breaks"], "no holiday care: the series leaves the Ferien out")
	assert.EqualValues(t, 34, series[1]["staff_id"])
	assert.Equal(t, true, series[1]["include_school_breaks"], "this person also works in the Ferien")
	for _, payload := range series {
		assert.EqualValues(t, 6, payload["calendar_period_id"], "the school year, not the Ferien period")
		validFrom, err := time.Parse(seedDateLayout, payload["valid_from"].(string))
		require.NoError(t, err)
		validUntil, err := time.Parse(seedDateLayout, payload["valid_until"].(string))
		require.NoError(t, err)
		assert.True(t, validFrom.Before(start), "the series starts before the Ferien")
		assert.True(t, validUntil.After(start.AddDate(0, 0, 4)), "and runs past them")
	}
}
