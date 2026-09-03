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
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestSchoolArrivalExceptionsScopeRatchet pins the scope split of the one
// write surface moto schule has (#2970), the counterpart of
// TestSchoolScopeRejectedOnAllAPIRoutes: a tenant token gets 401 on every
// /school/class-day/arrival-exceptions route (permission or not), and a
// school token with the permission gets 403 school_write_disabled while the
// school keeps operations.school_portal_write_scope at its default "none".
// setupSchoolArrivalExceptionRoute wires the school portal with the class-day
// write seam (#2970) the way api/base.go does; the other school surfaces stay
// unmounted.
func setupSchoolArrivalExceptionRoute(t *testing.T) (*testpkg.DB, *school.Resource) {
	t.Helper()
	db, services := testutil.SetupAPITest(t)
	classDay := classday.NewResource(services.EnrollmentReport, services.UserContext, db, nil,
		classday.WithArrivalExceptions(services.ClassDayArrivalExceptions))
	return db, school.NewResource(services.Auth, services.MFA, classDay, nil, nil, nil)
}

func TestSchoolArrivalExceptionsScopeRatchet(t *testing.T) {
	t.Parallel()
	db, resource := setupSchoolArrivalExceptionRoute(t)
	tenantID := testpkg.Tenant(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Lehr", fmt.Sprintf("Scope-%d", time.Now().UnixNano()))
	className := fmt.Sprintf("sc%d", time.Now().UnixNano()%100000)
	assignment := testpkg.CreateTestClassTeacher(t, db, staff.ID, className)
	_ = testpkg.CreateTestStudent(t, db, "Scope", "Kind", className)
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("education.class_teachers").Where("id = ?", assignment.ID).Exec(testpkg.Ctx(t))
	})
	router := resource.Router()

	// A fixed future Monday: the service refuses past dates and weekends,
	// and nothing here may depend on the wall clock.
	date := timezone.NewDate(2099, time.March, 2)
	permissions := []string{"class_day:read", "class_day:arrival_exception_write", "users:update", "admin:*"}
	routes := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/class-day/arrival-exceptions?class=" + className, ""},
		{http.MethodGet, "/class-day/arrival-exceptions/block-start?class=" + className + "&date=" + date.String(), ""},
		{http.MethodPut, fmt.Sprintf("/class-day/arrival-exceptions/%s/%s", className, date), `{"arrival_time":"12:45"}`},
		{http.MethodDelete, fmt.Sprintf("/class-day/arrival-exceptions/%s/%s", className, date), ""},
	}

	tenantClaims := jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"admin"}, TenantID: tenantID, IsAdmin: true, Scope: tenant.ScopeTenant}
	schoolClaims := jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"lehrkraft"}, TenantID: tenantID, Scope: tenant.ScopeSchool}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			// Tenant scope, even over-privileged: 401 before any handler.
			req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
			req.Header.Set("Content-Type", "application/json")
			rec := testutil.ExecuteWithAuthPermissions(t, router, req, tenantClaims, permissions)
			require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())

			// School scope with the permission, setting at its default: the
			// list stays readable, every write is refused with the stable code.
			req = httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
			req.Header.Set("Content-Type", "application/json")
			rec = testutil.ExecuteWithAuthPermissions(t, router, req, schoolClaims, permissions)
			if route.method == http.MethodGet && !strings.Contains(route.path, "block-start") {
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				assert.Contains(t, rec.Body.String(), `"can_edit":false`)
				return
			}
			require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), "school_write_disabled")
		})
	}
}
