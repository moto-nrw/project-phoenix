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

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	baseModel "github.com/moto-nrw/project-phoenix/models/base"
)

func newRateLimitTestService(t *testing.T, account *authModel.Account) (*Service, *stubAccountRepository, *stubPasswordResetTokenRepository, *testRateLimitRepo, *capturingMailer, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	accountRepo := newStubAccountRepository(account)
	tokenRepo := newStubPasswordResetTokenRepository()
	rateRepo := newTestRateLimitRepo()
	mailer := newCapturingMailer()
	dispatcher := email.NewDispatcher(mailer)
	dispatcher.SetDefaults(3, []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond})

	// Create a mock repository factory for testing
	repos := &repositories.Factory{
		Account:                accountRepo,
		PasswordResetToken:     tokenRepo,
		PasswordResetRateLimit: rateRepo,
	}

	// Rate limiting is now configured via ServiceConfig (12-Factor compliant)
	service := &Service{
		repos:                repos,
		dispatcher:           dispatcher,
		defaultFrom:          newDefaultFromEmail(),
		frontendURL:          "http://localhost:3000",
		passwordResetExpiry:  30 * time.Minute,
		rateLimitEnabled:     true, // Enable rate limiting for these tests
		rateLimitMaxRequests: 3,    // Default threshold from config
		txHandler:            baseModel.NewTxHandler(bunDB),
	}

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}

	return service, accountRepo, tokenRepo, rateRepo, mailer, mock, cleanup
}

func TestInitiatePasswordReset_AllowsFirstThreeAttempts(t *testing.T) {
	service, _, tokenRepo, rateRepo, mailer, mock, cleanup := newRateLimitTestService(t, &authModel.Account{
		Model: baseModel.Model{ID: 1},
		Email: "user@example.com",
	})
	t.Cleanup(cleanup)

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		mock.ExpectBegin()
		mock.ExpectCommit()
		token, err := service.InitiatePasswordReset(ctx, "user@example.com")
		if err != nil {
			t.Fatalf("expected attempt %d to succeed, got error: %v", i+1, err)
		}
		if token == nil {
			t.Fatalf("expected token on attempt %d", i+1)
		}
		if !mailer.WaitForMessages(i+1, 50*time.Millisecond) {
			t.Fatalf("expected email to be sent on attempt %d", i+1)
		}
	}

	if got := rateRepo.Attempts(); got != 3 {
		t.Fatalf("expected attempts to equal 3, got %d", got)
	}

	if len(tokenRepo.tokens) != 1 {
		t.Fatalf("expected single active token after invalidation, got %d", len(tokenRepo.tokens))
	}
}

func TestInitiatePasswordReset_BlocksFourthAttempt(t *testing.T) {
	service, _, _, rateRepo, _, mock, cleanup := newRateLimitTestService(t, &authModel.Account{
		Model: baseModel.Model{ID: 42},
		Email: "user@example.com",
	})
	t.Cleanup(cleanup)

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		mock.ExpectBegin()
		mock.ExpectCommit()
		if _, err := service.InitiatePasswordReset(ctx, "user@example.com"); err != nil {
			t.Fatalf("setup attempt %d failed: %v", i+1, err)
		}
	}

	_, err := service.InitiatePasswordReset(ctx, "user@example.com")
	if err == nil {
		t.Fatalf("expected fourth attempt to fail")
	}

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError, got %T", err)
	}

	if !errors.Is(authErr.Err, ErrRateLimitExceeded) {
		t.Fatalf("expected ErrRateLimitExceeded, got %v", authErr.Err)
	}

	rateErr, ok := authErr.Err.(*RateLimitError)
	if !ok {
		t.Fatalf("expected underlying RateLimitError, got %T", authErr.Err)
	}
	const expectedThreshold = 3 // Default rate limit threshold
	if rateErr.Attempts != expectedThreshold {
		t.Fatalf("expected attempts to remain at threshold %d, got %d", expectedThreshold, rateErr.Attempts)
	}
	if !rateErr.RetryAt.After(time.Now()) {
		t.Fatalf("expected retry time to be in the future, got %s", rateErr.RetryAt)
	}

	if got := rateRepo.Attempts(); got != expectedThreshold {
		t.Fatalf("expected attempts to remain at %d, got %d", expectedThreshold, got)
	}
}

func TestInitiatePasswordReset_ResetAfterWindow(t *testing.T) {
	service, _, _, rateRepo, _, mock, cleanup := newRateLimitTestService(t, &authModel.Account{
		Model: baseModel.Model{ID: 7},
		Email: "user@example.com",
	})
	t.Cleanup(cleanup)

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		mock.ExpectBegin()
		mock.ExpectCommit()
		if _, err := service.InitiatePasswordReset(ctx, "user@example.com"); err != nil {
			t.Fatalf("setup attempt %d failed: %v", i+1, err)
		}
	}

	rateRepo.setWindow(time.Now().Add(-2*time.Hour), 3)

	mock.ExpectBegin()
	mock.ExpectCommit()
	token, err := service.InitiatePasswordReset(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("expected request after window reset to succeed, got error: %v", err)
	}
	if token == nil {
		t.Fatalf("expected token after window reset")
	}

	if got := rateRepo.Attempts(); got != 1 {
		t.Fatalf("expected attempts reset to 1 after window reset, got %d", got)
	}
}

func TestInitiatePasswordReset_IncrementsCounter(t *testing.T) {
	service, _, _, rateRepo, _, mock, cleanup := newRateLimitTestService(t, &authModel.Account{
		Model: baseModel.Model{ID: 5},
		Email: "user@example.com",
	})
	t.Cleanup(cleanup)

	ctx := context.Background()
	for i := 1; i <= 2; i++ {
		mock.ExpectBegin()
		mock.ExpectCommit()
		if _, err := service.InitiatePasswordReset(ctx, "user@example.com"); err != nil {
			t.Fatalf("attempt %d failed: %v", i, err)
		}
		if got := rateRepo.Attempts(); got != i {
			t.Fatalf("expected attempt counter to equal %d, got %d", i, got)
		}
	}
}
