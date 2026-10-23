package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingSecuritySettings struct {
	optionalMFASettings
	key string
}

func (s failingSecuritySettings) TrustedDeviceDays(ctx context.Context, tenantID int64) (int, error) {
	if s.key == "days" {
		return 0, errors.New("private security settings failure")
	}
	return s.optionalMFASettings.TrustedDeviceDays(ctx, tenantID)
}
func (s failingSecuritySettings) LockoutThreshold(ctx context.Context, tenantID int64) (int, error) {
	if s.key == "threshold" {
		return 0, errors.New("private security settings failure")
	}
	return s.optionalMFASettings.LockoutThreshold(ctx, tenantID)
}
func (s failingSecuritySettings) LockoutDuration(ctx context.Context, tenantID int64) (time.Duration, error) {
	if s.key == "duration" {
		return 0, errors.New("private security settings failure")
	}
	return s.optionalMFASettings.LockoutDuration(ctx, tenantID)
}

type securitySettingsRecords struct {
	*memoryMFARecords
	trusted []domain.AccountTrustedDevice
}

func (r *securitySettingsRecords) CreateTrustedDevice(_ context.Context, device domain.AccountTrustedDevice) (domain.AccountTrustedDevice, error) {
	device.ID = 1
	r.trusted = append(r.trusted, device)
	return device, nil
}
func (r *securitySettingsRecords) IncrementMFAAttempts(context.Context, int64, int, time.Duration) (domain.AccountLockout, error) {
	return domain.AccountLockout{}, nil
}
func TestAccountMFARejectsUnreadableSecuritySettings(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"days", "threshold", "duration"} {
		t.Run(key, func(t *testing.T) {
			records := &securitySettingsRecords{memoryMFARecords: newMemoryMFARecords(domain.AccountIdentity{ID: 1, Email: "mfa@example.test", Active: true})}
			codec := &fakeMFAChallengeCodec{parse: map[string]domain.MFAChallengeClaims{}}
			flows, err := NewAccountMFAFlows(AccountMFAFlowDependencies{Records: records, Settings: failingSecuritySettings{key: key}, Codec: codec, Codes: fakeShortCodes{}, Mail: &fakeMFAMail{}, Audit: &fakeMFAAudit{}, Runtime: &fakeRuntime{}, Secret: []byte("test-only-secret")})
			require.NoError(t, err)
			if key == "days" {
				token, expires, err := flows.IssueTrustedDevice(context.Background(), 1, 10, "ua", nil)
				require.ErrorIs(t, err, identityaccess.ErrMFAStatusUnavailable)
				assert.Empty(t, token)
				assert.True(t, expires.IsZero())
				assert.Empty(t, records.trusted)
				return
			}
			challengeRow, err := records.CreateChallenge(context.Background(), domain.AccountMFAChallenge{AccountID: 1, TenantID: 10, Scope: domain.MFAChallengeScopeTenant, CodeHash: "hash:123456", ExpiresAt: time.Now().Add(time.Minute)})
			require.NoError(t, err)
			codec.parse["challenge"] = domain.MFAChallengeClaims{AccountID: 1, TenantID: 10, Scope: domain.MFAChallengeScopeTenant, ChallengeID: challengeRow.ID}
			challenge := "challenge"
			verified, err := flows.VerifyChallenge(context.Background(), challenge, "wrong-code")
			require.ErrorIs(t, err, identityaccess.ErrMFAStatusUnavailable)
			assert.Zero(t, verified.AccountID)
			require.ErrorIs(t, flows.VerifyCodeForAccount(context.Background(), 1, 10, "wrong-code", domain.MFAChallengeScopeTenant), identityaccess.ErrMFAStatusUnavailable)
		})
	}
}
