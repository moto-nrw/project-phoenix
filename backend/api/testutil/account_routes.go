package testutil

import (
	"context"
	"encoding/json"
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
