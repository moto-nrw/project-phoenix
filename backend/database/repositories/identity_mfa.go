package repositories

import (
	"context"
	"fmt"
	"time"

	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
)

// The account second factor lives in Identity & Access (#3331). Its rows are
// still the retained auth.mfa_* repositories, so the record port it consumes
// is served here, next to them, until #3226 moves the tables under the
// module's own adapter.

// NewAccountMFARecords serves the module's record port over the retained
// repositories.
func NewAccountMFARecords(repos *Factory) identityCompose.AccountMFARecords {
	return accountMFARecords{repos: repos}
}

// accountMFARecords serves the module's record port over the retained
// repositories. A missing row is (zero, false, nil); a state change that did
// not apply stays an error, because that is what keeps a code single-use and
// a lockout counter honest.
type accountMFARecords struct{ repos *Factory }

func (r accountMFARecords) FindAccountIdentity(ctx context.Context, accountID int64) (identityaccess.AccountMFAIdentity, bool, error) {
	account, err := r.repos.Account.FindByID(ctx, accountID)
	if err != nil {
		if accountRowMissing(err) {
			return identityaccess.AccountMFAIdentity{}, false, nil
		}
		return identityaccess.AccountMFAIdentity{}, false, err
	}
	if account == nil {
		return identityaccess.AccountMFAIdentity{}, false, nil
	}
	return identityaccess.AccountMFAIdentity{
		ID: account.ID, Email: account.Email, Active: account.Active,
		MFAAttempts: account.MFAAttempts, MFALockedUntil: account.MFALockedUntil,
	}, true, nil
}

func (r accountMFARecords) AccountBelongsToTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	return r.repos.AccountTenant.ExistsByAccountAndTenant(ctx, accountID, tenantID)
}

// LockAccountForOverrideWrite takes auth.accounts FOR UPDATE before an
// override is read and written. An override is the per-account half of the
// MFA policy, and a session mint resolves it while holding exactly this row.
// Without the lock the two are unordered: a login could resolve "no
// override, MFA off", an admin commit force_on, and the login still mint a
// session that never saw a second factor. A missing account is not rejected
// here — the foreign keys of the rows written afterwards already report it.
func (r accountMFARecords) LockAccountForOverrideWrite(ctx context.Context, accountID int64) error {
	if _, err := r.repos.Account.FindByIDForUpdate(ctx, accountID); err != nil {
		if authRepo.IsMissingRow(err) {
			return nil
		}
		return fmt.Errorf("lock account %d for mfa override write: %w", accountID, err)
	}
	return nil
}

func (r accountMFARecords) IncrementMFAAttempts(ctx context.Context, accountID int64, threshold int, duration time.Duration) (identityaccess.AccountLockout, error) {
	result, err := r.repos.Account.IncrementMFAAttempts(ctx, accountID, threshold, duration)
	if err != nil {
		return identityaccess.AccountLockout{}, err
	}
	return identityaccess.AccountLockout{Attempts: result.Attempts, LockedUntil: result.LockedUntil}, nil
}

func (r accountMFARecords) ResetMFAAttempts(ctx context.Context, accountID int64) error {
	return r.repos.Account.ResetMFAAttempts(ctx, accountID)
}

func (r accountMFARecords) FindCredential(ctx context.Context, accountID int64) (identityaccess.AccountMFACredential, bool, error) {
	credential, err := r.repos.MFACredential.FindByAccountID(ctx, accountID)
	if err != nil {
		// A missing row is the legitimate "not enrolled" signal every fresh
		// account hits; anything else is an infrastructure failure the flow
		// refuses the login on.
		if authRepo.IsMissingRow(err) {
			return identityaccess.AccountMFACredential{}, false, nil
		}
		return identityaccess.AccountMFACredential{}, false, err
	}
	if credential == nil {
		return identityaccess.AccountMFACredential{}, false, nil
	}
	return identityaccess.AccountMFACredential{
		ID: credential.ID, AccountID: credential.AccountID, Method: credential.Method,
		EnrolledAt: credential.EnrolledAt, LastUsedAt: credential.LastUsedAt,
		CreatedAt: credential.CreatedAt, UpdatedAt: credential.UpdatedAt,
	}, true, nil
}

func (r accountMFARecords) CreateCredential(ctx context.Context, credential identityaccess.AccountMFACredential) error {
	return r.repos.MFACredential.Create(ctx, &authModels.MFACredential{
		AccountID: credential.AccountID, Method: credential.Method,
		EnrolledAt: credential.EnrolledAt, LastUsedAt: credential.LastUsedAt,
	})
}

