package behavior_test

// Coverage for the MFA service branches that depend on a wired
// SettingsService and a wired email Dispatcher. Without those collaborators
// the trusted-device helpers and the dispatch-* functions short-circuit on
// the "nil settings" / "nil dispatcher" guards, leaving large branches of
// real code uncovered. This file wires both so the per-tenant settings
// path and the synchronous email delivery path get exercised end-to-end.

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// --- shared helpers for this file ---

type wiredMFAFixture struct {
	svc      identityaccess.AccountMFA
	repos    *repositories.Factory
	db       *bun.DB
	tenantID int64
	mailer   *testpkg.CapturingMailer
}

// newWiredMFAFixture wires a real MFA service against the test DB with the
// module's real settings service over the tenant settings rows and a real
// dispatcher backed by a capturing mailer.
func newWiredMFAFixture(t *testing.T) *wiredMFAFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	mailer := testpkg.NewCapturingMailer()
	module := newMFATestModule(t, db,
		services.WithAuthTestMailer(mailer),
		// No retries: a refused send must fail in milliseconds, not in the
		// production twenty seconds.
		services.WithAuthTestMFABackoff(time.Millisecond),
	)

	return &wiredMFAFixture{
		svc:      module.MFA,
		repos:    module.Repos,
		db:       db,
		tenantID: tenantID,
		mailer:   mailer,
	}
}

// setOverride stores a raw tenant override for the fixture's tenant, so the
// suite can drive values the registry would refuse as well as valid ones.
func (fix *wiredMFAFixture) setOverride(t *testing.T, key string, value any) {
	t.Helper()
	writeTenantSettingOverride(t, fix.db, fix.tenantID, key, value)
}

// --- IsRequired: drive each mode value ---

func TestMFAService_IsRequired_ModeOff_ReturnsFalse(t *testing.T) {
	t.Parallel()

	fix := newWiredMFAFixture(t)
	fix.setOverride(t, settingKeyMFAMode, mfaModeOff)

	required, err := fix.svc.IsRequired(context.Background(), 4242, nil, fix.tenantID)
	require.NoError(t, err)
	assert.False(t, required, "mfa_mode=off must short-circuit to not-required")
}

func TestMFAService_IsRequired_ModeRequiredAll_ReturnsTrue(t *testing.T) {
	t.Parallel()

	fix := newWiredMFAFixture(t)
	fix.setOverride(t, settingKeyMFAMode, mfaModeRequiredAll)

	required, err := fix.svc.IsRequired(context.Background(), 4242, nil, fix.tenantID)
	require.NoError(t, err)
	assert.True(t, required, "mfa_mode=required_all must require for any role")
}

func TestMFAService_IsRequired_ModeRequiredAdmins_RequiresAdminsOnly(t *testing.T) {
	t.Parallel()

	fix := newWiredMFAFixture(t)
	fix.setOverride(t, settingKeyMFAMode, mfaModeRequiredAdmins)

	required, err := fix.svc.IsRequired(context.Background(), 4242, []string{"admin"}, fix.tenantID)
	require.NoError(t, err)
	assert.True(t, required, "admins must require MFA when mode=required_admins")

	required, err = fix.svc.IsRequired(context.Background(), 4243, []string{"teacher"}, fix.tenantID)
	require.NoError(t, err)
	assert.False(t, required, "non-admins must skip MFA when mode=required_admins")
}

func TestMFAService_IsRequired_UnknownModeFallsBackToOff(t *testing.T) {
	t.Parallel()

	fix := newWiredMFAFixture(t)
	fix.setOverride(t, settingKeyMFAMode, "garbage_mode_value")

	required, err := fix.svc.IsRequired(context.Background(), 4242, nil, fix.tenantID)
	require.NoError(t, err)
	assert.False(t, required, "unknown mode must be treated as off, not panic")
}

func TestMFAService_IsRequired_NoTenantID_UsesRegistryDefault(t *testing.T) {
	t.Parallel()

	fix := newWiredMFAFixture(t)
	// No override — should hit the ResolveString path (registry default = off).
	required, err := fix.svc.IsRequired(context.Background(), 4242, nil, 0)
	require.NoError(t, err)
	assert.False(t, required, "registry default for mfa_mode is 'off'")
}

// --- IsTrustedDeviceEnabled & TrustedDeviceDays: per-tenant resolution ---

func TestMFAService_IsTrustedDeviceEnabled_TenantOverrideRespected(t *testing.T) {
	t.Parallel()

	fix := newWiredMFAFixture(t)

	// Registry default for trusted_device_enabled is true. Explicitly flip
	// it to false for this tenant and the per-tenant resolver must pick it up.
	fix.setOverride(t, settingKeyMFATrustedDeviceEnabled, false)

	enabled := fix.svc.IsTrustedDeviceEnabled(context.Background(), fix.tenantID)
	assert.False(t, enabled, "tenant override must win over registry default")
}

