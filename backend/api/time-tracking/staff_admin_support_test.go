package timetracking

import (
	"log/slog"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// init seeds JWT viper defaults before any test (and before setupStaffRoute
// constructs a resource via jwt.MustNewTokenAuth). CI runs without a .env so
// AUTH_JWT_SECRET is unset; without a secret jwx refuses HMAC signing.
func init() {
	testutil.SeedTestJWTConfig()
}

// testContext holds shared test dependencies of the workforce routes.
type testContext struct {
	db       *bun.DB
	resource *StaffAdminResource
	router   chi.Router
}

// setupStaffRoute initializes test database, services, and resource. The
// router serves the resource through the production middleware chain
// (Verifier → Authenticator → TenantMiddleware → RequiresPermission →
// TenantTxMiddleware) exactly as the real server does, mounted at /staff.
func setupStaffRoute(t *testing.T, clocks ...func() time.Time) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t, clocks...)

	resource := NewStaffAdminResource(svc.Users, svc.StaffDocuments, svc.WorkSession, svc.StaffAbsence, svc.WorkTimeMonth, svc.StaffBalanceAdjust, svc.StaffMonthClose, svc.StaffOverview, svc.TimeTrackingAuditLog, svc.StaffTimeExport, db, slog.Default())

	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/staff", resource.Router())

	return &testContext{
		db:       db,
		resource: resource,
		router:   router,
	}
}

// authToken mints a bearer token from default admin claims narrowed to the
// given permissions. Requests flow through Router() and the middleware chain
// reads permissions from the signed token.
func authToken(t *testing.T, perms ...string) string {
	t.Helper()
	claims := testutil.DefaultTestClaims()
	claims.Permissions = perms
	return testutil.MintTestJWT(t, claims)
}
