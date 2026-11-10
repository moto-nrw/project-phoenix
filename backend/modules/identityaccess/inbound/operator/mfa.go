package operator

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/portalauth"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// errOperatorMFAServiceUnavailable mirrors the tenant-side guard. Operator
// deployments without an MFAService configured should still respond cleanly.
var errOperatorMFAServiceUnavailable = errors.New("operator mfa service is not configured for this deployment")

// MFAResource exposes the operator-side MFA endpoints. Mirror of the tenant
// auth.Resource handlers — split into its own file/struct so the credential
// login routes of Identity & Access stay focused on the password step.
type MFAResource struct {
	authService OperatorAccess
	mfaService  identityaccess.OperatorMFAFlows
}

// NewMFAResource wires the dependencies. mfaService may be nil — handlers
// will return 503 in that case so deployments that haven't enabled MFA yet
// still answer cleanly.
func NewMFAResource(authSvc OperatorAccess, mfaSvc identityaccess.OperatorMFAFlows) *MFAResource {
	return &MFAResource{authService: authSvc, mfaService: mfaSvc}
}

// requireMFA short-circuits with 503 when no MFAService is wired. Use as the
// first line of every handler.
func (rs *MFAResource) requireMFA(w http.ResponseWriter, r *http.Request) bool {
	return common.RequireDependency(w, r, rs.mfaService != nil, errOperatorMFAServiceUnavailable)
}

// mfaRules classify the operator second factor. The texts stay; the codes
// let the shared code form name the reason (#2517).
var mfaRules = []common.ErrorRule{
	{Target: identityaccess.ErrMFAChallengeTokenInvalid, Render: operatorRejection(http.StatusUnauthorized, common.CodeIdentityMfaCodeInvalid, "Unauthorized")},
	// A resend or enrollment start of a password changed since (#6308).
	{Target: identityaccess.ErrOperatorInvalidCredentials, Render: operatorRejection(http.StatusUnauthorized, common.CodeIdentityInvalidCredentials, "Unauthorized")},
	{Target: identityaccess.ErrMFACodeInvalid, Render: operatorRejection(http.StatusUnauthorized, common.CodeIdentityMfaCodeInvalid, "Unauthorized")},
	{Target: identityaccess.ErrMFALocked, Render: operatorRejection(http.StatusTooManyRequests, common.CodeIdentityMfaBlocked, "MFA temporarily locked")},
	{Target: identityaccess.ErrMFARateLimited, Render: operatorRejection(http.StatusTooManyRequests, common.CodeIdentityMfaBlocked, "Too many MFA emails — try again later")},
	{Target: identityaccess.ErrMFANotEnrolled, Render: operatorRejection(http.StatusForbidden, common.CodeIdentityMfaNotEnrolled, "MFA is not enrolled for this operator")},
	{Target: identityaccess.ErrMFAAlreadyEnrolled, Render: operatorRejection(http.StatusConflict, common.CodeIdentityMfaAlreadyEnrolled, "MFA is already enrolled")},
	{Target: identityaccess.ErrMFAPermissionDenied, Render: func(error) render.Renderer { return common.OperatorForbidden("Permission denied") }},
	{Target: identityaccess.ErrMFAStatusUnavailable, Render: func(error) render.Renderer {
		return common.OperatorServiceUnavailable("MFA ist gerade nicht verfügbar. Bitte versuchen Sie es erneut.")
	}},
}

func mapOperatorMFAError(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, common.RenderWithRules(err, mfaRules, func(error) render.Renderer {
		return common.OperatorInternal("MFA operation failed")
	}))
}

// ----- /auth/mfa/verify -----

// MFAVerifyRequest is the body for POST /operator/auth/mfa/verify. Shared
// with the tenant portal via api/common.
type MFAVerifyRequest = common.MFAVerifyRequest

