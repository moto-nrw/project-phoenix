package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Run 42532710545 seeded from 23:58 into the next Berlin day (#5511): the
// steps before the history recorded the open attendance of 08.11., then the
// history read the clock again and wrote 08.11. as a past day over it. The
// history ends before the day the school's seed started, whatever the clock
// says when the step runs.
func TestPastCareInputEndsBeforeTheSeedDay(t *testing.T) {
	t.Parallel()

	var holidayQueries []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/time-tracking/holidays":
			holidayQueries = append(holidayQueries, r.URL.RawQuery)
			_, _ = fmt.Fprint(w, `{"status":"success","data":[]}`)
		case "/api/timetable/closing-days/", "/api/timetable/periods/":
			_, _ = fmt.Fprint(w, `{"status":"success","data":[]}`)
		case "/api/students":
			_, _ = fmt.Fprint(w, `{"status":"success","data":[{"id":2}],"pagination":{"total_pages":1}}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()

	// The seed started at 23:58 on 08.11.2026; the test itself runs on any
	// later day, like the step after midnight.
	started := time.Date(2026, time.November, 8, 23, 58, 0, 0, calendar.Berlin)
	rt := &Runtime{
		Client:      newTestClient(srv.URL, false),
		TenantAuth:  AuthRef{Kind: AuthBearer, Token: "tenant"},
		FixedSeeder: &FixedSeeder{seedDay: seedDateAt(started)},
	}

	input, err := pastCareInputFor(rt, demoProfileDefinition{})
	require.NoError(t, err)
	seedDay := calendar.NewDate(2026, time.November, 8)
	assert.Equal(t, seedDay, input.today)
	assert.Equal(t, seedDay.AddDays(-presenceHistoryDays), input.from)
	assert.Equal(t, []string{fmt.Sprintf("from=%s&to=2026-11-07", seedDay.AddDays(-presenceHistoryDays))}, holidayQueries,
		"the closed days are read up to the day before the seed day")

	plan := planPresenceHistory(presenceHistoryInput{
		today: input.today, from: input.from, careDays: func(calendar.Date) bool { return true },
		students: input.students, deviceID: 7,
		sessions: []presenceHistorySession{{activityID: 50, roomID: 90, capacity: 25}},
	})
	for _, day := range plan.days {
		assert.True(t, day.Date.Before(seedDay), "history day %s is not before the seed day", day.Date)
	}
}

func TestHistoryTodayNeedsTheSeedDay(t *testing.T) {
	t.Parallel()

	_, err := (&Runtime{}).historyToday()
	require.Error(t, err)
	_, err = (&Runtime{FixedSeeder: &FixedSeeder{}}).historyToday()
	require.Error(t, err)
}
