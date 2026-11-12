package ports

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
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

// CatererMFAResetNotice queues the caterer's notice of an operator's reset
// of its second factor (#4497) in the reset's transaction, keyed by the
// reset it reports.
type CatererMFAResetNotice interface {
	NotifyCatererMFAReset(ctx context.Context, message identityaccess.CatererMFAResetMail, resetKey string) error
}

// AccountSecurityNotice queues the mails that tell an account holder about
// an administrator's change to their sign-in (#6862), in the transaction of
// the change they report and keyed by it.
type AccountSecurityNotice interface {
	NotifyMFAResetByAdmin(ctx context.Context, message identityaccess.MFAResetByAdminMail, resetKey string) error
	NotifyLoginAddressChanged(ctx context.Context, message identityaccess.LoginAddressChangedMail, changeKey string) error
}

// OperatorActionLog appends an operator entry in the caller's transaction.
// A failed append rolls back the audited change.
type OperatorActionLog interface {
	RecordOperatorAction(ctx context.Context, entry identityaccess.OperatorAuditEntry) error
}
