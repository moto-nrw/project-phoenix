package account

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/portalauth"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

var errMFAServiceUnavailable = errors.New("mfa service is not configured for this deployment")

func (rs *Resource) requireMFA(w http.ResponseWriter, r *http.Request) bool {
	return common.RequireDependency(w, r, rs.MFAService != nil, errMFAServiceUnavailable)
}

func mapMFAError(w http.ResponseWriter, r *http.Request, err error) {
	portalauth.Render(w, r, err, portalauth.MFARules)
}

// ----- /mfa/verify -----

// MFAVerifyRequest is the body for POST /auth/mfa/verify. Shared with the
// operator portal via api/common.
type MFAVerifyRequest = common.MFAVerifyRequest

// mfaVerify exchanges a challenge token + email code for a regular access /
// refresh token pair. When the request carries `remember_device: true` the
// service additionally issues a trusted-device cookie.
func (rs *Resource) mfaVerify(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &MFAVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	verified, err := rs.MFAService.VerifyMFAChallenge(r.Context(), req.ChallengeToken, req.Code)
	if err != nil {
		mapMFAError(w, r, err)
		return
	}

	rs.completeMFAExchange(w, r, verified, req.RememberDevice)
}

// ----- /mfa/resend -----

// MFAResendRequest is the body for POST /auth/mfa/resend. Shared with the
// operator portal via api/common.
type MFAResendRequest = common.MFAResendRequest

// MFAResendResponse carries the renewed challenge token — see api/common.
type MFAResendResponse = common.MFAResendResponse

// mfaResend re-issues an email code against the existing challenge token.
// Rate-limited inside the service. Returns the renewed challenge JWT —
// see MFAResendResponse for why the previous 204-shape was unsafe.
//
// Scope-enforcing (mirrors the school endpoint): a challenge of another
// portal is refused, so this surface cannot burn that account's
// 3-codes-per-15-minutes budget from outside its own portal.
func (rs *Resource) mfaResend(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &MFAResendRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	renewed, err := rs.MFAService.ResendMFAChallengeForScope(r.Context(), req.ChallengeToken, common.ParseClientIP(r), identityaccess.MFAChallengeScopeTenant)
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	render.JSON(w, r, MFAResendResponse{ChallengeToken: renewed})
}

// ----- /mfa/enroll/start -----

// mfaEnrollStart mails the code the account echoes back at
// /mfa/enroll/confirm. The route admits only a tenant enrollment token.
func (rs *Resource) mfaEnrollStart(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims, ok := sessiontoken.EnrollmentClaimsFromCtx(r.Context())
	if !ok {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	_, err := rs.MFAService.StartMFAChallenge(r.Context(), claims.AccountID, claims.TenantID, identityaccess.MFAChallengeScopeTenant, common.ParseClientIP(r))
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----- /mfa/enroll/confirm -----

// MFAEnrollConfirmRequest is the body for POST /auth/mfa/enroll/confirm.
// Shared with the operator portal via api/common.
type MFAEnrollConfirmRequest = common.MFAEnrollConfirmRequest

// mfaEnrollConfirm verifies the mailed code, records the factor and mints
// the tenant session.
func (rs *Resource) mfaEnrollConfirm(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims, ok := sessiontoken.EnrollmentClaimsFromCtx(r.Context())
	if !ok {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	req := &MFAEnrollConfirmRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	// The enrollment route runs outside the tenant transaction; the claims'
	// school carries the MFA audit events into audit.auth_events.
	ctx := rs.withTenantID(r.Context(), claims.TenantID)
	// The confirm names no challenge, so the newest code of this portal and
	// school answers: a school-portal code never mints a tenant session.
	verified, err := rs.MFAService.VerifyMFACodeForAccount(ctx, claims.AccountID, claims.TenantID, req.Code, identityaccess.MFAChallengeScopeTenant)
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	// A retried confirm must still produce a session.
	if err := rs.MFAService.EnrollMFA(ctx, claims.AccountID); err != nil && !errors.Is(err, identityaccess.ErrMFAAlreadyEnrolled) {
		mapMFAError(w, r, err)
		return
	}
	rs.completeMFAExchange(w, r, verified, req.RememberDevice)
}

// ----- /mfa/trusted-devices -----

// mfaListTrustedDevices returns the calling user's active trusted-device
// records so they can see and revoke them from the admin Sicherheit tab.
// Filtered to the authenticated account — ownership is enforced in the
// service layer too, but scoping the query here keeps the data path tight.
//
// The DTO + mapping helper live in api/common so the operator-side
// /operator/auth/mfa/trusted-devices endpoint can reuse them.
func (rs *Resource) mfaListTrustedDevices(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	// Trust is per-(account, tenant) — pass the tenant from claims so the
	// settings page never leaks devices trusted in another tenant. The
	// route runs inside TenantTxMiddleware, but threading the value
	// explicitly avoids relying on context state at the handler boundary.
	devices, err := rs.MFAService.ListTrustedDevices(r.Context(), int64(claims.ID), claims.TenantID)
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	render.JSON(w, r, common.MapTrustedDevices(devices, func(d identityaccess.AccountTrustedDevice) common.TrustedDeviceRow {
		return common.TrustedDeviceRow{
			ID:         d.ID,
			UserAgent:  d.UserAgent,
			IPAddress:  d.IPAddress,
			CreatedAt:  d.CreatedAt,
			ExpiresAt:  d.ExpiresAt,
			LastUsedAt: d.LastUsedAt,
		}
	}))
}

// mfaRevokeTrustedDevice deletes one trusted device the user has
// previously remembered. The service double-checks ownership so an
// attacker with an access token for account A can't revoke device rows
// belonging to account B via id-guessing.
func (rs *Resource) mfaRevokeTrustedDevice(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	idStr := chi.URLParam(r, "deviceId")
	deviceID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || deviceID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid device id")))
		return
	}
	// Tenant scope is enforced at the service layer too: a user with an
	// access token for (account A, tenant T1) can't revoke a device that
	// was trusted in tenant T2.
	if err := rs.MFAService.RevokeTrustedDevice(r.Context(), int64(claims.ID), claims.TenantID, deviceID); err != nil {
		mapMFAError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// completeMFAExchange mints the tenant session of a proven second factor
// and remembers the device when asked.
func (rs *Resource) completeMFAExchange(w http.ResponseWriter, r *http.Request, proof identityaccess.VerifiedMFAChallenge, rememberDevice bool) {
	if !common.RequireDependency(w, r, rs.Sessions != nil, errSessionsUnavailable) {
		return
	}
	client := portalauth.ClientOf(r)
	prepare := portalauth.RememberDevice(rememberDevice, func(ctx context.Context) (identityaccess.TrustedDeviceIssue, error) {
		return rs.MFAService.PrepareTrustedDevice(ctx, proof.AccountID, proof.TenantID, client.UserAgent, common.ParseClientIP(r))
	})
	tokens, err := portalauth.Exchange(w, r, proof.AccountID, prepare, func(ctx context.Context) (string, string, error) {
		return rs.Sessions.IssueTokensForVerifiedMFA(ctx, proof, client.IP, client.UserAgent)
	})
	if err != nil {
		portalauth.Render(w, r, err, portalauth.ExchangeRules)
		return
	}
	render.JSON(w, r, tokens)
}
