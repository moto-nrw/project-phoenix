// Router-level tests for the participant lists of the school that hosts a
// holiday week (#4665, ADR 0116): access only from a confirmed booking, only
// for the hosting school, only the minimum fields, the counts per time model
// and home school, the CSV and Excel export, the access log entry of every
// read, the end of the access after the week by the service clock, and the
// school permission.
package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type hostedWeekBody struct {
	ID           int64  `json:"id"`
	PeriodName   string `json:"period_name"`
	StartsOn     string `json:"starts_on"`
	EndsOn       string `json:"ends_on"`
	Participants int    `json:"participants"`
}

type participantListBody struct {
	Week       hostedWeekBody `json:"week"`
	TimeModels []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"time_models"`
	HomeSchools []struct {
		SchoolID *int64 `json:"school_id"`
		Name     string `json:"name"`
		Count    int    `json:"count"`
	} `json:"home_schools"`
	Participants []struct {
		FirstName      string `json:"first_name"`
		LastName       string `json:"last_name"`
		BirthDate      string `json:"birth_date"`
		HomeSchoolID   *int64 `json:"home_school_id"`
		HomeSchoolName string `json:"home_school_name"`
		TimeModelID    int64  `json:"time_model_id"`
	} `json:"participants"`
}

// holidayHostFixture is a published summer period of a Träger with two
// weeks: the first hosted by the OGS Nord, the second by the OGS Süd. The
// siblings Lia (OGS Nord, week 1 "bis 16 Uhr") and Ben (holiday guest, both
// weeks "bis 13 Uhr") are registered, as is Tom (OGS Süd, week 1 "bis
// 13 Uhr") in a second registration. Nothing is confirmed yet.
type holidayHostFixture struct {
	holidayRegistrationFixture
	office   traegerOffice
	offer    holidayOfferBody
	link     string
	siblings int64
	school   http.Handler
	today    *atomic.Value
}

func newHolidayHostFixture(t *testing.T, name string) holidayHostFixture {
	t.Helper()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, name, identityaccess.TraegerRoleAdministration)
	body := januaryHolidays(office)
	body.Name = "Sommerferien 2099"
	body.RegistrationOpensOn, body.RegistrationClosesOn = "2020-01-01", "2099-06-30"
	body.Weeks[0].StartsOn, body.Weeks[0].EndsOn = "2099-07-06", "2099-07-10"
	body.Weeks[1].StartsOn, body.Weeks[1].EndsOn = "2099-07-13", "2099-07-17"
	created := createHolidayPeriod(t, f.traeger, office, body)
	rec := traegerJSON(t, f.traeger, office.token, http.MethodPost, holidayPeriodPath(created.ID)+"/publish", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	link := decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())
	offer := holidayOffer(t, f, link.PublicToken)
	submit := func(body map[string]any) int64 {
		rec := publicJSON(t, f.public, http.MethodPost, registrationPath(link.PublicToken), body)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		return decodeSchoolPortalData[struct {
			ID int64 `json:"id"`
		}](t, rec.Body.Bytes()).ID
	}
	siblings := submit(siblingsRegistration(offer, "eva.muster@example.org"))
	tom := siblingsRegistration(offer, "tom.beispiel@example.org")
	tom["children"] = []map[string]any{{
		"first_name": "Tom", "last_name": "Beispiel", "birth_date": "2017-05-20", "home_school_id": offer.Schools[1].ID,
		"bookings": []map[string]any{{"week_id": offer.Weeks[0].ID, "time_model_id": offer.Weeks[0].TimeModels[0].ID}},
	}}
	submit(tom)

	today := &atomic.Value{}
	today.Store(calendar.NewDate(2099, 7, 1))
	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, f.db))
	require.NoError(t, err)
	resource := NewHolidayHostHTTP(organizations,
		owners.NewTestAuditPersistence(testpkg.NewDatabase(t, f.db), nil).DataAccessLog,
		func() calendar.Date { return today.Load().(calendar.Date) })
	unit := testpkg.TenantRuntime(t, f.db)
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithUnitOfWork(r.Context(), unit)))
		})
	})
	router.Mount("/api/holiday-weeks", resource.Router(ProtectedTenantRoutes))
	return holidayHostFixture{holidayRegistrationFixture: f, office: office, offer: offer, link: link.PublicToken, siblings: siblings, school: testpkg.SessionVerifier(router), today: today}
}

// staffOf signs a staff member of the school in, with the given permissions.
func (f holidayHostFixture) staffOf(t *testing.T, schoolID int64, perms ...string) string {
	t.Helper()
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, f.db, schoolID, "Hanna", "Ausrichtung")
	return testpkg.MintTestJWT(t, sessiontoken.AppClaims{
		ID: int(account.ID), Sub: account.Email, Username: "ausrichtung", FirstName: "Hanna", LastName: "Ausrichtung",
		Roles: []string{"admin"}, TenantID: schoolID, Permissions: perms,
	})
}

func (f holidayHostFixture) get(t *testing.T, token, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/holiday-weeks"+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.school.ServeHTTP(rec, req)
	return rec
}

func (f holidayHostFixture) weeks(t *testing.T, token string) []hostedWeekBody {
	t.Helper()
	rec := f.get(t, token, "/")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[[]hostedWeekBody](t, rec.Body.Bytes())
}

func (f holidayHostFixture) participants(t *testing.T, token string, week int64) participantListBody {
	t.Helper()
	rec := f.get(t, token, fmt.Sprintf("/%d/participants", week))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[participantListBody](t, rec.Body.Bytes())
}

func (f holidayHostFixture) confirm(t *testing.T, registration int64, action string) {
	t.Helper()
	path := fmt.Sprintf("%s/registrations/%d/%s", holidayPeriodPath(f.periodID(t)), registration, action)
	if action == "confirm" {
		// The Elternabrechnung confirms and freezes the contribution (#4664, ADR 0115).
		path = holidayConfirmPath(f.periodID(t), registration)
	}
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, path, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func (f holidayHostFixture) periodID(t *testing.T) int64 {
	t.Helper()
	var id int64
	require.NoError(t, f.db.NewRaw(`SELECT holiday_period_id FROM enrollment.holiday_registrations WHERE id = ?`, f.siblings).Scan(t.Context(), &id))
	return id
}

// accessLog lists the access log entries of the school as
// "resource_type:format:participants", oldest first.
func (f holidayHostFixture) accessLog(t *testing.T, schoolID int64) []string {
	t.Helper()
	var rows []struct {
		ResourceType string `bun:"resource_type"`
		Format       string `bun:"format"`
		Participants string `bun:"participants"`
		WeekID       int64  `bun:"week_id"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT resource_type, COALESCE(metadata->>'format', '') AS format,
			metadata->>'participants' AS participants, (metadata->>'week_id')::bigint AS week_id
		FROM audit.data_access_log WHERE tenant_id = ? AND resource_type LIKE 'holiday_participants%' ORDER BY id`, schoolID).
		Scan(t.Context(), &rows))
	entries := make([]string, len(rows))
	for i, row := range rows {
		assert.Equal(t, f.offer.Weeks[0].ID, row.WeekID)
		entries[i] = row.ResourceType + ":" + row.Format + ":" + row.Participants
	}
	return entries
}

func TestHolidayHostSeesOnlyConfirmedChildrenOfItsWeek(t *testing.T) {
	t.Parallel()
	f := newHolidayHostFixture(t, "Träger Ausrichtung")
	nord, sued := f.office.schools[0], f.office.schools[1]
	host := f.staffOf(t, nord, permissions.ConfigRead)
	week := f.offer.Weeks[0].ID

	weeks := f.weeks(t, host)
	require.Len(t, weeks, 1, "the school sees the week it hosts, not the other school's week")
	assert.Equal(t, hostedWeekBody{ID: week, PeriodName: "Sommerferien 2099", StartsOn: "2099-07-06", EndsOn: "2099-07-10"}, weeks[0])
	empty := f.participants(t, host, week)
	assert.Empty(t, empty.Participants, "without a confirmed booking the school sees no child")

	f.confirm(t, f.siblings, "confirm")
	weeks = f.weeks(t, host)
	require.Len(t, weeks, 1)
	assert.Equal(t, 2, weeks[0].Participants)
	list := f.participants(t, host, week)
	require.Len(t, list.Participants, 2, "Tom's registration is not confirmed")
	assert.Equal(t, "Ben", list.Participants[0].FirstName)
	assert.Nil(t, list.Participants[0].HomeSchoolID)
	assert.Equal(t, "Feriengast (Grundschule Melle)", list.Participants[0].HomeSchoolName)
	assert.Equal(t, "Lia", list.Participants[1].FirstName)
	assert.Equal(t, "2018-03-01", list.Participants[1].BirthDate)
	assert.Equal(t, f.offer.Schools[0].Name, list.Participants[1].HomeSchoolName)
	assert.Equal(t, f.offer.Weeks[0].TimeModels[1].ID, list.Participants[1].TimeModelID)
	require.Len(t, list.TimeModels, 2)
	assert.Equal(t, "bis 13 Uhr", list.TimeModels[0].Name)
	assert.Equal(t, 1, list.TimeModels[0].Count)
	assert.Equal(t, "bis 16 Uhr", list.TimeModels[1].Name)
	assert.Equal(t, 1, list.TimeModels[1].Count)
	require.Len(t, list.HomeSchools, 2)
	assert.Equal(t, f.offer.Schools[0].Name, list.HomeSchools[0].Name)
	assert.Equal(t, 1, list.HomeSchools[0].Count)
	assert.Nil(t, list.HomeSchools[1].SchoolID)
	assert.Equal(t, "Feriengäste", list.HomeSchools[1].Name)
	rec := f.get(t, host, fmt.Sprintf("/%d/participants", week))
	for _, private := range []string{"Erdnüsse", "eva.muster@example.org", "Oma Muster", "0151"} {
		assert.NotContains(t, rec.Body.String(), private, "only the minimum fields leave the Träger")
	}

	other := f.staffOf(t, sued, permissions.ConfigRead)
	otherWeeks := f.weeks(t, other)
	require.Len(t, otherWeeks, 1)
	assert.Equal(t, f.offer.Weeks[1].ID, otherWeeks[0].ID)
	assert.Equal(t, 1, otherWeeks[0].Participants, "the OGS Süd hosts Ben in its week")
	rec = f.get(t, other, fmt.Sprintf("/%d/participants", week))
	assert.Equal(t, http.StatusNotFound, rec.Code, "a school of the same Träger cannot open a week it does not host")
	assert.Contains(t, rec.Body.String(), "enrollment.hosted_holiday_week_not_found")

	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Ausrichtung Fremd", identityaccess.TraegerRoleAdministration)
	stranger := f.staffOf(t, foreign.schools[0], permissions.ConfigRead)
	assert.Empty(t, f.weeks(t, stranger), "a school of another Träger sees no week")
	assert.Equal(t, http.StatusNotFound, f.get(t, stranger, fmt.Sprintf("/%d/participants", week)).Code)

	noRight := f.staffOf(t, nord, permissions.StudentsRead)
	assert.Equal(t, http.StatusForbidden, f.get(t, noRight, "/").Code)
	assert.Equal(t, http.StatusForbidden, f.get(t, noRight, fmt.Sprintf("/%d/participants", week)).Code)
	assert.Equal(t, http.StatusForbidden, f.get(t, noRight, fmt.Sprintf("/%d/participants/export?format=csv", week)).Code)

	rec = f.get(t, host, fmt.Sprintf("/%d/participants/export?format=csv", week))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "teilnehmerliste-2099-07-06")
	csv := strings.TrimPrefix(rec.Body.String(), "\xEF\xBB\xBF")
	// The standard template shares the way home and the trips (#5202); the
	// siblings answered neither, so both columns stay empty.
	assert.Equal(t, strings.Join([]string{
		"Nachname;Vorname;Geburtsdatum;Heimatschule;Zeitmodell;Darf allein nach Hause gehen;Darf an Ausflügen teilnehmen",
		"Muster;Ben;12.09.2016;Feriengast (Grundschule Melle);bis 13 Uhr;;",
		"Muster;Lia;01.03.2018;" + f.offer.Schools[0].Name + ";bis 16 Uhr;;",
	}, "\n")+"\n", csv)
	rec = f.get(t, host, fmt.Sprintf("/%d/participants/export?format=xlsx", week))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Type"), "spreadsheetml")
	assert.Equal(t, http.StatusBadRequest, f.get(t, host, fmt.Sprintf("/%d/participants/export?format=pdf", week)).Code)

	assert.Equal(t, []string{
		"holiday_participants::0", "holiday_participants::2", "holiday_participants::2",
		"holiday_participants_export:csv:2", "holiday_participants_export:xlsx:2",
	}, f.accessLog(t, nord), "every read of the list and every export stands in the school's access log")
	assert.Empty(t, f.accessLog(t, sued), "a refused read is not logged")

	f.confirm(t, f.siblings, "cancel")
	assert.Empty(t, f.participants(t, host, week).Participants, "a cancelled booking ends the access at once")
}

func TestHolidayHostAccessEndsAfterTheWeek(t *testing.T) {
	t.Parallel()
	f := newHolidayHostFixture(t, "Träger Ausrichtung Ende")
	host := f.staffOf(t, f.office.schools[0], permissions.ConfigRead)
	week := f.offer.Weeks[0].ID
	f.confirm(t, f.siblings, "confirm")

	f.today.Store(calendar.NewDate(2099, 7, 10))
	assert.Len(t, f.participants(t, host, week).Participants, 2, "the last day of the week still shows the list")

	f.today.Store(calendar.NewDate(2099, 7, 11))
	assert.Empty(t, f.weeks(t, host), "after the week the school no longer sees it")
	for _, path := range []string{"/participants", "/participants/export?format=csv"} {
		rec := f.get(t, host, fmt.Sprintf("/%d%s", week, path))
		assert.Equal(t, http.StatusNotFound, rec.Code, path)
		assert.Contains(t, rec.Body.String(), "enrollment.hosted_holiday_week_not_found", path)
	}
	assert.Equal(t, []string{"holiday_participants::2"}, f.accessLog(t, f.office.schools[0]), "only the read before the end is logged")
}

func TestHolidayHostParticipantListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newHolidayHostFixture(t, "Träger Ausrichtung Budget")
	host := f.staffOf(t, f.office.schools[0], permissions.ConfigRead)
	week := f.offer.Weeks[0].ID
	read := func(path string) []string {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, "/api/holiday-weeks"+path, nil)
		req.Header.Set("Authorization", "Bearer "+host)
		rec := httptest.NewRecorder()
		f.school.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	participants := fmt.Sprintf("/%d/participants", week)
	oneWeek, oneList := read("/"), read(participants)
	f.confirm(t, f.siblings, "confirm")
	assert.Len(t, read("/"), len(oneWeek), "the weeks read the same statements for any number of children")
	assert.Len(t, read(participants), len(oneList), "the list reads the same statements for any number of children")
	testpkg.AssertQueryBudget(t, "api.holiday_weeks.list", oneWeek)
	testpkg.AssertQueryBudget(t, "api.holiday_weeks.participants", oneList)
}
