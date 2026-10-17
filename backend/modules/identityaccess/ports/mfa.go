package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// AccountMFARecords is the test-decoration view of native MFA persistence.
// A state change that did not apply is an error, keeping codes single-use.
type AccountMFARecords interface {
	FindAccountIdentity(ctx context.Context, accountID int64) (identityaccess.AccountMFAIdentity, bool, error)
	AccountBelongsToTenant(ctx context.Context, accountID, tenantID int64) (bool, error)
	// LockAccountForOverrideWrite takes the account row FOR UPDATE ahead of
	// an override write, so a session mint and an override cannot interleave
	// unordered. A missing account is not an error.
	LockAccountForOverrideWrite(ctx context.Context, accountID int64) error
	IncrementMFAAttempts(ctx context.Context, accountID int64, threshold int, duration time.Duration) (identityaccess.AccountLockout, error)
	ResetMFAAttempts(ctx context.Context, accountID int64) error

	FindCredential(ctx context.Context, accountID int64) (identityaccess.AccountMFACredential, bool, error)
	CreateCredential(ctx context.Context, credential identityaccess.AccountMFACredential) error
	TouchCredential(ctx context.Context, id int64, usedAt time.Time) error
	DeleteCredentials(ctx context.Context, accountID int64) error

	CreateChallenge(ctx context.Context, challenge identityaccess.AccountMFAChallenge) (identityaccess.AccountMFAChallenge, error)
	ActivateChallenge(ctx context.Context, id int64) error
	ConsumeChallenge(ctx context.Context, id int64, consumedAt time.Time) error
	CountChallengesSince(ctx context.Context, accountID int64, since time.Time) (int, error)
	FindActiveChallengeForAccount(ctx context.Context, id, accountID int64) (identityaccess.AccountMFAChallenge, bool, error)
	FindActiveChallengeInScope(ctx context.Context, accountID, tenantID int64, scope string) (identityaccess.AccountMFAChallenge, bool, error)

	CreateTrustedDevice(ctx context.Context, device identityaccess.AccountTrustedDevice) (identityaccess.AccountTrustedDevice, error)
	FindActiveTrustedDevice(ctx context.Context, accountID, tenantID int64, tokenHash string) (identityaccess.AccountTrustedDevice, bool, error)
	ListActiveTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]identityaccess.AccountTrustedDevice, error)
	TouchTrustedDevice(ctx context.Context, id int64, usedAt time.Time) error
	RevokeTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) error
	RevokeAllTrustedDevices(ctx context.Context, accountID int64, revokedAt time.Time) error
	RevokeTenantTrustedDevices(ctx context.Context, accountID, tenantID int64, revokedAt time.Time) error

	FindGlobalOverride(ctx context.Context, accountID int64) (identityaccess.AccountMFAOverride, bool, error)
	FindTenantOverride(ctx context.Context, accountID, tenantID int64) (identityaccess.AccountMFAOverride, bool, error)
	UpsertGlobalOverride(ctx context.Context, override identityaccess.AccountMFAOverride) error
	UpsertTenantOverride(ctx context.Context, override identityaccess.AccountMFAOverride) error
	DeleteGlobalOverride(ctx context.Context, accountID int64) error
	DeleteTenantOverride(ctx context.Context, accountID, tenantID int64) error
}

// MFAChallengeCodec signs and parses the short-lived challenge JWTs.
type MFAChallengeCodec interface {
	IssueChallengeToken(claims identityaccess.MFAChallengeClaims, ttl time.Duration) (string, error)
	ParseChallengeToken(token string) (identityaccess.MFAChallengeClaims, error)
}

// MFAMail sends the two MFA messages. DeliverCode is synchronous and
// fail-closed: the module activates a code only after the transport accepted
// it. NotifyTrustedDeviceAdded is fire-and-forget.
type MFAMail interface {
	DeliverCode(ctx context.Context, message identityaccess.MFACodeMail) error
	NotifyTrustedDeviceAdded(ctx context.Context, message identityaccess.TrustedDeviceMail)
}

// OperatorActionLog appends an operator entry outside the caller's outcome,
// so the request never pays for the insert and a failed append never fails a
// login.
type OperatorActionLog interface {
	RecordOperatorActionAsync(entry identityaccess.OperatorAuditEntry)
}
