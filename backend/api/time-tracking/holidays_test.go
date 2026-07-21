package timetracking

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/holidays"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

type mockHolidayService struct {
	list []holidays.Holiday
	err  error
}

func (m *mockHolidayService) HolidaysInRange(_ context.Context, _, _ timezone.Date) ([]holidays.Holiday, error) {
	return m.list, m.err
}

func (m *mockHolidayService) HolidayDates(_ context.Context, _, _ timezone.Date) (map[timezone.Date]bool, error) {
	return nil, m.err
}

func TestGetHolidays(t *testing.T) {
	rs := &Resource{HolidayService: &mockHolidayService{list: []holidays.Holiday{
		{Date: timezone.NewDate(2026, 5, 1), Name: "Tag der Arbeit"},
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
	rs := &Resource{HolidayService: &mockHolidayService{}}

	w := httptest.NewRecorder()
	rs.getHolidays(w, httptest.NewRequest("GET", "/holidays?from=nope&to=2026-05-31", nil))
	assert.Equal(t, 400, w.Code)

	w = httptest.NewRecorder()
	rs.getHolidays(w, httptest.NewRequest("GET", "/holidays?from=2026-01-01&to=2027-12-31", nil))
	assert.Equal(t, 400, w.Code, "ranges over 400 days are rejected")
}

func TestGetHolidaysServiceError(t *testing.T) {
	rs := &Resource{HolidayService: &mockHolidayService{err: errors.New("boom")}}

	w := httptest.NewRecorder()
	rs.getHolidays(w, httptest.NewRequest("GET", "/holidays?from=2026-05-01&to=2026-05-31", nil))
	assert.Equal(t, 500, w.Code)
}
