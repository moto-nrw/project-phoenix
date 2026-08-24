package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// The tests in this file lift PR-diff coverage on the MFA model helpers
// from 0% to ~100%. These are pure-function helpers (no DB, no goroutines)
// so they're trivially testable; they're absent because the integration
// tests exercise the methods transitively but don't import the model
// package, leaving coverage at zero for SonarCloud's diff metric.

// --- MFACredential ---

func TestMFACredential_Validate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		cred    MFACredential
		wantErr string
	}{
		{
			name:    "missing account_id rejected",
			cred:    MFACredential{Method: MFAMethodEmail},
			wantErr: "account_id is required",
		},
		{
			name:    "unsupported method rejected",
			cred:    MFACredential{AccountID: 42, Method: "totp"},
			wantErr: "unsupported MFA method",
		},
		{
			name:    "email method + account_id accepted",
			cred:    MFACredential{AccountID: 42, Method: MFAMethodEmail},
			wantErr: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cred.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}

// --- MFAEmailChallenge ---

func TestMFAEmailChallenge_IsConsumed(t *testing.T) {
	t.Parallel()

	unconsumed := &MFAEmailChallenge{}
	now := time.Now()
	consumed := &MFAEmailChallenge{ConsumedAt: &now}
	assert.False(t, unconsumed.IsConsumed())
	assert.True(t, consumed.IsConsumed())
}

// --- MFATrustedDevice ---

func TestMFATrustedDevice_IsRevoked(t *testing.T) {
	t.Parallel()

	now := time.Now()
	assert.False(t, (&MFATrustedDevice{ExpiresAt: now.Add(time.Hour)}).IsRevoked())
	assert.True(t, (&MFATrustedDevice{ExpiresAt: now.Add(time.Hour), RevokedAt: &now}).IsRevoked())
}

// The Account MFA lockout helpers (IsMFALocked / IncrementMFAAttempts /
// ResetMFAAttempts) moved off the model in issue #586 (Rule 12). The decision
// is now services/auth mfaService.isMFALocked (clock injected) and the atomic
// counter mutations are database/repositories/auth AccountRepository
// (covered by account_mfa_atomic_test.go).