func (r accountMFARecords) TouchCredential(ctx context.Context, id int64, usedAt time.Time) error {
	return r.repos.MFACredential.UpdateLastUsedAt(ctx, id, usedAt)
}

func (r accountMFARecords) DeleteCredentials(ctx context.Context, accountID int64) error {
	return r.repos.MFACredential.DeleteByAccountID(ctx, accountID)
}

func (r accountMFARecords) CreateChallenge(ctx context.Context, challenge identityaccess.AccountMFAChallenge) (identityaccess.AccountMFAChallenge, error) {
	row := &authModels.MFAEmailChallenge{
		AccountID: challenge.AccountID, Scope: challenge.Scope, TenantID: challenge.TenantID,
		CodeHash: challenge.CodeHash, ExpiresAt: challenge.ExpiresAt,
		ConsumedAt: challenge.ConsumedAt, IPAddress: challenge.IPAddress,
	}
	// The timestamps are set through the promoted fields: the row's embedded
	// base model belongs to the transaction runtime's domain, which this
	// composition seam does not name.
	row.CreatedAt, row.UpdatedAt = challenge.CreatedAt, challenge.UpdatedAt
	if err := r.repos.MFAEmailChallenge.Create(ctx, row); err != nil {
		return identityaccess.AccountMFAChallenge{}, err
	}
	return accountMFAChallenge(row), nil
}

func (r accountMFARecords) ActivateChallenge(ctx context.Context, id int64) error {
	return r.repos.MFAEmailChallenge.MarkActive(ctx, id)
}

func (r accountMFARecords) ConsumeChallenge(ctx context.Context, id int64, consumedAt time.Time) error {
	return r.repos.MFAEmailChallenge.MarkConsumed(ctx, id, consumedAt)
}

func (r accountMFARecords) CountChallengesSince(ctx context.Context, accountID int64, since time.Time) (int, error) {
	return r.repos.MFAEmailChallenge.CountRecentByAccountID(ctx, accountID, since)
}

func (r accountMFARecords) FindActiveChallengeForAccount(ctx context.Context, id, accountID int64) (identityaccess.AccountMFAChallenge, bool, error) {
	row, err := r.repos.MFAEmailChallenge.FindActiveByIDForAccount(ctx, id, accountID)
	if err != nil || row == nil {
		return identityaccess.AccountMFAChallenge{}, false, err
	}
	return accountMFAChallenge(row), true, nil
}

func (r accountMFARecords) FindActiveChallengeInScope(ctx context.Context, accountID, tenantID int64, scope string) (identityaccess.AccountMFAChallenge, bool, error) {
	row, err := r.repos.MFAEmailChallenge.FindActiveByAccountIDInScope(ctx, accountID, tenantID, scope)
	if err != nil || row == nil {
		return identityaccess.AccountMFAChallenge{}, false, err
	}
	return accountMFAChallenge(row), true, nil
}

func (r accountMFARecords) CreateTrustedDevice(ctx context.Context, device identityaccess.AccountTrustedDevice) (identityaccess.AccountTrustedDevice, error) {
	row := &authModels.MFATrustedDevice{
		AccountID: device.AccountID, TenantID: device.TenantID, TokenHash: device.TokenHash,
		UserAgent: device.UserAgent, IPAddress: device.IPAddress, ExpiresAt: device.ExpiresAt,
		LastUsedAt: device.LastUsedAt, RevokedAt: device.RevokedAt,
	}
	if err := r.repos.MFATrustedDevice.Create(ctx, row); err != nil {
		return identityaccess.AccountTrustedDevice{}, err
	}
	return accountTrustedDevice(row), nil
}

func (r accountMFARecords) FindActiveTrustedDevice(ctx context.Context, accountID, tenantID int64, tokenHash string) (identityaccess.AccountTrustedDevice, bool, error) {
	row, err := r.repos.MFATrustedDevice.FindActiveByAccountTenantAndTokenHash(ctx, accountID, tenantID, tokenHash)
	if err != nil || row == nil {
		return identityaccess.AccountTrustedDevice{}, false, err
	}
	return accountTrustedDevice(row), true, nil
}

