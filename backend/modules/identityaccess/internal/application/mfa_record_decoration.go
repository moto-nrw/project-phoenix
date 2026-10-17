package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// publicAccountMFARecords exposes native records only when a test decorates
// persistence to exercise an infrastructure failure through the public flows.
type PublicAccountMFARecords struct {
	Source ports.AccountMFARecords
}

func (r PublicAccountMFARecords) FindAccountIdentity(ctx context.Context, accountID int64) (identityaccess.AccountMFAIdentity, bool, error) {
	identity, found, err := r.Source.FindAccountIdentity(ctx, accountID)
	return identityaccess.AccountMFAIdentity(identity), found, err
}

func (r PublicAccountMFARecords) AccountBelongsToTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	return r.Source.AccountBelongsToTenant(ctx, accountID, tenantID)
}

func (r PublicAccountMFARecords) LockAccountForOverrideWrite(ctx context.Context, accountID int64) error {
	return r.Source.LockAccountForOverrideWrite(ctx, accountID)
}

func (r PublicAccountMFARecords) IncrementMFAAttempts(ctx context.Context, accountID int64, threshold int, duration time.Duration) (identityaccess.AccountLockout, error) {
	lockout, err := r.Source.IncrementMFAAttempts(ctx, accountID, threshold, duration)
	return identityaccess.AccountLockout(lockout), err
}

func (r PublicAccountMFARecords) ResetMFAAttempts(ctx context.Context, accountID int64) error {
	return r.Source.ResetMFAAttempts(ctx, accountID)
}

func (r PublicAccountMFARecords) FindCredential(ctx context.Context, accountID int64) (identityaccess.AccountMFACredential, bool, error) {
	credential, found, err := r.Source.FindCredential(ctx, accountID)
	return identityaccess.AccountMFACredential(credential), found, err
}

func (r PublicAccountMFARecords) CreateCredential(ctx context.Context, credential identityaccess.AccountMFACredential) error {
	return r.Source.CreateCredential(ctx, domain.AccountMFACredential(credential))
}

func (r PublicAccountMFARecords) TouchCredential(ctx context.Context, id int64, usedAt time.Time) error {
	return r.Source.TouchCredential(ctx, id, usedAt)
}

func (r PublicAccountMFARecords) DeleteCredentials(ctx context.Context, accountID int64) error {
	return r.Source.DeleteCredentials(ctx, accountID)
}

func (r PublicAccountMFARecords) CreateChallenge(ctx context.Context, challenge identityaccess.AccountMFAChallenge) (identityaccess.AccountMFAChallenge, error) {
	stored, err := r.Source.CreateChallenge(ctx, domain.AccountMFAChallenge(challenge))
	return identityaccess.AccountMFAChallenge(stored), err
}

func (r PublicAccountMFARecords) ActivateChallenge(ctx context.Context, id int64) error {
	return r.Source.ActivateChallenge(ctx, id)
}

func (r PublicAccountMFARecords) ConsumeChallenge(ctx context.Context, id int64, consumedAt time.Time) error {
	return r.Source.ConsumeChallenge(ctx, id, consumedAt)
}

func (r PublicAccountMFARecords) CountChallengesSince(ctx context.Context, accountID int64, since time.Time) (int, error) {
	return r.Source.CountChallengesSince(ctx, accountID, since)
}

func (r PublicAccountMFARecords) FindActiveChallengeForAccount(ctx context.Context, id, accountID int64) (identityaccess.AccountMFAChallenge, bool, error) {
	challenge, found, err := r.Source.FindActiveChallengeForAccount(ctx, id, accountID)
	return identityaccess.AccountMFAChallenge(challenge), found, err
}

func (r PublicAccountMFARecords) FindActiveChallengeInScope(ctx context.Context, accountID, tenantID int64, scope string) (identityaccess.AccountMFAChallenge, bool, error) {
	challenge, found, err := r.Source.FindActiveChallengeInScope(ctx, accountID, tenantID, scope)
	return identityaccess.AccountMFAChallenge(challenge), found, err
}

func (r PublicAccountMFARecords) CreateTrustedDevice(ctx context.Context, device identityaccess.AccountTrustedDevice) (identityaccess.AccountTrustedDevice, error) {
	stored, err := r.Source.CreateTrustedDevice(ctx, domain.AccountTrustedDevice(device))
	return identityaccess.AccountTrustedDevice(stored), err
}

func (r PublicAccountMFARecords) FindActiveTrustedDevice(ctx context.Context, accountID, tenantID int64, tokenHash string) (identityaccess.AccountTrustedDevice, bool, error) {
	device, found, err := r.Source.FindActiveTrustedDevice(ctx, accountID, tenantID, tokenHash)
	return identityaccess.AccountTrustedDevice(device), found, err
}

func (r PublicAccountMFARecords) ListActiveTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]identityaccess.AccountTrustedDevice, error) {
	devices, err := r.Source.ListActiveTrustedDevices(ctx, accountID, tenantID)
	if err != nil {
		return nil, err
	}
	result := make([]identityaccess.AccountTrustedDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, identityaccess.AccountTrustedDevice(device))
	}
	return result, nil
}

func (r PublicAccountMFARecords) TouchTrustedDevice(ctx context.Context, id int64, usedAt time.Time) error {
	return r.Source.TouchTrustedDevice(ctx, id, usedAt)
}

func (r PublicAccountMFARecords) RevokeTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) error {
	return r.Source.RevokeTrustedDevice(ctx, id, revokedAt)
}

func (r PublicAccountMFARecords) RevokeAllTrustedDevices(ctx context.Context, accountID int64, revokedAt time.Time) error {
	return r.Source.RevokeAllTrustedDevices(ctx, accountID, revokedAt)
}

func (r PublicAccountMFARecords) RevokeTenantTrustedDevices(ctx context.Context, accountID, tenantID int64, revokedAt time.Time) error {
	return r.Source.RevokeTenantTrustedDevices(ctx, accountID, tenantID, revokedAt)
}

func (r PublicAccountMFARecords) FindGlobalOverride(ctx context.Context, accountID int64) (identityaccess.AccountMFAOverride, bool, error) {
	override, found, err := r.Source.FindGlobalOverride(ctx, accountID)
	return identityaccess.AccountMFAOverride(override), found, err
}

func (r PublicAccountMFARecords) FindTenantOverride(ctx context.Context, accountID, tenantID int64) (identityaccess.AccountMFAOverride, bool, error) {
	override, found, err := r.Source.FindTenantOverride(ctx, accountID, tenantID)
	return identityaccess.AccountMFAOverride(override), found, err
}

func (r PublicAccountMFARecords) UpsertGlobalOverride(ctx context.Context, override identityaccess.AccountMFAOverride) error {
	return r.Source.UpsertGlobalOverride(ctx, domain.AccountMFAOverride(override))
}

func (r PublicAccountMFARecords) UpsertTenantOverride(ctx context.Context, override identityaccess.AccountMFAOverride) error {
	return r.Source.UpsertTenantOverride(ctx, domain.AccountMFAOverride(override))
}

func (r PublicAccountMFARecords) DeleteGlobalOverride(ctx context.Context, accountID int64) error {
	return r.Source.DeleteGlobalOverride(ctx, accountID)
}

func (r PublicAccountMFARecords) DeleteTenantOverride(ctx context.Context, accountID, tenantID int64) error {
	return r.Source.DeleteTenantOverride(ctx, accountID, tenantID)
}
