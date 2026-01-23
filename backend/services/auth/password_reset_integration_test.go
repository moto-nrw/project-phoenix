package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

func newPasswordResetTestEnv(t *testing.T) (*Service, *stubAccountRepository, *stubPasswordResetTokenRepository, *testRateLimitRepo, *stubTokenRepository, *capturingMailer, sqlmock.Sqlmock, func()) {
	service, accounts, tokens, rateRepo, sessions, mailer, mock, cleanup := newPasswordResetTestEnvWithMailer(t, newCapturingMailer())
	capturing, _ := mailer.(*capturingMailer)
	return service, accounts, tokens, rateRepo, sessions, capturing, mock, cleanup
}

func newPasswordResetTestEnvWithMailer(t *testing.T, mailer email.Mailer) (*Service, *stubAccountRepository, *stubPasswordResetTokenRepository, *testRateLimitRepo, *stubTokenRepository, email.Mailer, sqlmock.Sqlmock, func()) {
	t.Helper()

	prevRateLimitEnabled := viper.GetBool("rate_limit_enabled")
	viper.Set("rate_limit_enabled", true)
	t.Cleanup(func() {
		viper.Set("rate_limit_enabled", prevRateLimitEnabled)
	})

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	account := &authModel.Account{
		Model:  modelBase.Model{ID: 1},
		Email:  "user@example.com",
		Active: true,
	}
	accounts := newStubAccountRepository(account)
	resetTokens := newStubPasswordResetTokenRepository()
	rateRepo := newTestRateLimitRepo()
	sessionTokens := newStubTokenRepository()

	dispatcher := email.NewDispatcher(mailer)
	dispatcher.SetDefaults(3, []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond})

	// Create a mock repository factory for testing
	repos := &repositories.Factory{
		Account:                accounts,
		PasswordResetToken:     resetTokens,
		PasswordResetRateLimit: rateRepo,
		Token:                  sessionTokens,
	}

	service := &Service{
		repos:               repos,
		dispatcher:          dispatcher,
		defaultFrom:         newDefaultFromEmail(),
		frontendURL:         "http://localhost:3000",
		passwordResetExpiry: 30 * time.Minute,
		txHandler:           modelBase.NewTxHandler(bunDB),
	}

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}

	return service, accounts, resetTokens, rateRepo, sessionTokens, mailer, mock, cleanup
}

func TestInitiatePasswordResetSendsEmail(t *testing.T) {
	service, _, tokens, _, _, mailer, mock, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectCommit()

	token, err := service.InitiatePasswordReset(ctx, "user@example.com")
	require.NoError(t, err)
	require.NotNil(t, token)

	require.Eventually(t, func() bool {
		return len(mailer.Messages()) == 1
	}, 200*time.Millisecond, 10*time.Millisecond)

	msg := mailer.Messages()[0]
	require.Equal(t, "Passwort zurücksetzen", msg.Subject)
	require.Equal(t, "password-reset.html", msg.Template)
	content := msg.Content.(map[string]any)
	require.Contains(t, content, "ResetURL")
	require.Contains(t, content, "ExpiryMinutes")

	stored, ok := tokens.tokens[token.Token]
	require.True(t, ok, "token should be persisted")
	ttl := time.Until(stored.Expiry)
	require.GreaterOrEqual(t, ttl, 29*time.Minute)
	require.LessOrEqual(t, ttl, 31*time.Minute)
}

func TestInitiatePasswordResetEmailFailureRecordsError(t *testing.T) {
	flaky := newFlakyMailer(3, errors.New("smtp down"))
	originalBackoff := passwordResetEmailBackoff
	passwordResetEmailBackoff = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond}
	t.Cleanup(func() {
		passwordResetEmailBackoff = originalBackoff
	})
	service, _, tokens, _, _, _, mock, cleanup := newPasswordResetTestEnvWithMailer(t, flaky)
	t.Cleanup(cleanup)

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectCommit()

	token, err := service.InitiatePasswordReset(ctx, "user@example.com")
	require.NoError(t, err)
	require.NotNil(t, token)

	require.Eventually(t, func() bool {
		stored, findErr := tokens.FindByID(context.Background(), token.ID)
		if findErr != nil {
			return false
		}
		return stored.EmailRetryCount == 3 && stored.EmailError != nil && *stored.EmailError != "" && stored.EmailSentAt == nil
	}, time.Second, 20*time.Millisecond)

	require.Equal(t, 3, flaky.Attempts())
	require.Len(t, flaky.Messages(), 0)
}

