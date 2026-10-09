// Package testutil provides shared test utilities for API handler tests.
//
// # Test Pattern
//
// API tests follow the hermetic test pattern established in the codebase:
// - Real database with test fixtures
// - Real services via route/module-sized builders
// - httptest for HTTP request/response
// - Context injection for JWT claims and permissions
//
// Example:
//
//	func setupAuthRoute(t *testing.T) (*bun.DB, *Resource) {
//	    db, module := testutil.SetupAuthModule(t)
//	    return db, NewResource(module.Auth, module.Invitation)
//	}
//
//	func TestHandler(t *testing.T) {
//	    db, resource := setupAuthRoute(t)
//	    router := chi.NewRouter()
//	    router.Mount("/auth", resource.Router())
//
//	    req := testutil.NewAuthenticatedRequest("GET", "/auth/account", nil,
//	        testutil.WithPermissions("users:read"),
//	        testutil.WithClaims(jwt.AppClaims{ID: 1, Username: "test"}),
//	    )
//
//	    rr := testutil.ExecuteRequest(router, req)
//	    testutil.AssertSuccessResponse(t, rr, http.StatusOK)
//	}
package testutil

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	feedbackCompose "github.com/moto-nrw/project-phoenix/modules/feedback/compose"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// SetupSettingsModule builds the settings route's real application boundary.
func SetupSettingsModule(t *testing.T) (*bun.DB, services.SettingsTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	return db, SetupSettingsModuleWithDB(t, db)
}

// SetupSettingsModuleWithDB preserves an isolated pool for query-budget tests.
func SetupSettingsModuleWithDB(t *testing.T, db *bun.DB) services.SettingsTestModule {
	t.Helper()
	module, err := services.NewSettingsTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return module
}

// SetupFileStoreModule composes the File Storage capability over the test
// database and the caller's uploads object store (#2707).
func SetupFileStoreModule(t *testing.T, objects services.UploadsBackend) (*bun.DB, services.FileStoreTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewFileStoreTestModule(db, testpkg.TenantRuntime(t, db), objects)
	require.NoError(t, err)
	return db, module
}

func SetupAbsenceTypeModule(t *testing.T) (*bun.DB, services.AbsenceTypeTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewAbsenceTypeTestModule(db)
	require.NoError(t, err)
	return db, module
}

// NewBirthdayCapability composes the birthday capability over the retained
// repositories and the given settings, for the module's behavior tests.
func NewBirthdayCapability(db *bun.DB, settings services.BirthdaySettingsSource, now func() time.Time) services.BirthdayCapability {
	return services.NewBirthdayCapabilityForTests(db, settings, now)
}

func SetupBirthdayModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, services.BirthdayTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewBirthdayTestModule(db, testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module
}

func SetupShiftTypeModule(t *testing.T) (*bun.DB, services.ShiftTypeTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewShiftTypeTestModule(db)
	require.NoError(t, err)
	return db, module
}

