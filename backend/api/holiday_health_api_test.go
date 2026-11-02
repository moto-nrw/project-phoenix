// Router-level tests for the health information of the holiday
// registrations (#4722): sealed at rest, opened in the Träger portal only
// with the right "Gesundheitsangaben sehen", shared with the hosting OGS only
// for registrations under the consent texts of version 2, until the last day
// of the week, carried into the health field of a holiday guest until its
// contacts go, shown beside the health field of the school's own child, and
// every read in the access log.
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

type hostHealthBody struct {
	Allergies   string `json:"allergies"`
	Medication  string `json:"medication"`
	HealthNotes string `json:"health_notes"`
	Answers     []struct {
		Key   string `json:"key"`
		Label string `json:"label"`
		Value any    `json:"value"`
	} `json:"answers"`
}

type healthParticipantBody struct {
	FirstName string          `json:"first_name"`
	Health    *hostHealthBody `json:"health"`
}

// registerWithHealth submits Mila (a child of the OGS Nord) and Finn (a
// holiday guest) Sonne for week 1 under the consent texts of version 2, with
// health information and an answer to the template's shared health question.
func registerWithHealth(t *testing.T, f holidayGuestFixture) int64 {
	t.Helper()
	body := siblingsRegistration(f.offer, "familie.sonne@example.org")
	body["consent_version"] = enrollment.HolidayConsentVersionHostHealth
	body["children"] = []map[string]any{
		{
			"first_name": "Mila", "last_name": "Sonne", "birth_date": "2018-04-02", "home_school_id": f.offer.Schools[0].ID,
			"allergies": "Erdnüsse-MILA", "custom_data": map[string]any{"tetanus_impfschutz": "ja"},
			"bookings": []map[string]any{{"week_id": f.offer.Weeks[0].ID, "time_model_id": f.offer.Weeks[0].TimeModels[1].ID}},
		},
		{
			"first_name": "Finn", "last_name": "Sonne", "birth_date": "2016-01-05", "home_school_id": 0,
			"medication": "Asthmaspray-FINN", "health_notes": "Trägt eine Brille",
			"bookings": []map[string]any{{"week_id": f.offer.Weeks[0].ID, "time_model_id": f.offer.Weeks[0].TimeModels[0].ID}},
		},
	}
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(f.link), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID
}

// traegerMemberToken signs another member of the office's Träger in with role.
func traegerMemberToken(t *testing.T, f holidayGuestFixture, role string) string {
	t.Helper()
	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.office.organization, account.ID, role)
	code, answer := traegerSignIn(t, f.traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	return answer.AccessToken
}

func participantsByName(t *testing.T, f holidayGuestFixture, token string, week int64) map[string]healthParticipantBody {
	t.Helper()
	rec := f.get(t, token, fmt.Sprintf("/%d/participants", week))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	list := decodeSchoolPortalData[struct {
		Participants []healthParticipantBody `json:"participants"`
	}](t, rec.Body.Bytes())
	byName := map[string]healthParticipantBody{}
	for _, participant := range list.Participants {
		byName[participant.FirstName] = participant
	}
	return byName
}

