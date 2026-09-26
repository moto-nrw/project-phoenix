package testutil

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The helpers below serve the adapter tests of the Identity & Access account
// routes (modules/identityaccess/inbound/account, #2736). They name the root's
// bindings and the token package through the test boundary, so the adapter
// tests import neither the root composition nor the legacy token package.

// The two helpers below drive the parents portal for the demo role parent
// (#3468): the adapter test proves what the issued parent session may reach,
// without naming the root composition or the portal's own packages. They
// answer plain values, so nothing of the portal crosses this boundary.

// ParentPortalChildIDs are the students the account sees in the parents
// portal: exactly the children its guardian relationships permit.
func ParentPortalChildIDs(t *testing.T, ctx context.Context, db *bun.DB, module services.StudentTestModule, accountID int64) []int64 {
	t.Helper()
	portal, err := services.NewParentCareScheduleTestService(db, module)
	require.NoError(t, err)
	children, err := portal.ListChildrenForAccount(ctx, accountID)
	require.NoError(t, err)
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		ids = append(ids, child.StudentID)
	}
	return ids
}

// SubmitParentCareScheduleRequest asks for a new pickup time as the account
// and returns the pending request. The error is the portal's own: a child the
// account may not reach is refused here.
func SubmitParentCareScheduleRequest(t *testing.T, ctx context.Context, db *bun.DB, module services.StudentTestModule, accountID, studentID int64, payload map[string]any) (requestID int64, err error) {
	t.Helper()
	portal, composeErr := services.NewParentCareScheduleTestService(db, module)
	require.NoError(t, composeErr)
	view, err := portal.CreateCareScheduleRequest(ctx, accountID, studentID, payload)
	if err != nil {
		return 0, err
	}
	require.NotNil(t, view.PendingRequest)
	return view.PendingRequest.ID, nil
}

// AccountRouteRuntime is the tenant runtime the account routes run in.
type AccountRouteRuntime interface {
	WithinCurrentTenant(ctx context.Context, fn func(context.Context) error) error
	WithinAdmin(ctx context.Context, fn func(context.Context) error) error
	MarkRollback(ctx context.Context)
	WithTenantID(ctx context.Context, tenantID int64) context.Context
	TenantID(ctx context.Context) int64
}

// AccountRouteTenantRuntime returns the tenant runtime the root binds to the
// account routes.
func AccountRouteTenantRuntime() AccountRouteRuntime {
	return services.AccountRouteTenantRuntime()
}

// AccountRoleGrantPolicy returns Security Runtime's role grant decision the
// root binds to the account routes.
func AccountRoleGrantPolicy() services.RoleGrantPolicy {
	return services.RoleGrantPolicy{}
}

// IsDemoEnvironment reports whether appEnv mounts the demo routes, the
// decision the root passes to the account routes' demo mount.
func IsDemoEnvironment(appEnv string) bool {
	return services.IsDemoEnvironment(appEnv)
}

// AuthTestOption overrides a default of the composed auth test module.
type AuthTestOption = services.AuthTestOption

// WithDemoMaxActiveSchools composes the demo access with this capacity.
func WithDemoMaxActiveSchools(capacity int) AuthTestOption {
	return services.WithDemoMaxActiveSchools(capacity)
}

// WithStandingDemoSchool composes the demo access so every access enters the
// school with this slug.
func WithStandingDemoSchool(slug string) AuthTestOption {
	return services.WithStandingDemoSchool(slug)
}

// WithDemoCapturingMailer composes the module on mails behind the demo
// environment's mail lock, so a test reads the mails the demo flows send.
func WithDemoCapturingMailer(mails *testpkg.CapturingMailer) AuthTestOption {
	return services.WithAuthTestMailer(mails.InDemoEnvironment())
}

// WithAuthTestLogger composes the module on the given logger.
func WithAuthTestLogger(logger *slog.Logger) AuthTestOption {
	return services.WithAuthTestLogger(logger)
}

// SetupAuthModuleOn composes the auth test module over db with the options.
func SetupAuthModuleOn(t *testing.T, db *bun.DB, options ...AuthTestOption) services.AuthTestModule {
	t.Helper()
	module, err := services.NewAuthTestModule(db, testpkg.TenantRuntime(t, db), options...)
	require.NoError(t, err)
	return module
}

// StoreRawTenantSetting stores a tenant override without the registry's
// checks, for tests of readers meeting a row the registry no longer accepts.
func StoreRawTenantSetting(t *testing.T, db *bun.DB, tenantID int64, key string, raw json.RawMessage) {
	t.Helper()
	ctx := testpkg.TenantContext(tenantID)
	require.NoError(t, services.StoreRawTenantSettingForTests(ctx, db, tenantID, key, raw),
		"store raw %s override", key)
}

// SettingsRequestCacheMiddleware attaches the request-scoped settings memo
// cache the root router attaches to every request.
func SettingsRequestCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(services.WithSettingsRequestCacheForTests(r.Context())))
	})
}

// ClaimsFromContext returns the claims the auth middleware put on ctx.
func ClaimsFromContext(ctx context.Context) Claims {
	return jwt.ClaimsFromCtx(ctx)
}

// MFAEnrollmentClaims is the enrollment-only token's claims.
type MFAEnrollmentClaims = jwt.MFAEnrollmentClaims

// Enrollment token scopes.
const (
	MFAEnrollmentScopeTenant   = jwt.MFAEnrollmentScopeTenant
	MFAEnrollmentScopePlatform = jwt.MFAEnrollmentScopePlatform
)

// WithEnrollmentClaims puts enrollment-only claims on ctx the way the
// enrollment authenticator does.
func WithEnrollmentClaims(ctx context.Context, claims MFAEnrollmentClaims) context.Context {
	return context.WithValue(ctx, jwt.CtxEnrollmentClaims, claims)
}

// OpaqueCapabilityFingerprint returns the stored fingerprint of an opaque
// capability token.
func OpaqueCapabilityFingerprint(token string) string {
	return jwt.OpaqueCapabilityFingerprint(token)
}
