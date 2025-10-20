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

	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

func newPasswordResetTestEnv(t *testing.T) (*Service, *stubAccountRepository, *stubPasswordResetTokenRepository, *testRateLimitRepo, *stubTokenRepository, *capturingMailer, sqlmock.Sqlmock, func()) {
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
	mailer := newCapturingMailer()

	service := &Service{
		accountRepo:                accounts,
		passwordResetTokenRepo:     resetTokens,
		passwordResetRateLimitRepo: rateRepo,
		tokenRepo:                  sessionTokens,
		mailer:                     mailer,
		defaultFrom:                newDefaultFromEmail(),
		frontendURL:                "http://localhost:3000",
		passwordResetExpiry:        30 * time.Minute,
		txHandler:                  modelBase.NewTxHandler(bunDB),
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
	service, _, tokens, _, _, mailer, _, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	token, err := service.InitiatePasswordReset(ctx, "user@example.com")
	require.NoError(t, err)
	require.NotNil(t, token)

	require.Eventually(t, func() bool {
		return len(mailer.Messages()) == 1
	}, 200*time.Millisecond, 10*time.Millisecond)

	msg := mailer.Messages()[0]
	require.Equal(t, "Password Reset Request", msg.Subject)
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

func TestResetPasswordWithValidToken(t *testing.T) {
	service, accounts, tokens, _, sessionTokens, _, mock, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
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
	service, _, _, rateRepo, _, _, _, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	for i := 0; i < 3; i++ {
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
