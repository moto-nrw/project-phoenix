package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/traegerview"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// traegerSchoolsToday reads GET /traeger/schools/today with the session's
// token; a counter, when given, counts the statements of the request.
func traegerSchoolsToday(t *testing.T, router http.Handler, token string, counter *testpkg.QueryCounter) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/traeger/schools/today", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if counter != nil {
		req = req.WithContext(counter.Context(req.Context()))
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// justNow is an instant of today shortly before the request, so an open stay
// that starts there belongs to the day the figures read.
func justNow() (calendar.Date, time.Time) {
	today := calendar.TodayDate()
	at := time.Now().Add(-time.Minute)
	if midnight := today.BerlinMidnight(); at.Before(midnight) {
		at = midnight
	}
	return today, at
}

func TestTraegerSchoolsTodaySumsTheFiguresOfEverySchool(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Heute", true)
	busy := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Am Bach")
	quiet := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Lindenweg")
	foreignOrganization := traegerOrganization(t, db, "Träger Fremd Heute", true)
	foreign := testpkg.CreateTestSchoolInOrganization(t, db, foreignOrganization, "OGS Fremd Heute")
	today, at := justNow()

	staff := testpkg.CreateTestStaffForTenant(t, db, busy, "Bea", "Betreuung")
	insertFigureWorkSession(t, db, busy, staff.ID, today, "present", at, nil)
	device := testpkg.CreateTestDeviceForTenant(t, db, busy, "today")
	for _, name := range []string{"Anna", "Ben"} {
		child := testpkg.CreateTestStudentForTenant(t, db, busy, name, "Hier", "1a")
		testpkg.CreateTestAttendanceForTenant(t, db, busy, child.ID, staff.ID, device.ID, today, at, nil)
	}
	testpkg.CreateTestStudentForTenant(t, db, busy, "Carl", "Daheim", "1a")
	testpkg.CreateTestStudentForTenant(t, db, quiet, "Dora", "Daheim", "1a")
	testpkg.CreateTestStaffForTenant(t, db, quiet, "Olaf", "Ohne")
	testpkg.CreateTestStudentForTenant(t, db, foreign, "Fiona", "Fremd", "1a")

	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleReadOnly)
	code, answer := traegerSignIn(t, router, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)

	rec := traegerSchoolsToday(t, router, answer.AccessToken, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Anna", "the overview carries figures, no names")
	assert.NotContains(t, rec.Body.String(), "OGS Fremd Heute", "no school of another Träger")

	got := decodeSchoolPortalData[traegerview.SchoolsToday](t, rec.Body.Bytes())
	require.Len(t, got.Schools, 2)
	assert.Equal(t, "OGS Am Bach", got.Schools[0].Name, "ordered by name")
	assert.Equal(t, busy, got.Schools[0].ID)
	assert.Equal(t, 3, got.Schools[0].Figures.ChildrenTotal)
	assert.Equal(t, 2, got.Schools[0].Figures.ChildrenPresent)
	assert.Equal(t, 1, got.Schools[0].Figures.StaffPresent)
	require.NotNil(t, got.Schools[0].Figures.CareRatio)
	assert.InDelta(t, 2.0, *got.Schools[0].Figures.CareRatio, 0.001)
	assert.Equal(t, "OGS Lindenweg", got.Schools[1].Name)
	assert.Equal(t, 1, got.Schools[1].Figures.ChildrenTotal)
	assert.Nil(t, got.Schools[1].Figures.CareRatio, "without present staff there is no ratio")

	assert.Equal(t, 4, got.Total.ChildrenTotal)
	assert.Equal(t, 2, got.Total.ChildrenPresent)
	assert.Equal(t, 2, got.Total.StaffTotal)
	assert.Equal(t, 1, got.Total.StaffPresent)
	require.NotNil(t, got.Total.CareRatio)
	assert.InDelta(t, 2.0, *got.Total.CareRatio, 0.001, "present children per present staff member across all schools")
	assert.False(t, got.Total.AsOf.IsZero())
}

func TestTraegerSchoolsTodayNeedsTheOverviewRight(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Ohne Recht", true)
	testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Ohne Recht")
	withoutRole := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, withoutRole.ID)
	code, answer := traegerSignIn(t, router, withoutRole.Email, password, 0)
	require.Equal(t, http.StatusOK, code)

	rec := traegerSchoolsToday(t, router, answer.AccessToken, nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "OGS Ohne Recht")
}

func TestTraegerSchoolsTodayQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Heute Budget", true)
	for _, name := range []string{"OGS Eins", "OGS Zwei", "OGS Drei"} {
		school := testpkg.CreateTestSchoolInOrganization(t, db, organization, name)
		testpkg.CreateTestStudentForTenant(t, db, school, "Kind", name, "1a")
	}
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleReadOnly)
	code, answer := traegerSignIn(t, router, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	read := func() []string {
		counter := testpkg.CaptureQueriesForContext(t, db)
		rec := traegerSchoolsToday(t, router, answer.AccessToken, counter)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	three := read()
	for _, name := range []string{"OGS Vier", "OGS Fünf"} {
		school := testpkg.CreateTestSchoolInOrganization(t, db, organization, name)
		testpkg.CreateTestStudentForTenant(t, db, school, "Kind", name, "1a")
	}
	assert.Len(t, read(), len(three), "the overview reads the same statements for any number of schools")
	querybudgets.AssertQueryBudget(t, "api.traeger.schools_today", three)
}
