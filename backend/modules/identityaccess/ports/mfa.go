package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

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
