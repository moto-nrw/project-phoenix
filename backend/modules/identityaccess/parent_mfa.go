package identityaccess

import (
	"context"
	"errors"
	"net"
)

// The parents' second factor (#7051, ADR 0216 addendum). It is voluntary,
// like the caterer's (ADR 0077 point 7): a parent switches the code by
// e-mail on in the portal's settings, and no school or Träger can require
// it. It is the account's e-mail factor, so an account that already has one
// from its work at a school is asked for it here too. A parent session names
// no school, so its challenges and remembered devices name none either: a
// device remembered at the parents portal answers only there. A parent who
// lost the mailbox gets the factor reset by the moto team, through the
// operator's reset of an account at one of its schools.

// MFAChallengeScopeParent is the parents portal (#7051). A code started
// there is redeemable only there.
const MFAChallengeScopeParent = "parent"

var (
	// ErrParentMFANotEnabled refuses switching off a second factor the
	// account does not have.
	ErrParentMFANotEnabled = errors.New("parent second factor is not switched on")
	// ErrParentMFANeedsSchool refuses switching the factor on for an
	// account that is a parent at no school: the moto team resets a lost
	// factor through a school of the account, and there is none.
	ErrParentMFANeedsSchool = errors.New("parent second factor needs a school of the account")
)

// The parents portal shares the request and answer shapes of the caterer's
// sign-in: a password with the remember-device cookie, the session pair or
// the challenge of the mailed code, and the redeemed code.
type (
	ParentLoginRequest    = CatererLoginRequest
	ParentLoginResult     = CatererLoginResult
	ParentMFAVerification = CatererMFAVerification
	ParentSession         = CatererSession
	ParentMFAChallenge    = CatererMFAChallenge
	ParentMFAConfirmation = CatererMFAConfirmation
)

// ParentMFAState is what the parents portal's settings show. Available is
// false for an account that is a parent at no school (ErrParentMFANeedsSchool).
type ParentMFAState struct {
	Enabled     bool
	Available   bool
	MaskedEmail string
}

// ParentSignIn is the parents portal's sign-in: the password with the
// second factor where the account has one.
type ParentSignIn interface {
	LoginParent(ctx context.Context, request ParentLoginRequest) (ParentLoginResult, error)
	// VerifyParentMFA redeems the mailed code and mints the session; the
	// session names the signed-in address.
	VerifyParentMFA(ctx context.Context, verification ParentMFAVerification) (ParentSession, error)
	// ResendParentMFA mails a new code for a parents-portal challenge and
	// returns the renewed challenge.
	ResendParentMFA(ctx context.Context, challengeToken string, ip net.IP) (string, error)
}

// ParentSecondFactor is the parent's own management of the second factor
// in the portal's settings.
type ParentSecondFactor interface {
	ParentMFAStatus(ctx context.Context, accountID int64) (ParentMFAState, error)
	StartParentMFA(ctx context.Context, accountID int64, ip net.IP) (ParentMFAChallenge, error)
	// ConfirmParentMFA switches the factor on and mints a session proven by
	// it; the account's other sessions end at their next refresh.
	ConfirmParentMFA(ctx context.Context, confirmation ParentMFAConfirmation) (ParentSession, error)
	// DisableParentMFA switches the factor off after checking the current
	// password like a sign-in.
	DisableParentMFA(ctx context.Context, accountID int64, currentPassword, ipAddress, userAgent string) error
	ListParentTrustedDevices(ctx context.Context, accountID int64) ([]AccountTrustedDevice, error)
	RevokeParentTrustedDevice(ctx context.Context, accountID, deviceID int64) error
}

// ParentPortalAccess is what the composed owner serves beside its Owner
// facade for the parents portal: Parents returns the sign-in with its
// second factor, or nil when the owner was composed without the sessions.
type ParentPortalAccess interface {
	Parents() *ParentAccess
}

// ParentAccess is the capability of the parents' sign-in with its second
// factor and of its management.
type ParentAccess struct {
	ParentSignIn
	ParentSecondFactor
}

func NewParentAccess(signIn ParentSignIn, secondFactor ParentSecondFactor) *ParentAccess {
	if signIn == nil || secondFactor == nil {
		panic("parent access requires every part")
	}
	return &ParentAccess{ParentSignIn: signIn, ParentSecondFactor: secondFactor}
}
