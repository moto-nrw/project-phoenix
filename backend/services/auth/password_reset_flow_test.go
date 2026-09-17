package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/services"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The password reset flows live in Identity & Access (#2722); the tests
// below drive them through the retained auth service the way the reset
// routes do, against a real database.

const resetPassword = "Str0ngP@ssword!" //nolint:gosec // test-only value, never a real credential

func newResetService(t *testing.T, db *bun.DB, options ...services.AuthTestOption) authService.AuthService {
	t.Helper()
	module, err := services.NewAuthTestModule(db, testpkg.TenantRuntime(t, db), options...)
	require.NoError(t, err)
	return module.Auth
}

func resetAccount(t *testing.T, db *bun.DB, name string) string {
	t.Helper()
	account := testpkg.CreateTestAccount(t, db, name)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	forgetResetWindow(t, db, account.Email)
	return account.Email
}

// forgetResetWindow removes the address's rate-limit window afterwards: the
// table carries no tenant, so the row would survive the test's tenant.
func forgetResetWindow(t *testing.T, db *bun.DB, emails ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, email := range emails {
			_, err := db.NewRaw(`DELETE FROM auth.password_reset_rate_limits WHERE email = ?`, email).Exec(context.Background())
			require.NoError(t, err)
		}
	})
}

func storedResetToken(t *testing.T, db *bun.DB, id int64) (used bool, expiry time.Time) {
	t.Helper()
	var row struct {
		Used   bool      `bun:"used"`
		Expiry time.Time `bun:"expiry"`
	}
	require.NoError(t, db.NewRaw(`SELECT used, expiry FROM auth.password_reset_tokens WHERE id = ?`, id).Scan(context.Background(), &row))
	return row.Used, row.Expiry
}

// A link is issued only for an address that can act on it, sets the password
// exactly once and never outlives its expiry.
func TestInitiatePasswordResetIssuesARedeemableLink(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := newResetService(t, db)
	ctx := testpkg.Ctx(t)
	email := resetAccount(t, db, "reset-flow")

	unknown, err := service.InitiatePasswordReset(ctx, "does-not-exist@test.local")
	require.NoError(t, err, "an unknown address answers like a known one")
	assert.Nil(t, unknown)

	token, err := service.InitiatePasswordReset(ctx, email)
	require.NoError(t, err)
	require.NotNil(t, token)
	assert.NotEmpty(t, token.Token)
	assert.True(t, token.Expiry.After(time.Now()), "a link is never issued already expired")

	require.NoError(t, service.ResetPassword(ctx, token.Token, resetPassword))
	used, _ := storedResetToken(t, db, token.ID)
	assert.True(t, used)

	err = service.ResetPassword(ctx, token.Token, resetPassword)
	require.ErrorIs(t, err, authService.ErrInvalidToken, "a spent link cannot set a second password")
	require.ErrorIs(t, service.ResetPassword(ctx, "unknown-token", resetPassword), authService.ErrInvalidToken)

	weak, err := service.InitiatePasswordReset(ctx, email)
	require.NoError(t, err)
	require.ErrorIs(t, service.ResetPassword(ctx, weak.Token, "weak"), authService.ErrPasswordTooWeak)
	used, _ = storedResetToken(t, db, weak.ID)
	assert.False(t, used, "a refused password leaves the link redeemable")

	expired, err := service.InitiatePasswordReset(ctx, email)
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE auth.password_reset_tokens SET expiry = NOW() - INTERVAL '1 minute' WHERE id = ?`, expired.ID).Exec(ctx)
	require.NoError(t, err)
	require.ErrorIs(t, service.ResetPassword(ctx, expired.Token, resetPassword), authService.ErrInvalidToken,
		"an expired link is refused")
}

// Issuing a link spends the account's previous one, so only the newest link
// in an inbox works.
func TestInitiatePasswordResetInvalidatesThePreviousLink(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := newResetService(t, db)
	ctx := testpkg.Ctx(t)
	email := resetAccount(t, db, "reset-previous")

	first, err := service.InitiatePasswordReset(ctx, email)
	require.NoError(t, err)
	second, err := service.InitiatePasswordReset(ctx, email)
	require.NoError(t, err)

	require.ErrorIs(t, service.ResetPassword(ctx, first.Token, resetPassword), authService.ErrInvalidToken)
	require.NoError(t, service.ResetPassword(ctx, second.Token, resetPassword))
}

// The parents and school portals serve only accounts that hold a role of
// that portal; every other address answers identically.
func TestPasswordResetScopesServeTheirPortalOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := newResetService(t, db)
	ctx := testpkg.Ctx(t)
	staff := resetAccount(t, db, "reset-scope-staff")
	parent := testpkg.CreateTestParentGuardianChain(t, db)
	teacher := testpkg.CreateTestAccount(t, db, "reset-scope-lehrkraft")
	testpkg.EnsureAccountTenant(t, db, teacher.ID, testpkg.Tenant(t))
	testpkg.AssignLehrkraftSystemRole(t, db, teacher.ID, testpkg.Tenant(t))

	parentLink, err := service.InitiateParentPasswordReset(ctx, parent.Email)
	require.NoError(t, err)
	assert.NotNil(t, parentLink, "a guardian resets on the parents portal")

	staffOnParents, err := service.InitiateParentPasswordReset(ctx, staff)
	require.NoError(t, err, "an ineligible account answers like an eligible one")
	assert.Nil(t, staffOnParents)

	schoolLink, err := service.InitiateSchoolPasswordReset(ctx, teacher.Email)
	require.NoError(t, err)
	assert.NotNil(t, schoolLink, "a Lehrkraft resets on the school portal")

	staffOnSchool, err := service.InitiateSchoolPasswordReset(ctx, staff)
	require.NoError(t, err)
	assert.Nil(t, staffOnSchool)

	staffLink, err := service.InitiatePasswordReset(ctx, staff)
	require.NoError(t, err)
	assert.NotNil(t, staffLink)
}

// The per-address window allows three requests an hour; the rejection tells
// the caller when to retry, which the frontend countdown reads.
func TestPasswordResetRateLimitBlocksAfterThreeAttempts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := newResetService(t, db, services.WithAuthTestPasswordResetRateLimit(true))
	ctx := testpkg.Ctx(t)
	email := resetAccount(t, db, "reset-rate-limit")

	for attempt := 1; attempt <= 3; attempt++ {
		link, err := service.InitiatePasswordReset(ctx, email)
		require.NoError(t, err, "attempt %d is within the window", attempt)
		require.NotNil(t, link)
	}

	_, err := service.InitiatePasswordReset(ctx, email)
	require.Error(t, err)
	var authErr *authService.AuthError
	require.True(t, errors.As(err, &authErr))
	require.ErrorIs(t, authErr.Err, authService.ErrRateLimitExceeded)
	var limited *authService.RateLimitError
	require.True(t, errors.As(authErr.Err, &limited))
	assert.Equal(t, 3, limited.Attempts)
	assert.True(t, limited.RetryAt.After(time.Now()))
	assert.Positive(t, limited.RetryAfterSeconds(time.Now()))

	other := resetAccount(t, db, "reset-rate-limit-other")
	link, err := service.InitiatePasswordReset(ctx, other)
	require.NoError(t, err, "the window is per address")
	require.NotNil(t, link)
}

// An address that receives no link is never counted, so an unauthenticated
// caller cannot fill the window table with arbitrary addresses.
func TestPasswordResetRateLimitCountsOnlyActionableRequests(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := newResetService(t, db, services.WithAuthTestPasswordResetRateLimit(true))
	ctx := testpkg.Ctx(t)
	unknown := "reset-unknown@test.local"
	forgetResetWindow(t, db, unknown)

	for range 5 {
		_, err := service.InitiatePasswordReset(ctx, unknown)
		require.NoError(t, err)
	}

	var windows int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM auth.password_reset_rate_limits WHERE email = ?`, unknown).Scan(ctx, &windows))
	assert.Zero(t, windows)
}

