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
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce/workforcetest"
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

// The typed root composes the workforce graph these route suites run over. It
// imports this package, so these internal tests cannot import it; the seams
// in workforcetest are bound by root_composition_test.go.
func newWorkforceTestRepositories(t *testing.T, db *bun.DB) workforcetest.Rows {
	t.Helper()
	return workforcetest.ComposeRows(t, db)
}

// newWorkforceCapability composes the Workforce module the schedule views read
// from, pinned to the given clock when one is supplied.
func newWorkforceCapability(t *testing.T, db *bun.DB, clocks ...func() time.Time) workforce.Capability {
	t.Helper()
	var now func() time.Time
	if len(clocks) > 0 {
		now = clocks[0]
	}
	return workforcetest.ComposeCapability(t, db, now)
}

// setupStaffRoute initializes test database, services, and resource. The
// router serves the resource through the production middleware chain
// (Verifier → Authenticator → TenantMiddleware → RequiresPermission →
// TenantTxMiddleware) exactly as the real server does, mounted at /staff.
func setupStaffRoute(t *testing.T, clocks ...func() time.Time) *testContext {
	t.Helper()

	db, capabilities := workforcetest.ComposeAdmin(t, clocks...)
	cleanup, err := workforceCompose.NewDocumentCleanup(db, nil)
	require.NoError(t, err)
	documentFiles, err := workforceCompose.NewStaffDocumentFileRecovery(capabilities.Documents, cleanup, nativeUploads.UploadFiles{}, slog.Default())
	require.NoError(t, err)
	workforceCapability := newWorkforceCapability(t, db, clocks...)
	qualifications, err := workforceCompose.NewQualifications(db, qualificationStaffStub{}, workforce.QualificationReminderPorts{}, nil)
	require.NoError(t, err)

	resource := NewStaffAdminResource(StaffAdminDependencies{
		Uploads:            nativeUploads.UploadFiles{},
		PayrollDispatch:    payrollDispatchOff{},
		OffboardingCleanup: cleanup,
		DocumentFiles:      documentFiles,
		Staff:              capabilities.Staff,
		Documents:          capabilities.Documents,
		WorkSessions:       capabilities.WorkSessions,
		StaffAbsences:      capabilities.StaffAbsences,
		AbsenceWrites:      capabilities.AbsenceWrites,
		WorkTimeMonth:      capabilities.WorkTimeMonth,
		BalanceAdjustments: capabilities.BalanceAdjustments,
		MonthClosing:       capabilities.MonthClosing,
		Overview:           capabilities.Overview,
		AuditLog:           capabilities.AuditLog,
		TimeExport:         capabilities.TimeExport,
		Schedules:          workforceCapability,
		TargetOverrides:    workforceCapability,
		ExportTransfer:     stubExportTransfer{},
		Identity:           testIdentity,
		QualificationTypes: qualifications,
		Qualifications:     qualifications,
		Logger:             slog.Default(),
	})

	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/staff", resource.Router(scopeTestTenantRoutes(testpkg.TenantSessionGatesWithCaches(scopeTestSettingsCache))))

	return &testContext{
		db:       db,
		resource: resource,
		router:   router,
	}
}

// accountStartSetting is models/config.KeyTimeTrackingAccountStartDate,
// spelled out so these route tests keep their imports;
// services/config/defaults pins the same key.
const accountStartSetting = "operations.time_tracking_account_start_date"

// setAccountStart pins the test school's Stundenkonto start (YYYY-MM-DD).
// Without it the account begins on 1 January of the live year, so every
// booking dated in an earlier year is refused once the clock passes New Year
// (#4027).
func setAccountStart(t *testing.T, tc *testContext, start string) {
	t.Helper()
	testpkg.SetTestTenantSetting(t, tc.db, testpkg.Tenant(t), accountStartSetting, `"`+start+`"`)
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

// payrollDispatchOff is a school without the automatic send of #4805: the
// month close queues nothing.
type payrollDispatchOff struct{}

func (payrollDispatchOff) QueueAfterMonthClose(context.Context, workforce.PayrollDispatchTrigger) error {
	return nil
}

func (payrollDispatchOff) Overview(context.Context) (workforce.PayrollDispatchOverview, error) {
	return workforce.PayrollDispatchOverview{}, nil
}

func (payrollDispatchOff) RunDue(context.Context) (int, error) { return 0, nil }

func (payrollDispatchOff) Download(context.Context, int64, string) (workforce.PayrollDownload, error) {
	return workforce.PayrollDownload{}, workforce.ErrPayrollDownloadUnavailable
}

// qualificationStaffStub lists the staff the Nachweise routes read (#4808);
// tests that need people on the list set members.
type qualificationStaffStub struct {
	members []workforce.QualificationStaffMember
}

func (s qualificationStaffStub) ListStaffMembers(context.Context) ([]workforce.QualificationStaffMember, error) {
	return s.members, nil
}

func (s qualificationStaffStub) StaffContact(_ context.Context, staffID int64) (workforce.QualificationStaffMember, error) {
	return workforce.QualificationStaffMember{StaffID: staffID}, nil
}

func (s qualificationStaffStub) StammdatenManagers(context.Context) ([]workforce.QualificationStaffMember, error) {
	return nil, nil
}