// Verify exchanges a challenge token + email code for a regular access /
// refresh token pair. When the request carries `remember_device: true` we
// additionally issue a trusted-device cookie.
func (rs *MFAResource) Verify(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &MFAVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}

	proof, err := rs.mfaService.VerifyOperatorMFAChallenge(r.Context(), req.ChallengeToken, req.Code)
	if err != nil {
		mapOperatorMFAError(w, r, err)
		return
	}
	rs.completeMFAExchange(w, r, proof, req.RememberDevice)
}

// ----- /auth/mfa/resend -----

// MFAResendRequest is the body for POST /operator/auth/mfa/resend. Shared
// with the tenant portal via api/common.
type MFAResendRequest = common.MFAResendRequest

// MFAResendResponse carries the renewed challenge token — see api/common.
type MFAResendResponse = common.MFAResendResponse

// Resend re-issues an email code against the existing challenge token.
// Rate-limited inside the service. Returns the renewed challenge JWT.
func (rs *MFAResource) Resend(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &MFAResendRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	renewed, err := rs.mfaService.ResendOperatorMFAChallenge(r.Context(), req.ChallengeToken, common.ParseClientIP(r))
	if err != nil {
		mapOperatorMFAError(w, r, err)
		return
	}
	render.JSON(w, r, MFAResendResponse{ChallengeToken: renewed})
}

// ----- /auth/mfa/enroll/start -----

// EnrollStart mails the code the operator echoes back at
// /auth/mfa/enroll/confirm. The route admits only an operator enrollment
// token, which names the operator and the credential generation its password
// check read: a token from before a password change starts nothing (#6308).
func (rs *MFAResource) EnrollStart(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims, ok := sessiontoken.EnrollmentClaimsFromCtx(r.Context())
	if !ok {
		common.RenderError(w, r, common.OperatorUnauthorized())
		return
	}
	if _, err := rs.mfaService.StartOperatorMFAChallenge(r.Context(), claims.AccountID, common.ParseClientIP(r), &claims.CredentialGeneration); err != nil {
		mapOperatorMFAError(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}

// ----- /auth/mfa/enroll/confirm -----

// MFAEnrollConfirmRequest is the body for POST
// /operator/auth/mfa/enroll/confirm. Shared with the tenant portal via
// api/common.
type MFAEnrollConfirmRequest = common.MFAEnrollConfirmRequest

// EnrollConfirm verifies the just-emailed code, marks the operator as
// enrolled, and mints a full access/refresh token pair so the frontend can
// seed a real operator session. Replaces the original 204 response that
// re-used the pre-MFA access token.
func (rs *MFAResource) EnrollConfirm(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims, ok := sessiontoken.EnrollmentClaimsFromCtx(r.Context())
	if !ok {
		common.RenderError(w, r, common.OperatorUnauthorized())
		return
	}
	req := &MFAEnrollConfirmRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}

	operatorID := claims.AccountID

	if err := rs.mfaService.VerifyOperatorMFACode(r.Context(), operatorID, req.Code, &claims.CredentialGeneration); err != nil {
		mapOperatorMFAError(w, r, err)
		return
	}
	// The enrollment is recorded only while the operator still holds the
	// generation the enrollment token names: a password change committed
	// since the code was redeemed records nothing (#6459).
	proof := identityaccess.OperatorMFAProof{OperatorID: operatorID, CredentialGeneration: claims.CredentialGeneration}
	if err := rs.mfaService.EnrollOperatorMFA(r.Context(), proof); err != nil {
		// Already-enrolled is not fatal — a retried request must still mint
		// a real session (no longer the pre-enrollment token).
		if !errors.Is(err, identityaccess.ErrMFAAlreadyEnrolled) {
			mapOperatorMFAError(w, r, err)
			return
		}
	}

	rs.completeMFAExchange(w, r, proof, req.RememberDevice)
}

// ----- /auth/mfa/trusted-devices -----

// ListTrustedDevices returns the operator's active trusted devices so
// they can be displayed + revoked from the operator settings page.
// Uses the shared TrustedDeviceDTO + mapper from api/common — same
// wire shape as the tenant-side /auth/mfa/trusted-devices endpoint.
func (rs *MFAResource) ListTrustedDevices(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.OperatorUnauthorized())
		return
	}
	devices, err := rs.mfaService.ListOperatorTrustedDevices(r.Context(), int64(claims.ID))
	if err != nil {
		mapOperatorMFAError(w, r, err)
		return
	}
	dtos := common.MapTrustedDevices(devices, func(d identityaccess.OperatorTrustedDevice) common.TrustedDeviceRow {
		return common.TrustedDeviceRow{
			ID:         d.ID,
			UserAgent:  d.UserAgent,
			IPAddress:  d.IPAddress,
			CreatedAt:  d.CreatedAt,
			ExpiresAt:  d.ExpiresAt,
			LastUsedAt: d.LastUsedAt,
		}
	})
	common.Respond(w, r, http.StatusOK, dtos, "trusted devices")
}

