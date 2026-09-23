package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The seams the MFA and passkey flows need beyond the record stores (#3331):
// the account MFA rows, the school settings that parameterise the gate, the
// challenge-token codec, the code hasher and the two mails the flows send.
//
// AccountMFARecords is bound to the module's native Postgres persistence.
// Missing active challenges and devices may return a not-found error;
// callers refuse verification for either an error or found=false.
type AccountMFARecords interface {
	AccountDirectory
	// AccountBelongsToTenant reports an active school mapping.
	AccountBelongsToTenant(ctx context.Context, accountID, tenantID int64) (bool, error)
	// LockAccountForOverrideWrite takes the account row FOR UPDATE. A
	// missing account is not an error: the foreign keys of the rows written
	// afterwards already report it.
	LockAccountForOverrideWrite(ctx context.Context, accountID int64) error
	// IncrementMFAAttempts bumps the failure counter atomically and applies
	// the lockout when this increment crossed the threshold.
	IncrementMFAAttempts(ctx context.Context, accountID int64, threshold int, duration time.Duration) (domain.AccountLockout, error)
	ResetMFAAttempts(ctx context.Context, accountID int64) error

	FindCredential(ctx context.Context, accountID int64) (domain.AccountMFACredential, bool, error)
	CreateCredential(ctx context.Context, credential domain.AccountMFACredential) error
	TouchCredential(ctx context.Context, id int64, usedAt time.Time) error
	DeleteCredentials(ctx context.Context, accountID int64) error

	// CreateChallenge stores the code and writes the stored identity back
	// into the returned value.
	CreateChallenge(ctx context.Context, challenge domain.AccountMFAChallenge) (domain.AccountMFAChallenge, error)
	// ActivateChallenge makes a delivered code redeemable.
	ActivateChallenge(ctx context.Context, id int64) error
	// ConsumeChallenge redeems a code exactly once; the loser of two
	// concurrent verifications receives an error.
	ConsumeChallenge(ctx context.Context, id int64, consumedAt time.Time) error
	CountChallengesSince(ctx context.Context, accountID int64, since time.Time) (int, error)
	// FindActiveChallengeForAccount resolves the exact row a challenge JWT
	// names.
	FindActiveChallengeForAccount(ctx context.Context, id, accountID int64) (domain.AccountMFAChallenge, bool, error)
	// FindActiveChallengeInScope resolves the newest active code of one
	// account at one portal, for the flows that carry no challenge id.
	FindActiveChallengeInScope(ctx context.Context, accountID, tenantID int64, scope string) (domain.AccountMFAChallenge, bool, error)

	CreateTrustedDevice(ctx context.Context, device domain.AccountTrustedDevice) (domain.AccountTrustedDevice, error)
	FindActiveTrustedDevice(ctx context.Context, accountID, tenantID int64, tokenHash string) (domain.AccountTrustedDevice, bool, error)
	ListActiveTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]domain.AccountTrustedDevice, error)
	TouchTrustedDevice(ctx context.Context, id int64, usedAt time.Time) error
	RevokeTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) error
	// RevokeAllTrustedDevices revokes across every school; the disable
	// cascade and the platform-wide force_off use it.
	RevokeAllTrustedDevices(ctx context.Context, accountID int64, revokedAt time.Time) error
	// RevokeTenantTrustedDevices revokes the devices of one school only.
	RevokeTenantTrustedDevices(ctx context.Context, accountID, tenantID int64, revokedAt time.Time) error

	FindGlobalOverride(ctx context.Context, accountID int64) (domain.AccountMFAOverride, bool, error)
	FindTenantOverride(ctx context.Context, accountID, tenantID int64) (domain.AccountMFAOverride, bool, error)
	UpsertGlobalOverride(ctx context.Context, override domain.AccountMFAOverride) error
	UpsertTenantOverride(ctx context.Context, override domain.AccountMFAOverride) error
	DeleteGlobalOverride(ctx context.Context, accountID int64) error
	DeleteTenantOverride(ctx context.Context, accountID, tenantID int64) error
}

// AccountDirectory is the account facts both the MFA gate and the passkey
// ceremonies read: who to mail, whether the account is active and whether it
// is inside its failure cooldown.
type AccountDirectory interface {
	FindAccountIdentity(ctx context.Context, accountID int64) (domain.AccountIdentity, bool, error)
}

// MFASettings is the consumer-owned port over the Settings platform values
// that parameterise the account gate. Each method reports the school's
// override when tenantID is positive and the registry default otherwise; a
// failed read is an error the caller fails closed on.
type MFASettings interface {
	// MFAMode resolves security.mfa_mode.
	MFAMode(ctx context.Context, tenantID int64) (string, error)
	// MFAModeInTx resolves security.mfa_mode on the caller's transaction,
	// past the request-scoped memo cache.
	MFAModeInTx(ctx context.Context, tenantID int64) (string, error)
	// TrustedDeviceEnabled resolves security.mfa_trusted_device_enabled.
	TrustedDeviceEnabled(ctx context.Context, tenantID int64) (bool, error)
	// TrustedDeviceDays resolves security.mfa_trusted_device_days.
	TrustedDeviceDays(ctx context.Context, tenantID int64) (int, error)
	// LockoutThreshold resolves security.account_lockout_threshold.
	LockoutThreshold(ctx context.Context, tenantID int64) (int, error)
	// LockoutDuration resolves security.account_lockout_duration_minutes.
	LockoutDuration(ctx context.Context, tenantID int64) (time.Duration, error)
}

// MFAChallengeCodec signs and parses the short-lived challenge JWTs.
type MFAChallengeCodec interface {
	IssueChallengeToken(claims domain.MFAChallengeClaims, ttl time.Duration) (string, error)
	ParseChallengeToken(token string) (domain.MFAChallengeClaims, error)
}

// ShortCodeHasher hashes and verifies the e-mail codes with the project-wide
// Argon2id helper, so a parameter change reaches MFA codes too.
type ShortCodeHasher interface {
	HashShortCode(plain string) (string, error)
	VerifyShortCode(plain, encodedHash string) (bool, error)
}

// MFAMail is the consumer-owned port over the two mails the MFA flows send.
// DeliverCode is synchronous and fail-closed: the flow only activates the
// code after the transport accepted the message. NotifyTrustedDeviceAdded is
// fire-and-forget, because the login must not break when the security
// notification does.
type MFAMail interface {
	DeliverCode(ctx context.Context, message domain.MFACodeMail) error
	NotifyTrustedDeviceAdded(ctx context.Context, message domain.TrustedDeviceMail)
}

// MFAAuditTrail is the consumer-owned port over the two ledgers the MFA
// flows append to: the tenant-scoped authentication ledger and the operator
// action log. Both are best-effort by contract — the flows log a failure and
// continue, exactly as the retained services did.
type MFAAuditTrail interface {
	// RecordAuthEvent appends to the authentication ledger. Implementations
	// join the caller's transaction when one is active and otherwise open
	// the school's own transaction for the append.
	RecordAuthEvent(ctx context.Context, event domain.AuthEvent) error
	// RecordOperatorActionAsync appends to the operator action log outside the
	// caller's outcome, so the request never pays for the insert.
	RecordOperatorActionAsync(entry domain.OperatorAuditEntry)
}