func (r accountMFARecords) ListActiveTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]identityaccess.AccountTrustedDevice, error) {
	rows, err := r.repos.MFATrustedDevice.ListActiveByAccountTenant(ctx, accountID, tenantID)
	if err != nil {
		return nil, err
	}
	devices := make([]identityaccess.AccountTrustedDevice, 0, len(rows))
	for _, row := range rows {
		devices = append(devices, accountTrustedDevice(row))
	}
	return devices, nil
}

func (r accountMFARecords) TouchTrustedDevice(ctx context.Context, id int64, usedAt time.Time) error {
	return r.repos.MFATrustedDevice.UpdateLastUsedAt(ctx, id, usedAt)
}

func (r accountMFARecords) RevokeTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) error {
	return r.repos.MFATrustedDevice.Revoke(ctx, id, revokedAt)
}

func (r accountMFARecords) RevokeAllTrustedDevices(ctx context.Context, accountID int64, revokedAt time.Time) error {
	return r.repos.MFATrustedDevice.RevokeAllByAccountID(ctx, accountID, revokedAt)
}

func (r accountMFARecords) RevokeTenantTrustedDevices(ctx context.Context, accountID, tenantID int64, revokedAt time.Time) error {
	return r.repos.MFATrustedDevice.RevokeAllByAccountTenant(ctx, accountID, tenantID, revokedAt)
}

func (r accountMFARecords) FindGlobalOverride(ctx context.Context, accountID int64) (identityaccess.AccountMFAOverride, bool, error) {
	row, err := r.repos.MFAOverride.FindGlobal(ctx, accountID)
	if err != nil || row == nil {
		return identityaccess.AccountMFAOverride{}, false, err
	}
	return accountMFAOverride(row), true, nil
}

func (r accountMFARecords) FindTenantOverride(ctx context.Context, accountID, tenantID int64) (identityaccess.AccountMFAOverride, bool, error) {
	row, err := r.repos.MFAOverride.FindByAccountAndTenant(ctx, accountID, tenantID)
	if err != nil || row == nil {
		return identityaccess.AccountMFAOverride{}, false, err
	}
	return accountMFAOverride(row), true, nil
}

func (r accountMFARecords) UpsertGlobalOverride(ctx context.Context, override identityaccess.AccountMFAOverride) error {
	return r.repos.MFAOverride.UpsertGlobal(ctx, overrideRow(override))
}

func (r accountMFARecords) UpsertTenantOverride(ctx context.Context, override identityaccess.AccountMFAOverride) error {
	return r.repos.MFAOverride.UpsertTenant(ctx, overrideRow(override))
}

func (r accountMFARecords) DeleteGlobalOverride(ctx context.Context, accountID int64) error {
	return r.repos.MFAOverride.DeleteGlobal(ctx, accountID)
}

func (r accountMFARecords) DeleteTenantOverride(ctx context.Context, accountID, tenantID int64) error {
	return r.repos.MFAOverride.DeleteTenant(ctx, accountID, tenantID)
}

func accountRowMissing(err error) bool {
	return authRepo.IsMissingRow(err)
}

func accountMFAChallenge(row *authModels.MFAEmailChallenge) identityaccess.AccountMFAChallenge {
	return identityaccess.AccountMFAChallenge{
		ID: row.ID, AccountID: row.AccountID, Scope: row.Scope, TenantID: row.TenantID,
		CodeHash: row.CodeHash, ExpiresAt: row.ExpiresAt, ConsumedAt: row.ConsumedAt,
		IPAddress: row.IPAddress, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func accountTrustedDevice(row *authModels.MFATrustedDevice) identityaccess.AccountTrustedDevice {
	return identityaccess.AccountTrustedDevice{
		ID: row.ID, AccountID: row.AccountID, TenantID: row.TenantID, TokenHash: row.TokenHash,
		UserAgent: row.UserAgent, IPAddress: row.IPAddress, ExpiresAt: row.ExpiresAt,
		LastUsedAt: row.LastUsedAt, RevokedAt: row.RevokedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func accountMFAOverride(row *authModels.MFAOverride) identityaccess.AccountMFAOverride {
	return identityaccess.AccountMFAOverride{
		ID: row.ID, AccountID: row.AccountID, TenantID: row.TenantID, Override: row.Override,
		SetBy: row.SetBy, SetByType: row.SetByType, Reason: row.Reason,
	}
}

func overrideRow(override identityaccess.AccountMFAOverride) *authModels.MFAOverride {
	return &authModels.MFAOverride{
		AccountID: override.AccountID, TenantID: override.TenantID, Override: override.Override,
		SetBy: override.SetBy, SetByType: override.SetByType, Reason: override.Reason,
	}
}
