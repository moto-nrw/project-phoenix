package studentpresencehttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	studentpresencehttp "github.com/moto-nrw/project-phoenix/modules/studentpresence/http"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingPresenceHistory captures what the route hands the owner and the
// school it writes in.
type recordingPresenceHistory struct {
	tenantID int64
	days     []studentpresence.PastPresenceDay
	err      error
}

func (h *recordingPresenceHistory) RecordPresenceHistory(ctx context.Context, days []studentpresence.PastPresenceDay) (int, error) {
	id, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, err
	}
	h.tenantID, h.days = id.Int64(), days
	if h.err != nil {
		return 0, h.err
	}
	return len(days), nil
}

func demoPresenceHistoryRequest(t *testing.T, history studentpresence.PresenceHistory, schoolID string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	router.Post("/schools/{id}/demo-presence-history", studentpresencehttp.DemoPresenceHistoryHandler(history))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/schools/"+schoolID+"/demo-presence-history", bytes.NewReader(body)))
	return rr
}

func demoPresenceHistoryBody(t *testing.T, days ...map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"days": days})
	require.NoError(t, err)
	return raw
}

// The demo seeder's days reach the owner in the school of the path, with
// their visit, and the route reports how many it wrote (ADR 0127).
func TestDemoPresenceHistoryWritesTheDaysInTheSchoolOfThePath(t *testing.T) {
	t.Parallel()
	history := &recordingPresenceHistory{}
	checkIn := calendar.NewDate(2026, time.October, 20).BerlinMidnight().Add(8 * time.Hour)
	rr := demoPresenceHistoryRequest(t, history, "42", demoPresenceHistoryBody(t, map[string]any{
		"student_id": 7, "date": "2026-10-20", "device_id": 3,
		"check_in": checkIn, "check_out": checkIn.Add(7 * time.Hour),
		"visit": map[string]any{
			"activity_group_id": 50, "room_id": 90,
			"session_start": checkIn, "session_end": checkIn.Add(2 * time.Hour),
			"entry": checkIn.Add(time.Hour), "exit": checkIn.Add(90 * time.Minute),
		},
	}))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.JSONEq(t, `{"written":1}`, string(mustData(t, rr)))
	assert.Equal(t, int64(42), history.tenantID)
	require.Len(t, history.days, 1)
	day := history.days[0]
	assert.Equal(t, calendar.NewDate(2026, time.October, 20), day.Date)
	assert.Equal(t, [2]int64{7, 3}, [2]int64{day.StudentID, day.DeviceID})
	assert.True(t, day.CheckIn.Equal(checkIn) && day.CheckOut.Equal(checkIn.Add(7*time.Hour)))
	require.NotNil(t, day.Visit)
	assert.Equal(t, [2]int64{50, 90}, [2]int64{day.Visit.ActivityGroupID, day.Visit.RoomID})
	assert.True(t, day.Visit.Entry.Equal(checkIn.Add(time.Hour)) && day.Visit.Exit.Equal(checkIn.Add(90*time.Minute)))
}

// A day the owner rejects is the caller's mistake, not a server failure, and
// malformed bodies never reach the owner.
func TestDemoPresenceHistoryRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	tooMany := make([]map[string]any, 4001)
	for i := range tooMany {
		tooMany[i] = map[string]any{"student_id": i + 1, "date": "2026-10-20", "device_id": 3}
	}
	cases := map[string]struct {
		school string
		body   []byte
		reject error
	}{
		"owner rejects the day": {school: "42", body: demoPresenceHistoryBody(t, map[string]any{"student_id": 7, "date": "2026-10-20", "device_id": 3}),
			reject: fmt.Errorf("%w: student 7 leaves before arriving", studentpresence.ErrPresenceHistoryInvalid)},
		"invalid date":      {school: "42", body: demoPresenceHistoryBody(t, map[string]any{"student_id": 7, "date": "20.10.2026", "device_id": 3})},
		"unknown field":     {school: "42", body: []byte(`{"days":[],"tenant_id":1}`)},
		"second JSON value": {school: "42", body: []byte(`{"days":[]}{"days":[]}`)},
		"trailing bytes":    {school: "42", body: []byte(`{"days":[]}not-json`)},
		"too many days":     {school: "42", body: demoPresenceHistoryBody(t, tooMany...)},
		"body too large":    {school: "42", body: []byte(`{"days":[` + strings.Repeat(" ", 4<<20) + `]}`)},
		"invalid school":    {school: "0", body: demoPresenceHistoryBody(t)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			history := &recordingPresenceHistory{err: tc.reject}
			rr := demoPresenceHistoryRequest(t, history, tc.school, tc.body)
			routetest.AssertErrorResponse(t, rr, http.StatusBadRequest)
			if tc.reject == nil {
				assert.Nil(t, history.days, "the owner is not called")
			}
		})
	}
}

// Any other failure of the owner is a server error.
func TestDemoPresenceHistoryReportsOwnerFailures(t *testing.T) {
	t.Parallel()
	history := &recordingPresenceHistory{err: fmt.Errorf("database unavailable")}
	rr := demoPresenceHistoryRequest(t, history, "42", demoPresenceHistoryBody(t))
	routetest.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

func mustData(t *testing.T, rr *httptest.ResponseRecorder) json.RawMessage {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
	return envelope.Data
}
