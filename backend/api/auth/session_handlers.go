package auth

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/rotation"
	"github.com/moto-nrw/project-phoenix/internal/clientip"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
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

	result, err := rs.AuthService.LoginWithMFAGate(
		r.Context(), req.Email, req.Password, ipAddress, userAgent, req.TenantSlug, trustedDeviceCookie,
	)
	if err != nil {
		rs.handleLoginError(w, r, err)
		return
	}

	switch result.Status {
	case authService.LoginStatusMFARequired:
		tde := result.TrustedDeviceEnabled
		tdd := result.TrustedDeviceDays
		render.JSON(w, r, LoginResponse{
			Status:               string(authService.LoginStatusMFARequired),
			ChallengeToken:       result.ChallengeToken,
			MaskedEmail:          result.MaskedEmail,
			TrustedDeviceEnabled: &tde,
			TrustedDeviceDays:    &tdd,
		})
		return
	case authService.LoginStatusMFAEnrollmentRequired:
		render.JSON(w, r, LoginResponse{
			Status:                string(authService.LoginStatusMFAEnrollmentRequired),
			AccessToken:           result.AccessToken,
			MaskedEmail:           result.MaskedEmail,
			MFAEnrollmentRequired: true,
		})
		return
	}

	render.JSON(w, r, LoginResponse{
		Status:       string(authService.LoginStatusAuthenticated),
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	})
}