func TestMFAService_IsTrustedDeviceEnabled_NoOverride_UsesRegistryDefault(t *testing.T) {
	t.Parallel()

	fix := newWiredMFAFixture(t)
	enabled := fix.svc.IsTrustedDeviceEnabled(context.Background(), fix.tenantID)
	assert.True(t, enabled, "registry default for trusted_device_enabled is true")
}

func TestMFAService_TrustedDeviceDays_TenantOverrideRespected(t *testing.T) {
	t.Parallel()

	fix := newWiredMFAFixture(t)
	fix.setOverride(t, settingKeyMFATrustedDeviceDays, 14)

	days := fix.svc.TrustedDeviceDays(context.Background(), fix.tenantID)
	assert.Equal(t, 14, days, "tenant override must override the registry default")
}

func TestMFAService_TrustedDeviceDays_NegativeOverrideFallsBackToDefault(t *testing.T) {
	t.Parallel()

	fix := newWiredMFAFixture(t)
	// A bad value (≤0) must fall through to the package constant default.
	fix.setOverride(t, settingKeyMFATrustedDeviceDays, -5)
	days := fix.svc.TrustedDeviceDays(context.Background(), fix.tenantID)
	assert.Positive(t, days, "negative override must not break the resolver — it must fall back to the default")
}

// --- IssueTrustedDevice: trusted-device-disabled short-circuit ---

func TestMFAService_IssueTrustedDevice_DisabledBySetting_NoCookieIssued(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fix := newWiredMFAFixture(t)

	// Flip the per-tenant setting off — IssueTrustedDevice must short-circuit
	// to ("", zero, nil) instead of persisting a row.
	fix.setOverride(t, settingKeyMFATrustedDeviceEnabled, false)

	// Need a real account so the account FK is satisfied for any downstream call.
	// (IssueTrustedDevice never reaches persistence in the disabled branch.)
	_ = fix.repos.AuthEvent // any repo's db is fine; just confirm fixture wired
	cookie, expiresAt, err := fix.svc.IssueTrustedDevice(ctx, 99999, fix.tenantID, "ua", net.ParseIP("203.0.113.99"))
	require.NoError(t, err)
	assert.Empty(t, cookie, "disabled setting must skip cookie issuance")
	assert.True(t, expiresAt.IsZero(), "no expiry when no cookie is issued")
}

func TestMFAService_VerifyTrustedDevice_DisabledBySetting_ReturnsFalse(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fix := newWiredMFAFixture(t)

	fix.setOverride(t, settingKeyMFATrustedDeviceEnabled, false)
	ok, err := fix.svc.VerifyTrustedDevice(ctx, 99999, fix.tenantID, "any-cookie-value")
	require.NoError(t, err)
	assert.False(t, ok, "disabled setting must reject the cookie before signature check")
}

// --- dispatchChallengeEmail: real dispatcher + capturing mailer ---

func TestMFAService_StartChallenge_DispatchesEmail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fix := newWiredMFAFixture(t)

	db := testpkg.SetupTestDB(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-dispatch-challenge")
	require.NoError(t, fix.svc.EnrollMFA(ctx, acc.ID))

	_, err := fix.svc.StartMFAChallenge(ctx, acc.ID, fix.tenantID, identityaccess.MFAChallengeScopeTenant, net.ParseIP("203.0.113.42"))
	require.NoError(t, err)

	templates := fix.mailer.Templates()
	require.NotEmpty(t, templates)
	assert.Equal(t, "mfa-email-code.html", templates[0],
		"StartChallenge must dispatch the mfa-email-code template")
}

func TestMFAService_IssueTrustedDevice_DispatchesAddedEmail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fix := newWiredMFAFixture(t)

	db := testpkg.SetupTestDB(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-dispatch-trusted-added")
	require.NoError(t, fix.svc.EnrollMFA(ctx, acc.ID))

	cookie, _, err := fix.svc.IssueTrustedDevice(ctx, acc.ID, fix.tenantID,
		"Mozilla/5.0 (Macintosh; Intel Mac OS X) Chrome/120.0",
		net.ParseIP("203.0.113.50"))
	require.NoError(t, err)
	require.NotEmpty(t, cookie)

	if !fix.mailer.WaitForMessages(1, 3*time.Second) {
		t.Fatal("expected a trusted-device-added email to be dispatched")
	}
	templates := fix.mailer.Templates()
	require.NotEmpty(t, templates)
	assert.Equal(t, "trusted-device-added.html", templates[0],
		"IssueTrustedDevice must dispatch the trusted-device-added template")
}