// Cleanup removes spent links and stale windows and keeps what is still in
// use: a live link and a window the rate limit still counts.
func TestPasswordResetCleanupKeepsLiveLinksAndTheWindow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service := newResetService(t, db, services.WithAuthTestPasswordResetRateLimit(true))
	ctx := testpkg.Ctx(t)
	email := resetAccount(t, db, "reset-cleanup")

	spent, err := service.InitiatePasswordReset(ctx, email)
	require.NoError(t, err)
	require.NoError(t, service.ResetPassword(ctx, spent.Token, resetPassword))
	live, err := service.InitiatePasswordReset(ctx, email)
	require.NoError(t, err)

	deleted, err := service.CleanupExpiredPasswordResetTokens(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, deleted, 1)
	var remaining []int64
	require.NoError(t, db.NewRaw(`SELECT id FROM auth.password_reset_tokens WHERE account_id = (SELECT account_id FROM auth.password_reset_tokens WHERE id = ?) ORDER BY id`, live.ID).Scan(ctx, &remaining))
	assert.Equal(t, []int64{live.ID}, remaining, "a redeemable link survives the cleanup")

	_, err = db.NewRaw(`UPDATE auth.password_reset_rate_limits SET window_start = NOW() - INTERVAL '25 hours' WHERE email = ?`, email).Exec(ctx)
	require.NoError(t, err)
	recent := resetAccount(t, db, "reset-cleanup-recent")
	_, err = service.InitiatePasswordReset(ctx, recent)
	require.NoError(t, err)

	deleted, err = service.CleanupExpiredRateLimits(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)
	var windows []string
	require.NoError(t, db.NewRaw(`SELECT email FROM auth.password_reset_rate_limits WHERE email IN (?, ?)`, email, recent).Scan(ctx, &windows))
	assert.Equal(t, []string{recent}, windows, "a window inside the day the cleanup keeps stays")
}
