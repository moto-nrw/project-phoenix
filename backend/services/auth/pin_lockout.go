package auth

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
)

// PIN brute-force lockout policy (issue #586 — extracted from the model). The
// PIN counter shares the same threshold/duration as the MFA counter; both
// reference these constants so the policy has a single source of truth.
// Per-tenant overrides live behind the security.account_lockout_* settings.
const (
	PINLockoutThreshold = MFALockoutThreshold
	PINLockoutDuration  = MFALockoutDuration
)

// IsPINLocked reports whether the account is inside its PIN-failure lockout
// window relative to now. The decision lives in the service (clock injected),
// not on the model (Rule 12); the account row only holds pin_locked_until.
func (s *Service) IsPINLocked(account *auth.Account, now time.Time) bool {
	return account != nil && account.PINLockedUntil != nil && now.Before(*account.PINLockedUntil)
}

// RecordFailedPINAttempt atomically bumps the account's PIN-failure counter
// and applies the lockout once the threshold is crossed. Replaces the racy
// read-modify-write (model IncrementPINAttempts + UpdateAccount) so concurrent
// failures count independently (issue #586).
func (s *Service) RecordFailedPINAttempt(ctx context.Context, accountID int64) error {
	_, err := s.repos.Account.IncrementPINAttempts(ctx, accountID, PINLockoutThreshold, PINLockoutDuration)
	return err
}

// ResetPINLockout atomically clears the account's PIN-failure counter and
// lockout window after a successful verify or PIN change.
func (s *Service) ResetPINLockout(ctx context.Context, accountID int64) error {
	return s.repos.Account.ResetPINAttempts(ctx, accountID)
}
