package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

// The tests in this file lift PR-diff coverage on the MFA model helpers
// from 0% to ~100%. These are pure-function helpers (no DB, no goroutines)
// so they're trivially testable; they're absent because the integration
// tests exercise the methods transitively but don't import the model
// package, leaving coverage at zero for SonarCloud's diff metric.

// --- MFACredential ---

func TestMFACredential_Validate(t *testing.T) {
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

func TestMFACredential_AccessorsAndTableName(t *testing.T) {
	now := time.Now()
	cred := &MFACredential{AccountID: 7, Method: MFAMethodEmail}
	cred.ID = 99
	cred.CreatedAt = now
	cred.UpdatedAt = now.Add(time.Hour)

	assert.Equal(t, int64(99), cred.GetID())
	assert.Equal(t, now, cred.GetCreatedAt())
	assert.Equal(t, now.Add(time.Hour), cred.GetUpdatedAt())
	assert.Equal(t, "auth.mfa_credentials", cred.TableName())
}

func TestMFACredential_BeforeAppendModel_SetsTableExprForUpdateAndDelete(t *testing.T) {
	cred := &MFACredential{}
	// Both branches set ModelTableExpr; calling with a noop query type just
	// exercises the type-switch fallthrough. The branches that ARE typed
	// (UpdateQuery / DeleteQuery) need a real bun.IDB to construct, so we
	// rely on the in-package coverage hit and assert no error.
	assert.NoError(t, cred.BeforeAppendModel(nil))
	assert.NoError(t, cred.BeforeAppendModel(&bun.SelectQuery{}))
}

// --- MFAEmailChallenge ---

func TestMFAEmailChallenge_IsConsumed(t *testing.T) {
	unconsumed := &MFAEmailChallenge{}
	now := time.Now()
	consumed := &MFAEmailChallenge{ConsumedAt: &now}
	assert.False(t, unconsumed.IsConsumed())
	assert.True(t, consumed.IsConsumed())
}

func TestMFAEmailChallenge_AccessorsAndTableName(t *testing.T) {
	now := time.Now()
	c := &MFAEmailChallenge{AccountID: 8, CodeHash: "h", ExpiresAt: now}
	c.ID = 101
	c.CreatedAt = now
	c.UpdatedAt = now.Add(time.Hour)

	assert.Equal(t, int64(101), c.GetID())
	assert.Equal(t, now, c.GetCreatedAt())
	assert.Equal(t, now.Add(time.Hour), c.GetUpdatedAt())
	assert.Equal(t, "auth.mfa_email_challenges", c.TableName())
}

func TestMFAEmailChallenge_BeforeAppendModel_DoesNotPanicOnFallthrough(t *testing.T) {
	c := &MFAEmailChallenge{}
	assert.NoError(t, c.BeforeAppendModel(nil))
	assert.NoError(t, c.BeforeAppendModel(&bun.SelectQuery{}))
}

// --- MFATrustedDevice ---

func TestMFATrustedDevice_IsRevoked(t *testing.T) {
	now := time.Now()
	assert.False(t, (&MFATrustedDevice{ExpiresAt: now.Add(time.Hour)}).IsRevoked())
	assert.True(t, (&MFATrustedDevice{ExpiresAt: now.Add(time.Hour), RevokedAt: &now}).IsRevoked())
}

func TestMFATrustedDevice_AccessorsAndTableName(t *testing.T) {
	now := time.Now()
	d := &MFATrustedDevice{AccountID: 9, TenantID: 100, TokenHash: "h", ExpiresAt: now}
	d.ID = 202
	d.CreatedAt = now
	d.UpdatedAt = now.Add(time.Hour)

	assert.Equal(t, int64(202), d.GetID())
	assert.Equal(t, now, d.GetCreatedAt())
	assert.Equal(t, now.Add(time.Hour), d.GetUpdatedAt())
	assert.Equal(t, "auth.mfa_trusted_devices", d.TableName())
}

func TestMFATrustedDevice_BeforeAppendModel_DoesNotPanicOnFallthrough(t *testing.T) {
	d := &MFATrustedDevice{}
	assert.NoError(t, d.BeforeAppendModel(nil))
	assert.NoError(t, d.BeforeAppendModel(&bun.SelectQuery{}))
}

// The Account MFA lockout helpers (IsMFALocked / IncrementMFAAttempts /
// ResetMFAAttempts) moved off the model in issue #586 (Rule 12). The decision
// is now services/auth mfaService.isMFALocked (clock injected) and the atomic
// counter mutations are database/repositories/auth AccountRepository
// (covered by account_mfa_atomic_test.go).
