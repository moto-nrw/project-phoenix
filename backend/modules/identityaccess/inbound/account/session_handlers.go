package account

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/rotation"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

const headerUserAgent = "User-Agent"

// login handles user login. The handler is a thin orchestrator: it pulls
// the trusted-device cookie off the request, calls LoginWithMFAGate, and
// translates the discriminated LoginResult into a JSON shape the frontend
// can branch on.
func (rs *Resource) login(w http.ResponseWriter, r *http.Request) {
	req := &LoginRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	ipAddress := getClientIP(r)
	userAgent := r.Header.Get(headerUserAgent)

	// Pull the trusted-device cookie if the browser sent one. The MFA gate
	// uses it to skip the second factor for users on a previously-marked
	// device. Empty / missing cookie is fine — the service is nil-tolerant.
	var trustedDeviceCookie string
	if c, err := r.Cookie(trustedDeviceCookieName); err == nil {
		trustedDeviceCookie = c.Value
	}

	if rs.Sessions == nil {
		common.RenderError(w, r, common.ErrorServiceUnavailable(errors.New("login unavailable")))
		return
	}
	result, err := rs.Sessions.LoginWithMFAGate(
		r.Context(), req.Email, req.Password, ipAddress, userAgent, req.TenantSlug, trustedDeviceCookie,
	)
	if err != nil {
		rs.handleLoginError(w, r, err)
		return
	}

	switch result.Status {
	case identityaccess.LoginStatusMFARequired:
		tde := result.TrustedDeviceEnabled
		tdd := result.TrustedDeviceDays
		render.JSON(w, r, LoginResponse{
			Status:               string(identityaccess.LoginStatusMFARequired),
			ChallengeToken:       result.ChallengeToken,
			MaskedEmail:          result.MaskedEmail,
			TrustedDeviceEnabled: &tde,
			TrustedDeviceDays:    &tdd,
		})
		return
	case identityaccess.LoginStatusMFAEnrollmentRequired:
		render.JSON(w, r, LoginResponse{
			Status:                string(identityaccess.LoginStatusMFAEnrollmentRequired),
			AccessToken:           result.AccessToken,
			MaskedEmail:           result.MaskedEmail,
			MFAEnrollmentRequired: true,
		})
		return
	}

	render.JSON(w, r, LoginResponse{
		Status:       string(identityaccess.LoginStatusAuthenticated),
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	})
}

// handleLoginError centralises the error-to-HTTP mapping for /auth/login.
// The Identity & Access flow reports which operation failed and carries the
// public sentinel; the MFA sentinels still come from the retained MFA
// service through the gate.
func (rs *Resource) handleLoginError(w http.ResponseWriter, r *http.Request, err error) {
	var authErr *identityaccess.AuthenticationError
	if errors.As(err, &authErr) {
		switch {
		case errors.Is(err, identityaccess.ErrInvalidCredentials):
			common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrInvalidCredentials))
		case errors.Is(err, identityaccess.ErrAccountNotFound):
			// Mask the specific error so attackers can't enumerate accounts.
			common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrInvalidCredentials))
		case errors.Is(err, identityaccess.ErrAccountInactive):
			// The stable code the parents and school portals already send
			// (#3376). Only reachable once the credential check accepted the
			// password, so it tells a caller nothing about an account they do
			// not own — and without it the frontend renders a deactivated
			// account as "wrong password" and sends the owner into a reset
			// loop that cannot help.
			common.RenderError(w, r, common.ErrorUnauthorizedWithCode(
				identityaccess.ErrAccountInactive, "account_inactive"))
		case errors.Is(err, identityaccess.ErrTenantNotFound):
			common.RenderError(w, r, common.ErrorNotFound(identityaccess.ErrTenantNotFound))
		case errors.Is(err, identityaccess.ErrTenantAccessDenied):
			common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrTenantAccessDenied))
		case errors.Is(err, identityaccess.ErrParentMustUseParentPortal):
			// Guardian-only account at the staff login. The code is what the
			// frontend switches on to point the user at the parents portal —
			// matching on the English message text would be brittle. Safe to
			// be specific: this branch is only reachable once the credential
			// check has already accepted the password, so it tells the caller
			// nothing about an account they don't own.
			common.RenderError(w, r, common.ErrorForbiddenWithCode(
				identityaccess.ErrParentMustUseParentPortal, "use_parent_portal"))
		case errors.Is(err, identityaccess.ErrMustUseSchoolPortal):
			// School-portal-only account at the staff login (#2207). Same
			// shape as the guardian split above: a stable code the frontend
			// switches on to point the user at moto schule.
			common.RenderError(w, r, common.ErrorForbiddenWithCode(
				identityaccess.ErrMustUseSchoolPortal, "use_school_portal"))
		case errors.Is(err, identityaccess.ErrMFARateLimited):
			// MFA challenge initiation tripped the 3/15min sliding-window
			// cap. Surface as 429 so the frontend shows the dedicated "too
			// many code requests" message instead of a generic 5xx.
			common.RenderError(w, r, common.ErrorTooManyRequests(authErr.Err))
		case errors.Is(err, identityaccess.ErrMFALocked):
			// Account hit the failed-attempt lockout threshold while we were
			// preparing the next challenge. Same HTTP status as rate limit,
			// distinct message body — handled separately on the frontend.
			common.RenderError(w, r, common.ErrorTooManyRequests(authErr.Err))
		case errors.Is(err, identityaccess.ErrMFAStatusUnavailable):
			// MFA gate couldn't determine required/enrolled status (settings
			// or credentials lookup failed with a non-not-found error).
			// Refuse this login rather than fail-open. 503 lets the client
			// retry — the frontend renders it as "Bitte versuche es erneut".
			common.RenderError(w, r, common.ErrorServiceUnavailable(authErr.Err))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	common.RenderError(w, r, common.ErrorInternalServer(err))
}

