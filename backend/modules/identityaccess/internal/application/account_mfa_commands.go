package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// Account MFA commands preserve the independently committed credential, attempt,
// and challenge operations of authentication. Persistence never chooses a
// connection or opens a unit of work. An outer MFA/mail command remains atomic.
type accountMFACommands struct {
	ports.AccountMFARecords
	runtime ports.Runtime
}

func NewAccountMFACommands(records ports.AccountMFARecords, runtime ports.Runtime) ports.AccountMFARecords {
	return accountMFACommands{records, runtime}
}
func (r accountMFACommands) AccountBelongsToTenant(ctx context.Context, accountID, tenantID int64) (value0 bool, err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		value0, err = r.AccountMFARecords.AccountBelongsToTenant(txCtx, accountID, tenantID)
		return err
	})
	return value0, err
}
func (r accountMFACommands) IncrementMFAAttempts(ctx context.Context, accountID int64, threshold int, duration time.Duration) (value0 domain.AccountLockout, err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		value0, err = r.AccountMFARecords.IncrementMFAAttempts(txCtx, accountID, threshold, duration)
		return err
	})
	return value0, err
}
func (r accountMFACommands) ResetMFAAttempts(ctx context.Context, accountID int64) (err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		err = r.AccountMFARecords.ResetMFAAttempts(txCtx, accountID)
		return err
	})
	return err
}
func (r accountMFACommands) FindCredential(ctx context.Context, accountID int64) (value0 domain.AccountMFACredential, value1 bool, err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		value0, value1, err = r.AccountMFARecords.FindCredential(txCtx, accountID)
		return err
	})
	return value0, value1, err
}
func (r accountMFACommands) CreateCredential(ctx context.Context, credential domain.AccountMFACredential) (err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		err = r.AccountMFARecords.CreateCredential(txCtx, credential)
		return err
	})
	return err
}
func (r accountMFACommands) TouchCredential(ctx context.Context, id int64, usedAt time.Time) (err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		err = r.AccountMFARecords.TouchCredential(txCtx, id, usedAt)
		return err
	})
	return err
}
func (r accountMFACommands) DeleteCredentials(ctx context.Context, accountID int64) (err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		err = r.AccountMFARecords.DeleteCredentials(txCtx, accountID)
		return err
	})
	return err
}
func (r accountMFACommands) CreateChallenge(ctx context.Context, challenge domain.AccountMFAChallenge) (value0 domain.AccountMFAChallenge, err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		value0, err = r.AccountMFARecords.CreateChallenge(txCtx, challenge)
		return err
	})
	return value0, err
}
func (r accountMFACommands) ActivateChallenge(ctx context.Context, id int64) (err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error { err = r.AccountMFARecords.ActivateChallenge(txCtx, id); return err })
	return err
}
func (r accountMFACommands) ConsumeChallenge(ctx context.Context, id int64, consumedAt time.Time) (err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		err = r.AccountMFARecords.ConsumeChallenge(txCtx, id, consumedAt)
		return err
	})
	return err
}
func (r accountMFACommands) CountChallengesSince(ctx context.Context, accountID int64, since time.Time) (value0 int, err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		value0, err = r.AccountMFARecords.CountChallengesSince(txCtx, accountID, since)
		return err
	})
	return value0, err
}
func (r accountMFACommands) FindActiveChallengeForAccount(ctx context.Context, id, accountID int64) (value0 domain.AccountMFAChallenge, value1 bool, err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		value0, value1, err = r.AccountMFARecords.FindActiveChallengeForAccount(txCtx, id, accountID)
		return err
	})
	return value0, value1, err
}
func (r accountMFACommands) FindActiveChallengeInScope(ctx context.Context, accountID, tenantID int64, scope string) (value0 domain.AccountMFAChallenge, value1 bool, err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		value0, value1, err = r.AccountMFARecords.FindActiveChallengeInScope(txCtx, accountID, tenantID, scope)
		return err
	})
	return value0, value1, err
}
func (r accountMFACommands) CreateTrustedDevice(ctx context.Context, device domain.AccountTrustedDevice) (value0 domain.AccountTrustedDevice, err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		value0, err = r.AccountMFARecords.CreateTrustedDevice(txCtx, device)
		return err
	})
	return value0, err
}
func (r accountMFACommands) FindActiveTrustedDevice(ctx context.Context, accountID, tenantID int64, tokenHash string) (value0 domain.AccountTrustedDevice, value1 bool, err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		value0, value1, err = r.AccountMFARecords.FindActiveTrustedDevice(txCtx, accountID, tenantID, tokenHash)
		return err
	})
	return value0, value1, err
}
func (r accountMFACommands) ListActiveTrustedDevices(ctx context.Context, accountID, tenantID int64) (value0 []domain.AccountTrustedDevice, err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		value0, err = r.AccountMFARecords.ListActiveTrustedDevices(txCtx, accountID, tenantID)
		return err
	})
	return value0, err
}
func (r accountMFACommands) TouchTrustedDevice(ctx context.Context, id int64, usedAt time.Time) (err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		err = r.AccountMFARecords.TouchTrustedDevice(txCtx, id, usedAt)
		return err
	})
	return err
}
func (r accountMFACommands) RevokeTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) (err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		err = r.AccountMFARecords.RevokeTrustedDevice(txCtx, id, revokedAt)
		return err
	})
	return err
}
func (r accountMFACommands) RevokeAllTrustedDevices(ctx context.Context, accountID int64, revokedAt time.Time) (err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		err = r.AccountMFARecords.RevokeAllTrustedDevices(txCtx, accountID, revokedAt)
		return err
	})
	return err
}
func (r accountMFACommands) RevokeTenantTrustedDevices(ctx context.Context, accountID, tenantID int64, revokedAt time.Time) (err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		err = r.AccountMFARecords.RevokeTenantTrustedDevices(txCtx, accountID, tenantID, revokedAt)
		return err
	})
	return err
}
func (r accountMFACommands) FindGlobalOverride(ctx context.Context, accountID int64) (value0 domain.AccountMFAOverride, value1 bool, err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		value0, value1, err = r.AccountMFARecords.FindGlobalOverride(txCtx, accountID)
		return err
	})
	return value0, value1, err
}
func (r accountMFACommands) FindTenantOverride(ctx context.Context, accountID, tenantID int64) (value0 domain.AccountMFAOverride, value1 bool, err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		value0, value1, err = r.AccountMFARecords.FindTenantOverride(txCtx, accountID, tenantID)
		return err
	})
	return value0, value1, err
}
func (r accountMFACommands) UpsertGlobalOverride(ctx context.Context, override domain.AccountMFAOverride) (err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		err = r.AccountMFARecords.UpsertGlobalOverride(txCtx, override)
		return err
	})
	return err
}
func (r accountMFACommands) UpsertTenantOverride(ctx context.Context, override domain.AccountMFAOverride) (err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		err = r.AccountMFARecords.UpsertTenantOverride(txCtx, override)
		return err
	})
	return err
}
func (r accountMFACommands) DeleteGlobalOverride(ctx context.Context, accountID int64) (err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		err = r.AccountMFARecords.DeleteGlobalOverride(txCtx, accountID)
		return err
	})
	return err
}
func (r accountMFACommands) DeleteTenantOverride(ctx context.Context, accountID, tenantID int64) (err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		err = r.AccountMFARecords.DeleteTenantOverride(txCtx, accountID, tenantID)
		return err
	})
	return err
}
func (r accountMFACommands) FindAccountIdentity(ctx context.Context, accountID int64) (value0 domain.AccountIdentity, value1 bool, err error) {
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		value0, value1, err = r.AccountMFARecords.FindAccountIdentity(txCtx, accountID)
		return err
	})
	return value0, value1, err
}
