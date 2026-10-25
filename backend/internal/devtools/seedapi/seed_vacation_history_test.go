package api

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The summer vacation leaves 10 to 13 of 30 days, lies between the break
// block and the recent Freizeitausgleich, and skips the first staff member
// and every fifth one.
func TestPastVacationIsAStaggeredSummerBlock(t *testing.T) {
	t.Parallel()

	today := time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC) // Tuesday
	cases := []struct {
		idx         int
		first, last string
	}{
		{1, "2026-08-17", "2026-09-08"},
		{2, "2026-08-24", "2026-09-16"},
		{3, "2026-08-10", "2026-09-04"},
		{5, "2026-08-24", "2026-09-16"},
	}
	for _, tc := range cases {
		first, last, ok := pastVacation(today, tc.idx)
		require.True(t, ok, "staff %d", tc.idx)
		assert.Equal(t, tc.first, toDateKey(first), "staff %d", tc.idx)
		assert.Equal(t, tc.last, toDateKey(last), "staff %d", tc.idx)
		assert.Equal(t, time.Monday, first.Weekday(), "staff %d", tc.idx)
		assert.Equal(t, pastVacationWorkdays[tc.idx%3], weekdaysBetween(first, last), "staff %d", tc.idx)
		assert.True(t, first.After(breakSessionDay(today, nil)), "staff %d keeps the break block", tc.idx)
		assert.True(t, last.Before(today.AddDate(0, 0, -21)), "staff %d keeps the recent Fridays", tc.idx)
	}
	for _, idx := range []int{0, 4, 9} {
		_, _, ok := pastVacation(today, idx)
		assert.False(t, ok, "staff %d takes no past vacation", idx)
	}
}

// Early in the year the summer lies in the previous year, whose quota the
// seed does not set.
func TestPastVacationStaysInTheCurrentYear(t *testing.T) {
	t.Parallel()

	_, _, ok := pastVacation(time.Date(2027, time.February, 2, 0, 0, 0, 0, time.UTC), 1)
	assert.False(t, ok)
}

func TestFreeVacationRangesSplitAroundTakenDays(t *testing.T) {
	t.Parallel()

	first := time.Date(2026, time.August, 17, 0, 0, 0, 0, time.UTC) // Monday
	last := time.Date(2026, time.August, 28, 0, 0, 0, 0, time.UTC)  // Friday
	ranges := freeVacationRanges(first, last, map[string]bool{"2026-08-19": true, "2026-08-24": true})
	keys := make([][2]string, 0, len(ranges))
	for _, part := range ranges {
		keys = append(keys, [2]string{toDateKey(part[0]), toDateKey(part[1])})
	}
	assert.Equal(t, [][2]string{
		{"2026-08-17", "2026-08-18"},
		{"2026-08-20", "2026-08-23"},
		{"2026-08-25", "2026-08-28"},
	}, keys)
	assert.Empty(t, freeVacationRanges(time.Date(2026, time.August, 22, 0, 0, 0, 0, time.UTC), time.Date(2026, time.August, 23, 0, 0, 0, 0, time.UTC), nil), "a weekend alone is no vacation")
}

// The Leitung books the block around the sick day the history drew inside it.
func TestBookPastVacationBooksAroundExistingAbsences(t *testing.T) {
	t.Parallel()

	var booked []map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		assert.Equal(t, "/api/staff/7/absences", r.URL.Path)
		assert.Equal(t, "Bearer admin", r.Header.Get("Authorization"))
		if r.Method == "GET" {
			assert.Equal(t, "2026-08-17", r.URL.Query().Get("from"))
			assert.Equal(t, "2026-09-08", r.URL.Query().Get("to"))
			_, _ = fmt.Fprint(w, `{"status":"success","data":[{"date_start":"2026-08-26","date_end":"2026-08-26","absence_type":"sick"}]}`)
			return
		}
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		booked = append(booked, body)
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"id":"%d"}}`, len(booked))
	})
	defer srv.Close()

	admin := newTestClient(srv.URL, false)
	admin.BindAuth(AuthRef{Kind: AuthBearer, Token: "admin"})
	first := time.Date(2026, time.August, 17, 0, 0, 0, 0, time.UTC)
	count, err := bookPastVacation(admin, 7, first, time.Date(2026, time.September, 8, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	require.Len(t, booked, 2)
	assert.Equal(t, map[string]any{"absence_type": "vacation", "date_start": "2026-08-17", "date_end": "2026-08-25", "note": "Sommerurlaub"}, booked[0])
	assert.Equal(t, map[string]any{"absence_type": "vacation", "date_start": "2026-08-27", "date_end": "2026-09-08", "note": "Sommerurlaub"}, booked[1])
}

func weekdaysBetween(first, last time.Time) int {
	count := 0
	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		if day.Weekday() != time.Saturday && day.Weekday() != time.Sunday {
			count++
		}
	}
	return count
}