// register handles user registration
func (rs *Resource) register(w http.ResponseWriter, r *http.Request) {
	req := &RegisterRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Authorize role assignment (if role_id specified)
	roleID, callerTenantID, shouldReturn := rs.authorizeRoleAssignment(w, r, req.RoleID)
	if shouldReturn {
		return
	}

	provisioned, err := rs.Sessions.RegisterSchoolAccount(r.Context(), identityaccess.SchoolAccountRegistration{
		TenantID: callerTenantID, Email: req.Email, Username: req.Username, Password: req.Password,
		RoleID:   roleID,
		Identity: schoolIdentityFrom(req.FirstName, req.LastName, req.TagID),
	})
	if err != nil {
		rs.markProvisioningRollback(r)
		rs.handleRegistrationError(w, r, err)
		return
	}

	resp := buildAccountResponse(provisioned.Account)
	resp.SchoolIdentity = buildSchoolIdentityResponse(provisioned.Identity)
	common.Respond(w, r, http.StatusCreated, resp, "Account registered successfully")
}

// schoolIdentityFrom packages the request's identity fields for provisioning.
//
// Always non-nil, and deliberately without a name check: whether a name is
// needed is not a question the HTTP layer can answer. An account that already
// carries a person at this school is completed without one — reusing that
// person is the whole point of the link endpoint, and it is the only person the
// partial unique index on (tenant_id, account_id) allows. Only creating a
// person needs a name, and there the provisioning itself refuses with
// ErrSchoolIdentityNamesRequired, which both handlers render as 400. Deciding
// it here refused the reuse case for a name it never needed.
//
// Guardian-tier roles provision nothing; EnsureSchoolIdentity returns early for
// them whatever this carries.
func schoolIdentityFrom(firstName, lastName string, tagID *string) *identityaccess.SchoolAccountIdentity {
	return &identityaccess.SchoolAccountIdentity{
		FirstName: strings.TrimSpace(firstName),
		LastName:  strings.TrimSpace(lastName),
		TagID:     tagID,
	}
}

// markProvisioningRollback tells the tenant middleware to roll back after a
// refused account provisioning.
//
// The account, its school mapping and its role are written before the identity
// step runs, all in the request's tenant transaction — which TenantTxMiddleware
// commits for every response below 500. Without this a 400 leaves behind
// exactly the half-written account #2222 is about: school access and a role,
// no person, no staff record. The rollback marker is the established way to
// refuse a request that has already touched the database (see
// tenant.MarkRollback).
func (rs *Resource) markProvisioningRollback(r *http.Request) {
	rs.markRollback(r.Context())
}

