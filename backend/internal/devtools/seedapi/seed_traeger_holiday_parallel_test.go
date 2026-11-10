package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// A closing day an earlier seed run of the same Träger put into a booked
// holiday week (#6434) refuses a new week with that day (#4702). The parallel
// draft then moves on a week, like the period itself, instead of failing the
// second demo school's seed.
func TestSeedParallelHolidayDraftMovesPastADayWithoutCare(t *testing.T) {
	t.Parallel()
	var starts []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "/traeger/holiday-periods/", r.URL.Path)
		var body struct {
			Weeks []struct {
				StartsOn string `json:"starts_on"`
			} `json:"weeks"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Len(t, body.Weeks, 2)
		assert.Equal(t, body.Weeks[0].StartsOn, body.Weeks[1].StartsOn, "both schools plan the same week")
		starts = append(starts, body.Weeks[0].StartsOn)
		if len(starts) == 1 {
			w.WriteHeader(http.StatusConflict)
			_, _ = fmt.Fprint(w, `{"status":409,"code":"enrollment.holiday_week_no_care_day","detail":"week 1: closing day"}`)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":7}}`)
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	today := calendar.NewDate(2026, 11, 10)
	require.NoError(t, seedParallelHolidayDraft(client, AuthRef{Kind: AuthBearer, Token: "traeger-token"}, today, []int64{11, 12}))
	require.Len(t, starts, 2)
	first, err := calendar.ParseDate(starts[0])
	require.NoError(t, err)
	assert.Equal(t, first.AddDays(7).String(), starts[1], "the draft moves on one week")
}
