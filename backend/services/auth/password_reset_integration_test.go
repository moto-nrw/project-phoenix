package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/internal/adapter/mailer"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

func newPasswordResetTestEnv(t *testing.T) (*Service, *stubAccountRepository, *stubPasswordResetTokenRepository, *testRateLimitRepo, *stubTokenRepository, *capturingMailer, sqlmock.Sqlmock, func()) {
	service, accounts, tokens, rateRepo, sessions, m, mock, cleanup := newPasswordResetTestEnvWithMailer(t, newCapturingMailer())
	capturing, _ := m.(*capturingMailer)
	return service, accounts, tokens, rateRepo, sessions, capturing, mock, cleanup
}

func newPasswordResetTestEnvWithMailer(t *testing.T, m port.EmailSender) (*Service, *stubAccountRepository, *stubPasswordResetTokenRepository, *testRateLimitRepo, *stubTokenRepository, port.EmailSender, sqlmock.Sqlmock, func()) {
	t.Helper()

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

	dispatcher := mailer.NewDispatcher(m)
	dispatcher.SetDefaults(3, []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond})

	// Create a mock repository factory for testing
	repos := &repositories.Factory{
		Account:                accounts,
		PasswordResetToken:     resetTokens,
		PasswordResetRateLimit: rateRepo,
		Token:                  sessionTokens,
	}

	// Rate limiting is now configured via ServiceConfig (12-Factor compliant)
	service := &Service{
		repos:                repos,
		dispatcher:           dispatcher,
		defaultFrom:          newDefaultFromEmail(),
		frontendURL:          "http://localhost:3000",
		passwordResetExpiry:  30 * time.Minute,
		rateLimitEnabled:     true, // Enable for testing rate limit behavior
		rateLimitMaxRequests: 3,    // Default threshold from config
		txHandler:            modelBase.NewTxHandler(bunDB),
	}

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}

	return service, accounts, resetTokens, rateRepo, sessionTokens, m, mock, cleanup
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
	require.Equal(t, 3, rateErr.Attempts) // Default rate limit threshold
	require.Equal(t, 3, rateRepo.Attempts())
	require.True(t, rateErr.RetryAt.After(time.Now()))
}