// linkToTenant links an existing account to the caller's tenant.
// Requires admin authentication with a valid tenant context.
func (rs *Resource) linkToTenant(w http.ResponseWriter, r *http.Request) {
	req := &LinkToTenantRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Require admin auth and resolve role + tenant from JWT
	roleID, callerTenantID, shouldReturn := rs.authorizeRoleAssignment(w, r, req.RoleID)
	if shouldReturn {
		return
	}

	provisioned, err := rs.Sessions.LinkSchoolAccount(r.Context(), identityaccess.SchoolAccountLink{
		TenantID: callerTenantID, Email: req.Email, RoleID: roleID,
		Identity: schoolIdentityFrom(req.FirstName, req.LastName, req.TagID),
	})
	if err != nil {
		rs.markProvisioningRollback(r)

		var authErr *identityaccess.AuthenticationError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, identityaccess.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorNotFound(authErr.Err))
			case errors.Is(authErr.Err, identityaccess.ErrAccountInactive):
				common.RenderError(w, r, common.ErrorConflict(authErr.Err))
			case errors.Is(authErr.Err, identityaccess.ErrRoleNotAssignable),
				errors.Is(authErr.Err, identityaccess.ErrRoleForeignTenant),
				errors.Is(authErr.Err, identityaccess.ErrRoleGuardianNotAssignable),
				errors.Is(authErr.Err, identityaccess.ErrRoleLegacyTeacherNotAssignable),
				// Linking an existing account can meet an identity it already has
				// at this school: a caregiver profile the Lehrkraft role must not
				// be put on top of (#1772), or a transponder that is not this
				// school's / not free.
				errors.Is(authErr.Err, identityaccess.ErrRoleLehrkraftCaregiverProfile),
				identityaccess.IsSchoolIdentityRequestError(authErr.Err):
				common.RenderError(w, r, common.ErrorInvalidRequest(authErr.Err))
			default:
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Return ONLY id, email and what was provisioned at THIS school — never leak
	// roles, username, or active status from other tenants
	common.Respond(w, r, http.StatusOK, map[string]any{
		"id":              provisioned.Account.ID,
		"email":           provisioned.Account.Email,
		"school_identity": buildSchoolIdentityResponse(provisioned.Identity),
	}, "Account linked to tenant successfully")
}

// authorizeRoleAssignment validates the role_id from the request and returns it
// along with the caller's tenant ID. Auth and permission checks are handled by
// middleware (Authenticator + TenantMiddleware + RequiresPermission). Returns
// the role ID, the tenant ID, and whether the handler should return early
// (true = error rendered).
func (rs *Resource) authorizeRoleAssignment(w http.ResponseWriter, r *http.Request, requestedRoleID *int64) (*int64, int64, bool) {
	claims := jwt.ClaimsFromCtx(r.Context())

	if requestedRoleID == nil || *requestedRoleID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("role_id is required when creating accounts")))
		return nil, 0, true
	}

	if claims.TenantID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			identityaccess.ErrTenantRequiredForRoleAssignment))
		return nil, 0, true
	}

	// Exists, belongs to this school, and is not reserved for another flow —
	// the same policy the staff invitation uses, so guardian and retired roles
	// cannot be handed out through the back door of account creation.
	// TenantTxMiddleware ensures RLS sees tenant-scoped roles.
	role, err := rs.Sessions.ResolveAssignableSchoolRole(r.Context(), *requestedRoleID, claims.TenantID)
	if err != nil {
		renderRoleAssignmentError(w, r, *requestedRoleID, err)
		return nil, 0, true
	}

	// Assignable is not the same as assignable *by this caller*: creating an
	// account hands out a role, and no caller may hand out more than they hold.
	if !rs.canGrantRole(role, claims.Permissions) {
		slog.Default().Warn("role grant denied",
			"role_id", *requestedRoleID,
			"account_id", claims.ID,
			"tenant_id", claims.TenantID,
		)
		common.RenderError(w, r, common.ErrorForbidden(identityaccess.ErrRoleGrantNotPermitted))
		return nil, 0, true
	}

	return requestedRoleID, claims.TenantID, false
}

