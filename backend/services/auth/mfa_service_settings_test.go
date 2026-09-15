package auth_test

// Coverage for the MFA service branches that depend on a wired
// SettingsService and a wired email Dispatcher. Without those collaborators
// the trusted-device helpers and the dispatch-* functions short-circuit on
// the "nil settings" / "nil dispatcher" guards, leaving large branches of
// real code uncovered. This file wires both so the per-tenant settings
// path and the synchronous email delivery path get exercised end-to-end.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/auth"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	_ "github.com/moto-nrw/project-phoenix/services/config/defaults"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// --- minimal in-memory settings repos (real settingsService backed by mocks) ---

type mfaTestValueRepo struct {
	mu     sync.Mutex
	values map[string]*configModel.SettingValue
}

func newMFATestValueRepo() *mfaTestValueRepo {
	return &mfaTestValueRepo{values: make(map[string]*configModel.SettingValue)}
}

func (r *mfaTestValueRepo) key(tenantID int64, settingKey string) string {
	return fmt.Sprintf("%d:%s", tenantID, settingKey)
}

func (r *mfaTestValueRepo) FindByTenantAndKey(_ context.Context, tenantID int64, settingKey string) (*configModel.SettingValue, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sv, ok := r.values[r.key(tenantID, settingKey)]; ok {
		return sv, nil
	}
	return nil, nil
}

func (r *mfaTestValueRepo) FindByTenantAndKeys(_ context.Context, tenantID int64, settingKeys []string) ([]*configModel.SettingValue, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	requested := make(map[string]struct{}, len(settingKeys))
	for _, key := range settingKeys {
		requested[key] = struct{}{}
	}
	prefix := fmt.Sprintf("%d:", tenantID)
	out := []*configModel.SettingValue{}
	for k, v := range r.values {
		if len(k) <= len(prefix) || k[:len(prefix)] != prefix {
			continue
		}
		if _, ok := requested[v.SettingKey]; !ok {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

func (r *mfaTestValueRepo) FindByTenantsAndKeys(_ context.Context, tenantIDs []int64, settingKeys []string) ([]*configModel.SettingValue, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tenants := make(map[int64]struct{}, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		tenants[tenantID] = struct{}{}
	}
	requested := make(map[string]struct{}, len(settingKeys))
	for _, key := range settingKeys {
		requested[key] = struct{}{}
	}
	var out []*configModel.SettingValue
	for _, value := range r.values {
		if _, ok := tenants[value.GetTenantID()]; !ok {
			continue
		}
		if _, ok := requested[value.SettingKey]; ok {
			out = append(out, value)
		}
	}
	return out, nil
}

func (r *mfaTestValueRepo) Upsert(_ context.Context, sv *configModel.SettingValue) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values[r.key(sv.GetTenantID(), sv.SettingKey)] = sv
	return nil
}

func (r *mfaTestValueRepo) Delete(_ context.Context, tenantID int64, settingKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.values, r.key(tenantID, settingKey))
	return nil
}

func (r *mfaTestValueRepo) setOverride(t *testing.T, tenantID int64, key string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	sv := &configModel.SettingValue{
		SettingKey: key,
		Value:      raw,
	}
	sv.SetTenantID(tenantID)
	require.NoError(t, r.Upsert(context.Background(), sv))
}

type mfaTestAuditRepo struct{}

func (m *mfaTestAuditRepo) Create(_ context.Context, _ *configModel.SettingAuditEntry) error {
	return nil
}

// --- capturing mailer reused across this file ---

// --- shared helpers for this file ---

const settingsJWTSecret = "test-secret-must-be-at-least-32-chars-long-for-real"

type wiredMFAFixture struct {
	svc        auth.MFAService
	repos      *repositories.Factory
	tenantID   int64
	valueRepo  *mfaTestValueRepo
	mailer     *testpkg.CapturingMailer
	dispatcher *email.Dispatcher
	settings   configSvc.SettingsService
}

// newWiredMFAFixture wires a real MFA service against the test DB with a
// real settings service backed by an in-memory value repo and a real
// dispatcher backed by a capturing mailer.
func newWiredMFAFixture(t *testing.T) *wiredMFAFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	valueRepo := newMFATestValueRepo()
	settings := configSvc.NewSettingsService(valueRepo, &mfaTestAuditRepo{}, nil, testpkg.SettingsRuntime(t, db), slog.Default())

	mailer := testpkg.NewCapturingMailer()
	dispatcher := email.NewDispatcher(mailer, slog.Default())
	dispatcher.SetDefaults(1, []time.Duration{time.Millisecond}) // no retries, instant

	tokenAuth, err := authjwt.NewTokenAuthWithSecret(settingsJWTSecret)
	require.NoError(t, err)

	svc, err := auth.NewMFAService(auth.MFAServiceConfig{
		Repos:       repos,
		TokenAuth:   tokenAuth,
		Settings:    settings,
		Dispatcher:  dispatcher,
		DefaultFrom: email.NewEmail("Moto Tests", "tests@example.test"),
		FrontendURL: "https://moto.test/",
		JWTSecret:   settingsJWTSecret,
		DB:          db,
	})
	require.NoError(t, err)
	testpkg.SetTenantRuntime(t, svc, db)
	testpkg.SetTenantRuntime(t, settings, db)

	return &wiredMFAFixture{
		svc:        svc,
		repos:      repos,
		tenantID:   tenantID,
		valueRepo:  valueRepo,
		mailer:     mailer,
		dispatcher: dispatcher,
		settings:   settings,
	}
}

