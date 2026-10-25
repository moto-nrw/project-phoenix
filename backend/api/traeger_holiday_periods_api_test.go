// Router-level tests for the holiday periods of a Träger (#4659): the
// Träger-Büro plans weeks at its own schools with time models and prices,
// changes and publishes them; a foreign school, a foreign Träger and a role
// without the right "Anmeldung verwalten" are refused.
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type holidayTimeModelBody struct {
	ID         int64  `json:"id,omitempty"`
	Name       string `json:"name"`
	StartsAt   string `json:"starts_at"`
	EndsAt     string `json:"ends_at"`
	PriceCents int64  `json:"price_cents"`
}

type holidayWeekBody struct {
	ID           int64                  `json:"id,omitempty"`
	StartsOn     string                 `json:"starts_on"`
	EndsOn       string                 `json:"ends_on"`
	HostSchoolID int64                  `json:"host_school_id"`
	TimeModels   []holidayTimeModelBody `json:"time_models"`
}

type holidayPeriodBody struct {
	ID                   int64             `json:"id,omitempty"`
	Name                 string            `json:"name"`
	RegistrationOpensOn  string            `json:"registration_opens_on"`
	RegistrationClosesOn string            `json:"registration_closes_on"`
	Binding              bool              `json:"binding"`
	Notes                string            `json:"notes"`
	Status               string            `json:"status,omitempty"`
	PublishedAt          *string           `json:"published_at,omitempty"`
	Weeks                []holidayWeekBody `json:"weeks"`
}

type holidayErrorBody struct {
	Code   string `json:"code"`
	Errors []struct {
		Field string `json:"field"`
	} `json:"errors"`
}

// newHolidayPeriodPortal mounts /traeger as the serving root does.
func newHolidayPeriodPortal(t *testing.T) (*bun.DB, *traegerPortalHandler) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	portal, _ := traegerPortalOn(t, db, testpkg.TenantRuntime(t, db))
	return db, portal
}

// traegerOffice is a Träger with two schools and a signed-in person of its
// Träger-Büro holding role.
type traegerOffice struct {
	organization int64
	schools      []int64
	token        string
}

func newTraegerOffice(t *testing.T, db *bun.DB, router *traegerPortalHandler, name, role string) traegerOffice {
	t.Helper()
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, name, true)
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	schools := []int64{
		testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS Nord"),
		testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS Süd"),
	}
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, role)
	code, answer := traegerSignIn(t, router, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	return traegerOffice{organization: organization, schools: schools, token: answer.AccessToken}
}

func traegerJSON(t *testing.T, router http.Handler, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
	}
	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func holidayPeriodPath(id int64) string {
	return "/traeger/holiday-periods/" + strconv.FormatInt(id, 10)
}

func januaryHolidays(office traegerOffice) holidayPeriodBody {
	return holidayPeriodBody{
		Name:                 "Winterferien 2027",
		RegistrationOpensOn:  "2026-11-02",
		RegistrationClosesOn: "2026-12-04",
		Binding:              true,
		Notes:                "Mittagessen gibt es vor Ort.",
		Weeks: []holidayWeekBody{
			{StartsOn: "2027-01-04", EndsOn: "2027-01-08", HostSchoolID: office.schools[0], TimeModels: []holidayTimeModelBody{
				{Name: "bis 13 Uhr", StartsAt: "8:00", EndsAt: "13:00", PriceCents: 4500},
				{Name: "bis 16 Uhr", StartsAt: "08:00", EndsAt: "16:00", PriceCents: 6750},
			}},
			{StartsOn: "2027-01-11", EndsOn: "2027-01-15", HostSchoolID: office.schools[1], TimeModels: []holidayTimeModelBody{
				{Name: "bis 13 Uhr", StartsAt: "08:00", EndsAt: "13:00", PriceCents: 4500},
			}},
		},
	}
}

func createHolidayPeriod(t *testing.T, router http.Handler, office traegerOffice, body holidayPeriodBody) holidayPeriodBody {
	t.Helper()
	rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/holiday-periods/", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[holidayPeriodBody](t, rec.Body.Bytes())
}

