package platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

// Lift PR-diff coverage on the operator-side MFA model helpers. Mirror of
// models/auth/mfa_models_test.go for the platform.* tables. All pure helpers
// (no DB, no goroutines).

// --- OperatorMFACredential ---

func TestOperatorMFACredential_Validate(t *testing.T) {
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

func TestOperatorMFACredential_AccessorsAndTableName(t *testing.T) {
	now := time.Now()
	c := &OperatorMFACredential{OperatorID: 7, Method: OperatorMFAMethodEmail}
	c.ID = 99
	c.CreatedAt = now
	c.UpdatedAt = now.Add(time.Hour)

	assert.Equal(t, int64(99), c.GetID())
	assert.Equal(t, now, c.GetCreatedAt())
	assert.Equal(t, now.Add(time.Hour), c.GetUpdatedAt())
	assert.Equal(t, "platform.operator_mfa_credentials", c.TableName())
}

func TestOperatorMFACredential_BeforeAppendModel(t *testing.T) {
	c := &OperatorMFACredential{}
	assert.NoError(t, c.BeforeAppendModel(nil))
	assert.NoError(t, c.BeforeAppendModel(&bun.SelectQuery{}))
}

// --- OperatorMFAEmailChallenge ---

func TestOperatorMFAEmailChallenge_IsExpired(t *testing.T) {
	past := &OperatorMFAEmailChallenge{ExpiresAt: time.Now().Add(-time.Minute)}
	future := &OperatorMFAEmailChallenge{ExpiresAt: time.Now().Add(time.Minute)}
	assert.True(t, past.IsExpired())
	assert.False(t, future.IsExpired())
}

func TestOperatorMFAEmailChallenge_IsConsumed(t *testing.T) {
	unconsumed := &OperatorMFAEmailChallenge{}
	now := time.Now()
	consumed := &OperatorMFAEmailChallenge{ConsumedAt: &now}
	assert.False(t, unconsumed.IsConsumed())
	assert.True(t, consumed.IsConsumed())
}

func TestOperatorMFAEmailChallenge_AccessorsAndTableName(t *testing.T) {
	now := time.Now()
	c := &OperatorMFAEmailChallenge{OperatorID: 8, CodeHash: "h", ExpiresAt: now}
	c.ID = 101
	c.CreatedAt = now
	c.UpdatedAt = now.Add(time.Hour)

	assert.Equal(t, int64(101), c.GetID())
	assert.Equal(t, now, c.GetCreatedAt())
	assert.Equal(t, now.Add(time.Hour), c.GetUpdatedAt())
	assert.Equal(t, "platform.operator_mfa_email_challenges", c.TableName())
}

func TestOperatorMFAEmailChallenge_BeforeAppendModel(t *testing.T) {
	c := &OperatorMFAEmailChallenge{}
	assert.NoError(t, c.BeforeAppendModel(nil))
	assert.NoError(t, c.BeforeAppendModel(&bun.SelectQuery{}))
}

// --- OperatorMFATrustedDevice ---

func TestOperatorMFATrustedDevice_IsExpiredIsRevokedIsActive(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name        string
		dev         OperatorMFATrustedDevice
		wantExpired bool
		wantRevoked bool
		wantActive  bool
	}{
		{
			name:        "fresh device is active",
			dev:         OperatorMFATrustedDevice{ExpiresAt: now.Add(time.Hour)},
			wantExpired: false,
			wantRevoked: false,
			wantActive:  true,
		},
		{
			name:        "past-expiry is expired and inactive",
			dev:         OperatorMFATrustedDevice{ExpiresAt: now.Add(-time.Hour)},
			wantExpired: true,
			wantRevoked: false,
			wantActive:  false,
		},
		{
			name:        "revoked is inactive regardless of expiry",
			dev:         OperatorMFATrustedDevice{ExpiresAt: now.Add(time.Hour), RevokedAt: &now},
			wantExpired: false,
			wantRevoked: true,
			wantActive:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantExpired, tc.dev.IsExpired())
			assert.Equal(t, tc.wantRevoked, tc.dev.IsRevoked())
			assert.Equal(t, tc.wantActive, tc.dev.IsActive())
		})
	}
}

func TestOperatorMFATrustedDevice_AccessorsAndTableName(t *testing.T) {
	now := time.Now()
	d := &OperatorMFATrustedDevice{OperatorID: 9, TokenHash: "h", ExpiresAt: now}
	d.ID = 202
	d.CreatedAt = now
	d.UpdatedAt = now.Add(time.Hour)

	assert.Equal(t, int64(202), d.GetID())
	assert.Equal(t, now, d.GetCreatedAt())
	assert.Equal(t, now.Add(time.Hour), d.GetUpdatedAt())
	assert.Equal(t, "platform.operator_mfa_trusted_devices", d.TableName())
}

func TestOperatorMFATrustedDevice_BeforeAppendModel(t *testing.T) {
	d := &OperatorMFATrustedDevice{}
	assert.NoError(t, d.BeforeAppendModel(nil))
	assert.NoError(t, d.BeforeAppendModel(&bun.SelectQuery{}))
}

// --- Operator MFA lockout helpers ---

func TestOperator_IsMFALocked(t *testing.T) {
	o := &Operator{MFALockedUntil: nil}
	assert.False(t, o.IsMFALocked())

	past := time.Now().Add(-time.Minute)
	future := time.Now().Add(time.Minute)
	assert.False(t, (&Operator{MFALockedUntil: &past}).IsMFALocked())
	assert.True(t, (&Operator{MFALockedUntil: &future}).IsMFALocked())
}

func TestOperator_IncrementMFAAttempts_LocksAtFifthFailure(t *testing.T) {
	o := &Operator{MFAAttempts: 4}
	o.IncrementMFAAttempts()
	assert.Equal(t, 5, o.MFAAttempts)
	assert.NotNil(t, o.MFALockedUntil, "5th failed attempt must set the lockout window")
	assert.True(t, o.MFALockedUntil.After(time.Now()))

	// Sub-threshold increment leaves the lockout alone.
	b := &Operator{MFAAttempts: 1}
	b.IncrementMFAAttempts()
	assert.Equal(t, 2, b.MFAAttempts)
	assert.Nil(t, b.MFALockedUntil)
}

func TestOperator_ResetMFAAttempts(t *testing.T) {
	now := time.Now().Add(15 * time.Minute)
	o := &Operator{MFAAttempts: 5, MFALockedUntil: &now}
	o.ResetMFAAttempts()
	assert.Equal(t, 0, o.MFAAttempts)
	assert.Nil(t, o.MFALockedUntil)
}
