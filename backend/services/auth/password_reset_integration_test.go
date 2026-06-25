package auth

import (
	"context"
	"errors"
	"log/slog"
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

	dispatcher := email.NewDispatcher(mailer, slog.Default())
	dispatcher.SetDefaults(3, []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond})

	// Create a mock repository factory for testing
	guardianRole := &authModel.Role{
		Model: modelBase.Model{ID: 10},
		Name:  guardianRoleName,
	}
	repos := &repositories.Factory{
		Account:                accounts,
		AccountTenant:          newStubAccountTenantRepository(),
		AccountRole:            newStubAccountRoleRepository(),
		Role:                   newStubRoleRepository(guardianRole),
		PasswordResetToken:     resetTokens,
		PasswordResetRateLimit: rateRepo,
		Token:                  sessionTokens,
	}

	service := &Service{
		repos:               repos,
		dispatcher:          dispatcher,
		defaultFrom:         newDefaultFromEmail(),
		frontendURL:         "http://localhost:3000",
		parentsURL:          "http://parents.localhost:3000",
		passwordResetExpiry: 30 * time.Minute,
		txHandler:           modelBase.NewTxHandler(bunDB),
		db:                  bunDB,
	}

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}

	return service, accounts, resetTokens, rateRepo, sessionTokens, mailer, mock, cleanup
}

func grantGuardianRoleForPasswordReset(t *testing.T, service *Service, accountID, tenantID int64) {
	t.Helper()
	require.NoError(t, service.repos.AccountTenant.Create(context.Background(), &authModel.AccountTenant{
		AccountID: accountID,
		TenantID:  tenantID,
		Status:    authModel.AccountTenantStatusActive,
	}))
	require.NoError(t, service.repos.AccountRole.Create(context.Background(), &authModel.AccountRole{
		TenantModel: modelBase.TenantModel{TenantID: tenantID},
		AccountID:   accountID,
		RoleID:      10,
	}))
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

func TestInitiateParentPasswordResetSendsParentPortalLink(t *testing.T) {
	service, _, tokens, _, _, mailer, mock, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)
	grantGuardianRoleForPasswordReset(t, service, 1, 123)

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectCommit()

	token, err := service.InitiateParentPasswordReset(ctx, "user@example.com")
	require.NoError(t, err)
	require.NotNil(t, token)

	require.Eventually(t, func() bool {
		return len(mailer.Messages()) == 1
	}, 200*time.Millisecond, 10*time.Millisecond)

	msg := mailer.Messages()[0]
	require.Equal(t, "Passwort zurücksetzen", msg.Subject)
	content := msg.Content.(map[string]any)
	require.Equal(t, "http://parents.localhost:3000/reset-password?token="+token.Token, content["ResetURL"])
	require.Equal(t, "http://parents.localhost:3000/images/moto_transparent.png", content["LogoURL"])

	_, ok := tokens.tokens[token.Token]
	require.True(t, ok, "token should be persisted")
}

func TestInitiateParentPasswordResetNonGuardianIsNeutral(t *testing.T) {
	service, _, tokens, rateRepo, _, mailer, mock, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)
	require.NoError(t, service.repos.AccountTenant.Create(context.Background(), &authModel.AccountTenant{
		AccountID: 1,
		TenantID:  123,
		Status:    authModel.AccountTenantStatusActive,
	}))

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	token, err := service.InitiateParentPasswordReset(ctx, "user@example.com")
	require.NoError(t, err)
	require.Nil(t, token)
	require.Empty(t, mailer.Messages())
	require.Empty(t, tokens.tokens)
	require.Equal(t, 1, rateRepo.Attempts())
}

func TestInitiateParentPasswordResetPropagatesRoleLookupError(t *testing.T) {
	service, _, tokens, _, _, mailer, mock, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	// Active tenant mapping so the guardian check reaches the role lookup.
	require.NoError(t, service.repos.AccountTenant.Create(context.Background(), &authModel.AccountTenant{
		AccountID: 1,
		TenantID:  123,
		Status:    authModel.AccountTenantStatusActive,
	}))

	// Simulate an RLS/config/transient DB failure on the role lookup. This
	// must NOT be swallowed as "not a guardian" — a real guardian would then
	// silently get no token and no recovery email.
	dbErr := errors.New("connection reset by peer")
	service.repos.AccountRole.(*stubAccountRoleRepository).findByTenantErr = dbErr

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	token, err := service.InitiateParentPasswordReset(ctx, "user@example.com")
	require.Error(t, err)
	require.ErrorIs(t, err, dbErr)
	require.Nil(t, token)
	require.Empty(t, mailer.Messages())
	require.Empty(t, tokens.tokens)
}

func TestInitiateParentPasswordResetUnknownEmailIsNeutralAndRateLimited(t *testing.T) {
	service, _, tokens, rateRepo, _, mailer, _, cleanup := newPasswordResetTestEnv(t)
	t.Cleanup(cleanup)

	token, err := service.InitiateParentPasswordReset(context.Background(), "unknown@example.com")
	require.NoError(t, err)
	require.Nil(t, token)
	require.Empty(t, mailer.Messages())
	require.Empty(t, tokens.tokens)
	require.Equal(t, 1, rateRepo.Attempts())
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
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
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
