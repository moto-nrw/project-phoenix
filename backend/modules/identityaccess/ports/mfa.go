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

// MFAMail queues MFA messages in the caller's transaction. EnqueueCode returns
// a continuation that must run after commit; activation remains synchronous
// and fail-closed until the transport accepts the code. Trusted-device notices
// are durable but their eventual transport result does not affect login.
type MFAMail interface {
	EnqueueCode(ctx context.Context, message identityaccess.MFACodeMail, challengeID int64, expiresAt time.Time) (func(context.Context) error, error)
	NotifyTrustedDeviceAdded(ctx context.Context, message identityaccess.TrustedDeviceMail, deviceID int64) error
}

// OperatorActionLog appends an operator entry in the caller's transaction.
// A failed append rolls back the audited change.
type OperatorActionLog interface {
	RecordOperatorAction(ctx context.Context, entry identityaccess.OperatorAuditEntry) error
}