// renderRoleAssignmentError maps a role resolution failure to a response. The
// policy sentinels are all caller mistakes (400); anything else means the lookup
// itself failed and must not be reported as a bad role.
func renderRoleAssignmentError(w http.ResponseWriter, r *http.Request, roleID int64, err error) {
	switch {
	case errors.Is(err, identityaccess.ErrRoleNotAssignable),
		errors.Is(err, identityaccess.ErrRecordMissing):
		common.RenderError(w, r, common.ErrorInvalidRequest(identityaccess.ErrRoleNotAssignable))
	case errors.Is(err, identityaccess.ErrRoleForeignTenant),
		errors.Is(err, identityaccess.ErrRoleGuardianNotAssignable),
		errors.Is(err, identityaccess.ErrRoleLegacyTeacherNotAssignable):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		slog.Default().Error("role lookup failed",
			"role_id", roleID,
			"error", err,
		)
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to verify role", err))
	}
}

// handleRegistrationError handles authentication errors during registration
func (rs *Resource) handleRegistrationError(w http.ResponseWriter, r *http.Request, err error) {
	var authErr *identityaccess.AuthenticationError
	if !errors.As(err, &authErr) {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	switch {
	case errors.Is(authErr.Err, identityaccess.ErrEmailAlreadyExists):
		common.RenderError(w, r, common.ErrorInvalidRequest(identityaccess.ErrEmailAlreadyExists))
	case errors.Is(authErr.Err, identityaccess.ErrUsernameAlreadyExists):
		common.RenderError(w, r, common.ErrorInvalidRequest(identityaccess.ErrUsernameAlreadyExists))
	case errors.Is(authErr.Err, identityaccess.ErrPasswordTooWeak):
		common.RenderError(w, r, common.ErrorInvalidRequest(identityaccess.ErrPasswordTooWeak))
	case errors.Is(authErr.Err, identityaccess.ErrTenantRequiredForRoleAssignment):
		common.RenderError(w, r, common.ErrorInvalidRequest(identityaccess.ErrTenantRequiredForRoleAssignment))
	case errors.Is(authErr.Err, identityaccess.ErrRoleNotAssignable),
		errors.Is(authErr.Err, identityaccess.ErrRoleForeignTenant),
		errors.Is(authErr.Err, identityaccess.ErrRoleGuardianNotAssignable),
		errors.Is(authErr.Err, identityaccess.ErrRoleLegacyTeacherNotAssignable),
		// Everything provisioning refuses on the request's own terms: a nameless
		// staff-tier request (schoolIdentityFor already catches it, so this is
		// defense in depth), an account linked to a child's person record, and a
		// transponder that is unknown here or already taken by this person.
		identityaccess.IsSchoolIdentityRequestError(authErr.Err):
		common.RenderError(w, r, common.ErrorInvalidRequest(authErr.Err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// buildAccountResponse constructs an AccountResponse from a registered
// account. A newly created account holds no role assignment the read would
// see yet, so the role list stays empty, as it was.
func buildAccountResponse(account identityaccess.RegisteredAccount) *AccountResponse {
	return &AccountResponse{
		ID:       account.ID,
		Email:    account.Email,
		Username: account.Username,
		Active:   account.Active,
		Roles:    []string{},
	}
}

// buildSchoolIdentityResponse exposes the provisioned ids, or nil when nothing
// was provisioned.
func buildSchoolIdentityResponse(identity *identityaccess.SchoolIdentity) *SchoolIdentityResponse {
	if identity == nil || identity.PersonID == 0 || identity.StaffID == 0 {
		return nil
	}
	return &SchoolIdentityResponse{
		PersonID:  identity.PersonID,
		StaffID:   identity.StaffID,
		TeacherID: identity.TeacherID,
	}
}

// refreshToken handles token refresh
func (rs *Resource) refreshToken(w http.ResponseWriter, r *http.Request) {
	// Get refresh token from context
	refreshToken := jwt.RefreshTokenFromCtx(r.Context())

	// Get IP address and user agent for audit logging
	ipAddress := getClientIP(r)
	userAgent := r.Header.Get(headerUserAgent)

	if rs.Sessions == nil {
		common.RenderError(w, r, common.ErrorServiceUnavailable(errors.New("session refresh unavailable")))
		return
	}
	ctx := rotation.WithRecoveryProof(r.Context(), r.Header.Get(rotation.RecoveryProofHeader))
	accessToken, newRefreshToken, err := rs.Sessions.RefreshTokenWithAudit(ctx, refreshToken, ipAddress, userAgent)
	if err != nil {
		var authErr *identityaccess.AuthenticationError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(err, identityaccess.ErrInvalidToken):
				common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrInvalidToken))
			case errors.Is(err, identityaccess.ErrTokenExpired):
				common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrTokenExpired))
			case errors.Is(err, identityaccess.ErrTokenNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrTokenNotFound))
			case errors.Is(err, identityaccess.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrAccountNotFound))
			case errors.Is(err, identityaccess.ErrAccountInactive):
				common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrAccountInactive))
			case errors.Is(err, identityaccess.ErrTenantNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrTenantNotFound))
			case errors.Is(err, identityaccess.ErrTenantAccessDenied):
				common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrTenantAccessDenied))
			case errors.Is(err, identityaccess.ErrMustUseSchoolPortal):
				// The account is school-portal-only at this school (#2207).
				// 401, not 403: the tenant session is simply over, and the
				// frontend's refresh path turns a 401 into a clean logout.
				common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrMustUseSchoolPortal))
			default:
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Special case for token refresh endpoint - frontend expects direct token response
	render.JSON(w, r, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	})
}