// handleLoginError centralises the error-to-HTTP mapping for /auth/login so
// both the legacy and the MFA-aware paths agree on what comes back.
func (rs *Resource) handleLoginError(w http.ResponseWriter, r *http.Request, err error) {
	var authErr *authService.AuthError
	if errors.As(err, &authErr) {
		switch {
		case errors.Is(authErr.Err, authService.ErrInvalidCredentials):
			common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrInvalidCredentials))
		case errors.Is(authErr.Err, authService.ErrAccountNotFound):
			// Mask the specific error so attackers can't enumerate accounts.
			common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrInvalidCredentials))
		case errors.Is(authErr.Err, authService.ErrAccountInactive):
			common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountInactive))
		case errors.Is(authErr.Err, authService.ErrTenantNotFound):
			common.RenderError(w, r, common.ErrorNotFound(authService.ErrTenantNotFound))
		case errors.Is(authErr.Err, authService.ErrTenantAccessDenied):
			common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTenantAccessDenied))
		case errors.Is(authErr.Err, authService.ErrParentMustUseParentPortal):
			// Guardian-only account at the staff login. The code is what the
			// frontend switches on to point the user at the parents portal —
			// matching on the English message text would be brittle. Safe to
			// be specific: this branch is only reachable once
			// validateLoginCredentials has already accepted the password, so
			// it tells the caller nothing about an account they don't own.
			common.RenderError(w, r, common.ErrorForbiddenWithCode(
				authService.ErrParentMustUseParentPortal, "use_parent_portal"))
		case errors.Is(authErr.Err, authService.ErrMustUseSchoolPortal):
			// School-portal-only account at the staff login (#2207). Same
			// shape as the guardian split above: a stable code the frontend
			// switches on to point the user at moto schule.
			common.RenderError(w, r, common.ErrorForbiddenWithCode(
				authService.ErrMustUseSchoolPortal, "use_school_portal"))
		case errors.Is(authErr.Err, authService.ErrMFARateLimited):
			// MFA challenge initiation tripped the 3/15min sliding-window
			// cap. Surface as 429 so the frontend shows the dedicated "too
			// many code requests" message instead of a generic 5xx.
			common.RenderError(w, r, common.ErrorTooManyRequests(authErr.Err))
		case errors.Is(authErr.Err, authService.ErrMFALocked):
			// Account hit the failed-attempt lockout threshold while we were
			// preparing the next challenge. Same HTTP status as rate limit,
			// distinct message body — handled separately on the frontend.
			common.RenderError(w, r, common.ErrorTooManyRequests(authErr.Err))
		case errors.Is(authErr.Err, authService.ErrMFAStatusUnavailable):
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

	account, schoolIdentity, err := rs.AuthService.RegisterSchoolAccount(
		r.Context(), req.Email, req.Username, req.Password, roleID, callerTenantID,
		schoolIdentityFrom(req.FirstName, req.LastName, req.TagID))
	if err != nil {
		markProvisioningRollback(r)
		rs.handleRegistrationError(w, r, err)
		return
	}

	resp := buildAccountResponse(account)
	resp.SchoolIdentity = buildSchoolIdentityResponse(schoolIdentity)
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
func schoolIdentityFrom(firstName, lastName string, tagID *string) *authService.SchoolAccountIdentity {
	return &authService.SchoolAccountIdentity{
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
func markProvisioningRollback(r *http.Request) {
	tenant.MarkRollback(r.Context())
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

	account, schoolIdentity, err := rs.AuthService.LinkSchoolAccount(
		r.Context(), req.Email, roleID, callerTenantID,
		schoolIdentityFrom(req.FirstName, req.LastName, req.TagID))
	if err != nil {
		markProvisioningRollback(r)

		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorNotFound(authErr.Err))
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				common.RenderError(w, r, common.ErrorConflict(authErr.Err))
			case errors.Is(authErr.Err, authService.ErrRoleNotAssignable),
				errors.Is(authErr.Err, authService.ErrRoleForeignTenant),
				errors.Is(authErr.Err, authService.ErrRoleGuardianNotAssignable),
				errors.Is(authErr.Err, authService.ErrRoleLegacyTeacherNotAssignable),
				// Linking an existing account can meet an identity it already has
				// at this school: a caregiver profile the Lehrkraft role must not
				// be put on top of (#1772), or a transponder that is not this
				// school's / not free.
				errors.Is(authErr.Err, authService.ErrRoleLehrkraftCaregiverProfile),
				authService.IsSchoolIdentityRequestError(authErr.Err):
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
		"id":              account.ID,
		"email":           account.Email,
		"school_identity": buildSchoolIdentityResponse(schoolIdentity),
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
			authService.ErrTenantRequiredForRoleAssignment))
		return nil, 0, true
	}

	// Exists, belongs to this school, and is not reserved for another flow —
	// the same policy the staff invitation uses, so guardian and retired roles
	// cannot be handed out through the back door of account creation.
	// TenantTxMiddleware ensures RLS sees tenant-scoped roles.
	role, err := rs.AuthService.ResolveAssignableSchoolRole(r.Context(), *requestedRoleID, claims.TenantID)
	if err != nil {
		renderRoleAssignmentError(w, r, *requestedRoleID, err)
		return nil, 0, true
	}

	// Assignable is not the same as assignable *by this caller*: creating an
	// account hands out a role, and no caller may hand out more than they hold.
	if !authorize.CanGrantRole(role, claims.Permissions) {
		slog.Default().Warn("role grant denied",
			"role_id", *requestedRoleID,
			"account_id", claims.ID,
			"tenant_id", claims.TenantID,
		)
		common.RenderError(w, r, common.ErrorForbidden(authService.ErrRoleGrantNotPermitted))
		return nil, 0, true
	}

	return requestedRoleID, claims.TenantID, false
}

