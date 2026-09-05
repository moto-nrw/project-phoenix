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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	feedbackCompose "github.com/moto-nrw/project-phoenix/modules/feedback/compose"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// HTTP header constants (S1192 - avoid duplicate string literals)
const (
	headerContentType = "Content-Type"
	contentTypeJSON   = "application/json"
)

// SetupSettingsModule builds the settings route's real application boundary.
func SetupSettingsModule(t *testing.T) (*bun.DB, services.SettingsTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewSettingsTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func SetupFileStoreModule(t *testing.T) (*bun.DB, services.FileStoreTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewFileStoreTestModule(db, testpkg.TenantRuntime(t, db))
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
	module, err := services.NewGradeTransitionTestModule(db, clocks...)
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

func SetupCheckinModule(t *testing.T) (*bun.DB, services.CheckinTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewCheckinTestModule(db, testpkg.TenantRuntime(t, db))
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

func SetupGroupsModule(t *testing.T) (*bun.DB, services.GroupsTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewGroupsTestModule(db, testpkg.TenantRuntime(t, db))
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

func SetupImportModule(t *testing.T) (*bun.DB, services.ImportTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
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

// RequestOption configures an HTTP request for testing.
type RequestOption func(*http.Request)

// WithPermissions adds permissions to the request context.
func WithPermissions(permissions ...string) RequestOption {
	return func(req *http.Request) {
		ctx := context.WithValue(req.Context(), jwt.CtxPermissions, permissions)
		*req = *req.WithContext(ctx)
	}
}

// WithClaims adds JWT claims to the request context.
// Also injects tenant context (mirroring TenantMiddleware) so that
// handler-level WithTenantTx can read the tenant ID. Claims carrying the
// bootstrap tenant follow the test into its own tenant (#2419).
func WithClaims(tb testing.TB, claims jwt.AppClaims) RequestOption {
	claims.TenantID = testpkg.RebaseTenantID(tb, claims.TenantID)
	return func(req *http.Request) {
		ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
		if claims.TenantID != 0 {
			ctx = tenant.WithTenantID(ctx, claims.TenantID)
		}
		*req = *req.WithContext(ctx)
	}
}

// WithTestTenant supplies the package test's tenant without exposing tenant
// runtime details to adapter tests.
func WithTestTenant(tb testing.TB) RequestOption {
	tenantID := testpkg.Tenant(tb)
	return func(req *http.Request) {
		*req = *req.WithContext(tenant.WithTenantID(req.Context(), tenantID))
	}
}

// ProtectedTestTenantGroup is the authorization-free test boundary for HTTP
// adapters whose permission middleware is tested separately. It retains the
// real tenant transaction and request caches.
func ProtectedTestTenantGroup(db *bun.DB, r chi.Router, fn func(chi.Router, func(http.Handler) http.Handler)) {
	r.Group(func(gr chi.Router) {
		gr.Use(testpkg.TenantTxMiddleware(db))
		fn(gr, func(next http.Handler) http.Handler { return next })
	})
}

func ProtectedTestTenantGroupFunc(db *bun.DB) func(chi.Router, func(chi.Router, func(http.Handler) http.Handler)) {
	return func(r chi.Router, fn func(chi.Router, func(http.Handler) http.Handler)) {
		ProtectedTestTenantGroup(db, r, fn)
	}
}

func UnprotectedGroupFunc() func(chi.Router, func(chi.Router, func(http.Handler) http.Handler)) {
	return func(r chi.Router, fn func(chi.Router, func(http.Handler) http.Handler)) {
		fn(r, IdentityMiddleware)
	}
}

func RecordingUnprotectedGroupFunc(called *bool) func(chi.Router, func(chi.Router, func(http.Handler) http.Handler)) {
	return func(r chi.Router, fn func(chi.Router, func(http.Handler) http.Handler)) {
		*called = true
		fn(r, IdentityMiddleware)
	}
}

func IdentityMiddleware(next http.Handler) http.Handler { return next }

func RespondSuccess(w http.ResponseWriter, r *http.Request, status int, data any, message string) {
	render.Status(r, status)
	render.JSON(w, r, Response{Status: "success", Data: data, Message: message})
}

func RespondNoContent(w http.ResponseWriter, r *http.Request) { render.NoContent(w, r) }

func RespondError(w http.ResponseWriter, r *http.Request, status int, err error) {
	render.Status(r, status)
	render.JSON(w, r, Response{Status: "error", Error: err.Error()})
}

func RespondInvalidRequest(w http.ResponseWriter, r *http.Request, err error) {
	RespondError(w, r, http.StatusBadRequest, err)
}

func ErrorResponder(resolve func(error) (int, error)) func(http.ResponseWriter, *http.Request, error, string) {
	return func(w http.ResponseWriter, r *http.Request, err error, _ string) {
		status, responseErr := resolve(err)
		RespondError(w, r, status, responseErr)
	}
}

// WithJWTBearer sets an Authorization: Bearer <token> header on the request.
// Use together with MintTestJWT when exercising a Resource via Router(), where
// the production JWT middleware chain (Verifier → Authenticator → TenantMiddleware)
// runs and rejects requests that lack a real signed token.
func WithJWTBearer(token string) RequestOption {
	return func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// MintTestJWT signs a JWT for the given claims using the same configuration as
// production (jwt.NewTokenAuth reads the JWT secret from viper / env). Pair it
// with WithJWTBearer when calling handlers through Resource.Router() so the
// production auth middleware accepts the request.
//
// Callers must arrange for a non-empty auth_jwt_secret to be present before the
// Resource is constructed (typically via SeedTestJWTConfig in init() or
// TestMain). Without a secret, jwx refuses to HMAC-sign and this helper fails.
//
// Claims carrying the bootstrap tenant are rebased onto the tenant the test
// owns, so a test that opted into a per-test tenant gets a matching token
// without passing the tenant through every claims helper (#2419).
func MintTestJWT(t testing.TB, claims jwt.AppClaims) string {
	t.Helper()
	claims.TenantID = testpkg.RebaseTenantID(t, claims.TenantID)
	tokenAuth, err := jwt.NewTokenAuth()
	require.NoError(t, err, "MintTestJWT: NewTokenAuth")
	token, err := tokenAuth.CreateJWT(claims)
	require.NoError(t, err, "MintTestJWT: CreateJWT")
	return token
}

// SeedTestJWTConfig installs deterministic viper defaults for JWT auth so that
// tests work in environments without a populated .env (e.g. CI). Use it from
// init() or TestMain in test packages that build a Resource which calls
// jwt.MustNewTokenAuth() and then use MintTestJWT to sign requests — both
// callers must see the same secret.
//
// SetDefault leaves env-supplied values alone, so this is safe to call when
// AUTH_JWT_SECRET is already set in the environment.
func SeedTestJWTConfig() {
	viper.SetDefault("auth_jwt_secret", testpkg.TestJWTSecret)
	viper.SetDefault("auth_jwt_expiry", "15m")
	viper.SetDefault("auth_jwt_refresh_expiry", "1h")
}

// WithDeviceContext adds an IoT device to the request context.
// This is used for testing device-authenticated endpoints.
// Also injects the device's tenant_id so TenantTxMiddleware can create
// a tenant-scoped transaction (mirrors production device auth middleware).
func WithDeviceContext(d *iot.Device) RequestOption {
	return func(req *http.Request) {
		ctx := context.WithValue(req.Context(), device.CtxDevice, d)
		if tid := d.GetTenantID(); tid != 0 {
			ctx = tenant.WithTenantID(ctx, tid)
		}
		*req = *req.WithContext(ctx)
	}
}

// WithStaffContext adds a staff member to the request context.
// This is used for testing endpoints that require staff authentication.
func WithStaffContext(s *users.Staff) RequestOption {
	return func(req *http.Request) {
		ctx := context.WithValue(req.Context(), device.CtxStaff, s)
		*req = *req.WithContext(ctx)
	}
}

// NewRequest creates a new HTTP request for testing.
func NewRequest(method, target string, body io.Reader, opts ...RequestOption) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set(headerContentType, contentTypeJSON)

	for _, opt := range opts {
		opt(req)
	}

	return req
}

// NewAuthenticatedRequest creates a request with authentication context.
// This is a convenience function that combines common options.
func NewAuthenticatedRequest(t *testing.T, method, target string, body interface{}, opts ...RequestOption) *http.Request {
	t.Helper()

	var reader io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal JSON body: %v", err)
		}
		reader = bytes.NewBuffer(jsonBytes)
	}

	req := httptest.NewRequest(method, target, reader)
	req.Header.Set(headerContentType, contentTypeJSON)

	for _, opt := range opts {
		opt(req)
	}

	return req
}

// NewJSONRequest creates a request with JSON body.
func NewJSONRequest(t *testing.T, method, target string, body interface{}) *http.Request {
	t.Helper()

	var reader io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal JSON body: %v", err)
		}
		reader = bytes.NewBuffer(jsonBytes)
	}

	req := httptest.NewRequest(method, target, reader)
	req.Header.Set(headerContentType, contentTypeJSON)

	return req
}

// NewMultipartRequest creates a multipart form request with file upload.
func NewMultipartRequest(t *testing.T, method, target string, fieldName, fileName, content string, opts ...RequestOption) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Create form file field
	fw, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}

	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write file content: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(method, target, &buf)
	req.Header.Set(headerContentType, writer.FormDataContentType())

	for _, opt := range opts {
		opt(req)
	}

	return req
}