func healthAccessCount(t *testing.T, f holidayGuestFixture, school int64, resourceType string) int {
	t.Helper()
	var count int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM audit.data_access_log WHERE tenant_id = ? AND resource_type = ?`,
		school, resourceType).Scan(t.Context(), &count))
	return count
}

func TestTraegerSeesHealthInformationOnlyWithTheRight(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Gesundheit Recht")
	registration := registerWithHealth(t, f)
	path := fmt.Sprintf("%s/registrations/%d/health", holidayPeriodPath(f.periodID(t)), registration)

	for _, role := range []string{identityaccess.TraegerRoleBilling, identityaccess.TraegerRolePersonnel, identityaccess.TraegerRoleReadOnly} {
		rec := traegerJSON(t, f.traeger, traegerMemberToken(t, f, role), http.MethodGet, path, nil)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s: %s", role, rec.Body.String())
		assert.NotContains(t, rec.Body.String(), "Erdnüsse")
	}

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	health := decodeSchoolPortalData[enrollment.HolidayRegistrationHealthDetail](t, rec.Body.Bytes())
	require.Len(t, health.Children, 2)
	assert.Equal(t, "Erdnüsse-MILA", health.Children[0].Allergies)
	assert.JSONEq(t, `"ja"`, string(health.Children[0].CustomData["tetanus_impfschutz"]))
	assert.Equal(t, "Asthmaspray-FINN", health.Children[1].Medication)

	var actions []string
	require.NoError(t, f.db.NewRaw(`SELECT action FROM enrollment.holiday_registration_audit
		WHERE holiday_registration_id = ? ORDER BY id`, registration).Scan(t.Context(), &actions))
	assert.Equal(t, []string{"health"}, actions, "the read of the health information stands in the audit trail; refused reads read nothing")

	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Gesundheit Fremd", identityaccess.TraegerRoleAdministration)
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, "another Träger sees nothing: %s", rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Erdnüsse")
}

func TestHostingSchoolSeesHealthInformationOfItsWeekUntilItsLastDay(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Gesundheit Liste")
	registration := registerWithHealth(t, f)
	f.confirm(t, f.siblings, "confirm")
	f.confirm(t, registration, "confirm")
	nord, sued := f.office.schools[0], f.office.schools[1]
	week := f.offer.Weeks[0].ID
	lead := f.staffOf(t, nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)

	byName := participantsByName(t, f, lead, week)
	require.NotNil(t, byName["Mila"].Health, "the hosting school sees the health information of its child")
	assert.Equal(t, "Erdnüsse-MILA", byName["Mila"].Health.Allergies)
	require.Len(t, byName["Mila"].Health.Answers, 1, "the shared health question of the form")
	assert.Equal(t, "Tetanus-Impfschutz", byName["Mila"].Health.Answers[0].Label)
	assert.Equal(t, "Ja", byName["Mila"].Health.Answers[0].Value, "a selection reads as its label")
	require.NotNil(t, byName["Finn"].Health)
	assert.Equal(t, "Asthmaspray-FINN", byName["Finn"].Health.Medication)
	assert.Nil(t, byName["Lia"].Health, "a registration under the consent of before shares no health information")
	assert.Nil(t, byName["Ben"].Health)
	var healthEntries string
	require.NoError(t, f.db.NewRaw(`SELECT metadata->>'health_participants' FROM audit.data_access_log
		WHERE tenant_id = ? AND resource_type = 'holiday_participants' ORDER BY id DESC LIMIT 1`, nord).Scan(t.Context(), &healthEntries))
	assert.Equal(t, "2", healthEntries, "the access log names how many children's health information the list showed")

	noHealthRight := f.staffOf(t, nord, permissions.ConfigRead)
	assert.Nil(t, participantsByName(t, f, noHealthRight, week)["Mila"].Health,
		"without users:read the list leaves the health information out")

	rec := f.get(t, lead, fmt.Sprintf("/%d/participants/export?format=csv", week))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, value := range []string{"Erdnüsse-MILA", "Asthmaspray-FINN", "Brille"} {
		assert.NotContains(t, rec.Body.String(), value, "the export carries no health information")
	}

	other := f.staffOf(t, sued, permissions.ConfigRead, permissions.UsersRead)
	rec = f.get(t, other, fmt.Sprintf("/%d/participants", week))
	assert.Equal(t, http.StatusNotFound, rec.Code, "another school of the same Träger sees nothing: %s", rec.Body.String())

	f.today.Store(calendar.NewDate(2099, 7, 11))
	rec = f.get(t, lead, fmt.Sprintf("/%d/participants", week))
	assert.Equal(t, http.StatusNotFound, rec.Code, "after the last day of the week the list and its health information are gone")
	assert.NotContains(t, rec.Body.String(), "Erdnüsse")
}

func TestHolidayGuestCarriesItsHealthInformationUntilItsContactsGo(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Gesundheit Gast")
	registration := registerWithHealth(t, f)
	f.confirm(t, f.siblings, "confirm")
	f.confirm(t, registration, "confirm")
	week := f.offer.Weeks[0].ID
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)

	require.Equal(t, http.StatusOK, f.assign(t, lead, week, f.group).Code)
	f.today.Store(calendar.NewDate(2099, 7, 6))
	f.reconcile(t)
	roster := f.roster(t, calendar.NewDate(2099, 7, 8))
	finn, ben := roster["Finn Sonne"], roster["Ben Muster"]
	require.NotZero(t, finn, "the confirmed guest joins the group")
	_, _, _, _, health := f.child(t, finn)
	require.NotNil(t, health, "the guest's health field carries the health information")
	assert.Contains(t, *health, "Asthmaspray-FINN")
	assert.Contains(t, *health, "Trägt eine Brille")
	_, _, _, _, benHealth := f.child(t, ben)
	assert.Nil(t, benHealth, "a guest of a registration under the consent of before gets none")

	edited := "Von der Schule ergänzt"
	setHealthInfo(t, f, finn, &edited)
	f.reconcile(t)
	_, _, _, _, health = f.child(t, finn)
	require.NotNil(t, health)
	assert.Equal(t, edited, *health, "the hourly reconciliation leaves what the team wrote alone")

	rec := f.sheet(t, lead, finn)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	sheet := decodeSchoolPortalData[struct {
		Health *hostHealthBody `json:"health"`
	}](t, rec.Body.Bytes())
	require.NotNil(t, sheet.Health, "the sheet shows the health information")
	assert.Equal(t, "Asthmaspray-FINN", sheet.Health.Medication)
	var sealed []byte
	require.NoError(t, f.db.NewRaw(`SELECT health_data FROM enrollment.holiday_guests WHERE tenant_id = ? AND student_id = ?`,
		f.nord, finn).Scan(t.Context(), &sealed))
	assert.NotEmpty(t, sealed)
	assert.False(t, strings.Contains(string(sealed), "Asthmaspray"), "the copy keeps the health information sealed")

	f.today.Store(calendar.NewDate(2099, 7, 11))
	rec = f.sheet(t, lead, finn)
	require.Equal(t, http.StatusOK, rec.Code, "the sheet stays readable after the week: %s", rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Asthmaspray", "after the last day of the week the sheet shows no health information")
	f.reconcile(t)
	_, _, _, _, health = f.child(t, finn)
	assert.Nil(t, health, "the health field goes with the end of the week")
	require.NoError(t, f.db.NewRaw(`SELECT health_data FROM enrollment.holiday_guests WHERE tenant_id = ? AND student_id = ?`,
		f.nord, finn).Scan(t.Context(), &sealed))
	assert.Nil(t, sealed, "the sealed copy goes too")

	f.today.Store(calendar.NewDate(2099, 7, 18))
	f.reconcile(t)
	assert.Equal(t, http.StatusNotFound, f.sheet(t, lead, finn).Code, "the contacts go after the read-only days")
}

func TestOwnChildKeepsItsHealthFieldAndSeesTheRegistrationBesideIt(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Gesundheit Eigen")
	registration := registerWithHealth(t, f)
	f.confirm(t, registration, "confirm")
	mila := testpkg.CreateTestStudentForTenant(t, f.db, f.nord, "Mila", "Sonne", "2a")
	_, err := f.db.ExecContext(context.Background(), `UPDATE users.persons SET birthday = '2018-04-02' WHERE id = ?`, mila.PersonID)
	require.NoError(t, err)
	school := "Schule: Laktose"
	setHealthInfo(t, f, mila.ID, &school)
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersRead)
	path := fmt.Sprintf("/students/%d/health", mila.ID)

	noHealthRight := f.staffOf(t, f.nord, permissions.ConfigRead)
	assert.Equal(t, http.StatusForbidden, f.get(t, noHealthRight, path).Code, "the health information needs users:read")

	rec := f.get(t, lead, path)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	entries := decodeSchoolPortalData[[]struct {
		WeekID int64          `json:"week_id"`
		Health hostHealthBody `json:"health"`
	}](t, rec.Body.Bytes())
	require.Len(t, entries, 1)
	assert.Equal(t, f.offer.Weeks[0].ID, entries[0].WeekID)
	assert.Equal(t, "Erdnüsse-MILA", entries[0].Health.Allergies)
	_, _, _, _, health := f.child(t, mila.ID)
	require.NotNil(t, health)
	assert.Equal(t, school, *health, "the registration does not overwrite the school's health field")
	assert.Equal(t, 1, healthAccessCount(t, f, f.nord, "holiday_student_health"), "the read stands in the access log")

	lia := testpkg.CreateTestStudentForTenant(t, f.db, f.nord, "Lia", "Muster", "2b")
	rec = f.get(t, lead, fmt.Sprintf("/students/%d/health", lia.ID))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"data":[]`, "a child without a matching registration sees nothing")
	assert.Equal(t, 1, healthAccessCount(t, f, f.nord, "holiday_student_health"), "an empty answer reads no health information")

	f.today.Store(calendar.NewDate(2099, 7, 11))
	rec = f.get(t, lead, path)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Erdnüsse", "after the last day of the week the school sees it no more")
}

