package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/traegerview"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// getTraegerKeyDateCounts reads GET /traeger/schools/key-date-counts with the
// session's token; a counter, when given, counts the statements of the
// request.
func getTraegerKeyDateCounts(t *testing.T, router http.Handler, token string, counter *testpkg.QueryCounter) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/traeger/schools/key-date-counts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if counter != nil {
		req = req.WithContext(counter.Context(req.Context()))
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// newIsolatedTraegerPortal mounts /traeger over a database of its own: the
// billing capture writes a row for every school of the database, which a
// shared database would hand to the tests running beside it.
func newIsolatedTraegerPortal(t *testing.T) (*bun.DB, *traegerPortalHandler, tenant.UnitOfWork) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	unit := testpkg.TenantRuntime(t, db)
	portal, _ := traegerPortalOn(t, db, unit)
	return db, portal, unit
}

// operatorBilling is the operator billing report the operator router and the
// worker compose, over the runtime of the test.
func operatorBilling(t *testing.T, unit tenant.UnitOfWork) (organizationtenancy.BillingReport, context.Context) {
	t.Helper()
	billing, err := owners.NewOperatorBilling(slog.Default())
	require.NoError(t, err)
	return billing, tenant.WithUnitOfWork(context.Background(), unit)
}

// operatorRowsOf keeps the operator report's rows of the given schools.
func operatorRowsOf(rows []organizationtenancy.BillingKeyDateCount, schools ...int64) map[int64][]organizationtenancy.BillingKeyDateCount {
	result := map[int64][]organizationtenancy.BillingKeyDateCount{}
	for _, row := range rows {
		for _, school := range schools {
			if row.SchoolID == school {
				result[school] = append(result[school], row)
			}
		}
	}
	return result
}

func TestTraegerKeyDateCountsShowTheNumbersOfTheOperatorReport(t *testing.T) {
	t.Parallel()
	db, router, unit := newIsolatedTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Stichtag", true)
	bach := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Am Bach")
	linden := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Lindenweg")
	foreign := testpkg.CreateTestSchoolInOrganization(t, db, traegerOrganization(t, db, "Träger Fremd Stichtag", true), "OGS Fremd Stichtag")

	testpkg.CreateTestStudentForTenant(t, db, bach, "Anna", "Aktiv", "1a")
	testpkg.CreateTestStudentForTenant(t, db, bach, "Ben", "Aktiv", "1a")
	testpkg.CreateTestDeviceForTenant(t, db, bach, "stichtag")
	testpkg.CreateTestStudentForTenant(t, db, linden, "Dora", "Aktiv", "1a")
	testpkg.CreateTestStudentForTenant(t, db, foreign, "Fiona", "Fremd", "1a")
	testpkg.CreateTestDeviceForTenant(t, db, foreign, "fremd")

	// The capture skips schools founded after the key date; these schools
	// existed before both months.
	_, err := db.NewRaw(`UPDATE platform.schools SET created_at = '2026-01-01 08:00+01' WHERE id IN (?, ?, ?)`, bach, linden, foreign).Exec(context.Background())
	require.NoError(t, err)
	billing, ctx := operatorBilling(t, unit)
	// Two captured months: the earlier one before a third child and a
	// second terminal came to the school at the brook.
	earlier := calendar.NewDate(2026, time.September, 15).BerlinMidnight().Add(7 * time.Hour)
	_, err = billing.SeedBillingKeyDateCounts(ctx, earlier)
	require.NoError(t, err)
	testpkg.CreateTestStudentForTenant(t, db, bach, "Carl", "Neu", "1a")
	testpkg.CreateTestDeviceForTenant(t, db, bach, "stichtag-zwei")
	later := calendar.NewDate(2026, time.October, 15).BerlinMidnight().Add(7 * time.Hour)
	_, err = billing.SeedBillingKeyDateCounts(ctx, later)
	require.NoError(t, err)

	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleReadOnly)
	code, answer := traegerSignIn(t, router, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)

	rec := getTraegerKeyDateCounts(t, router, answer.AccessToken, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "OGS Fremd Stichtag", "no school of another Träger")
	assert.NotContains(t, rec.Body.String(), "Anna", "the Stichtagszahlen carry figures, no names")
	got := decodeSchoolPortalData[traegerview.KeyDateCounts](t, rec.Body.Bytes())

	report, err := billing.ListBillingKeyDateCounts(ctx)
	require.NoError(t, err)
	operator := operatorRowsOf(report, bach, linden, foreign)
	require.Len(t, operator[bach], 2)
	require.Len(t, operator[foreign], 2, "the operator report holds the other Träger's school")

	require.Len(t, got.Schools, 2)
	assert.Equal(t, "OGS Am Bach", got.Schools[0].Name, "ordered by name")
	for _, school := range got.Schools {
		rows := operator[school.ID]
		require.Len(t, school.Counts, len(rows), school.Name)
		for i, row := range rows {
			count := school.Counts[i]
			assert.Equal(t, row.Period, count.Period.String(), "same month, newest first")
			assert.Equal(t, row.KeyDate, count.KeyDate.String())
			assert.Equal(t, row.ActiveStudents, count.ActiveStudents, "same children as the operator report")
			assert.Equal(t, row.ActiveTerminals, count.ActiveTerminals, "same terminals as the operator report")
			assert.True(t, row.RecordedAt.Equal(count.RecordedAt))
		}
	}
	bachCounts := got.Schools[0].Counts
	assert.Equal(t, "2026-10-15", bachCounts[0].KeyDate.String())
	assert.Equal(t, 3, bachCounts[0].ActiveStudents)
	assert.Equal(t, 2, bachCounts[0].ActiveTerminals)
	assert.Equal(t, 2, bachCounts[1].ActiveStudents, "a captured month never changes")
	assert.Equal(t, 1, bachCounts[1].ActiveTerminals)

	// Organisation & Tenancy itself reads only the rows of the Träger's
	// schools; the projection's own school filter is the second guard.
	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	ownerRows, err := organizations.ListTraegerBillingKeyDateCounts(ctx, organization)
	require.NoError(t, err)
	require.Len(t, ownerRows, 4, "two months of two schools")
	for _, row := range ownerRows {
		assert.NotEqual(t, foreign, row.SchoolID, "no row of another Träger's school")
	}

	require.Len(t, got.Months, 2)
	assert.Equal(t, traegerview.KeyDateMonth{
		Period: calendar.NewDate(2026, time.October, 1), KeyDate: calendar.NewDate(2026, time.October, 15),
		ActiveStudents: 4, ActiveTerminals: 2, Schools: 2,
	}, got.Months[0], "the sum leaves out the school of the other Träger")
	assert.Equal(t, 3, got.Months[1].ActiveStudents)
	assert.Equal(t, 1, got.Months[1].ActiveTerminals)
}

