package timetracking

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services/schedule/scheduletest"
)

func TestGetClosingDays(t *testing.T) {
	t.Parallel()

	days := []*schedule.ClosingDay{{
		StartDate: timezone.NewDate(2026, 12, 24),
		EndDate:   timezone.NewDate(2026, 12, 31),
		Reason:    "Weihnachtswoche",
	}}
	rs := &Resource{ClosingDayService: &scheduletest.ClosingDayServiceMock{
		ClosingDaysInRangeFn: func(_ context.Context, _, _ timezone.Date) ([]*schedule.ClosingDay, error) {
			return days, nil
		},
	}}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/closing-days?from=2026-12-01&to=2026-12-31", nil)
	rs.getClosingDays(w, r)

	require.Equal(t, 200, w.Code)
	var body struct {
		Data []struct {
			StartDate string `json:"start_date"`
			EndDate   string `json:"end_date"`
			Reason    string `json:"reason"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	assert.Equal(t, "2026-12-24", body.Data[0].StartDate)
	assert.Equal(t, "2026-12-31", body.Data[0].EndDate)
	assert.Equal(t, "Weihnachtswoche", body.Data[0].Reason)
}

func TestGetClosingDaysInvalidRange(t *testing.T) {
	t.Parallel()

	rs := &Resource{ClosingDayService: &scheduletest.ClosingDayServiceMock{}}

	w := httptest.NewRecorder()
	rs.getClosingDays(w, httptest.NewRequest("GET", "/closing-days?from=nope&to=2026-05-31", nil))
	assert.Equal(t, 400, w.Code)

	w = httptest.NewRecorder()
	rs.getClosingDays(w, httptest.NewRequest("GET", "/closing-days?from=2026-01-01&to=2027-12-31", nil))
	assert.Equal(t, 400, w.Code, "ranges over 400 days are rejected")
}

func TestGetClosingDaysServiceError(t *testing.T) {
	t.Parallel()

	rs := &Resource{ClosingDayService: &scheduletest.ClosingDayServiceMock{
		ClosingDaysInRangeFn: func(_ context.Context, _, _ timezone.Date) ([]*schedule.ClosingDay, error) {
			return nil, errors.New("boom")
		},
	}}

	w := httptest.NewRecorder()
	rs.getClosingDays(w, httptest.NewRequest("GET", "/closing-days?from=2026-05-01&to=2026-05-31", nil))
	assert.Equal(t, 500, w.Code)
}