func SetupDeviceModule(t *testing.T) (*bun.DB, services.DeviceTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewDeviceTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func SetupTimetableModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, services.TimetableTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewTimetableTestModule(db, testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module
}

func SetupAuthModule(t *testing.T) (*bun.DB, services.AuthTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewAuthTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func SetupInvitationModule(t *testing.T) (*bun.DB, services.InvitationTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewInvitationTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func SetupScheduleModule(t *testing.T) (*bun.DB, services.ScheduleTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewScheduleTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func SetupStatisticsModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, services.StatisticsTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewStatisticsTestModule(db, testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module
}

func SetupGradeTransitionModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, services.GradeTransitionTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewGradeTransitionTestModule(db, nil, clocks...)
	require.NoError(t, err)
	return db, module
}

func SetupUserContextModule(t *testing.T) (*bun.DB, services.UserContextTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewUserContextTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func SetupRoomsModule(t *testing.T) (*bun.DB, services.RoomsTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewRoomsTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

// SetupCheckinModule composes the device-scan workflow over the test
// database. An optional clock pins the instant the kiosk scans are admitted.
func SetupCheckinModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, services.CheckinTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewCheckinTestModule(db, testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module
}

func SetupStaffMessagingModule(t *testing.T) (*bun.DB, services.StaffMessagingTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewStaffMessagingTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func SetupRFIDModule(t *testing.T) (*bun.DB, services.RFIDTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewRFIDTestModule(db)
	require.NoError(t, err)
	return db, module
}

func SetupGroupsModule(t *testing.T, publishers ...*testpkg.RecordingBroadcaster) (*bun.DB, services.GroupsTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	var broadcaster *testpkg.RecordingBroadcaster
	if len(publishers) > 0 {
		broadcaster = publishers[0]
	}
	var module services.GroupsTestModule
	var err error
	if broadcaster != nil {
		module, err = services.NewGroupsTestModule(db, testpkg.TenantRuntime(t, db), broadcaster)
	} else {
		module, err = services.NewGroupsTestModule(db, testpkg.TenantRuntime(t, db))
	}
	require.NoError(t, err)
	return db, module
}

func SetupClassListModule(t *testing.T) (*bun.DB, services.ClassListTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewClassListTestModule(db)
	require.NoError(t, err)
	return db, module
}

func SetupWorkSessionModule(t *testing.T) (*bun.DB, services.WorkSessionTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewWorkSessionTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func SetupIoTDataModule(t *testing.T) (*bun.DB, services.IoTDataTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewIoTDataTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func SetupActiveModule(t *testing.T) (*bun.DB, services.ActiveTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewActiveTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func SetupRemindersModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, services.RemindersTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewRemindersTestModule(db, testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module
}

func SetupOperatorSettingsModule(t *testing.T) (*bun.DB, services.OperatorSettingsTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewOperatorSettingsTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func SetupSettingsCallbacksModule(t *testing.T, photos services.StudentPhotoBootstrap) (*bun.DB, services.SettingsCallbacksTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewSettingsCallbacksTestModule(db, testpkg.TenantRuntime(t, db), photos.Unlinker)
	require.NoError(t, err)
	return db, module
}

func SetupClassDayModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, services.ClassDayTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewClassDayTestModule(db, testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module
}

func SetupSchoolModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, services.SchoolTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewSchoolTestModule(db, testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module
}

func SetupGuardianModule(t *testing.T) (*bun.DB, services.GuardianTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewGuardianTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func SetupWorkforceModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, services.WorkforceTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewWorkforceTestModule(db, testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module
}

func SetupStaffModule(t *testing.T) (*bun.DB, services.StaffTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewStaffTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func SetupStudentModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, services.StudentTestModule) {
	t.Helper()
	db, feedback := SetupFeedbackModule(t)
	module, err := services.NewStudentTestModule(db, testpkg.TenantRuntime(t, db), feedback.Feedback, clocks...)
	require.NoError(t, err)
	return db, module
}

func SetupImportModule(t *testing.T, databases ...*bun.DB) (*bun.DB, services.ImportTestModule) {
	t.Helper()
	var db *bun.DB
	if len(databases) > 0 {
		db = databases[0]
	} else {
		db = testpkg.SetupTestDB(t)
	}
	module, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func SetupTimetableScenarioModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, services.TimetableScenarioTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewTimetableScenarioTestModule(db, testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module
}

func SetupCalendarModule(t *testing.T) (*bun.DB, services.CalendarTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewCalendarTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

type FeedbackTestModule struct {
	services.RFIDTestModule
	services.SettingsTestModule
	Feedback *feedbackModule.Module
}

func SetupFeedbackModule(t *testing.T) (*bun.DB, FeedbackTestModule) {
	t.Helper()
	db, settings := SetupSettingsModule(t)
	identity, err := services.NewRFIDTestModule(db)
	require.NoError(t, err)
	resolvers := feedbackCompose.NewSettings()
	resolvers.Bind(func(ctx context.Context) (bool, error) {
		return settings.Settings.ResolveBool(ctx, "feedback.enabled")
	}, func(ctx context.Context) (int, error) {
		return settings.Settings.ResolveInt(ctx, "feedback.data_retention_days")
	})
	feedback, err := feedbackCompose.New(feedbackCompose.Dependencies{
		DB: db, Settings: resolvers, Today: feedbackModule.Today, Observe: func(feedbackCompose.Observation) {},
	})
	require.NoError(t, err)
	return db, FeedbackTestModule{RFIDTestModule: identity, SettingsTestModule: settings, Feedback: feedback}
}

func SetupActivitiesModule(t *testing.T) (*bun.DB, services.ActivitiesTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewActivitiesTestModule(db)
	require.NoError(t, err)
	return db, module
}

// DeviceIdentity exposes the authenticated principal facts to HTTP adapter
// runtimes without making each adapter test import the device middleware.
func DeviceIdentity(ctx context.Context) (int64, string, bool) {
	principal := device.DeviceFromCtx(ctx)
	if principal == nil {
		return 0, "", false
	}
	return principal.ID, principal.DeviceID, true
}

// NewDeviceAuthenticators composes the production device authentication
// middleware for handler tests. settings may be nil to authenticate with
// fallbackPIN alone.
func NewDeviceAuthenticators(
	devices deviceauth.Fleet,
	schools deviceauth.SchoolDirectory,
	settings deviceauth.Settings,
	fallbackPIN string,
) *deviceauth.Authenticators {
	return deviceauth.New(deviceauth.Dependencies{
		Devices:     devices,
		Schools:     schools,
		Settings:    settings,
		FallbackPIN: fallbackPIN,
	})
}

// AuthenticatedDeviceID reads the kiosk the device authenticator admitted on
// a request, as production binds it for routes that attribute the kiosk.
func AuthenticatedDeviceID(ctx context.Context) (string, bool) {
	return deviceauth.DeviceID(ctx)
}

// DevicePrincipal converts a device row into the principal the device auth
// middleware binds to a request, so handler tests see exactly what
// production handlers see. It never copies the API key.
func DevicePrincipal(d *iot.Device) *device.AuthenticatedDevice {
	if d == nil {
		return nil
	}
	return &device.AuthenticatedDevice{
		ID:         d.ID,
		TenantID:   d.TenantID,
		DeviceID:   d.DeviceID,
		DeviceType: d.DeviceType,
		Name:       d.Name,
		Status:     string(d.Status),
		LastSeen:   d.LastSeen,
	}
}

// StaffPrincipal converts a staff row into the principal a verified web
// boundary binds to a request.
func StaffPrincipal(s *users.Staff) *device.AuthenticatedStaff {
	if s == nil {
		return nil
	}
	return &device.AuthenticatedStaff{ID: s.ID, TenantID: s.TenantID}
}

// WithDeviceActor binds the device and staff principals the device auth
// middleware would bind for a verified kiosk request, so a scenario can call
// a presence capability directly as that actor without naming the auth
// context keys itself.
func WithDeviceActor(ctx context.Context, d *iot.Device, s *users.Staff) context.Context {
	ctx = context.WithValue(ctx, device.CtxDevice, DevicePrincipal(d))
	return context.WithValue(ctx, device.CtxStaff, StaffPrincipal(s))
}

// WithDeviceContext adds an IoT device to the request context.
// This is used for testing device-authenticated endpoints.
// Also injects the device's tenant_id so TenantTxMiddleware can create
// a tenant-scoped transaction (mirrors production device auth middleware).
func WithDeviceContext(d *iot.Device) RequestOption {
	return func(req *http.Request) {
		ctx := context.WithValue(req.Context(), device.CtxDevice, DevicePrincipal(d))
		if tid := d.GetTenantID(); tid != 0 {
			ctx = tenant.WithTenantID(ctx, tid)
		}
		*req = *req.WithContext(ctx)
	}
}

// WithDeviceIdentity supplies a device principal to hermetic HTTP adapter tests
// that do not need a persisted device or a tenant transaction.
func WithDeviceIdentity(id int64, deviceID string) RequestOption {
	return func(req *http.Request) {
		principal := &device.AuthenticatedDevice{ID: id, DeviceID: deviceID}
		*req = *req.WithContext(context.WithValue(req.Context(), device.CtxDevice, principal))
	}
}

// Rollback helpers exercise the same tenant marker as production middleware.
func MarkRollback(ctx context.Context)                       { tenant.MarkRollback(ctx) }
func WithRollbackMarker(ctx context.Context) context.Context { return tenant.WithRollbackMarker(ctx) }
func RollbackRequested(ctx context.Context) bool             { return tenant.RollbackRequested(ctx) }

// WithIoTDeviceRequest marks the request as a kiosk request, the way the
// device middleware does, so services authorize through the device session.
func WithIoTDeviceRequest() RequestOption {
	return func(req *http.Request) {
		*req = *req.WithContext(context.WithValue(req.Context(), device.CtxIsIoTDevice, true))
	}
}

// WithStaffContext adds a staff member to the request context.
// This is used for testing endpoints that require staff authentication.
func WithStaffContext(s *users.Staff) RequestOption {
	return func(req *http.Request) {
		ctx := context.WithValue(req.Context(), device.CtxStaff, StaffPrincipal(s))
		*req = *req.WithContext(ctx)
	}
}