// NewTenantRouter creates a chi.Router with the test transaction boundary
// (testpkg.TenantTxMiddleware) at the root. Tests that inject identity into
// the request context get the same transaction decision as production;
// resource routers that apply the production jwt + TenantTxMiddleware chain
// themselves run it unchanged underneath, since their requests arrive here
// unauthenticated and pass through.
func NewTenantRouter(db *bun.DB) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Use(testpkg.TenantTxMiddleware(db))
	return router
}

// AuthenticationContext returns the identity values injected by request
// options, preferring an explicit test permission set over claims defaults.
func AuthenticationContext(ctx context.Context) (jwt.AppClaims, []string) {
	claims := jwt.ClaimsFromCtx(ctx)
	if granted := jwt.PermissionsFromCtx(ctx); granted != nil {
		return claims, granted
	}
	return claims, claims.Permissions
}

// ExecuteRequest executes an HTTP request against a Chi router and returns the response recorder.
func ExecuteRequest(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	ctx := testpkg.WithPackageTenantRuntime(req.Context())
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		// Request options inject identity before this helper installs the runtime.
		// Reapply the tenant so the adapter-owned repository scope matches the
		// production middleware order (runtime first, authentication second).
		ctx = tenant.WithTenantID(ctx, tenantID)
	}
	router.ServeHTTP(rr, req.WithContext(ctx))
	return rr
}

