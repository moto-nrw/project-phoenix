// Router-level tests for parallel holiday weeks (#5689): one period offers the
// same days at several schools of the Träger, and a child takes part in only
// one of the weeks that share a day.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// parallelHolidays offers the first January week at both schools of the Träger.
func parallelHolidays(office traegerOffice) holidayPeriodBody {
	body := januaryHolidays(office)
	body.Weeks[1].StartsOn, body.Weeks[1].EndsOn = body.Weeks[0].StartsOn, body.Weeks[0].EndsOn
	return body
}

func TestHolidayPeriodOffersTheSameWeekAtSeveralSchools(t *testing.T) {
	t.Parallel()
	db, router := newHolidayPeriodPortal(t)
	office := newTraegerOffice(t, db, router, "Träger Parallel", identityaccess.TraegerRoleAdministration)

	created := createHolidayPeriod(t, router, office, parallelHolidays(office))

	require.Len(t, created.Weeks, 2)
	assert.Equal(t, created.Weeks[0].StartsOn, created.Weeks[1].StartsOn)
	assert.ElementsMatch(t, office.schools, []int64{created.Weeks[0].HostSchoolID, created.Weeks[1].HostSchoolID})
}

func TestHolidayPeriodRefusesOverlappingWeeksAtOneSchool(t *testing.T) {
	t.Parallel()
	db, router := newHolidayPeriodPortal(t)
	office := newTraegerOffice(t, db, router, "Träger Doppelt", identityaccess.TraegerRoleAdministration)
	body := parallelHolidays(office)
	body.Weeks[1].HostSchoolID = body.Weeks[0].HostSchoolID

	rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/holiday-periods/", body)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_period_invalid", code)
	assert.Equal(t, []string{"weeks.1.starts_on"}, fields)
}

// oneChildRegistration registers one holiday guest for the given bookings.
func oneChildRegistration(offer holidayOfferBody, email string, bookings ...map[string]any) map[string]any {
	registration := siblingsRegistration(offer, email)
	child := registration["children"].([]map[string]any)[1]
	child["bookings"] = bookings
	registration["children"] = []map[string]any{child}
	return registration
}

func TestChildBooksOnlyOneOfTheParallelWeeks(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Standortwahl", identityaccess.TraegerRoleAdministration)
	body := parallelHolidays(office)
	body.RegistrationOpensOn, body.RegistrationClosesOn = "2020-01-01", "2099-12-31"
	created := createHolidayPeriod(t, f.traeger, office, body)
	rec := traegerJSON(t, f.traeger, office.token, http.MethodPost, holidayPeriodPath(created.ID)+"/publish", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	period := decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())
	offer := holidayOffer(t, f, period.PublicToken)
	first := map[string]any{"week_id": offer.Weeks[0].ID, "time_model_id": offer.Weeks[0].TimeModels[0].ID}
	second := map[string]any{"week_id": offer.Weeks[1].ID, "time_model_id": offer.Weeks[1].TimeModels[0].ID}

	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken),
		oneChildRegistration(offer, "beide@example.org", first, second))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	_, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, []string{"children.0.bookings.1"}, fields)

	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken),
		oneChildRegistration(offer, "eine@example.org", second))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var receipt struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &receipt))

	// The office moves the child to the other school; both at once is refused.
	base := fmt.Sprintf("/traeger/holiday-periods/%d/registrations/%d", period.ID, receipt.Data.ID)
	rec = traegerJSON(t, f.traeger, office.token, http.MethodGet, base, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodeSchoolPortalData[reviewRegistration](t, rec.Body.Bytes())
	bookings := fmt.Sprintf("%s/children/%d/bookings", base, detail.Children[0].ID)

	rec = traegerJSON(t, f.traeger, office.token, http.MethodPut, bookings, map[string]any{"bookings": []map[string]any{first, second}})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, office.token, http.MethodPut, bookings, map[string]any{"bookings": []map[string]any{first}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestBookedWeeksOfOneChildCannotMoveOntoTheSameDays(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Verschieben", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken),
		oneChildRegistration(offer, "zwei@example.org",
			map[string]any{"week_id": offer.Weeks[0].ID, "time_model_id": offer.Weeks[0].TimeModels[0].ID},
			map[string]any{"week_id": offer.Weeks[1].ID, "time_model_id": offer.Weeks[1].TimeModels[0].ID}))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, office.token, http.MethodGet, holidayPeriodPath(period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	current := decodeSchoolPortalData[holidayPeriodBody](t, rec.Body.Bytes())
	moved := current
	moved.Weeks = []holidayWeekBody{current.Weeks[0], current.Weeks[1]}
	moved.Weeks[1].StartsOn, moved.Weeks[1].EndsOn = current.Weeks[0].StartsOn, current.Weeks[0].EndsOn

	rec = traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayPeriodPath(period.ID), moved)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_weeks_booked_together", code)
	assert.Equal(t, []string{"weeks.1.starts_on"}, fields)
}

// A move stays possible while no child has booked both weeks.
func TestUnbookedWeekMovesParallelToABookedWeek(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Umzug", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken),
		oneChildRegistration(offer, "nur-erste@example.org",
			map[string]any{"week_id": offer.Weeks[0].ID, "time_model_id": offer.Weeks[0].TimeModels[0].ID}))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, office.token, http.MethodGet, holidayPeriodPath(period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	current := decodeSchoolPortalData[holidayPeriodBody](t, rec.Body.Bytes())
	moved := current
	moved.Weeks = []holidayWeekBody{current.Weeks[0], current.Weeks[1]}
	moved.Weeks[1].StartsOn, moved.Weeks[1].EndsOn = current.Weeks[0].StartsOn, current.Weeks[0].EndsOn

	rec = traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayPeriodPath(period.ID), moved)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