// RevokeTrustedDevice removes a single device the operator no longer
// wants to trust. The service double-checks ownership so an attacker
// with a stolen access token can't revoke another operator's devices.
func (rs *MFAResource) RevokeTrustedDevice(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.OperatorUnauthorized())
		return
	}
	idStr := chi.URLParam(r, "deviceId")
	deviceID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || deviceID <= 0 {
		common.RenderError(w, r, common.OperatorInvalidRequest(errors.New("invalid device id")))
		return
	}
	if err := rs.mfaService.RevokeOperatorTrustedDeviceOwned(r.Context(), int64(claims.ID), deviceID); err != nil {
		mapOperatorMFAError(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}

// ----- shared helpers -----

// completeMFAExchange mints the operator session of a proven second factor
// and remembers the device when asked. Operator MFA has no per-school device
// policy, so preparing reads nothing and the device is stored after the mint,
// like on the account portals. Both are bound to the proof's credential
// generation: after a password change the mint refuses and no device is
// stored (#6308).
func (rs *MFAResource) completeMFAExchange(w http.ResponseWriter, r *http.Request, proof identityaccess.OperatorMFAProof, rememberDevice bool) {
	operatorID := proof.OperatorID
	ipAddress := ""
	if clientIP := common.ParseClientIP(r); clientIP != nil {
		ipAddress = clientIP.String()
	}
	prepare := portalauth.RememberDevice(rememberDevice, func(context.Context) (identityaccess.TrustedDeviceIssue, error) {
		return func(ctx context.Context) (string, time.Time, error) {
			return rs.mfaService.IssueOperatorTrustedDevice(ctx, proof, r.UserAgent(), common.ParseClientIP(r))
		}, nil
	})
	tokens, err := portalauth.Exchange(w, r, operatorID, prepare, func(ctx context.Context) (string, string, error) {
		return rs.authService.IssueTokensForAuthenticatedOperator(ctx, operatorID, &proof.CredentialGeneration, ipAddress, r.UserAgent())
	})
	if err != nil {
		common.RenderError(w, r, common.RenderWithRules(err, exchangeRules, func(error) render.Renderer {
			return common.OperatorInternal("Failed to issue tokens")
		}))
		return
	}
	common.Respond(w, r, http.StatusOK, &tokens, "MFA verification successful")
}

// exchangeRules classify a refused mint after a correct second factor: a
// refused credential like the tenant exchange, not an expired session.
var exchangeRules = []common.ErrorRule{
	{Target: identityaccess.ErrOperatorInactive, Render: func(error) render.Renderer { return operatorInactive("Operator account is inactive") }},
	// The password the proof stems from changed meanwhile (#6308).
	{Target: identityaccess.ErrOperatorInvalidCredentials, Render: operatorRejection(http.StatusUnauthorized, common.CodeIdentityInvalidCredentials, "Unauthorized")},
	{Target: identityaccess.ErrOperatorNotFound, Render: operatorRejection(http.StatusUnauthorized, common.CodeIdentityInvalidCredentials, "Unauthorized")},
}