// ExecuteWithAuth signs a JWT for the given claims (used as-is, including any
// permissions they already carry) and executes the request through the router.
func ExecuteWithAuth(t *testing.T, router chi.Router, req *http.Request, claims jwt.AppClaims) *httptest.ResponseRecorder {
	t.Helper()
	req.Header.Set("Authorization", "Bearer "+MintTestJWT(t, claims))
	return ExecuteRequestForTest(t, router, req)
}

// ExecuteWithAuthPermissions folds the given permission set into the claims
// (replacing whatever they carried — an empty slice deliberately produces a
// permissionless token), signs a JWT, and executes the request.
func ExecuteWithAuthPermissions(t *testing.T, router chi.Router, req *http.Request, claims jwt.AppClaims, permissions []string) *httptest.ResponseRecorder {
	t.Helper()
	claims.Permissions = permissions
	req.Header.Set("Authorization", "Bearer "+MintTestJWT(t, claims))
	return ExecuteRequestForTest(t, router, req)
}

// ExecuteRequestForTest is ExecuteRequest with t's disposable database
// runtime when the test opted into one.
func ExecuteRequestForTest(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	ctx := testpkg.WithTestTenantRuntime(t, req.Context())
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		ctx = tenant.WithTenantID(ctx, tenantID)
	}
	router.ServeHTTP(rr, req.WithContext(ctx))
	return rr
}

