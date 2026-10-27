package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The public demo's sidecar records its complete bounded plan through the
// operator route (ADR 0127), so the owner commits it in one transaction.
func TestOperatorPresenceHistoryWriterPostsPlanAsOperator(t *testing.T) {
	t.Parallel()
	var sizes []int
	var auths []string
	var first presenceHistoryRouteDay
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost || r.URL.Path != "/operator/schools/42/demo-presence-history" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(seedHTTPStatusNotFound)
			return
		}
		var body struct {
			Days []presenceHistoryRouteDay `json:"days"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		if len(sizes) == 0 {
			first = body.Days[0]
		}
		sizes = append(sizes, len(body.Days))
		auths = append(auths, r.Header.Get("Authorization"))
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"written":%d}}`, len(body.Days))
	})
	defer srv.Close()

	date := calendar.NewDate(2026, time.October, 20)
	checkIn := date.BerlinMidnight().Add(8 * time.Hour)
	var deviceID int64 = 7
	days := make([]PastPresenceDay, 1100)
	for i := range days {
		days[i] = PastPresenceDay{StudentID: int64(1000 + i), Date: date, DeviceID: deviceID, CheckIn: checkIn, CheckOut: checkIn.Add(7 * time.Hour)}
	}
	days[0].Visit = &PastVisit{ActivityGroupID: 50, RoomID: 90, SessionStart: checkIn, SessionEnd: checkIn.Add(2 * time.Hour), Entry: checkIn.Add(time.Hour), Exit: checkIn.Add(90 * time.Minute)}
	rt := &Runtime{
		Client:       newTestClient(srv.URL, false),
		OperatorAuth: AuthRef{Kind: AuthBearer, Token: "operator-token"},
		TenantAuth:   AuthRef{Kind: AuthBearer, Token: "tenant-token"},
	}

	written, err := presenceHistoryWriterFor(SeedOptions{PresenceHistoryRoute: true}, rt)(context.Background(), 42, days)
	require.NoError(t, err)
	assert.Equal(t, 1100, written)
	assert.Equal(t, []int{1100}, sizes)
	assert.Equal(t, []string{"Bearer operator-token"}, auths)
	assert.Equal(t, date, first.Date)
	assert.Equal(t, deviceID, first.DeviceID)
	assert.True(t, first.CheckIn.Equal(checkIn))
	require.NotNil(t, first.Visit)
	visit := *first.Visit
	assert.Equal(t, [2]int64{50, 90}, [2]int64{visit.ActivityGroupID, visit.RoomID})
	assert.True(t, visit.SessionStart.Equal(checkIn) && visit.SessionEnd.Equal(checkIn.Add(2*time.Hour)))
	assert.True(t, visit.Entry.Equal(checkIn.Add(time.Hour)) && visit.Exit.Equal(checkIn.Add(90*time.Minute)))
}

// A bound database writer wins; without one, only the demo asks for the
// route, and every other seed leaves the past without presence.
func TestPresenceHistoryWriterForPrefersTheDatabase(t *testing.T) {
	t.Parallel()
	rt := &Runtime{}
	called := false
	bound := PresenceHistoryWriter(func(context.Context, int64, []PastPresenceDay) (int, error) {
		called = true
		return 0, nil
	})

	write := presenceHistoryWriterFor(SeedOptions{PresenceHistory: bound, PresenceHistoryRoute: true}, rt)
	_, err := write(context.Background(), 1, nil)
	require.NoError(t, err)
	assert.True(t, called)
	assert.Nil(t, presenceHistoryWriterFor(SeedOptions{}, rt))
	assert.NotNil(t, presenceHistoryWriterFor(SeedOptions{PresenceHistoryRoute: true}, rt))
}
