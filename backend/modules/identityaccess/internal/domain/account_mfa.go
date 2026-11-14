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
// cookie. Portal, TenantID and OrganizationID pin the trust to the scope
// that remembered it (#6955): a cookie issued for school A never bypasses
// MFA at school B, nor one of the tenant portal in moto schule, nor one of
// a Träger at another Träger.
type AccountTrustedDevice struct {
	ID             int64
	AccountID      int64
	Portal         string
	TenantID       int64
	OrganizationID int64
	TokenHash      string
	UserAgent      *string
	IPAddress      net.IP
	ExpiresAt      time.Time
	LastUsedAt     *time.Time
	RevokedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
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

// TrustedDeviceScope is where a remembered device counts (#6955): one portal
// and, for a portal of a school, that school; for the Träger-Portal, its
// Träger. The caterer's devices name neither (ADR 0077 addendum).
type TrustedDeviceScope struct {
	Portal         string
	TenantID       int64
	OrganizationID int64
}

// DeviceScopeOf is the device scope of a portal's sign-in. A portal without
// a school names none, and only the Träger-Portal names its Träger, so a
// stray ID never widens or narrows a scope.
func DeviceScopeOf(portal Portal, tenantID, organizationID int64) TrustedDeviceScope {
	scope := TrustedDeviceScope{Portal: portal.Name}
	if !portal.Schoolless() {
		scope.TenantID = tenantID
	}
	if portal.Org == ShapeRequired {
		scope.OrganizationID = organizationID
	}
	return scope
}

// Valid reports a scope a device may be remembered or found under: a portal
// that remembers devices, with the school or the Träger its shape demands.
func (s TrustedDeviceScope) Valid() bool {
	portal, ok := PortalNamed(s.Portal)
	if !ok || !portal.TrustedDevice || portal.Name == PortalPlatform.Name {
		return false
	}
	if s != DeviceScopeOf(portal, s.TenantID, s.OrganizationID) {
		return false
	}
	return (portal.Schoolless() || s.TenantID > 0) && (portal.Org != ShapeRequired || s.OrganizationID > 0)
}

// SettingsSchool is the school whose trusted-device settings apply: the
// device's school, or none, whose registry defaults then apply.
func (s TrustedDeviceScope) SettingsSchool() int64 { return s.TenantID }