// Response represents a standard API response for testing.
type Response struct {
	Status  string      `json:"status"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ParseResponse parses the response body into a Response struct.
func ParseResponse(t *testing.T, body []byte) Response {
	t.Helper()

	var response Response
	err := json.Unmarshal(body, &response)
	require.NoError(t, err, "Failed to parse response body: %s", string(body))
	return response
}

// ParseJSONResponse parses the response body into a map.
func ParseJSONResponse(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()

	var response map[string]interface{}
	err := json.Unmarshal(body, &response)
	require.NoError(t, err, "Failed to parse response body: %s", string(body))
	return response
}

// AssertSuccessResponse validates that the response has success status and expected HTTP code.
func AssertSuccessResponse(t *testing.T, rr *httptest.ResponseRecorder, expectedStatus int) {
	t.Helper()

	assert.Equal(t, expectedStatus, rr.Code, "Unexpected HTTP status code. Body: %s", rr.Body.String())

	if rr.Code == http.StatusNoContent {
		return // No body to parse
	}

	response := ParseResponse(t, rr.Body.Bytes())
	assert.Equal(t, "success", response.Status, "Expected success status. Body: %s", rr.Body.String())
}

// AssertErrorResponse validates that the response has error status and expected HTTP code.
// Note: Some handlers return {"status":"Invalid Request"} or {"status":"Not Found"} etc.
// instead of {"status":"error"}, so we only check the HTTP status code.
func AssertErrorResponse(t *testing.T, rr *httptest.ResponseRecorder, expectedStatus int) {
	t.Helper()

	assert.Equal(t, expectedStatus, rr.Code, "Unexpected HTTP status code. Body: %s", rr.Body.String())
}

// AssertUnauthorized validates a 401 Unauthorized response.
func AssertUnauthorized(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	AssertErrorResponse(t, rr, http.StatusUnauthorized)
}

// AssertForbidden validates a 403 Forbidden response.
// Note: The authorize middleware returns {"status":"Forbidden"} not {"status":"error"},
// so we only check the HTTP status code here, not the response body format.
func AssertForbidden(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, http.StatusForbidden, rr.Code, "Expected 403 Forbidden. Body: %s", rr.Body.String())
}

// AssertNotFound validates a 404 Not Found response.
func AssertNotFound(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	AssertErrorResponse(t, rr, http.StatusNotFound)
}

// AssertBadRequest validates a 400 Bad Request response.
func AssertBadRequest(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	AssertErrorResponse(t, rr, http.StatusBadRequest)
}

// DefaultTestClaims returns default JWT claims for testing.
func DefaultTestClaims() jwt.AppClaims {
	return jwt.AppClaims{
		ID:          1,
		Sub:         "test@example.com",
		Username:    "testuser",
		FirstName:   "Test",
		LastName:    "User",
		Roles:       []string{"admin"},
		Permissions: []string{"admin:*"},
		IsAdmin:     true,
		TenantID:    1,
	}
}

// TeacherTestClaims returns JWT claims for a teacher user.
func TeacherTestClaims(accountID int) jwt.AppClaims {
	return jwt.AppClaims{
		ID:          accountID,
		Sub:         "teacher@example.com",
		Username:    "teacher",
		FirstName:   "Test",
		LastName:    "Teacher",
		Roles:       []string{"user"},
		Permissions: []string{"students:read", "groups:read", "groups:update", "groups:list", "visits:read", "visits:create", "visits:update", "visits:delete", "visits:list", "activities:update", "activities:delete", "activities:list", "activities:manage", "activities:enroll", "activities:assign", "users:list", "rooms:list", "schedules:read", "schedules:list", "feedback:read", "feedback:list", "substitutions:read"},
		TenantID:    1,
	}
}

// AdminTestClaims returns JWT claims for an admin user.
func AdminTestClaims(accountID int) jwt.AppClaims {
	return AdminTestClaimsForTenant(accountID, 1)
}

// AdminTestClaimsForTenant returns admin JWT claims scoped to a specific tenant.
// Use this for tests that run in an isolated tenant (e.g., to avoid cross-package
// fixture interference) instead of the default test tenant id=1.
func AdminTestClaimsForTenant(accountID int, tenantID int64) jwt.AppClaims {
	return jwt.AppClaims{
		ID:          accountID,
		Sub:         "admin@example.com",
		Username:    "admin",
		FirstName:   "Admin",
		LastName:    "User",
		Roles:       []string{"admin"},
		Permissions: []string{"admin:*"},
		IsAdmin:     true,
		TenantID:    tenantID,
	}
}