// renderRoleAssignmentError maps a role resolution failure to a response. The
// policy sentinels are all caller mistakes (400); anything else means the lookup
// itself failed and must not be reported as a bad role.
func renderRoleAssignmentError(w http.ResponseWriter, r *http.Request, roleID int64, err error) {
	switch {
	case errors.Is(err, authService.ErrRoleNotAssignable),
		errors.Is(err, sql.ErrNoRows):
		common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrRoleNotAssignable))
	case errors.Is(err, authService.ErrRoleForeignTenant),
		errors.Is(err, authService.ErrRoleGuardianNotAssignable),
		errors.Is(err, authService.ErrRoleLegacyTeacherNotAssignable):
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
	var authErr *authService.AuthError
	if !errors.As(err, &authErr) {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	switch {
	case errors.Is(authErr.Err, authService.ErrEmailAlreadyExists):
		common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrEmailAlreadyExists))
	case errors.Is(authErr.Err, authService.ErrUsernameAlreadyExists):
		common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrUsernameAlreadyExists))
	case errors.Is(authErr.Err, authService.ErrPasswordTooWeak):
		common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrPasswordTooWeak))
	case errors.Is(authErr.Err, authService.ErrTenantRequiredForRoleAssignment):
		common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrTenantRequiredForRoleAssignment))
	case errors.Is(authErr.Err, authService.ErrRoleNotAssignable),
		errors.Is(authErr.Err, authService.ErrRoleForeignTenant),
		errors.Is(authErr.Err, authService.ErrRoleGuardianNotAssignable),
		errors.Is(authErr.Err, authService.ErrRoleLegacyTeacherNotAssignable),
		// Everything provisioning refuses on the request's own terms: a nameless
		// staff-tier request (schoolIdentityFor already catches it, so this is
		// defense in depth), an account linked to a child's person record, and a
		// transponder that is unknown here or already taken by this person.
		authService.IsSchoolIdentityRequestError(authErr.Err):
		common.RenderError(w, r, common.ErrorInvalidRequest(authErr.Err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// buildAccountResponse constructs an AccountResponse from an Account model
func buildAccountResponse(account *authModel.Account) *AccountResponse {
	resp := &AccountResponse{
		ID:     account.ID,
		Email:  account.Email,
		Active: account.Active,
	}

	if account.Username != nil {
		resp.Username = *account.Username
	}

	roleNames := make([]string, 0, len(account.Roles))
	for _, role := range account.Roles {
		roleNames = append(roleNames, role.Name)
	}
	resp.Roles = roleNames

	return resp
}

// buildSchoolIdentityResponse exposes the provisioned ids, or nil when nothing
// was provisioned.
func buildSchoolIdentityResponse(identity *authService.SchoolIdentity) *SchoolIdentityResponse {
	if identity == nil || identity.Person == nil || identity.Staff == nil {
		return nil
	}
	resp := &SchoolIdentityResponse{
		PersonID: identity.Person.ID,
		StaffID:  identity.Staff.ID,
	}
	if identity.Teacher != nil {
		resp.TeacherID = identity.Teacher.ID
	}
	return resp
}

// refreshToken handles token refresh
func (rs *Resource) refreshToken(w http.ResponseWriter, r *http.Request) {
	// Get refresh token from context
	refreshToken := jwt.RefreshTokenFromCtx(r.Context())

	// Get IP address and user agent for audit logging
	ipAddress := getClientIP(r)
	userAgent := r.Header.Get(headerUserAgent)

	ctx := rotation.WithRecoveryProof(r.Context(), r.Header.Get(rotation.RecoveryProofHeader))
	accessToken, newRefreshToken, err := rs.AuthService.RefreshTokenWithAudit(ctx, refreshToken, ipAddress, userAgent)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidToken):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrInvalidToken))
			case errors.Is(authErr.Err, authService.ErrTokenExpired):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTokenExpired))
			case errors.Is(authErr.Err, authService.ErrTokenNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTokenNotFound))
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountNotFound))
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountInactive))
			case errors.Is(authErr.Err, authService.ErrTenantNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTenantNotFound))
			case errors.Is(authErr.Err, authService.ErrTenantAccessDenied):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTenantAccessDenied))
			case errors.Is(authErr.Err, authService.ErrMustUseSchoolPortal):
				// The account is school-portal-only at this school (#2207).
				// 401, not 403: the tenant session is simply over, and the
				// frontend's refresh path turns a 401 into a clean logout.
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrMustUseSchoolPortal))
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

	err := rs.AuthService.LogoutWithAudit(r.Context(), refreshToken, ipAddress, userAgent)
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

	err := rs.AuthService.ChangePassword(r.Context(), claims.ID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidCredentials):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrInvalidCredentials))
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountNotFound))
			case errors.Is(authErr.Err, authService.ErrPasswordTooWeak):
				common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrPasswordTooWeak))
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
	return clientip.GetClientIPString(r)
}
