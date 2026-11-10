package enrollmenthttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The public offer names the days without care of each week (#4702), so the
// parents' form can refuse the week before the server does; an unmarked week
// answers an empty list, never null.
func TestHolidayOfferNamesTheDaysWithoutCare(t *testing.T) {
	t.Parallel()
	day := capability.HolidayNoCareDay{Date: calendar.NewDate(2026, 12, 4), Kind: capability.HolidayNoCareClosingDay, Name: "Betriebsausflug"}
	response := holidayOfferResponse(capability.HolidayRegistrationLink{}, capability.HolidayRegistrationOffer{Weeks: []capability.HolidayWeek{
		{ID: 3}, {ID: 4, NoCareDays: []capability.HolidayNoCareDay{day}},
	}}, "")
	raw, err := json.Marshal(response)
	require.NoError(t, err)
	var decoded struct {
		Weeks []struct {
			NoCareDays []map[string]string `json:"no_care_days"`
		} `json:"weeks"`
	}
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Len(t, decoded.Weeks, 2)
	assert.NotNil(t, decoded.Weeks[0].NoCareDays)
	assert.Empty(t, decoded.Weeks[0].NoCareDays)
	assert.Equal(t, []map[string]string{{"date": "2026-12-04", "kind": "closing_day", "name": "Betriebsausflug"}}, decoded.Weeks[1].NoCareDays)
}

// A refused week answers 409 with the code and the day it names.
func TestHolidayWeekWithoutCareAnswersItsDay(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	handled := renderHolidayWeekWithoutCare(rec, req, &capability.HolidayWeekNoCareDayRefusal{
		Week: 1, WeekID: 4, Day: capability.HolidayNoCareDay{Date: calendar.NewDate(2026, 12, 4), Kind: capability.HolidayNoCareHoliday, Name: "Feiertag"},
	})
	require.True(t, handled)
	assert.Equal(t, http.StatusConflict, rec.Code)
	var body struct {
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "enrollment.holiday_week_no_care_day", body.Code)
	assert.Equal(t, "04.12.2026", body.Details["date"])
	assert.Equal(t, "Feiertag", body.Details["name"])
	assert.False(t, renderHolidayWeekWithoutCare(httptest.NewRecorder(), req, capability.ErrHolidayRegistrationClosed))
}

func TestForeignHolidayHostSchoolRefusalHasAValidationCode(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	renderHolidayPeriodError(rec, req, capability.ErrHolidayHostSchoolOutsideTraeger)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var body struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "enrollment.holiday_host_school_outside_traeger", body.Code)
}
