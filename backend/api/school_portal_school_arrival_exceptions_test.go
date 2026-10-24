package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	classdayhttp "github.com/moto-nrw/project-phoenix/modules/classday/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/schoolportal"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestSchoolArrivalExceptionsScopeRatchet pins the scope split of the one
// write surface moto schule has (#2970), the counterpart of
// TestSchoolScopeRejectedOnAllAPIRoutes: a tenant token gets 401 on every
// /school/class-day/arrival-exceptions route (permission or not), and a
// school token with the permission gets 403 school_write_disabled while the
// school keeps operations.school_portal_write_scope at its default empty list.
// setupSchoolArrivalExceptionRoute wires the school portal with the class-day
// write seam (#2970) the way the service factory composes it; the other school surfaces stay
// unmounted.
func setupSchoolArrivalExceptionRoute(t *testing.T) (*testpkg.DB, *schoolportal.Resource) {
	t.Helper()
	db, services, classDayModule, _ := setupSchoolPortalModule(t)
	classDay := classdayhttp.NewResource(classDayModule.ClassDay, nil)
	return db, NewSchoolPortalHTTP(services.SchoolAuth, services.SchoolMFA, schoolportal.PasswordResetRuntime{}, classDay, nil, nil, nil, nil)
}

func TestSchoolArrivalExceptionsScopeRatchet(t *testing.T) {
	t.Parallel()
	db, resource := setupSchoolArrivalExceptionRoute(t)
	tenantID := testpkg.Tenant(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Lehr", fmt.Sprintf("Scope-%d", time.Now().UnixNano()))
	className := fmt.Sprintf("sc%d", time.Now().UnixNano()%100000)
	_ = testpkg.CreateTestClassTeacher(t, db, staff.ID, className)
	_ = testpkg.CreateTestStudent(t, db, "Scope", "Kind", className)
	router := resource.Router(SchoolSessionChain, ProtectedSchoolRoutes)

	// A fixed future Monday: the service refuses past dates and weekends,
	// and nothing here may depend on the wall clock.
	date := calendar.NewDate(2099, time.March, 2)
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

	tenantClaims := sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"admin"}, TenantID: tenantID, IsAdmin: true, Scope: tenant.ScopeTenant}
	schoolClaims := sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"lehrkraft"}, TenantID: tenantID, Scope: tenant.ScopeSchool}

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
			assert.Contains(t, rec.Body.String(), "classday.school_write_disabled")
		})
	}
}
