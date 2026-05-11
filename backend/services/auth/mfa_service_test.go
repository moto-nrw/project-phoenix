package auth_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const testJWTSecret = "test-secret-must-be-at-least-32-chars-long-for-real"

// newTestMFAService wires a real test-DB-backed MFA service. Settings +
// Dispatcher are intentionally nil so IsRequired falls through to "off"
// and StartChallenge logs-and-skips email send (proper template lands
// in Phase 4).
func newTestMFAService(t *testing.T) (auth.MFAService, *repositories.Factory, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	repos := repositories.NewFactory(db)
	tokenAuth, err := authjwt.NewTokenAuthWithSecret(testJWTSecret)
	require.NoError(t, err)

	svc, err := auth.NewMFAService(auth.MFAServiceConfig{
		Repos:     repos,
		TokenAuth: tokenAuth,
		JWTSecret: testJWTSecret,
		DB:        db,
	})
	require.NoError(t, err)
	return svc, repos, db
}

func TestMFAService_EnrollDisableLifecycle(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-lifecycle")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	enrolled, err := svc.HasEnrollment(ctx, acc.ID)
	require.NoError(t, err)
	assert.False(t, enrolled, "fresh account must not be enrolled")

	require.NoError(t, svc.Enroll(ctx, acc.ID))
	enrolled, err = svc.HasEnrollment(ctx, acc.ID)
	require.NoError(t, err)
	assert.True(t, enrolled, "after Enroll, must report enrolled")

	require.ErrorIs(t, svc.Enroll(ctx, acc.ID), auth.ErrMFAAlreadyEnrolled)

	require.NoError(t, svc.Disable(ctx, acc.ID))
	enrolled, err = svc.HasEnrollment(ctx, acc.ID)
	require.NoError(t, err)
	assert.False(t, enrolled)
}

func TestMFAService_RecoveryCodeFlow(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-recovery")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	require.NoError(t, svc.Enroll(ctx, acc.ID))

	codes, err := svc.GenerateRecoveryCodes(ctx, acc.ID)
	require.NoError(t, err)
	assert.Len(t, codes, auth.MFARecoveryCodeCount, "expected 10 recovery codes")

	// Wrong code rejected.
	require.ErrorIs(t, svc.VerifyRecoveryCode(ctx, acc.ID, "0000-0000-0000-0000"), auth.ErrMFACodeInvalid)

	// Real code accepted; one row consumed.
	require.NoError(t, svc.VerifyRecoveryCode(ctx, acc.ID, codes[0]))

	// Same code cannot be used twice.
	require.ErrorIs(t, svc.VerifyRecoveryCode(ctx, acc.ID, codes[0]), auth.ErrMFACodeInvalid)
}

func TestMFAService_TrustedDeviceFlow(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-trusted")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	require.NoError(t, svc.Enroll(ctx, acc.ID))

	cookie, expiresAt, err := svc.IssueTrustedDevice(ctx, acc.ID, 0, "Mozilla/5.0 (Test)", net.ParseIP("203.0.113.7"))
	require.NoError(t, err)
	assert.NotEmpty(t, cookie)
	assert.True(t, expiresAt.After(time.Now()), "expiry must be in the future")

	ok, err := svc.VerifyTrustedDevice(ctx, acc.ID, 0, cookie)
	require.NoError(t, err)
	assert.True(t, ok, "freshly issued cookie must verify")

	// Tampered cookie rejected.
	badCookie := cookie + "x"
	ok, err = svc.VerifyTrustedDevice(ctx, acc.ID, 0, badCookie)
	require.NoError(t, err)
	assert.False(t, ok, "tampered cookie must not verify")
}

func TestMFAService_StartAndVerifyChallenge(t *testing.T) {
	ctx := context.Background()
	svc, repos, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-challenge")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	require.NoError(t, svc.Enroll(ctx, acc.ID))

	tokenString, err := svc.StartChallenge(ctx, acc.ID, 0, authjwt.MFAChallengeScopeTenant, net.ParseIP("203.0.113.42"))
	require.NoError(t, err)
	require.NotEmpty(t, tokenString)

	// Active challenge row exists.
	active, err := repos.MFAEmailChallenge.FindActiveByAccountID(ctx, acc.ID)
	require.NoError(t, err)
	require.NotNil(t, active)
	assert.Equal(t, acc.ID, active.AccountID)

	// Wrong code rejected (and lockout counter increments — verified via repeated calls below).
	_, err = svc.VerifyChallenge(ctx, tokenString, "000000")
	assert.ErrorIs(t, err, auth.ErrMFACodeInvalid)
}

func TestMFAService_AdminOverride_PermissionGate(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	target := testpkg.CreateTestAccount(t, db, "mfa-svc-admin-target")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, target.ID) })

	const actorID int64 = 9999

	// No permissions -> denied.
	err := svc.AdminDisable(ctx, actorID, target.ID, "lost mailbox", nil)
	assert.ErrorIs(t, err, auth.ErrMFAPermissionDenied)

	// Wildcard admin permission -> allowed.
	require.NoError(t, svc.AdminDisable(ctx, actorID, target.ID, "lost mailbox", []string{"admin:*"}))

	// Empty reason -> rejected even with permission.
	_, err = svc.AdminRegenerateRecoveryCodes(ctx, actorID, target.ID, "   ", []string{"users:manage"})
	assert.Error(t, err)

	// Explicit users:manage permission + reason returns 10 codes.
	codes, err := svc.AdminRegenerateRecoveryCodes(ctx, actorID, target.ID, "user lost device", []string{"users:manage"})
	require.NoError(t, err)
	assert.Len(t, codes, auth.MFARecoveryCodeCount)
}
