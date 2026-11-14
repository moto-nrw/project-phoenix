package identityaccess

import "time"

// The voluntary second factor is the one an account switches on itself at a
// portal without a school: the caterer portal (#4497, ADR 0077 point 7) and
// the parents portal (#7051, ADR 0216 addendum). Both portals speak in these
// shapes.

// VoluntaryLoginRequest is a password sign-in. TrustedDeviceCookie is the
// remember-device cookie the browser sent, or empty.
type VoluntaryLoginRequest struct {
	Email               string
	Password            string
	IPAddress           string
	UserAgent           string
	TrustedDeviceCookie string
}

// VoluntaryLoginResult is either the session pair or, for an account with
// the second factor, the challenge of the code mailed to it.
type VoluntaryLoginResult struct {
	Status            LoginStatus
	AccessToken       string
	RefreshToken      string
	ChallengeToken    string
	MaskedEmail       string
	TrustedDeviceDays int
}

// VoluntaryMFAVerification redeems the mailed code of a sign-in.
type VoluntaryMFAVerification struct {
	ChallengeToken string
	Code           string
	RememberDevice bool
	IPAddress      string
	UserAgent      string
}

// VoluntarySession is a session pair proven by a second factor. A remembered
// device adds its signed cookie; an empty cookie remembers none.
type VoluntarySession struct {
	AccessToken  string
	RefreshToken string
	// Email is the signed-in address; the parents portal claims its holiday
	// registrations with it (ADR 0216).
	Email                  string
	TrustedDeviceCookie    string
	TrustedDeviceExpiresAt time.Time
}

// VoluntaryMFAChallenge names the code mailed to switch the second factor on.
type VoluntaryMFAChallenge struct {
	ChallengeToken string
	MaskedEmail    string
}

// VoluntaryMFAConfirmation switches the second factor on with the mailed code.
type VoluntaryMFAConfirmation struct {
	AccountID      int64
	ChallengeToken string
	Code           string
	IPAddress      string
	UserAgent      string
}
