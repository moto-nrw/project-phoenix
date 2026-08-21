package platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Lift PR-diff coverage on the operator-side MFA model helpers. Mirror of
// models/auth/mfa_models_test.go for the platform.* tables. All pure helpers
// (no DB, no goroutines).

// --- OperatorMFACredential ---

func TestOperatorMFACredential_Validate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		cred    OperatorMFACredential
		wantErr string
	}{
		{
			name:    "missing operator_id rejected",
			cred:    OperatorMFACredential{Method: OperatorMFAMethodEmail},
			wantErr: "operator_id is required",
		},
		{
			name:    "unsupported method rejected",
			cred:    OperatorMFACredential{OperatorID: 42, Method: "totp"},
			wantErr: "unsupported MFA method",
		},
		{
			name:    "email + operator_id accepted",
			cred:    OperatorMFACredential{OperatorID: 42, Method: OperatorMFAMethodEmail},
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

// --- OperatorMFAEmailChallenge ---

func TestOperatorMFAEmailChallenge_IsConsumed(t *testing.T) {
	t.Parallel()

	unconsumed := &OperatorMFAEmailChallenge{}
	now := time.Now()
	consumed := &OperatorMFAEmailChallenge{ConsumedAt: &now}
	assert.False(t, unconsumed.IsConsumed())
	assert.True(t, consumed.IsConsumed())
}

// --- OperatorMFATrustedDevice ---

func TestOperatorMFATrustedDevice_IsRevoked(t *testing.T) {
	t.Parallel()

	now := time.Now()
	active := &OperatorMFATrustedDevice{ExpiresAt: now.Add(time.Hour)}
	revoked := &OperatorMFATrustedDevice{ExpiresAt: now.Add(time.Hour), RevokedAt: &now}
	assert.False(t, active.IsRevoked())
	assert.True(t, revoked.IsRevoked())
}

// The operator MFA lockout decision (IsMFALocked) and the counter mutations
// (Increment/Reset) moved off the model in issue #586 (Rule 12). Their tests
// followed the logic to its new home: the decision is covered by
// services/platform.TestOperatorMFAService_isMFALocked, and the atomic counter
// mutations by the OperatorRepository tests in
// database/repositories/platform/operator_mfa_atomic_test.go.
//
// The wall-clock expiry/active decisions (OperatorMFAEmailChallenge.IsExpired,
// OperatorMFATrustedDevice.IsExpired/IsActive) likewise moved off the models
// and are enforced by the repositories' SQL expiry filters
// (FindActiveByOperatorID, FindActiveByOperatorIDAndTokenHash).