func TestResetPasswordWithValidToken(t *testing.T) {
	service, accounts, tokens, _, sessionTokens, _, mock, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectCommit()
	token, err := service.InitiatePasswordReset(ctx, "user@example.com")
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectCommit()

	err = service.ResetPassword(ctx, token.Token, "Str0ngP@ssword!")
	require.NoError(t, err)

	account, err := accounts.FindByEmail(ctx, "user@example.com")
	require.NoError(t, err)
	require.NotNil(t, account.PasswordHash)
	require.NotEqual(t, "", *account.PasswordHash)
	require.True(t, tokens.tokens[token.Token].Used)

	require.Len(t, sessionTokens.DeletedAccountIDs(), 1)
	require.Equal(t, int64(1), sessionTokens.DeletedAccountIDs()[0])
}

func TestResetPasswordWithExpiredToken(t *testing.T) {
	service, _, tokens, _, _, _, _, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	expired := &authModel.PasswordResetToken{
		AccountID: 1,
		Token:     "expired-token",
		Expiry:    time.Now().Add(-1 * time.Minute),
	}
	require.NoError(t, tokens.Create(ctx, expired))

	err := service.ResetPassword(ctx, "expired-token", "Str0ngP@ssword!")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidToken))
}

func TestPasswordResetRateLimitBlocksAfterThreeAttempts(t *testing.T) {
	service, _, _, rateRepo, _, _, mock, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		mock.ExpectBegin()
		mock.ExpectCommit()
		_, err := service.InitiatePasswordReset(ctx, "user@example.com")
		require.NoError(t, err)
	}

	_, err := service.InitiatePasswordReset(ctx, "user@example.com")
	require.Error(t, err)

	var authErr *AuthError
	require.True(t, errors.As(err, &authErr))
	require.True(t, errors.Is(authErr.Err, ErrRateLimitExceeded))

	rateErr := authErr.Err.(*RateLimitError)
	require.Equal(t, passwordResetRateLimitThreshold, rateErr.Attempts)
	require.Equal(t, passwordResetRateLimitThreshold, rateRepo.Attempts())
	require.True(t, rateErr.RetryAt.After(time.Now()))
}

func TestInitiatePasswordResetNonExistentEmail(t *testing.T) {
	service, _, _, _, _, _, _, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Request password reset for a non-existent email
	token, err := service.InitiatePasswordReset(ctx, "nonexistent@example.com")

	// Should return nil, nil (no error revealed to avoid user enumeration)
	require.NoError(t, err)
	require.Nil(t, token)
}

func TestInitiatePasswordResetEmailNormalization(t *testing.T) {
	service, _, _, _, _, mailer, mock, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectCommit()

	// Use email with mixed case and whitespace
	token, err := service.InitiatePasswordReset(ctx, "  USER@Example.COM  ")
	require.NoError(t, err)
	require.NotNil(t, token)

	// Email should be normalized
	require.Eventually(t, func() bool {
		return len(mailer.Messages()) == 1
	}, 200*time.Millisecond, 10*time.Millisecond)
}

