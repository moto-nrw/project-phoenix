package domain

import (
	"net"
	"time"
)

// The account MFA records the flows read and write (#3331): the enrollment,
// the hashed e-mail codes, the server-side records of the remember-device
// cookies and the admin overrides. A code and a trusted device belong to one
// (account, school) pair; an override is either school-scoped or
// platform-wide.

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
// used. Scope and TenantID travel with the row, not just with the challenge
// JWT, because the enrollment-confirm and passkey-registration paths look a
// code up by account and would otherwise reach another portal's in-flight
// code.
type AccountMFAChallenge struct {
	MFAResetGeneration int64
	ID                 int64
	AccountID          int64
	Scope              string
	TenantID           int64
	CodeHash           string
	ExpiresAt          time.Time
	ConsumedAt         *time.Time
	IPAddress          net.IP
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// AccountTrustedDevice is the server-side record of a remember-device
// cookie. TenantID pins the trust to one school: a cookie issued for school
// A never bypasses MFA at school B.
type AccountTrustedDevice struct {
	ID        int64
	AccountID int64
	TenantID  int64
	// OrganizationID is the Träger of a device the Träger-Portal remembered
	// (#6955); zero for a school's and the caterer portal's device.
	OrganizationID int64
	// ParentPortal marks a device the parents portal remembered (#7051); it
	// names neither a school nor a Träger, like the caterer portal's.
	ParentPortal bool
	TokenHash    string
	UserAgent    *string
	IPAddress    net.IP
	ExpiresAt    time.Time
	LastUsedAt   *time.Time
	RevokedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// DeviceBinding names the portal a remembered device belongs to: a school's
// (TenantID > 0), the Träger-Portal's at one Träger (OrganizationID > 0),
// the parents portal's (ParentPortal) or the caterer portal's (none of
// them). A device of one portal never answers for another (#6955, #7051).
type DeviceBinding struct {
	TenantID       int64
	OrganizationID int64
	ParentPortal   bool
}

// Realm is the one reading of a binding: whose device it is.
func (b DeviceBinding) Realm() DeviceRealm {
	switch {
	case b.TenantID > 0:
		return DeviceOfSchool
	case b.OrganizationID > 0:
		return DeviceOfTraeger
	case b.ParentPortal:
		return DeviceOfParentPortal
	default:
		return DeviceOfAccount
	}
}

// Binding is the portal the device was remembered at.
func (d AccountTrustedDevice) Binding() DeviceBinding {
	return DeviceBinding{TenantID: d.TenantID, OrganizationID: d.OrganizationID, ParentPortal: d.ParentPortal}
}

// AccountMFAOverride is an admin's verdict on one account. TenantID is nil
// for the operator's platform-wide emergency switch and names the school for
// a tenant admin's row. There is no "none" row: the absence of a row is the
// none outcome.
type AccountMFAOverride struct {
	ID        int64
	AccountID int64
	TenantID  *int64
	Override  string
	SetBy     int64
	SetByType string
	Reason    string
}

// AccountLockout is the outcome of one atomic increment of an account's MFA
// failure counter.
type AccountLockout struct {
	Attempts    int
	LockedUntil *time.Time
}

// MFAAdminState is the read-side snapshot the admin "Manage MFA" modal
// needs. Override is "none" when no row exists.
type MFAAdminState struct {
	Enrolled bool
	Override string
}

// VerifiedMFAChallenge is what a redeemed challenge hands the login flow so
// it can mint the token pair without re-decoding the challenge JWT.
type VerifiedMFAChallenge struct {
	MFAResetGeneration int64
	AccountID          int64
	Scope              string
	TenantID           int64
}

// MFAChallengeClaims are the claims of a challenge JWT.
type MFAChallengeClaims struct {
	AccountID   int64
	Scope       string
	TenantID    int64
	ChallengeID int64
	// CredentialGeneration is the credential generation the code was issued
	// under (#6289).
	CredentialGeneration int64
}
