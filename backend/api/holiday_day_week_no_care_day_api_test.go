// Router-level test of a week by days with a statutory holiday and a closing
// day of its host school (#6447): the week is written, only those days are
// left out of the offer and refused, capacity and price count the booked
// days. A whole week with such a day stays refused (#4702).
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type noCareDayPeriodBody struct {
	ID          int64  `json:"id"`
	PublicToken string `json:"public_token"`
	Weeks       []struct {
		ID          int64  `json:"id"`
		BookingUnit string `json:"booking_unit"`
		NoCareDays  []struct {
			Date string `json:"date"`
			Kind string `json:"kind"`
		} `json:"no_care_days"`
		TimeModels []struct {
			ID int64 `json:"id"`
		} `json:"time_models"`
	} `json:"weeks"`
}

// TestHolidayWeekByDaysLeavesOutOnlyItsDaysWithoutCare: the host school's
// calendar (bound in #4702 and tested there) names Wednesday 6 January 2027
// a statutory holiday and Thursday a closing day. The week by days from Monday to Friday is
// created (red before #6447) and changed; a whole week on the same days,
// and turning the stored week into a whole week, stay refused. Parents see
// Monday, Tuesday and Friday; a booking of Wednesday is refused, one of
// Monday and Friday gets its place on both days and costs two days.
func TestHolidayWeekByDaysLeavesOutOnlyItsDaysWithoutCare(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixtureWithDays(t, owners.HolidayTestSchoolCalendar{
		{Date: calendar.NewDate(2027, 1, 6), Kind: enrollment.HolidayNoCareHoliday, Name: "Heilige Drei Könige"},
		{Date: calendar.NewDate(2027, 1, 7), Kind: enrollment.HolidayNoCareClosingDay, Name: "Betriebsausflug"},
	})
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Feiertag Tageswoche", identityaccess.TraegerRoleAdministration)
	school := office.schools[0]

	week := func(id, model int64, unit string) map[string]any {
		return map[string]any{"id": id, "starts_on": dayMonday, "ends_on": dayFriday, "host_school_id": school, "booking_unit": unit,
			"time_models": []map[string]any{{"id": model, "name": "bis 13 Uhr", "starts_at": "08:00", "ends_at": "13:00", "price_cents": 1500}}}
	}
	period := func(weeks ...map[string]any) map[string]any {
		return map[string]any{"name": "Winterferien 2027", "registration_opens_on": "2020-01-01", "registration_closes_on": "2099-12-31",
			"binding": true, "notes": "", "weeks": weeks}
	}
	noCareDay := func(rec *httptest.ResponseRecorder) {
		t.Helper()
		require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		var refusal holidayErrorBody
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &refusal))
		assert.Equal(t, "enrollment.holiday_week_no_care_day", refusal.Code)
	}

	noCareDay(traegerJSON(t, f.traeger, office.token, http.MethodPost, "/traeger/holiday-periods/", period(week(0, 0, "week"))))

	rec := traegerJSON(t, f.traeger, office.token, http.MethodPost, "/traeger/holiday-periods/", period(week(0, 0, "day")))
	require.Equal(t, http.StatusCreated, rec.Code, "a week by days with a holiday is created: %s", rec.Body.String())
	created := decodeSchoolPortalData[noCareDayPeriodBody](t, rec.Body.Bytes())
	require.Len(t, created.Weeks, 1)
	stored := created.Weeks[0]
	assert.Equal(t, "day", stored.BookingUnit)
	require.Len(t, stored.NoCareDays, 2)
	assert.Equal(t, dayWednesday, stored.NoCareDays[0].Date)
	assert.Equal(t, "holiday", stored.NoCareDays[0].Kind)
	assert.Equal(t, dayThursday, stored.NoCareDays[1].Date)
	assert.Equal(t, "closing_day", stored.NoCareDays[1].Kind)

	model := stored.TimeModels[0].ID
	noCareDay(traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayPeriodPath(created.ID), period(week(stored.ID, model, "week"))))
	rec = traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayPeriodPath(created.ID), period(week(stored.ID, model, "day")))
	require.Equal(t, http.StatusOK, rec.Code, "the week by days stays writable: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, office.token, http.MethodPost, holidayPeriodPath(created.ID)+"/publish", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayCapacityPath(created.ID, stored.ID), map[string]any{"capacity": 1})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	d := dayFixture{holidayRegistrationFixture: f, office: office, period: traegerPeriodLink{ID: created.ID, PublicToken: created.PublicToken}}
	d.offer = d.readOffer(t)
	assert.Equal(t, []string{dayMonday, dayTuesday, dayFriday}, d.offer.Weeks[0].CareDays, "parents see only the bookable days")

	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(created.PublicToken), d.registration("eva.feiertag@example.org", "Ida", d.days(dayWednesday)))
	noCareDay(rec)
	ole := d.submit(t, d.registration("ole.feiertag@example.org", "Ole", d.days(dayMonday, dayFriday)))
	assert.Equal(t, "placed", ole.Placements[0].Placement)
	assert.Equal(t, map[string]int{dayMonday: 1, dayTuesday: 0, dayWednesday: 0, dayThursday: 0, dayFriday: 1}, d.placedOn(t),
		"the booking takes a place on its days only")

	rec = traegerJSON(t, f.traeger, office.token, http.MethodPost, holidayConfirmPath(created.ID, ole.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, int64(3000), decodeSchoolPortalData[confirmationBody](t, rec.Body.Bytes()).Contribution.TotalCents,
		"two booked days at 15 €; the days without care cost nothing")
}
