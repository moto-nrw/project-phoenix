package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/moto-nrw/project-phoenix/email"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	baseModel "github.com/moto-nrw/project-phoenix/models/base"
)

func newRateLimitTestService(t *testing.T, account *authModel.Account) (*Service, *stubAccountRepository, *stubPasswordResetTokenRepository, *testRateLimitRepo, *capturingMailer) {
	t.Helper()

	prevRateLimitEnabled := viper.GetBool("rate_limit_enabled")
	viper.Set("rate_limit_enabled", true)
	t.Cleanup(func() {
		viper.Set("rate_limit_enabled", prevRateLimitEnabled)
	})

	accountRepo := newStubAccountRepository(account)
	tokenRepo := newStubPasswordResetTokenRepository()
	rateRepo := newTestRateLimitRepo()
	mailer := newCapturingMailer()
	dispatcher := email.NewDispatcher(mailer)
	dispatcher.SetDefaults(3, []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond})

	service := &Service{
		accountRepo:                accountRepo,
		passwordResetTokenRepo:     tokenRepo,
		passwordResetRateLimitRepo: rateRepo,
		dispatcher:                 dispatcher,
		defaultFrom:                newDefaultFromEmail(),
		frontendURL:                "http://localhost:3000",
		passwordResetExpiry:        30 * time.Minute,
	}

	return service, accountRepo, tokenRepo, rateRepo, mailer
}

func TestInitiatePasswordReset_AllowsFirstThreeAttempts(t *testing.T) {
	service, _, tokenRepo, rateRepo, mailer := newRateLimitTestService(t, &authModel.Account{
		Model: baseModel.Model{ID: 1},
		Email: "user@example.com",
	})

	ctx := context.Background()
	for i := 0; i < 3; i++ {
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
	service, _, _, rateRepo, _ := newRateLimitTestService(t, &authModel.Account{
		Model: baseModel.Model{ID: 42},
		Email: "user@example.com",
	})

	ctx := context.Background()
	for i := 0; i < 3; i++ {
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
	if rateErr.Attempts != passwordResetRateLimitThreshold {
		t.Fatalf("expected attempts to remain at threshold %d, got %d", passwordResetRateLimitThreshold, rateErr.Attempts)
	}
	if !rateErr.RetryAt.After(time.Now()) {
		t.Fatalf("expected retry time to be in the future, got %s", rateErr.RetryAt)
	}

	if got := rateRepo.Attempts(); got != passwordResetRateLimitThreshold {
		t.Fatalf("expected attempts to remain at %d, got %d", passwordResetRateLimitThreshold, got)
	}
}

func TestInitiatePasswordReset_ResetAfterWindow(t *testing.T) {
	service, _, _, rateRepo, _ := newRateLimitTestService(t, &authModel.Account{
		Model: baseModel.Model{ID: 7},
		Email: "user@example.com",
	})

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := service.InitiatePasswordReset(ctx, "user@example.com"); err != nil {
			t.Fatalf("setup attempt %d failed: %v", i+1, err)
		}
	}

	rateRepo.setWindow(time.Now().Add(-2*time.Hour), 3)

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
	service, _, _, rateRepo, _ := newRateLimitTestService(t, &authModel.Account{
		Model: baseModel.Model{ID: 5},
		Email: "user@example.com",
	})

	ctx := context.Background()
	for i := 1; i <= 2; i++ {
		if _, err := service.InitiatePasswordReset(ctx, "user@example.com"); err != nil {
			t.Fatalf("attempt %d failed: %v", i, err)
		}
		if got := rateRepo.Attempts(); got != i {
			t.Fatalf("expected attempt counter to equal %d, got %d", i, got)
		}
	}
}
