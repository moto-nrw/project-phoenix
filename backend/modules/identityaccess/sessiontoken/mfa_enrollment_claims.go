package sessiontoken

import "github.com/moto-nrw/project-phoenix/modules/identityaccess/tokenprimitives"

// MFA enrollment scope values mirror the challenge scopes — same conventions,
// different stage in the flow. An enrollment token is issued when MFA is
// required for the tenant/operator role but the account has not yet
// registered a second factor; it must only authorize the /auth/mfa/enroll/*
// surface, never a fully authenticated route.
const (
	MFAEnrollmentScopeTenant   = tokenprimitives.PortalTenant
	MFAEnrollmentScopePlatform = tokenprimitives.PortalPlatform
	// MFAEnrollmentScopeSchool tags enrollment tokens minted by the school
	// portal login (#2207). Only the /school/auth/mfa/enroll/* handlers
	// accept it, and confirming there mints SCHOOL tokens — a school login
	// must never be laundered into a tenant session via the enrollment
	// detour.
	MFAEnrollmentScopeSchool = tokenprimitives.PortalSchool
	// MFAEnrollmentScopeOrg tags enrollment tokens minted by the Träger-Portal
	// login (#4651). Only the /traeger/auth/mfa/enroll/* handlers accept it,
	// and confirming there mints a Träger session.
	MFAEnrollmentScopeOrg = tokenprimitives.PortalOrg
)

// MFAEnrollmentClaims represents a short-lived JWT issued to an account that
// passed the password check but has no MFA credential on a tenant that
// requires one. It is exchanged at /auth/mfa/enroll/confirm (or the operator
// equivalent) for a regular access/refresh token pair only AFTER the user
// successfully enrolls. The token is intentionally narrow:
//
//   - It carries account identity plus tenant/scope so the enroll handlers
//     can dispatch the challenge email and look up the active code.
//   - `mfa_enrollment_pending` is the discriminator middleware uses to
//     reject this token at any other endpoint — the security boundary that
//     closes the "issue full session before MFA is set up" bypass.
type MFAEnrollmentClaims struct {
	// AccountID is auth.accounts.id (tenant scope) or platform.operators.id
	// (platform scope) of the user being enrolled.
	AccountID int64 `json:"account_id"`

	// Scope distinguishes tenant from platform tokens, constrained to the
	// two MFAEnrollmentScope* values.
	Scope string `json:"scope,omitempty"`

	// TenantID is set on tenant-scope tokens so the enroll handlers can
	// wrap the StartChallenge call in the right tenant context. Zero on
	// platform tokens.
	TenantID int64 `json:"tenant_id,omitempty"`

	// MFAEnrollmentPending must be true on every enrollment token —
	// middleware (a) accepts the token only when it is set on the
	// MFA-enrollment authenticator and (b) rejects the token everywhere
	// else as defense-in-depth.
	MFAEnrollmentPending bool `json:"mfa_enrollment_pending"`

	CommonClaims
}

func enrollmentClaimsOf(c tokenprimitives.EnrollmentClaims) MFAEnrollmentClaims {
	return MFAEnrollmentClaims{
		AccountID: c.AccountID, Scope: c.Scope, TenantID: c.TenantID, MFAEnrollmentPending: true,
		CommonClaims: CommonClaims{ExpiresAt: c.ExpiresAt},
	}
}
