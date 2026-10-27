package classdayhttp_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	classdayhttp "github.com/moto-nrw/project-phoenix/modules/classday/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const classDayAllClassesPermission = "class_day:all_classes"

// TestSchoolDayAPI drives the school-wide day overview (#4802): only
// class_day:all_classes opens it, it counts every class of the school in one
// pass whose statement count does not grow with the classes, and the access is
// logged once per actor, date and day.
func TestSchoolDayAPI(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, err := rootAPI.NewClassDayTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	router := classdayhttp.NewResource(module.ClassDay, nil).SchoolRouter(rootAPI.ProtectedSchoolRoutes)

	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Schul", fmt.Sprintf("Verwaltung-%d", time.Now().UnixNano()))
	claims := sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"schulverwaltung"}, TenantID: testpkg.Tenant(t), Scope: tenant.ScopeSchool}
	suffix := time.Now().UnixNano() % 100000
	addClass := func(i int) string {
		className := fmt.Sprintf("sd%d-%d", suffix, i)
		testpkg.CreateTestStudent(t, db, fmt.Sprintf("Kind%d", i), "Schultag", className)
		return className
	}
	first, second := addClass(1), addClass(2)
	get := func(path string, granted ...string) *httptest.ResponseRecorder {
		return testutil.ExecuteWithAuthPermissions(t, router, httptest.NewRequest(http.MethodGet, path, nil), claims, granted)
	}

	// A Lehrkraft holds class_day:read only: the overview stays closed and
	// the classes list does not offer it.
	rec := get("/school-day?date=2026-08-05", classDayReadPermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = get("/classes", classDayReadPermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"can_view_school_day":false`)
	rec = get("/classes", classDayReadPermission, classDayAllClassesPermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"can_view_school_day":true`)

	counter := testpkg.CaptureQueries(t, db)
	// Each measurement reads a date of its own, so both include the
	// access-log write the second read of a date skips.
	overview := func(date string, classes int) int {
		counter.Reset()
		rec := get("/school-day?date="+date, classDayReadPermission, classDayAllClassesPermission)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"school_class":"`+first+`"`)
		assert.Contains(t, rec.Body.String(), `"school_class":"`+second+`"`)
		assert.Contains(t, rec.Body.String(), fmt.Sprintf(`"totals":{"students":%d,`, classes))
		return counter.Total()
	}
	small := overview("2026-08-05", 2)
	addClass(3)
	addClass(4)
	large := overview("2026-08-06", 4)
	assert.Equal(t, small, large, "the overview must not grow with the number of classes")
	testpkg.AssertQueryBudget(t, "modules.classday.school_day", counter.Queries())

	rec = get("/school-day?date=2026-08-05", classDayReadPermission, classDayAllClassesPermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var logged int
	require.NoError(t, db.NewSelect().TableExpr("audit.data_access_log").
		Where("actor_account_id = ?", account.ID).Where("resource_type = ?", "school_day_view").
		ColumnExpr("count(*)").Scan(testpkg.Ctx(t), &logged))
	assert.Equal(t, 2, logged, "one access-log row per actor, date and day of access")

	rec = get("/school-day?date=05.08.2026", classDayReadPermission, classDayAllClassesPermission)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}
