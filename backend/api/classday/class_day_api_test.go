// Router-level tests for the class-day surface (#1772): every route is gated
// on class_day:read, and the roster is scoped to the caller's
// education.class_teachers assignments.
//
// Since the school-portal cutover (#2207 PR 3) the only mount is
// /school/class-day, so these drive SchoolRouter with school-scope claims.
// The behaviour under test is unchanged — same handlers, same permission
// gate, same projection.
package classday_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/classday"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func setupClassDayRoute(t *testing.T) (*testpkg.DB, *classday.Resource) {
	t.Helper()
	db, factory := testutil.SetupAPITest(t)
	return db, classday.NewResource(factory.EnrollmentReport, factory.UserContext, db, nil)
}

func TestClassDayAPI(t *testing.T) {
	t.Parallel()
	db, resource := setupClassDayRoute(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "ClassDay", fmt.Sprintf("API-%d", time.Now().UnixNano()))
	className := fmt.Sprintf("cd%d", time.Now().UnixNano()%100000)
	assignment := testpkg.CreateTestClassTeacher(t, db, staff.ID, className)
	_ = testpkg.CreateTestStudent(t, db, "Klara", "Klassentag", className)
	t.Cleanup(func() {
		tenantCtx := testpkg.Ctx(t)
		_, _ = db.NewDelete().TableExpr("education.class_teachers").Where("id = ?", assignment.ID).Exec(tenantCtx)
	})

	router := resource.SchoolRouter()

	claims := jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"lehrkraft"}, TenantID: testpkg.Tenant(t), Scope: tenant.ScopeSchool}

	// Wrong permission → 403 before any data access.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:read"})
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	// Assigned classes list.
	req = httptest.NewRequest(http.MethodGet, "/classes", nil)
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"class_day:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), className)

	// Day view for the assigned class (defaults to the first assignment when
	// no ?class= is sent).
	req = httptest.NewRequest(http.MethodGet, "/?date=2026-08-05", nil)
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"class_day:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Klassentag")
	assert.Contains(t, rec.Body.String(), `"school_class":"`+className+`"`)
	// Privacy reduction: the day view never carries guardian contact data.
	assert.NotContains(t, rec.Body.String(), "guardians")

	// A class the caller is NOT assigned to → 403, even with the permission.
	req = httptest.NewRequest(http.MethodGet, "/?class=fremde-klasse", nil)
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"class_day:read"})
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "nicht zugewiesen")

	// Broken date → 400.
	req = httptest.NewRequest(http.MethodGet, "/?date=05.08.2026", nil)
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"class_day:read"})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestClassDayAPINoAssignments(t *testing.T) {
	t.Parallel()
	db, resource := setupClassDayRoute(t)

	_, account := testpkg.CreateTestStaffWithAccount(t, db, "ClassDay", fmt.Sprintf("Empty-%d", time.Now().UnixNano()))

	router := resource.SchoolRouter()
	claims := jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"lehrkraft"}, TenantID: testpkg.Tenant(t), Scope: tenant.ScopeSchool}

	// Without any assignment the classes list is empty and the day view is
	// refused — there is nothing the caller may see.
	req := httptest.NewRequest(http.MethodGet, "/classes", nil)
	rec := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"class_day:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"class_day:read"})
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}
