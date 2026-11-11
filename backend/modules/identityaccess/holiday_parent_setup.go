package identityaccess

import (
	"context"
	"errors"
	"time"
)

// Parent account at a Träger (ADR 0216 points 5 and 6). A holiday
// registration without a parent session may mail the parents a link to set
// up an account. The link's key is a signed token with the address, the
// Träger and an expiry. It is issued when the mail is sent, never kept in
// the outbox or anywhere else, is no session and counts only at
// /auth/holiday-parent-setups/{token}. Redeeming it creates the account
// with a password when the address has none, and in both cases records the
// account's parents-portal access at the Träger. Redeeming it again changes
// nothing: an existing account keeps its password.

// HolidayParentSetupTTL is how long a setup link stays redeemable.
const HolidayParentSetupTTL = 30 * 24 * time.Hour

// ErrHolidayParentSetupInvalid refuses a setup link that is unreadable,
// expired, of another purpose, or names a Träger that is gone or inactive.
var ErrHolidayParentSetupInvalid = errors.New("the holiday parent setup link is invalid or expired")

// HolidayParentSetupClaims is the content of a setup token.
type HolidayParentSetupClaims struct {
	Email          string
	OrganizationID int64
	ExpiresAt      int64
}

// HolidayParentSetupPreview is what the setup page shows: whoever holds the
// link holds the mailbox, so it names the address and the Träger and tells
// whether an account for the address already exists.
type HolidayParentSetupPreview struct {
	Email         string
	TraegerName   string
	AccountExists bool
}

// HolidayParentSetups issues and redeems the setup link.
type HolidayParentSetups interface {
	// IssueHolidayParentSetupToken signs the setup token for the address and
	// the Träger. It reads nothing, so another owner's transaction may call
	// it.
	IssueHolidayParentSetupToken(email string, organizationID int64) (string, error)
	// PreviewHolidayParentSetup reads the setup page's facts.
	PreviewHolidayParentSetup(ctx context.Context, token string) (HolidayParentSetupPreview, error)
	// CompleteHolidayParentSetup creates the account when the address has
	// none and records its access at the Träger; accountExisted reports an
	// account that was there before, whose password stays unchanged.
	CompleteHolidayParentSetup(ctx context.Context, token string, registration GuardianRegistration) (accountExisted bool, err error)
	// HasTraegerParentAccess reports whether the account holds
	// parents-portal access at an active Träger: an account the parents
	// portal signs in without a school. It reads in the caller's
	// administrative transaction, or opens one.
	HasTraegerParentAccess(ctx context.Context, accountID int64) (bool, error)
}
