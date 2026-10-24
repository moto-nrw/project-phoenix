package identityaccess

import (
	"context"
	"time"
)

// AnalyticsConsent is an account's answer to the usage analytics in the
// browser (§ 25 TDDDG, #4901). Granted is nil while the account has not
// decided; DecidedAt is set with every answer.
type AnalyticsConsent struct {
	Granted   *bool
	DecidedAt *time.Time
}

// AccountAnalyticsConsent reads and records the account's own consent to the
// usage analytics. The caller authorizes the account: only the authenticated
// subject of a real session decides for itself. A missing or erased account
// reports ErrAccountNotFound. Like the password change, both run on the
// global identity unit and join a caller's transaction; the consent belongs to
// the account, not to a school.
type AccountAnalyticsConsent interface {
	FindAnalyticsConsent(ctx context.Context, accountID int64) (AnalyticsConsent, error)
	// DecideAnalyticsConsent records the answer and returns it as stored.
	DecideAnalyticsConsent(ctx context.Context, accountID int64, granted bool) (AnalyticsConsent, error)
}