// logout handles user logout
func (rs *Resource) logout(w http.ResponseWriter, r *http.Request) {
	// Get refresh token from context
	refreshToken := jwt.RefreshTokenFromCtx(r.Context())

	// Get IP address and user agent for audit logging
	ipAddress := getClientIP(r)
	userAgent := r.Header.Get(headerUserAgent)

	if rs.Sessions == nil {
		common.RenderError(w, r, common.ErrorServiceUnavailable(errors.New("logout unavailable")))
		return
	}
	err := rs.Sessions.LogoutWithAudit(r.Context(), refreshToken, ipAddress, userAgent)
	if err != nil {
		// Even if there's an error, we want to consider the logout successful from the client's perspective
		// Log the error on the server side for debugging
		slog.Default().WarnContext(r.Context(), "Logout audit logging failed (client logout still successful)",
			slog.String("ip", ipAddress),
			slog.String("error", err.Error()),
		)
	}

	common.RespondNoContent(w, r)
}

// changePassword handles password change
func (rs *Resource) changePassword(w http.ResponseWriter, r *http.Request) {
	req := &ChangePasswordRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Get user ID from JWT claims
	claims := jwt.ClaimsFromCtx(r.Context())

	err := rs.Sessions.ChangeAccountPassword(r.Context(), int64(claims.ID), req.CurrentPassword, req.NewPassword)
	if err != nil {
		var authErr *identityaccess.AuthenticationError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, identityaccess.ErrInvalidCredentials):
				common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrInvalidCredentials))
			case errors.Is(authErr.Err, identityaccess.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrAccountNotFound))
			case errors.Is(authErr.Err, identityaccess.ErrPasswordTooWeak):
				common.RenderError(w, r, common.ErrorInvalidRequest(identityaccess.ErrPasswordTooWeak))
			default:
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// getClientIP extracts the real client IP address from the request
func getClientIP(r *http.Request) string {
	return common.GetClientIPString(r)
}

// canGrantRole asks Security Runtime whether a caller holding permissions may
// hand out role. Without the policy nothing may be granted.
func (rs *Resource) canGrantRole(role *identityaccess.AssignableSchoolRole, permissions []string) bool {
	if rs.RoleGrants == nil || role == nil {
		return false
	}
	facts := identityaccess.RoleFacts{
		ID:       role.ID,
		TenantID: role.TenantID,
		Name:     role.Name,
		IsSystem: role.IsSystem,
		BaseRole: role.BaseRole,
	}
	return rs.RoleGrants.CanGrantRole(facts, permissions, append([]string(nil), role.Permissions...))
}
