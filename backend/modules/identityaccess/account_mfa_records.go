package identityaccess

import (
	"net"
	"time"
)

// The account MFA rows and the seams around them (#3331). Identity & Access
// owns these tables; until #3226 moves their repositories under the module,
// the composition binds them through the record port below, which keeps the
// lockout, expiry and single-use semantics exactly where they were.

// AccountMFAIdentity is the account facts the MFA gate and the passkey
// ceremonies read.
type AccountMFAIdentity struct {
	ID             int64
	Email          string
	Active         bool
	MFAAttempts    int
	MFALockedUntil *time.Time
}

// AccountMFACredential records that an account enrolled in e-mail MFA.
type AccountMFACredential struct {
	ID         int64
	AccountID  int64
	Method     string
	EnrolledAt time.Time
	LastUsedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AccountMFAChallenge is one hashed e-mail code. ConsumedAt is set while the
// code is not redeemable: before its delivery was confirmed and after it was
// used. Scope and TenantID travel with the row, because the flows without a
// challenge id look a code up by account and must not reach another portal's.
type AccountMFAChallenge struct {
	ID         int64
	AccountID  int64
	Scope      string
	TenantID   int64
	CodeHash   string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	IPAddress  net.IP
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AccountTrustedDevice is the server-side record of a remember-device
// cookie; TokenHash is the hash of the cookie's raw token. TenantID pins the
// trust to one school.
type AccountTrustedDevice struct {
	ID         int64
	AccountID  int64
	TenantID   int64
	TokenHash  string
	UserAgent  *string
	IPAddress  net.IP
	ExpiresAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AccountMFAOverride is an admin's verdict on one account. TenantID is nil
// for the operator's platform-wide emergency switch.
type AccountMFAOverride struct {
	ID        int64
	AccountID int64
	TenantID  *int64
	Override  string
	SetBy     int64
	SetByType string
	Reason    string
}

// AccountLockout is the outcome of one atomic increment of the MFA failure
// counter.
type AccountLockout struct {
	Attempts    int
	LockedUntil *time.Time
}

// MFAChallengeClaims are the claims of a challenge JWT. ChallengeID pins the
// token to the exact row it was minted for.
type MFAChallengeClaims struct {
	AccountID   int64
	Scope       string
	TenantID    int64
	ChallengeID int64
}

// MFACodeMail is one e-mail code message. The template carries no link or
// button by design: the code is pasted into the moto login by hand, which
// removes the "click here to confirm" phishing pattern.
type MFACodeMail struct {
	Recipient     string
	RecipientName string
	// ReferenceID is the account or operator the code belongs to; Operator
	// distinguishes the two id spaces and delivery types.
	ReferenceID          int64
	Operator             bool
	Code                 string
	ExpiryMinutes        int
	RequestIP            string
	TrustedDeviceEnabled bool
	TrustedDeviceDays    int
}

// TrustedDeviceMail notifies the holder that a remember-device cookie was
// issued, so trusting a device never happens silently.
type TrustedDeviceMail struct {
	Recipient     string
	RecipientName string
	ReferenceID   int64
	Operator      bool
	DeviceLabel   string
	RequestIP     string
	AddedAt       string
	TrustedDays   int
}

// MFAEvidence carries the metadata of one mfa_* authentication event.
type MFAEvidence struct {
	ChallengeID int64
	DeviceID    int64
	LockedUntil *time.Time
	Admin       *MFAAdminEvidence
}

// MFAAdminEvidence describes one admin override attempt, accepted or
// refused. Written reports a write that went through and adds the scope and
// the value it replaced.
type MFAAdminEvidence struct {
	ActorType        string
	ActorAccountID   int64
	Action           string
	Written          bool
	Scope            string
	Override         string
	PreviousOverride string
	Reason           string
}

// OperatorMFAEvidence carries the metadata of one operator MFA action.
type OperatorMFAEvidence struct {
	Reason      string
	LockedUntil *time.Time
	Override    *OperatorMFAOverrideEvidence
}

// OperatorMFAOverrideEvidence records one platform-wide override transition.
type OperatorMFAOverrideEvidence struct {
	Action           string
	Scope            string
	Override         string
	PreviousOverride string
	Reason           string
}
