package timetracking

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// init seeds JWT viper defaults before any test (and before setupStaffRoute
// constructs a resource via the protected tenant group). CI runs without a
// .env so AUTH_JWT_SECRET is unset; without a secret jwx refuses HMAC signing.
func init() {
	testutil.SeedTestJWTConfig()
}

// isoDay is a calendar day for the shared fixtures, which accept any value
// rendering as YYYY-MM-DD.
type isoDay string

func (d isoDay) String() string { return string(d) }

// testContext holds shared test dependencies of the workforce routes.
type testContext struct {
	db       *bun.DB
	module   services.WorkforceTestModule
	resource *StaffAdminResource
	router   chi.Router
}

// testIdentity reads the caller the request options or the signed token
// injected, the way the composition root derives it from the session claims.
func testIdentity(ctx context.Context) Identity {
	claims, permissions := testutil.AuthenticationContext(ctx)
	name := strings.TrimSpace(claims.FirstName + " " + claims.LastName)
	if name == "" {
		name = claims.Username
	}
	return Identity{AccountID: int64(claims.ID), Roles: claims.Roles, Permissions: permissions, DisplayName: name}
}

func newWorkforceTestRepositories(t *testing.T, db *bun.DB) repositories.WorkforceTestRepositories {
	t.Helper()
	repos, err := repositories.NewWorkforceTestRepositories(db, repositories.NewTestAuditStore(db))
	require.NoError(t, err)
	return repos
}

// newWorkforceCapability composes the Workforce module the schedule views read
// from, pinned to the given clock when one is supplied.
func newWorkforceCapability(t *testing.T, db *bun.DB, clocks ...func() time.Time) workforce.Capability {
	t.Helper()
	membership, err := repositories.NewSchoolMembership(db)
	require.NoError(t, err)
	var now func() time.Time
	if len(clocks) > 0 {
		now = clocks[0]
	}
	capability, err := repositories.NewWorkforceWithClock(db, membership, now)
	require.NoError(t, err)
	return capability
}

// testCapabilities adapts the retained services of a test module to the
// public Workforce contracts exactly as the composition root does.
func testCapabilities(svc services.WorkforceTestModule) services.WorkforceAdminCapabilities {
	return services.NewWorkforceAdminCapabilities(svc.Users, svc.StaffDocuments, svc.WorkSession, svc.StaffAbsence, svc.WorkTimeMonth,
		svc.StaffBalanceAdjust, svc.StaffMonthClose, svc.StaffOverview, svc.TimeTrackingAuditLog, svc.StaffTimeExport)
}

// setupStaffRoute initializes test database, services, and resource. The
// router serves the resource through the production middleware chain
// (Verifier → Authenticator → TenantMiddleware → RequiresPermission →
// TenantTxMiddleware) exactly as the real server does, mounted at /staff.
func setupStaffRoute(t *testing.T, clocks ...func() time.Time) *testContext {
	t.Helper()

	db, svc := testutil.SetupWorkforceModule(t, clocks...)
	capabilities := testCapabilities(svc)
	cleanup, err := repositories.NewStaffDocumentCleanup(db, nil)
	require.NoError(t, err)

	resource := NewStaffAdminResource(StaffAdminDependencies{
		OffboardingCleanup: cleanup,
		Staff:              capabilities.Staff,
		Documents:          capabilities.Documents,
		WorkSessions:       capabilities.WorkSessions,
		StaffAbsences:      capabilities.StaffAbsences,
		WorkTimeMonth:      capabilities.WorkTimeMonth,
		BalanceAdjustments: capabilities.BalanceAdjustments,
		MonthClosing:       capabilities.MonthClosing,
		Overview:           capabilities.Overview,
		AuditLog:           capabilities.AuditLog,
		TimeExport:         capabilities.TimeExport,
		Schedules:          newWorkforceCapability(t, db, clocks...),
		ExportTransfer:     stubExportTransfer{},
		Identity:           testIdentity,
		DB:                 db,
		Logger:             slog.Default(),
	})

	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/staff", resource.Router())

	return &testContext{
		db:       db,
		module:   svc,
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
