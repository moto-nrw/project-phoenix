package auth

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/auth"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
)

// Moved from models/auth/account_test.go (TestAccount_IsPINLocked) when the
// lockout decision left the model in issue #586 (Rule 12). The decision is now
// a pure account read with the clock injected, so no repos are needed.
func TestService_IsPINLocked(t *testing.T) {
	t.Parallel()

	s := &Service{}
	now := time.Now()

	tests := []struct {
		name           string
		pinLockedUntil *time.Time
		expected       bool
	}{
		{name: "nil locked until", pinLockedUntil: nil, expected: false},
		{name: "locked until past", pinLockedUntil: ptr(now.Add(-time.Hour)), expected: false},
		{name: "locked until future", pinLockedUntil: ptr(now.Add(time.Hour)), expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &auth.Account{PINLockedUntil: tt.pinLockedUntil}
			if got := s.IsPINLocked(account, now); got != tt.expected {
				t.Errorf("IsPINLocked() = %v, want %v", got, tt.expected)
			}
		})
	}

	// Nil account is treated as unlocked.
	if s.IsPINLocked(nil, now) {
		t.Error("IsPINLocked(nil) should return false")
	}
}

func ptr(t time.Time) *time.Time { return &t }

type capturePINAccountRepo struct {
	noopAccountRepository
	threshold int
	duration  time.Duration
}

func (r *capturePINAccountRepo) IncrementPINAttempts(_ context.Context, _ int64, threshold int, duration time.Duration) (auth.PINAttemptResult, error) {
	r.threshold = threshold
	r.duration = duration
	return auth.PINAttemptResult{Attempts: threshold}, nil
}

type lockoutSettingsStub struct {
	configSvc.SettingsService
	values map[string]int
}

func (s lockoutSettingsStub) HasTenantOverride(_ context.Context, key string) (bool, error) {
	_, ok := s.values[key]
	return ok, nil
}

func (s lockoutSettingsStub) ResolveInt(_ context.Context, key string) (int, error) {
	return s.values[key], nil
}

func TestService_RecordFailedPINAttempt_UsesTenantLockoutSettings(t *testing.T) {
	t.Parallel()

	accountRepo := &capturePINAccountRepo{}
	s := &Service{
		repos: &repositories.Factory{Account: accountRepo},
		settings: lockoutSettingsStub{values: map[string]int{
			configModel.KeyAccountLockoutThreshold:       3,
			configModel.KeyAccountLockoutDurationMinutes: 22,
		}},
	}

	if err := s.RecordFailedPINAttempt(context.Background(), 42); err != nil {
		t.Fatalf("RecordFailedPINAttempt() error = %v", err)
	}

	if accountRepo.threshold != 3 {
		t.Fatalf("threshold = %d, want 3", accountRepo.threshold)
	}
	if accountRepo.duration != 22*time.Minute {
		t.Fatalf("duration = %v, want 22m", accountRepo.duration)
	}
}

func TestService_RecordFailedPINAttempt_FallsBackWhenSettingsMissing(t *testing.T) {
	t.Parallel()

	accountRepo := &capturePINAccountRepo{}
	s := &Service{repos: &repositories.Factory{Account: accountRepo}}

	if err := s.RecordFailedPINAttempt(context.Background(), 42); err != nil {
		t.Fatalf("RecordFailedPINAttempt() error = %v", err)
	}

	if accountRepo.threshold != PINLockoutThreshold {
		t.Fatalf("threshold = %d, want %d", accountRepo.threshold, PINLockoutThreshold)
	}
	if accountRepo.duration != PINLockoutDuration {
		t.Fatalf("duration = %v, want %v", accountRepo.duration, PINLockoutDuration)
	}
}
