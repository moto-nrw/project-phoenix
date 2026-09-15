package timetracking

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// stubPlanningCalendar is the package-local double for
// workforce.PlanningCalendar backing GET /holidays and GET /closing-days.
type stubPlanningCalendar struct {
	holidays    []workforce.PublicHoliday
	closingDays []workforce.ClosingPeriod
	err         error
}

func (s *stubPlanningCalendar) HolidaysInRange(_ context.Context, _, _ string) ([]workforce.PublicHoliday, error) {
	return s.holidays, s.err
}

func (s *stubPlanningCalendar) ClosingDaysInRange(_ context.Context, _, _ string) ([]workforce.ClosingPeriod, error) {
	return s.closingDays, s.err
}

func TestGetHolidays(t *testing.T) {
	t.Parallel()

	rs := &Resource{Calendar: &stubPlanningCalendar{holidays: []workforce.PublicHoliday{
		{Date: "2026-05-01", Name: "Tag der Arbeit"},
	}}}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/holidays?from=2026-05-01&to=2026-05-31", nil)
	rs.getHolidays(w, r)

	require.Equal(t, 200, w.Code)
	var body struct {
		Data []struct {
			Date string `json:"date"`
			Name string `json:"name"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	assert.Equal(t, "2026-05-01", body.Data[0].Date)
	assert.Equal(t, "Tag der Arbeit", body.Data[0].Name)
}

func TestGetHolidaysInvalidRange(t *testing.T) {
	t.Parallel()

	rs := &Resource{Calendar: &stubPlanningCalendar{}}

	w := httptest.NewRecorder()
	rs.getHolidays(w, httptest.NewRequest("GET", "/holidays?from=nope&to=2026-05-31", nil))
	assert.Equal(t, 400, w.Code)

	w = httptest.NewRecorder()
	rs.getHolidays(w, httptest.NewRequest("GET", "/holidays?from=2026-01-01&to=2027-12-31", nil))
	assert.Equal(t, 400, w.Code, "ranges over 400 days are rejected")
}

func TestGetHolidaysServiceError(t *testing.T) {
	t.Parallel()

	rs := &Resource{Calendar: &stubPlanningCalendar{err: errors.New("boom")}}

	w := httptest.NewRecorder()
	rs.getHolidays(w, httptest.NewRequest("GET", "/holidays?from=2026-05-01&to=2026-05-31", nil))
	assert.Equal(t, 500, w.Code)
}
