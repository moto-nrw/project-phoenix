package api

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedSchoolYearStartTurnsOnTheFirstOfAugust(t *testing.T) {
	t.Parallel()

	day := func(year int, month time.Month, d int) seedDate {
		return seedDate{Time: time.Date(year, month, d, 0, 0, 0, 0, time.UTC)}
	}
	assert.Equal(t, 2025, seedSchoolYearStart(day(2026, time.July, 31)))
	assert.Equal(t, 2026, seedSchoolYearStart(day(2026, time.August, 1)))
	assert.Equal(t, 2026, seedSchoolYearStart(day(2027, time.January, 15)))
}

func TestSeedPhaseSchoolYearLinksTheBootstrappedYear(t *testing.T) {
	t.Parallel()

	var paths []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"periods":[`+
			`{"id":5,"period_type":"holiday","start_date":"2026-08-01","end_date":"2027-07-31","is_active":true},`+
			`{"id":6,"period_type":"school_year","start_date":"2026-08-01","end_date":"2027-07-31","is_active":true}]}}`)
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false)}
	period, err := seedPhaseSchoolYear(rt, AuthRef{Token: "admin"}, 2026)
	require.NoError(t, err)

	assert.Equal(t, []string{"/api/timetable/periods/bootstrap"}, paths, "an existing school year is reused, not created twice")
	assert.EqualValues(t, 6, period.ID)

	body := map[string]any{}
	linkPhaseToPeriod(body, period)
	assert.Equal(t, map[string]any{
		"calendar_period_id": "6",
		"service_start_date": "2026-08-01",
		"service_end_date":   "2027-07-31",
	}, body)
}

func TestSeedPhaseSchoolYearCreatesTheNextYear(t *testing.T) {
	t.Parallel()

	var paths []string
	var created map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/timetable/periods/bootstrap":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"periods":[`+
				`{"id":6,"period_type":"school_year","start_date":"2026-08-01","end_date":"2027-07-31","is_active":true}]}}`)
		case "/api/timetable/periods":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&created))
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":9}}`)
		}
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false)}
	period, err := seedPhaseSchoolYear(rt, AuthRef{Token: "admin"}, 2027)
	require.NoError(t, err)

	assert.Equal(t, []string{"/api/timetable/periods/bootstrap", "/api/timetable/periods"}, paths)
	assert.Equal(t, "Schuljahr 2027/2028", created["name"])
	assert.Equal(t, "school_year", created["period_type"])
	assert.Equal(t, "2027-08-01", created["start_date"])
	assert.Equal(t, "2028-07-31", created["end_date"])
	assert.Equal(t, true, created["is_active"])
	assert.Equal(t, seedCalendarPeriod{ID: 9, Name: "Schuljahr 2027/2028", PeriodType: "school_year", StartDate: "2027-08-01", EndDate: "2028-07-31", WeekCycleLength: 1, IsActive: true}, period)
}

func TestSeedPhaseSchoolYearRejectsAnOverlappingActiveYear(t *testing.T) {
	t.Parallel()

	const overlappingSchoolYearID = 6
	var paths []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"periods":[`+
			`{"id":%d,"name":"Schuljahr 2026/2027","period_type":"school_year","start_date":"2026-09-01","end_date":"2027-08-31","week_cycle_length":1,"is_active":true}]}}`, overlappingSchoolYearID)
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false)}
	_, err := seedPhaseSchoolYear(rt, AuthRef{Token: "admin"}, 2026)
	require.EqualError(t, err, "active school year \"Schuljahr 2026/2027\" (2026-09-01 to 2027-08-31) overlaps requested school year 2026/2027")

	assert.Equal(t, []string{"/api/timetable/periods/bootstrap"}, paths)
}

func TestSeedPhaseSchoolYearActivatesAnExactInactiveYear(t *testing.T) {
	t.Parallel()

	var paths []string
	var updated map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/timetable/periods/bootstrap":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"periods":[`+
				`{"id":6,"name":"Schuljahr 2026/2027","period_type":"school_year","start_date":"2026-08-01","end_date":"2027-07-31","week_cycle_length":2,"week_cycle_anchor":"2026-08-03","is_active":false}]}}`)
		case "/api/timetable/periods/6":
			require.Equal(t, "PUT", r.Method)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&updated))
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":6}}`)
		}
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false)}
	period, err := seedPhaseSchoolYear(rt, AuthRef{Token: "admin"}, 2026)
	require.NoError(t, err)

	assert.Equal(t, []string{"/api/timetable/periods/bootstrap", "/api/timetable/periods/6"}, paths)
	assert.Equal(t, map[string]any{
		"name": "Schuljahr 2026/2027", "period_type": "school_year",
		"start_date": "2026-08-01", "end_date": "2027-07-31",
		"week_cycle_length": float64(2), "week_cycle_anchor": "2026-08-03", "is_active": true,
	}, updated)
	assert.True(t, period.IsActive)
}