// setHealthInfo writes the school's health field of a child of the OGS Nord,
// as the school's own edit leaves it.
func setHealthInfo(t *testing.T, f holidayGuestFixture, student int64, value *string) {
	t.Helper()
	_, err := f.db.ExecContext(context.Background(), `UPDATE users.student_care_profiles SET health_info = ?
		WHERE tenant_id = ? AND membership_id = (SELECT id FROM users.student_school_memberships
			WHERE student_profile_id = ? AND deleted_at IS NULL)`, value, f.nord, student)
	require.NoError(t, err)
}

func TestHolidayStudentHealthQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Gesundheit Budget")
	mila := testpkg.CreateTestStudentForTenant(t, f.db, f.nord, "Mila", "Sonne", "2a")
	_, err := f.db.ExecContext(context.Background(), `UPDATE users.persons SET birthday = '2018-04-02' WHERE id = ?`, mila.PersonID)
	require.NoError(t, err)
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersRead)
	read := func() []string {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/holiday-weeks/students/%d/health", mila.ID), nil)
		req.Header.Set("Authorization", "Bearer "+lead)
		rec := httptest.NewRecorder()
		f.school.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	none := read()
	registration := registerWithHealth(t, f)
	f.confirm(t, registration, "confirm")
	f.confirm(t, f.siblings, "confirm")
	assert.Len(t, read(), len(none), "the read is flat in the number of registrations and weeks")
	querybudgets.AssertQueryBudget(t, "api.holiday_weeks.student_health", none)
}

// testHealthCipher is the field cipher of the test binary's key ring, as the
// roots build it from HEALTH_DATA_KEYS (#4722).
func testHealthCipher() enrollment.HolidayHealthCipher {
	cipher, err := HealthDataCipher(testpkg.TestHealthDataKeys)
	if err != nil {
		panic(err)
	}
	return cipher
}