// --- IsRequired: drive each mode value ---

func TestMFAService_IsRequired_ModeOff_ReturnsFalse(t *testing.T) {
	t.Parallel()

	fix := newWiredMFAFixture(t)
	fix.valueRepo.setOverride(t, fix.tenantID, configModel.KeyMFAMode, configModel.MFAModeOff)

	acc := &authModel.Account{}
	acc.ID = 4242
	required, err := fix.svc.IsRequired(context.Background(), acc, fix.tenantID)
	require.NoError(t, err)
	assert.False(t, required, "mfa_mode=off must short-circuit to not-required")
}

func TestMFAService_IsRequired_ModeRequiredAll_ReturnsTrue(t *testing.T) {
	t.Parallel()

	fix := newWiredMFAFixture(t)
	fix.valueRepo.setOverride(t, fix.tenantID, configModel.KeyMFAMode, configModel.MFAModeRequiredAll)

	acc := &authModel.Account{}
	acc.ID = 4242
	required, err := fix.svc.IsRequired(context.Background(), acc, fix.tenantID)
	require.NoError(t, err)
	assert.True(t, required, "mfa_mode=required_all must require for any role")
}

func TestMFAService_IsRequired_ModeRequiredAdmins_RequiresAdminsOnly(t *testing.T) {
	t.Parallel()

	fix := newWiredMFAFixture(t)
	fix.valueRepo.setOverride(t, fix.tenantID, configModel.KeyMFAMode, configModel.MFAModeRequiredAdmins)

	adminAcc := &authModel.Account{
		Roles: []*authModel.Role{{Name: "admin"}},
	}
	adminAcc.ID = 4242
	teacherAcc := &authModel.Account{
		Roles: []*authModel.Role{{Name: "teacher"}},
	}
	teacherAcc.ID = 4243

	required, err := fix.svc.IsRequired(context.Background(), adminAcc, fix.tenantID)
	require.NoError(t, err)
	assert.True(t, required, "admins must require MFA when mode=required_admins")

	required, err = fix.svc.IsRequired(context.Background(), teacherAcc, fix.tenantID)
	require.NoError(t, err)
	assert.False(t, required, "non-admins must skip MFA when mode=required_admins")
}

func TestMFAService_IsRequired_UnknownModeFallsBackToOff(t *testing.T) {
	t.Parallel()

	fix := newWiredMFAFixture(t)
	fix.valueRepo.setOverride(t, fix.tenantID, configModel.KeyMFAMode, "garbage_mode_value")

	acc := &authModel.Account{}
	acc.ID = 4242
	required, err := fix.svc.IsRequired(context.Background(), acc, fix.tenantID)
	require.NoError(t, err)
	assert.False(t, required, "unknown mode must be treated as off, not panic")
}

func TestMFAService_IsRequired_NoTenantID_UsesRegistryDefault(t *testing.T) {
	t.Parallel()

	fix := newWiredMFAFixture(t)
	// No override — should hit the ResolveString path (registry default = off).
	acc := &authModel.Account{}
	acc.ID = 4242
	required, err := fix.svc.IsRequired(context.Background(), acc, 0)
	require.NoError(t, err)
	assert.False(t, required, "registry default for mfa_mode is 'off'")
}

// --- IsTrustedDeviceEnabled & TrustedDeviceDays: per-tenant resolution ---

func TestMFAService_IsTrustedDeviceEnabled_TenantOverrideRespected(t *testing.T) {
	t.Parallel()

	fix := newWiredMFAFixture(t)

	// Registry default for trusted_device_enabled is true. Explicitly flip
	// it to false for this tenant and the per-tenant resolver must pick it up.
	fix.valueRepo.setOverride(t, fix.tenantID, configModel.KeyMFATrustedDeviceEnabled, false)

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
	fix.valueRepo.setOverride(t, fix.tenantID, configModel.KeyMFATrustedDeviceDays, 14)

	days := fix.svc.TrustedDeviceDays(context.Background(), fix.tenantID)
	assert.Equal(t, 14, days, "tenant override must override the registry default")
}

func TestMFAService_TrustedDeviceDays_NegativeOverrideFallsBackToDefault(t *testing.T) {
	t.Parallel()

	fix := newWiredMFAFixture(t)
	// A bad value (≤0) must fall through to the package constant default.
	fix.valueRepo.setOverride(t, fix.tenantID, configModel.KeyMFATrustedDeviceDays, -5)
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
	fix.valueRepo.setOverride(t, fix.tenantID, configModel.KeyMFATrustedDeviceEnabled, false)

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

	fix.valueRepo.setOverride(t, fix.tenantID, configModel.KeyMFATrustedDeviceEnabled, false)
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
	require.NoError(t, fix.svc.Enroll(ctx, acc.ID))

	_, err := fix.svc.StartChallenge(ctx, acc.ID, fix.tenantID, authjwt.MFAChallengeScopeTenant, net.ParseIP("203.0.113.42"))
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
	require.NoError(t, fix.svc.Enroll(ctx, acc.ID))

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