func TestTraegerHolidayPeriodIsPlannedChangedAndPublished(t *testing.T) {
	t.Parallel()
	db, router := newHolidayPeriodPortal(t)
	office := newTraegerOffice(t, db, router, "Träger Bissendorf", identityaccess.TraegerRoleAdministration)

	created := createHolidayPeriod(t, router, office, januaryHolidays(office))
	assert.Equal(t, "draft", created.Status)
	assert.Nil(t, created.PublishedAt)
	require.Len(t, created.Weeks, 2)
	assert.Equal(t, office.schools[0], created.Weeks[0].HostSchoolID)
	assert.Equal(t, "2027-01-04", created.Weeks[0].StartsOn)
	require.Len(t, created.Weeks[0].TimeModels, 2)
	assert.Equal(t, holidayTimeModelBody{ID: created.Weeks[0].TimeModels[0].ID, Name: "bis 13 Uhr", StartsAt: "08:00", EndsAt: "13:00", PriceCents: 4500},
		created.Weeks[0].TimeModels[0], "times are normalized to HH:MM, prices stay whole cent")
	assert.Equal(t, int64(6750), created.Weeks[0].TimeModels[1].PriceCents)

	// Change the price of the first week's first time model, drop its
	// second one and the second week, and add a third week.
	change := created
	first := change.Weeks[0]
	first.TimeModels = []holidayTimeModelBody{{ID: first.TimeModels[0].ID, Name: "bis 13 Uhr", StartsAt: "08:00", EndsAt: "13:00", PriceCents: 5000}}
	change.Weeks = []holidayWeekBody{first, {StartsOn: "2027-01-18", EndsOn: "2027-01-22", HostSchoolID: office.schools[1], TimeModels: []holidayTimeModelBody{
		{Name: "bis 16 Uhr", StartsAt: "08:00", EndsAt: "16:00", PriceCents: 7000},
	}}}
	change.Name = "Winterferien 2027 (geändert)"
	rec := traegerJSON(t, router, office.token, http.MethodPut, holidayPeriodPath(created.ID), change)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	updated := decodeSchoolPortalData[holidayPeriodBody](t, rec.Body.Bytes())
	assert.Equal(t, "Winterferien 2027 (geändert)", updated.Name)
	require.Len(t, updated.Weeks, 2)
	assert.Equal(t, created.Weeks[0].ID, updated.Weeks[0].ID, "a named week keeps its ID")
	require.Len(t, updated.Weeks[0].TimeModels, 1)
	assert.Equal(t, created.Weeks[0].TimeModels[0].ID, updated.Weeks[0].TimeModels[0].ID, "a named time model keeps its ID")
	assert.Equal(t, int64(5000), updated.Weeks[0].TimeModels[0].PriceCents)
	assert.Equal(t, "2027-01-18", updated.Weeks[1].StartsOn)
	assert.NotEqual(t, created.Weeks[1].ID, updated.Weeks[1].ID, "the left-out week is gone")

	rec = traegerJSON(t, router, office.token, http.MethodPost, holidayPeriodPath(created.ID)+"/publish", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	published := decodeSchoolPortalData[holidayPeriodBody](t, rec.Body.Bytes())
	assert.Equal(t, "published", published.Status)
	assert.NotNil(t, published.PublishedAt)

	// A published period stays complete: a change without weeks, or with a
	// week without time models, is refused and changes nothing.
	emptied := published
	emptied.Weeks = nil
	rec = traegerJSON(t, router, office.token, http.MethodPut, holidayPeriodPath(created.ID), emptied)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	noModels := published
	noModels.Weeks = []holidayWeekBody{published.Weeks[0], published.Weeks[1]}
	noModels.Weeks[1].TimeModels = nil
	rec = traegerJSON(t, router, office.token, http.MethodPut, holidayPeriodPath(created.ID), noModels)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	var refusal holidayErrorBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &refusal))
	assert.Equal(t, "enrollment.holiday_period_incomplete", refusal.Code)

	rec = traegerJSON(t, router, office.token, http.MethodGet, "/traeger/holiday-periods/", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	listed := decodeSchoolPortalData[[]holidayPeriodBody](t, rec.Body.Bytes())
	require.Len(t, listed, 1)
	assert.Equal(t, "published", listed[0].Status)
	assert.Len(t, listed[0].Weeks, 2)
}

func TestTraegerHolidayPeriodAcceptsOnlySchoolsOfTheTraeger(t *testing.T) {
	t.Parallel()
	db, router := newHolidayPeriodPortal(t)
	office := newTraegerOffice(t, db, router, "Träger Eigen", identityaccess.TraegerRoleAdministration)
	foreign := traegerOrganization(t, db, "Träger Fremd", true)
	foreignSchool := testpkg.CreateTestSchoolInOrganization(t, db, foreign, "OGS Fremd")

	body := januaryHolidays(office)
	body.Weeks[1].HostSchoolID = foreignSchool
	rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/holiday-periods/", body)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	var refusal holidayErrorBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &refusal))
	assert.Equal(t, "enrollment.holiday_host_school_outside_traeger", refusal.Code)
	require.Len(t, refusal.Errors, 1)
	assert.Equal(t, "weeks.1.host_school_id", refusal.Errors[0].Field, "the refusal names the week's host school field")

	rec = traegerJSON(t, router, office.token, http.MethodGet, "/traeger/holiday-periods/", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decodeSchoolPortalData[[]holidayPeriodBody](t, rec.Body.Bytes()), "the refused period is rolled back with its first week")

	// A school the Träger had, now deleted, cannot host a week either.
	created := createHolidayPeriod(t, router, office, januaryHolidays(office))
	testpkg.SoftDeleteTestSchool(t, db, office.schools[1])
	change := created
	change.Weeks[0].HostSchoolID = office.schools[1]
	rec = traegerJSON(t, router, office.token, http.MethodPut, holidayPeriodPath(created.ID), change)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &refusal))
	assert.Equal(t, "enrollment.holiday_host_school_outside_traeger", refusal.Code)
}

func TestTraegerHolidayPeriodsStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	db, router := newHolidayPeriodPortal(t)
	own := newTraegerOffice(t, db, router, "Träger Hardt", identityaccess.TraegerRoleAdministration)
	other := newTraegerOffice(t, db, router, "Träger Willich", identityaccess.TraegerRoleAdministration)
	created := createHolidayPeriod(t, router, own, januaryHolidays(own))

	rec := traegerJSON(t, router, other.token, http.MethodGet, "/traeger/holiday-periods/", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decodeSchoolPortalData[[]holidayPeriodBody](t, rec.Body.Bytes()), "another Träger sees no period of this one")

	for _, attempt := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, holidayPeriodPath(created.ID), nil},
		{http.MethodPut, holidayPeriodPath(created.ID), januaryHolidays(other)},
		{http.MethodPost, holidayPeriodPath(created.ID) + "/publish", nil},
	} {
		rec := traegerJSON(t, router, other.token, attempt.method, attempt.path, attempt.body)
		require.Equal(t, http.StatusNotFound, rec.Code, "%s %s: %s", attempt.method, attempt.path, rec.Body.String())
		var refusal holidayErrorBody
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &refusal))
		assert.Equal(t, "enrollment.holiday_period_not_found", refusal.Code)
	}

	rec = traegerJSON(t, router, own.token, http.MethodGet, holidayPeriodPath(created.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	unchanged := decodeSchoolPortalData[holidayPeriodBody](t, rec.Body.Bytes())
	assert.Equal(t, "draft", unchanged.Status, "the foreign publication changed nothing")
	assert.Equal(t, own.schools[0], unchanged.Weeks[0].HostSchoolID, "the foreign update changed nothing")
}

func TestTraegerHolidayPeriodsNeedTheEnrollmentRight(t *testing.T) {
	t.Parallel()
	db, router := newHolidayPeriodPortal(t)
	office := newTraegerOffice(t, db, router, "Träger Lesend", identityaccess.TraegerRoleReadOnly)
	billing := newTraegerOffice(t, db, router, "Träger Abrechnung", identityaccess.TraegerRoleBilling)

	for _, token := range []string{office.token, billing.token} {
		rec := traegerJSON(t, router, token, http.MethodGet, "/traeger/holiday-periods/", nil)
		assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		rec = traegerJSON(t, router, token, http.MethodPost, "/traeger/holiday-periods/", januaryHolidays(office))
		assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	}
}

func TestTraegerHolidayPeriodValidatesAndPublishesOnlyWhenComplete(t *testing.T) {
	t.Parallel()
	db, router := newHolidayPeriodPortal(t)
	office := newTraegerOffice(t, db, router, "Träger Prüfung", identityaccess.TraegerRoleAdministration)

	invalid := januaryHolidays(office)
	invalid.RegistrationClosesOn = "2026-10-01"
	invalid.Weeks[1].StartsOn = "2027-01-08"
	invalid.Weeks[1].EndsOn = "2027-01-12"
	invalid.Weeks[0].TimeModels[1].EndsAt = "07:00"
	invalid.Weeks[0].TimeModels[0].PriceCents = -1
	rec := traegerJSON(t, router, office.token, http.MethodPost, "/traeger/holiday-periods/", invalid)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	var refusal holidayErrorBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &refusal))
	assert.Equal(t, "enrollment.holiday_period_invalid", refusal.Code)
	fields := []string{}
	for _, field := range refusal.Errors {
		fields = append(fields, field.Field)
	}
	assert.ElementsMatch(t, []string{
		"registration_closes_on",
		"weeks.0.time_models.0.price_cents",
		"weeks.0.time_models.1.ends_at",
		"weeks.1.starts_on",
	}, fields)

	draft := januaryHolidays(office)
	draft.Weeks = nil
	created := createHolidayPeriod(t, router, office, draft)
	rec = traegerJSON(t, router, office.token, http.MethodPost, holidayPeriodPath(created.ID)+"/publish", nil)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &refusal))
	assert.Equal(t, "enrollment.holiday_period_incomplete", refusal.Code)

	withoutTimeModel := januaryHolidays(office)
	withoutTimeModel.Weeks[1].TimeModels = nil
	created = createHolidayPeriod(t, router, office, withoutTimeModel)
	rec = traegerJSON(t, router, office.token, http.MethodPost, holidayPeriodPath(created.ID)+"/publish", nil)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
}

func TestTraegerHolidayPeriodListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	db, router := newHolidayPeriodPortal(t)
	office := newTraegerOffice(t, db, router, "Träger Budget Ferien", identityaccess.TraegerRoleAdministration)
	createHolidayPeriod(t, router, office, januaryHolidays(office))
	list := func() []string {
		counter := testpkg.CaptureQueriesForContext(t, db)
		req := httptest.NewRequest(http.MethodGet, "/traeger/holiday-periods/", nil)
		req.Header.Set("Authorization", "Bearer "+office.token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	one := list()
	second := januaryHolidays(office)
	second.Name = "Osterferien 2027"
	createHolidayPeriod(t, router, office, second)
	assert.Len(t, list(), len(one), "the period list reads the same statements for any number of periods")
	testpkg.AssertQueryBudget(t, "api.traeger.holiday_periods", one)
}
