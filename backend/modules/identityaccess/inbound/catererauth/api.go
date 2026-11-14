// Package catererauth serves the caterer's sign-in and its own second factor
// (ADR 0077, #4497). The caterer portal mounts these routes into its
// /caterer router. They answer through the sign-in tables of portalauth,
// which every account portal shares (#6172).
package catererauth

import (
	"context"

	"github.com/go-chi/chi/v5"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/inbound/portalauth"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
)

// Resource serves the caterer's sign-in and second factor.
type Resource struct {
	signIn       identityaccess.CatererSignIn
	secondFactor identityaccess.CatererSecondFactor
}

// NewResource builds the resource; both flows are required.
func NewResource(signIn identityaccess.CatererSignIn, secondFactor identityaccess.CatererSecondFactor) *Resource {
	if signIn == nil || secondFactor == nil {
		panic("caterer auth HTTP: sign-in and second factor are required")
	}
	return &Resource{signIn: signIn, secondFactor: secondFactor}
}

// MountSignIn mounts the public sign-in routes: the password, the code of
// the second factor and its resend, and the passkey. They check
// credentials, so the caller puts them behind the auth rate limit.
func (rs *Resource) MountSignIn(r chi.Router) {
	r.Post("/auth/login", rs.login)
	r.Post("/auth/mfa/verify", rs.verifyMFA)
	r.Post("/auth/mfa/resend", rs.resendMFA)
	r.Post("/auth/passkeys/login/options", rs.passkeyLoginOptions)
	r.Post("/auth/passkeys/login/verify", rs.passkeyLoginVerify)
}

// MountSession mounts the routes of a caterer session that read or revoke
// its factors. They stay leaves beside the public sign-in routes of the
// same prefix.
func (rs *Resource) MountSession(r chi.Router) {
	r.Get("/account/mfa", rs.mfaStatus)
	r.Get("/auth/mfa/trusted-devices", rs.listTrustedDevices)
	r.Delete("/auth/mfa/trusted-devices/{"+portalauth.TrustedDeviceIDParam+"}", rs.revokeTrustedDevice)
	r.Get("/auth/passkeys", rs.listPasskeys)
	r.Delete("/auth/passkeys/{passkeyId}", rs.revokePasskey)
	r.Post("/auth/passkeys/register/verify", rs.passkeyRegisterVerify)
}

// MountCredentialChecks mounts the session routes that mail a code or
// check one or the current password. The caller puts them behind the auth
// rate limit.
func (rs *Resource) MountCredentialChecks(r chi.Router) {
	r.Post("/account/mfa/start", rs.startMFA)
	r.Post("/account/mfa/confirm", rs.confirmMFA)
	r.Post("/account/mfa/disable", rs.disableMFA)
	r.Post("/auth/passkeys/enrollment/challenge", rs.passkeyEnrollmentChallenge)
	r.Post("/auth/passkeys/register/options", rs.passkeyRegisterOptions)
}

func currentAccountID(ctx context.Context) int64 {
	return int64(sessiontoken.ClaimsFromCtx(ctx).ID)
}