func TestCheckPasswordResetRateLimitDisabled(t *testing.T) {
	// Disable rate limiting
	prevRateLimitEnabled := viper.GetBool("rate_limit_enabled")
	viper.Set("rate_limit_enabled", false)
	t.Cleanup(func() {
		viper.Set("rate_limit_enabled", prevRateLimitEnabled)
	})

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	account := &authModel.Account{
		Model:  modelBase.Model{ID: 1},
		Email:  "user@example.com",
		Active: true,
	}
	accounts := newStubAccountRepository(account)
	resetTokens := newStubPasswordResetTokenRepository()
	mailer := newCapturingMailer()
	dispatcher := email.NewDispatcher(mailer)

	// Create a mock repository factory without rate limit repo
	repos := &repositories.Factory{
		Account:            accounts,
		PasswordResetToken: resetTokens,
		// Note: no PasswordResetRateLimit
	}

	service := &Service{
		repos:               repos,
		dispatcher:          dispatcher,
		defaultFrom:         newDefaultFromEmail(),
		frontendURL:         "http://localhost:3000",
		passwordResetExpiry: 30 * time.Minute,
		txHandler:           modelBase.NewTxHandler(bunDB),
	}

	ctx := context.Background()

	// Should be able to make unlimited requests when rate limiting is disabled
	for i := 0; i < 5; i++ {
		mock.ExpectBegin()
		mock.ExpectCommit()
		token, tokenErr := service.InitiatePasswordReset(ctx, "user@example.com")
		require.NoError(t, tokenErr, "attempt %d should succeed", i+1)
		require.NotNil(t, token, "attempt %d should return token", i+1)
	}

	mock.ExpectClose()
	require.NoError(t, bunDB.Close())
	require.NoError(t, sqlDB.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckPasswordResetRateLimitNilRepository(t *testing.T) {
	// Enable rate limiting but use nil repository
	prevRateLimitEnabled := viper.GetBool("rate_limit_enabled")
	viper.Set("rate_limit_enabled", true)
	t.Cleanup(func() {
		viper.Set("rate_limit_enabled", prevRateLimitEnabled)
	})

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	account := &authModel.Account{
		Model:  modelBase.Model{ID: 1},
		Email:  "user@example.com",
		Active: true,
	}
	accounts := newStubAccountRepository(account)
	resetTokens := newStubPasswordResetTokenRepository()
	mailer := newCapturingMailer()
	dispatcher := email.NewDispatcher(mailer)

	repos := &repositories.Factory{
		Account:                accounts,
		PasswordResetToken:     resetTokens,
		PasswordResetRateLimit: nil, // Nil rate limit repo
	}

	service := &Service{
		repos:               repos,
		dispatcher:          dispatcher,
		defaultFrom:         newDefaultFromEmail(),
		frontendURL:         "http://localhost:3000",
		passwordResetExpiry: 30 * time.Minute,
		txHandler:           modelBase.NewTxHandler(bunDB),
	}

	ctx := context.Background()

	// Should succeed even with nil rate limit repo
	mock.ExpectBegin()
	mock.ExpectCommit()

	token, tokenErr := service.InitiatePasswordReset(ctx, "user@example.com")
	require.NoError(t, tokenErr)
	require.NotNil(t, token)

	mock.ExpectClose()
	require.NoError(t, bunDB.Close())
	require.NoError(t, sqlDB.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckPasswordResetRateLimitCheckError(t *testing.T) {
	service, _, _, rateRepo, _, _, _, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Inject error for CheckRateLimit
	rateRepo.setCheckError(errors.New("database connection failed"))

	_, err := service.InitiatePasswordReset(ctx, "user@example.com")
	require.Error(t, err)

	var authErr *AuthError
	require.True(t, errors.As(err, &authErr))
	require.Contains(t, authErr.Error(), "check password reset rate limit")
}

func TestCheckPasswordResetRateLimitIncrementError(t *testing.T) {
	service, _, _, rateRepo, _, _, _, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Inject error for IncrementAttempts
	rateRepo.setIncrementError(errors.New("database write failed"))

	_, err := service.InitiatePasswordReset(ctx, "user@example.com")
	require.Error(t, err)

	var authErr *AuthError
	require.True(t, errors.As(err, &authErr))
	require.Contains(t, authErr.Error(), "increment password reset rate limit")
}

// raceConditionRateLimiter simulates a race condition scenario.
type raceConditionRateLimiter struct {
	checkAttempts     int
	incrementAttempts int
}

func (r *raceConditionRateLimiter) CheckRateLimit(_ context.Context, _ string) (*authModel.RateLimitState, error) {
	return &authModel.RateLimitState{
		Attempts: r.checkAttempts,
		RetryAt:  time.Now().Add(time.Hour),
	}, nil
}

func (r *raceConditionRateLimiter) IncrementAttempts(_ context.Context, _ string) (*authModel.RateLimitState, error) {
	return &authModel.RateLimitState{
		Attempts: r.incrementAttempts,
		RetryAt:  time.Now().Add(time.Hour),
	}, nil
}

func (r *raceConditionRateLimiter) CleanupExpired(_ context.Context) (int, error) {
	return 0, nil
}

// TestCheckPasswordResetRateLimitAfterIncrement tests the race condition scenario
// where rate limit is exceeded after IncrementAttempts but not before (CheckRateLimit).
func TestCheckPasswordResetRateLimitAfterIncrement(t *testing.T) {
	prevRateLimitEnabled := viper.GetBool("rate_limit_enabled")
	viper.Set("rate_limit_enabled", true)
	t.Cleanup(func() {
		viper.Set("rate_limit_enabled", prevRateLimitEnabled)
	})

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	account := &authModel.Account{
		Model:  modelBase.Model{ID: 1},
		Email:  "user@example.com",
		Active: true,
	}
	accounts := newStubAccountRepository(account)
	resetTokens := newStubPasswordResetTokenRepository()

	// Create a rate limiter that simulates a race condition:
	// - CheckRateLimit returns attempts < threshold (passes first check)
	// - IncrementAttempts returns attempts > threshold (fails second check)
	raceRateLimiter := &raceConditionRateLimiter{
		checkAttempts:     2, // Below threshold of 3
		incrementAttempts: 4, // Above threshold of 3
	}

	repos := &repositories.Factory{
		Account:                accounts,
		PasswordResetToken:     resetTokens,
		PasswordResetRateLimit: raceRateLimiter,
	}

	mailer := newCapturingMailer()
	dispatcher := email.NewDispatcher(mailer)

	service := &Service{
		repos:               repos,
		dispatcher:          dispatcher,
		defaultFrom:         newDefaultFromEmail(),
		frontendURL:         "http://localhost:3000",
		passwordResetExpiry: 30 * time.Minute,
		txHandler:           modelBase.NewTxHandler(bunDB),
	}

	ctx := context.Background()

	_, err = service.InitiatePasswordReset(ctx, "user@example.com")
	require.Error(t, err)

	var authErr *AuthError
	require.True(t, errors.As(err, &authErr))
	require.True(t, errors.Is(authErr.Err, ErrRateLimitExceeded))

	rateErr, ok := authErr.Err.(*RateLimitError)
	require.True(t, ok)
	require.Equal(t, 4, rateErr.Attempts)

	mock.ExpectClose()
	require.NoError(t, bunDB.Close())
	require.NoError(t, sqlDB.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreatePasswordResetTokenInvalidateError(t *testing.T) {
	service, _, tokens, _, _, _, mock, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Inject error for InvalidateTokensByAccountID
	tokens.setInvalidateError(errors.New("invalidate failed"))

	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err := service.InitiatePasswordReset(ctx, "user@example.com")
	require.Error(t, err)

	var authErr *AuthError
	require.True(t, errors.As(err, &authErr))
	require.Contains(t, authErr.Error(), "initiate password reset transaction")
}

func TestCreatePasswordResetTokenCreateError(t *testing.T) {
	service, _, tokens, _, _, _, mock, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Inject error for Create
	tokens.setCreateError(errors.New("token create failed"))

	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err := service.InitiatePasswordReset(ctx, "user@example.com")
	require.Error(t, err)

	var authErr *AuthError
	require.True(t, errors.As(err, &authErr))
	require.Contains(t, authErr.Error(), "initiate password reset transaction")
}

func TestDispatchPasswordResetEmailNoDispatcher(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	account := &authModel.Account{
		Model:  modelBase.Model{ID: 1},
		Email:  "user@example.com",
		Active: true,
	}
	accounts := newStubAccountRepository(account)
	resetTokens := newStubPasswordResetTokenRepository()
	rateRepo := newTestRateLimitRepo()

	prevRateLimitEnabled := viper.GetBool("rate_limit_enabled")
	viper.Set("rate_limit_enabled", true)
	t.Cleanup(func() {
		viper.Set("rate_limit_enabled", prevRateLimitEnabled)
	})

	repos := &repositories.Factory{
		Account:                accounts,
		PasswordResetToken:     resetTokens,
		PasswordResetRateLimit: rateRepo,
	}

	service := &Service{
		repos:               repos,
		dispatcher:          nil, // No dispatcher
		defaultFrom:         newDefaultFromEmail(),
		frontendURL:         "http://localhost:3000",
		passwordResetExpiry: 30 * time.Minute,
		txHandler:           modelBase.NewTxHandler(bunDB),
	}

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectCommit()

	// Should succeed even without dispatcher (email skipped with log)
	token, tokenErr := service.InitiatePasswordReset(ctx, "user@example.com")
	require.NoError(t, tokenErr)
	require.NotNil(t, token)

	mock.ExpectClose()
	require.NoError(t, bunDB.Close())
	require.NoError(t, sqlDB.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestResetPasswordWeakPassword(t *testing.T) {
	service, _, _, _, _, _, mock, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectCommit()

	token, err := service.InitiatePasswordReset(ctx, "user@example.com")
	require.NoError(t, err)

	// Test various weak passwords
	weakPasswords := []string{
		"short",          // Too short
		"alllowercase1!", // No uppercase
		"ALLUPPERCASE1!", // No lowercase
		"NoNumbers!!!!",  // No digits
		"NoSpecial12AB",  // No special characters
	}

	for _, weakPwd := range weakPasswords {
		err = service.ResetPassword(ctx, token.Token, weakPwd)
		require.Error(t, err, "should reject weak password: %s", weakPwd)

		var authErr *AuthError
		require.True(t, errors.As(err, &authErr))
		require.True(t, errors.Is(authErr.Err, ErrPasswordTooWeak), "should be ErrPasswordTooWeak for: %s", weakPwd)
	}
}

func TestResetPasswordUpdatePasswordError(t *testing.T) {
	service, accounts, _, _, _, _, mock, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectCommit()

	token, err := service.InitiatePasswordReset(ctx, "user@example.com")
	require.NoError(t, err)

	// Inject error for UpdatePassword
	accounts.setUpdatePasswordError(errors.New("update password failed"))

	mock.ExpectBegin()
	mock.ExpectRollback()

	err = service.ResetPassword(ctx, token.Token, "Str0ngP@ssword!")
	require.Error(t, err)

	var authErr *AuthError
	require.True(t, errors.As(err, &authErr))
	require.Contains(t, authErr.Error(), "reset password transaction")
}

func TestResetPasswordMarkAsUsedError(t *testing.T) {
	service, _, tokens, _, _, _, mock, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectCommit()

	token, err := service.InitiatePasswordReset(ctx, "user@example.com")
	require.NoError(t, err)

	// Inject error for MarkAsUsed
	tokens.setMarkAsUsedError(errors.New("mark as used failed"))

	mock.ExpectBegin()
	mock.ExpectRollback()

	err = service.ResetPassword(ctx, token.Token, "Str0ngP@ssword!")
	require.Error(t, err)

	var authErr *AuthError
	require.True(t, errors.As(err, &authErr))
	require.Contains(t, authErr.Error(), "reset password transaction")
}

func TestResetPasswordUsedToken(t *testing.T) {
	service, _, _, _, _, _, mock, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectCommit()

	token, err := service.InitiatePasswordReset(ctx, "user@example.com")
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectCommit()

	// Use the token successfully
	err = service.ResetPassword(ctx, token.Token, "Str0ngP@ssword!")
	require.NoError(t, err)

	// Try to use the same token again
	err = service.ResetPassword(ctx, token.Token, "An0therStr0ng!")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidToken))
}

func TestResetPasswordNonExistentToken(t *testing.T) {
	service, _, _, _, _, _, _, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	err := service.ResetPassword(ctx, "non-existent-token", "Str0ngP@ssword!")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidToken))
}

func TestPersistPasswordResetDeliverySuccessSetsEmailSentAt(t *testing.T) {
	service, _, tokens, _, _, mailer, mock, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectCommit()

	token, err := service.InitiatePasswordReset(ctx, "user@example.com")
	require.NoError(t, err)
	require.NotNil(t, token)

	// Wait for async email delivery
	require.Eventually(t, func() bool {
		return len(mailer.Messages()) == 1
	}, 200*time.Millisecond, 10*time.Millisecond)

	// Check that EmailSentAt was set
	require.Eventually(t, func() bool {
		stored, findErr := tokens.FindByID(context.Background(), token.ID)
		if findErr != nil {
			return false
		}
		return stored.EmailSentAt != nil
	}, 500*time.Millisecond, 20*time.Millisecond)
}

func TestPersistPasswordResetDeliveryUpdateError(t *testing.T) {
	service, _, tokens, _, _, mailer, mock, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectCommit()

	// Inject error for UpdateDeliveryResult
	tokens.setUpdateDeliveryError(errors.New("update delivery failed"))

	token, err := service.InitiatePasswordReset(ctx, "user@example.com")
	require.NoError(t, err)
	require.NotNil(t, token)

	// Wait for async email delivery (should still complete)
	require.Eventually(t, func() bool {
		return len(mailer.Messages()) == 1
	}, 200*time.Millisecond, 10*time.Millisecond)

	// The error should be logged but not affect the main flow
	// Token should still exist but EmailSentAt won't be set due to error
	stored, findErr := tokens.FindByID(ctx, token.ID)
	require.NoError(t, findErr)
	require.NotNil(t, stored)
}

func TestSanitizeEmailErrorNil(t *testing.T) {
	result := sanitizeEmailError(nil)
	require.Equal(t, "", result)
}

func TestSanitizeEmailErrorWithWhitespace(t *testing.T) {
	result := sanitizeEmailError(errors.New("  error message with spaces  "))
	require.Equal(t, "error message with spaces", result)
}

func TestSanitizeEmailErrorNonNil(t *testing.T) {
	result := sanitizeEmailError(errors.New("smtp connection failed"))
	require.Equal(t, "smtp connection failed", result)
}
