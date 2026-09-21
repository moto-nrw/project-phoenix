package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// publicAccountMFARecords exposes native records only when a test decorates
// persistence to exercise an infrastructure failure through the public flows.
type publicAccountMFARecords struct{ source ports.AccountMFARecords }

func (r publicAccountMFARecords) FindAccountIdentity(ctx context.Context, accountID int64) (identityaccess.AccountMFAIdentity, bool, error) {
	identity, found, err := r.source.FindAccountIdentity(ctx, accountID)
	return identityaccess.AccountMFAIdentity(identity), found, err
}

func (r publicAccountMFARecords) AccountBelongsToTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	return r.source.AccountBelongsToTenant(ctx, accountID, tenantID)
}

func (r publicAccountMFARecords) LockAccountForOverrideWrite(ctx context.Context, accountID int64) error {
	return r.source.LockAccountForOverrideWrite(ctx, accountID)
}

func (r publicAccountMFARecords) IncrementMFAAttempts(ctx context.Context, accountID int64, threshold int, duration time.Duration) (identityaccess.AccountLockout, error) {
	lockout, err := r.source.IncrementMFAAttempts(ctx, accountID, threshold, duration)
	return identityaccess.AccountLockout(lockout), err
}

func (r publicAccountMFARecords) ResetMFAAttempts(ctx context.Context, accountID int64) error {
	return r.source.ResetMFAAttempts(ctx, accountID)
}

func (r publicAccountMFARecords) FindCredential(ctx context.Context, accountID int64) (identityaccess.AccountMFACredential, bool, error) {
	credential, found, err := r.source.FindCredential(ctx, accountID)
	return identityaccess.AccountMFACredential(credential), found, err
}

func (r publicAccountMFARecords) CreateCredential(ctx context.Context, credential identityaccess.AccountMFACredential) error {
	return r.source.CreateCredential(ctx, domain.AccountMFACredential(credential))
}

func (r publicAccountMFARecords) TouchCredential(ctx context.Context, id int64, usedAt time.Time) error {
	return r.source.TouchCredential(ctx, id, usedAt)
}

func (r publicAccountMFARecords) DeleteCredentials(ctx context.Context, accountID int64) error {
	return r.source.DeleteCredentials(ctx, accountID)
}

func (r publicAccountMFARecords) CreateChallenge(ctx context.Context, challenge identityaccess.AccountMFAChallenge) (identityaccess.AccountMFAChallenge, error) {
	stored, err := r.source.CreateChallenge(ctx, domain.AccountMFAChallenge(challenge))
	return identityaccess.AccountMFAChallenge(stored), err
}

func (r publicAccountMFARecords) ActivateChallenge(ctx context.Context, id int64) error {
	return r.source.ActivateChallenge(ctx, id)
}

func (r publicAccountMFARecords) ConsumeChallenge(ctx context.Context, id int64, consumedAt time.Time) error {
	return r.source.ConsumeChallenge(ctx, id, consumedAt)
}

func (r publicAccountMFARecords) CountChallengesSince(ctx context.Context, accountID int64, since time.Time) (int, error) {
	return r.source.CountChallengesSince(ctx, accountID, since)
}

func (r publicAccountMFARecords) FindActiveChallengeForAccount(ctx context.Context, id, accountID int64) (identityaccess.AccountMFAChallenge, bool, error) {
	challenge, found, err := r.source.FindActiveChallengeForAccount(ctx, id, accountID)
	return identityaccess.AccountMFAChallenge(challenge), found, err
}

func (r publicAccountMFARecords) FindActiveChallengeInScope(ctx context.Context, accountID, tenantID int64, scope string) (identityaccess.AccountMFAChallenge, bool, error) {
	challenge, found, err := r.source.FindActiveChallengeInScope(ctx, accountID, tenantID, scope)
	return identityaccess.AccountMFAChallenge(challenge), found, err
}

func (r publicAccountMFARecords) CreateTrustedDevice(ctx context.Context, device identityaccess.AccountTrustedDevice) (identityaccess.AccountTrustedDevice, error) {
	stored, err := r.source.CreateTrustedDevice(ctx, domain.AccountTrustedDevice(device))
	return identityaccess.AccountTrustedDevice(stored), err
}

func (r publicAccountMFARecords) FindActiveTrustedDevice(ctx context.Context, accountID, tenantID int64, tokenHash string) (identityaccess.AccountTrustedDevice, bool, error) {
	device, found, err := r.source.FindActiveTrustedDevice(ctx, accountID, tenantID, tokenHash)
	return identityaccess.AccountTrustedDevice(device), found, err
}

func (r publicAccountMFARecords) ListActiveTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]identityaccess.AccountTrustedDevice, error) {
	devices, err := r.source.ListActiveTrustedDevices(ctx, accountID, tenantID)
	if err != nil {
		return nil, err
	}
	result := make([]identityaccess.AccountTrustedDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, identityaccess.AccountTrustedDevice(device))
	}
	return result, nil
}

func (r publicAccountMFARecords) TouchTrustedDevice(ctx context.Context, id int64, usedAt time.Time) error {
	return r.source.TouchTrustedDevice(ctx, id, usedAt)
}

func (r publicAccountMFARecords) RevokeTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) error {
	return r.source.RevokeTrustedDevice(ctx, id, revokedAt)
}

func (r publicAccountMFARecords) RevokeAllTrustedDevices(ctx context.Context, accountID int64, revokedAt time.Time) error {
	return r.source.RevokeAllTrustedDevices(ctx, accountID, revokedAt)
}

func (r publicAccountMFARecords) RevokeTenantTrustedDevices(ctx context.Context, accountID, tenantID int64, revokedAt time.Time) error {
	return r.source.RevokeTenantTrustedDevices(ctx, accountID, tenantID, revokedAt)
}

func (r publicAccountMFARecords) FindGlobalOverride(ctx context.Context, accountID int64) (identityaccess.AccountMFAOverride, bool, error) {
	override, found, err := r.source.FindGlobalOverride(ctx, accountID)
	return identityaccess.AccountMFAOverride(override), found, err
}

func (r publicAccountMFARecords) FindTenantOverride(ctx context.Context, accountID, tenantID int64) (identityaccess.AccountMFAOverride, bool, error) {
	override, found, err := r.source.FindTenantOverride(ctx, accountID, tenantID)
	return identityaccess.AccountMFAOverride(override), found, err
}

func (r publicAccountMFARecords) UpsertGlobalOverride(ctx context.Context, override identityaccess.AccountMFAOverride) error {
	return r.source.UpsertGlobalOverride(ctx, domain.AccountMFAOverride(override))
}

func (r publicAccountMFARecords) UpsertTenantOverride(ctx context.Context, override identityaccess.AccountMFAOverride) error {
	return r.source.UpsertTenantOverride(ctx, domain.AccountMFAOverride(override))
}

func (r publicAccountMFARecords) DeleteGlobalOverride(ctx context.Context, accountID int64) error {
	return r.source.DeleteGlobalOverride(ctx, accountID)
}

func (r publicAccountMFARecords) DeleteTenantOverride(ctx context.Context, accountID, tenantID int64) error {
	return r.source.DeleteTenantOverride(ctx, accountID, tenantID)
}

var _ AccountMFARecords = publicAccountMFARecords{}