func TestTraegerKeyDateCountsWithoutCaptureListTheSchools(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Ohne Stichtag", true)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Ohne Stichtag")
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleReadOnly)
	code, answer := traegerSignIn(t, router, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)

	rec := getTraegerKeyDateCounts(t, router, answer.AccessToken, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decodeSchoolPortalData[traegerview.KeyDateCounts](t, rec.Body.Bytes())
	require.Len(t, got.Schools, 1)
	assert.Equal(t, school, got.Schools[0].ID)
	assert.NotNil(t, got.Schools[0].Counts)
	assert.Empty(t, got.Schools[0].Counts)
	assert.NotNil(t, got.Months)
	assert.Empty(t, got.Months)
}

func TestTraegerKeyDateCountsNeedTheOverviewRight(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Stichtag Ohne Recht", true)
	testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Stichtag Ohne Recht")
	withoutRole := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, withoutRole.ID)
	code, answer := traegerSignIn(t, router, withoutRole.Email, password, 0)
	require.Equal(t, http.StatusOK, code)

	rec := getTraegerKeyDateCounts(t, router, answer.AccessToken, nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "OGS Stichtag Ohne Recht")
}

func TestTraegerKeyDateCountsQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Stichtag Budget", true)
	for _, name := range []string{"OGS Eins", "OGS Zwei", "OGS Drei"} {
		testpkg.CreateTestSchoolInOrganization(t, db, organization, name)
	}
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleReadOnly)
	code, answer := traegerSignIn(t, router, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	read := func() []string {
		counter := testpkg.CaptureQueriesForContext(t, db)
		rec := getTraegerKeyDateCounts(t, router, answer.AccessToken, counter)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	three := read()
	for _, name := range []string{"OGS Vier", "OGS Fünf"} {
		testpkg.CreateTestSchoolInOrganization(t, db, organization, name)
	}
	assert.Len(t, read(), len(three), "the Stichtagszahlen read the same statements for any number of schools")
	querybudgets.AssertQueryBudget(t, "api.traeger.key_date_counts", three)
}
