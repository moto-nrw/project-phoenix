// Router-level tests for the school portal (#2207): the token matrix that
// pins the scope isolation (school tokens work ONLY on /school/*, every
// other scope is refused there), plus the school login handler's
// portal-role gate.
package school_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/classday"
	"github.com/moto-nrw/project-phoenix/api/school"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestSchoolPortalTokenMatrix(t *testing.T) {
	db, factory := testutil.SetupAPITest(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "School", fmt.Sprintf("Matrix-%d", time.Now().UnixNano()))
	className := fmt.Sprintf("sm%d", time.Now().UnixNano()%100000)
	assignment := testpkg.CreateTestClassTeacher(t, db, staff.ID, className)
	t.Cleanup(func() {
		tenantCtx := testpkg.TenantContext(1)
		_, _ = db.NewDelete().TableExpr("education.class_teachers").Where("id = ?", assignment.ID).Exec(tenantCtx)
		testpkg.CleanupStaffFixtures(t, db, staff.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
	})

	classDayResource := classday.NewResource(factory.EnrollmentReport, factory.UserContext, db, nil)
	schoolRouter := school.NewResource(factory.Auth, factory.MFA, classDayResource).Router()
	tenantClassDayRouter := classDayResource.Router()

	schoolClaims := jwt.AppClaims{
		ID: int(account.ID), Sub: account.Email,
		Roles: []string{"lehrkraft"}, TenantID: 1,
		Scope: tenant.ScopeSchool,
	}

	// School token on the school class-day surface → 200.
	req := httptest.NewRequest(http.MethodGet, "/class-day/classes", nil)
	rec := testutil.ExecuteWithAuthPermissions(t, schoolRouter, req, schoolClaims, []string{"class_day:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), className)

	// School token on the TENANT class-day mount → 401. Same handlers, but
	// the tenant mantle refuses the school scope.
	req = httptest.NewRequest(http.MethodGet, "/classes", nil)
	rec = testutil.ExecuteWithAuthPermissions(t, tenantClassDayRouter, req, schoolClaims, []string{"class_day:read"})
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())

	// Every non-school scope on the school surface → 401, permission or not.
	foreignScopes := []struct {
		name   string
		claims jwt.AppClaims
	}{
		{"tenant scope", jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"lehrkraft"}, TenantID: 1, Scope: tenant.ScopeTenant}},
		{"org scope", jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"admin"}, TenantID: 1, OrgID: 1, Scope: tenant.ScopeOrg}},
		{"platform scope", jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"operator"}, Scope: tenant.ScopePlatform}},
		{"parent scope", jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"guardian"}, Scope: tenant.ScopeParent}},
		{"school scope without tenant binding", jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"lehrkraft"}, Scope: tenant.ScopeSchool}},
	}
	for _, tc := range foreignScopes {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/class-day/classes", nil)
			rec := testutil.ExecuteWithAuthPermissions(t, schoolRouter, req, tc.claims, []string{"class_day:read"})
			require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
		})
	}

	// The protected school auth surface follows the same matrix: a tenant
	// token cannot switch schools.
	req = httptest.NewRequest(http.MethodPost, "/auth/switch-school", strings.NewReader(`{"tenant_slug":"t1"}`))
	req.Header.Set("Content-Type", "application/json")
	tenantClaims := jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"lehrkraft"}, TenantID: 1, Scope: tenant.ScopeTenant}
	rec = testutil.ExecuteWithAuthPermissions(t, schoolRouter, req, tenantClaims, []string{"class_day:read"})
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

func TestSchoolLoginHandler_PortalRoleGate(t *testing.T) {
	db, factory := testutil.SetupAPITest(t)

	classDayResource := classday.NewResource(factory.EnrollmentReport, factory.UserContext, db, nil)
	schoolRouter := school.NewResource(factory.Auth, factory.MFA, classDayResource).Router()

	const password = "Test1234%" //nolint:gosec // test credential
	unique := time.Now().UnixNano()
	email := fmt.Sprintf("school-login-%d@test.local", unique)
	account, err := factory.Auth.Register(testpkg.TenantContext(1), email, fmt.Sprintf("school-login-%d", unique), password, nil, 0)
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	testpkg.MapAccountToTenant(t, db, account.ID, 1)

	loginBody := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)

	// Mapped, but no school-portal role → 403 with the stable portal code.
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	schoolRouter.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "no_school_portal_role")

	// With the lehrkraft system role the same credentials authenticate and
	// the response carries the token pair.
	var lehrkraftRoleID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").TableExpr("auth.roles").
		Where("name = ?", "lehrkraft").Where("is_system = TRUE").
		Scan(testpkg.TenantContext(1), &lehrkraftRoleID))
	roleRow := map[string]any{"account_id": account.ID, "role_id": lehrkraftRoleID, "tenant_id": int64(1)}
	_, err = db.NewInsert().Model(&roleRow).TableExpr("auth.account_roles").Exec(testpkg.TenantContext(1))
	require.NoError(t, err)

	req = httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	schoolRouter.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"status":"authenticated"`)
	assert.Contains(t, rec.Body.String(), "access_token")

	// Wrong password stays a masked 401.
	req = httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(fmt.Sprintf(`{"email":%q,"password":"Wrong-1234%%"}`, email)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	schoolRouter.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "invalid_credentials")
}
