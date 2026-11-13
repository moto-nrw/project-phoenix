package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryCompose "github.com/moto-ogs/project-phoenix/modules/delivery/compose"
	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// Use the production composition and session/Träger boundary. Only the
// calendar input is controlled, so the test covers the Enrollment refusal,
// the root's port translation and the Elternabrechnung's HTTP renderer.
func parentBillingCalendarRouter(t *testing.T, f reviewFixture, days owners.HolidayTestSchoolCalendar) http.Handler {
	t.Helper()
	database, unit := testpkg.NewDatabase(t, f.db), testpkg.TenantRuntime(t, f.db)
	organizations, err := owners.NewOrganizationTenancy(database)
	require.NoError(t, err)
	settings, err := owners.BuildTraegerSettings(database, unit, slog.Default(), organizations, nil)
	require.NoError(t, err)
	delivery, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{
		DB: f.db, People: owners.EnrollmentFlowGuardianDirectory{}, Observe: func(deliveryContract.Observation) {},
	})
	require.NoError(t, err)
	_, holiday := NewParentBillingHTTP(organizations, settings, delivery.Module, testHealthCipher(), days, slog.Default())
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithUnitOfWork(r.Context(), unit)))
		})
	})
	router.Mount("/traeger/holiday-contributions", holiday.TraegerRouter(ProtectedTraegerRoutes))
	return testpkg.SessionVerifier(router)
}

func TestParentBillingRebookingPreservesCalendarRefusal(t *testing.T) {
	t.Parallel()
	for _, day := range []enrollment.HolidayNoCareDay{
		{Date: calendar.NewDate(2027, 1, 11), Kind: enrollment.HolidayNoCareHoliday, Name: "Feiertag"},
		{Date: calendar.NewDate(2027, 1, 11), Kind: enrollment.HolidayNoCareClosingDay, Name: "Betriebsausflug"},
	} {
		t.Run(day.Kind, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			f := newReviewFixture(t, "Umbuchung Kalendermeldung")
			registration := f.submitPlaces(t, "kalendermeldung@example.test", map[string][]int{"Tom": {0}})
			require.Equal(t, http.StatusOK, f.confirmWith(t, registration.ID, nil).Code)
			before := f.detail(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(registration.ID, ""), nil))
			readContribution := func() contributionBody {
				rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayContributionPath(f.period.ID, registration.ID), nil)
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				return decodeSchoolPortalData[contributionBody](t, rec.Body.Bytes())
			}
			contribution := readContribution()
			places := f.places(t)
			first, second := f.offer.Weeks[0], f.offer.Weeks[1]
			previousDay := day
			previousDay.Date = calendar.NewDate(2027, 1, 4)
			router := parentBillingCalendarRouter(t, f, owners.HolidayTestSchoolCalendar{previousDay, day})
			path := holidayRebookPath(f.period.ID, registration.ID, before.Children[0].ID)
			rebook := map[string]any{"bookings": []map[string]any{{"week_id": second.ID, "time_model_id": second.TimeModels[0].ID}}}

			rec := traegerJSON(t, router, f.office.token, http.MethodPut, path, rebook)
			assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
			var refusal struct {
				Status  int            `json:"status"`
				Code    string         `json:"code"`
				Details map[string]any `json:"details"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &refusal))
			assert.Equal(t, http.StatusConflict, refusal.Status)
			assert.Equal(t, "enrollment.holiday_week_no_care_day", refusal.Code)
			assert.Equal(t, map[string]any{
				"week": float64(0), "week_id": float64(second.ID), "date": "11.01.2027",
				"iso_date": "2027-01-11", "kind": day.Kind, "name": day.Name,
			}, refusal.Details)
			after := f.detail(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(registration.ID, ""), nil))
			assert.Equal(t, before.Children, after.Children, "a refused rebooking changes no booking")
			assert.Equal(t, contribution, readContribution(), "the frozen contribution and its positions stay")
			assert.Equal(t, places, f.places(t), "no capacity, placement or waiting-list change")

			foreign := newTraegerOffice(t, f.db, f.traeger, "Fremder Träger Kalender", identityaccess.TraegerRoleAdministration)
			rec = traegerJSON(t, router, foreign.token, http.MethodPut, path, rebook)
			require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
			var foreignRefusal holidayErrorBody
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &foreignRefusal))
			assert.Equal(t, "enrollment.holiday_period_not_found", foreignRefusal.Code)
			assert.NotContains(t, rec.Body.String(), day.Name, "a foreign office receives no calendar details")

			// The same week may be kept even after it became closed.
			keep := map[string]any{"bookings": []map[string]any{{"week_id": first.ID, "time_model_id": first.TimeModels[0].ID}}}
			rec = traegerJSON(t, router, f.office.token, http.MethodPut, path, keep)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Equal(t, contribution, readContribution())

			invalid := map[string]any{"bookings": []map[string]any{{"week_id": first.ID, "time_model_id": second.TimeModels[0].ID}}}
			rec = traegerJSON(t, router, f.office.token, http.MethodPut, path, invalid)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			var validation holidayErrorBody
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &validation))
			assert.Equal(t, "enrollment.holiday_registration_invalid", validation.Code)
		})
	}
}
