package account

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/portalauth"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// login signs a staff account in. The trusted-device cookie lets the MFA
// gate skip the second factor on a device the account remembered.
func (rs *Resource) login(w http.ResponseWriter, r *http.Request) {
	req := &LoginRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !common.RequireDependency(w, r, rs.Sessions != nil, errSessionsUnavailable) {
		return
	}
	client := portalauth.ClientOf(r)
	result, err := rs.Sessions.LoginWithMFAGate(
		r.Context(), req.Email, req.Password, client.IP, client.UserAgent, req.TenantSlug, common.TrustedDevice(r),
	)
	if err != nil {
		portalauth.Render(w, r, err, portalauth.LoginRules)
		return
	}
	portalauth.RenderLogin(w, r, result)
}

var errSessionsUnavailable = errors.New("account sessions are not composed")

// register handles user registration
func (rs *Resource) register(w http.ResponseWriter, r *http.Request) {
	req := &RegisterRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

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
		// The account, its school mapping and its role were written in the
		// request's tenant transaction, which commits below 500: roll it back
		// so a refusal leaves no half-provisioned account (#2222).
		rs.markRollback(r.Context())
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
		rs.markRollback(r.Context())

		var authErr *identityaccess.AuthenticationError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, identityaccess.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorNotFound(authErr.Err))
			case errors.Is(authErr.Err, identityaccess.ErrAccountInactive):
				common.RenderError(w, r, common.ErrorConflict(authErr.Err))
			case errors.Is(authErr.Err, identityaccess.ErrEmailAlreadyExists):
				// A caterer account joins no school (ADR 0077).
				common.RenderError(w, r, common.ErrorConflictOnField(identityaccess.ErrEmailAlreadyExists, common.CodeIdentityEmailAlreadyExists, "email"))
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
// middleware (Authenticator + TenantScopeMiddleware + RequiresPermission). Returns
// the role ID, the tenant ID, and whether the handler should return early
// (true = error rendered).
func (rs *Resource) authorizeRoleAssignment(w http.ResponseWriter, r *http.Request, requestedRoleID *int64) (*int64, int64, bool) {
	claims := sessiontoken.ClaimsFromCtx(r.Context())

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
		// The staff form offers to link the existing account on this code (#2517).
		common.RenderError(w, r, common.ErrorInvalidOnField(identityaccess.ErrEmailAlreadyExists, common.CodeIdentityEmailAlreadyExists, "email"))
	case errors.Is(authErr.Err, identityaccess.ErrUsernameAlreadyExists):
		common.RenderError(w, r, common.ErrorInvalidRequest(identityaccess.ErrUsernameAlreadyExists))
	case errors.Is(authErr.Err, identityaccess.ErrPasswordTooWeak):
		common.RenderError(w, r, common.ErrorInvalidOnField(identityaccess.ErrPasswordTooWeak, common.CodeIdentityPasswordTooWeak, "password"))
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

// refreshToken rotates the presented refresh token. A refusal before the
// rotation leaves the presented token valid.
func (rs *Resource) refreshToken(w http.ResponseWriter, r *http.Request) {
	if !common.RequireDependency(w, r, rs.Sessions != nil, errSessionsUnavailable) {
		return
	}
	client := portalauth.ClientOf(r)
	accessToken, refreshToken, err := rs.Sessions.RefreshTokenWithAudit(
		r.Context(), sessiontoken.RefreshTokenFromCtx(r.Context()), r.Header.Get(sessiontoken.RecoveryProofHeader), client.IP, client.UserAgent,
	)
	if err != nil {
		portalauth.Render(w, r, err, portalauth.RefreshRules)
		return
	}
	render.JSON(w, r, portalauth.TokenResponse{AccessToken: accessToken, RefreshToken: refreshToken})
}

// logout handles user logout
func (rs *Resource) logout(w http.ResponseWriter, r *http.Request) {
	if !common.RequireDependency(w, r, rs.Sessions != nil, errSessionsUnavailable) {
		return
	}
	refreshToken := sessiontoken.RefreshTokenFromCtx(r.Context())
	client := portalauth.ClientOf(r)
	// The audit rows follow the commit and never fail the call. An error here
	// means the family was not revoked: its access tokens stay valid, so the
	// browser must not take the logout for done (#5194).
	if err := rs.Sessions.LogoutWithAudit(r.Context(), refreshToken, client.IP, client.UserAgent); err != nil {
		if errors.Is(err, identityaccess.ErrInvalidToken) {
			// Not a refresh token of any session: nothing is left to revoke.
			common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrInvalidToken))
			return
		}
		slog.Default().ErrorContext(r.Context(), "logout did not revoke the session family",
			slog.String("ip", client.IP),
			slog.String("error", err.Error()),
		)
		common.RenderError(w, r, common.ErrorServiceUnavailable(errors.New("logout could not revoke the session, please retry")))
		return
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

	claims := sessiontoken.ClaimsFromCtx(r.Context())

	err := rs.Sessions.ChangeAccountPassword(r.Context(), int64(claims.ID), req.CurrentPassword, req.NewPassword)
	if err != nil {
		var authErr *identityaccess.AuthenticationError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, identityaccess.ErrInvalidCredentials):
				// The status stays 401; the code and field tell the form that
				// the current password was refused, not the session (#2517).
				common.RenderError(w, r, common.ErrorUnauthorizedOnField(identityaccess.ErrInvalidCredentials, common.CodeIdentityCurrentPasswordWrong, "current_password"))
			case errors.Is(authErr.Err, identityaccess.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrAccountNotFound))
			case errors.Is(authErr.Err, identityaccess.ErrPasswordTooWeak):
				common.RenderError(w, r, common.ErrorInvalidOnField(identityaccess.ErrPasswordTooWeak, common.CodeIdentityPasswordTooWeak, "new_password"))
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
		// A Lehrkraft copy is graded like the Lehrkraft role (#4574).
		SchoolPortal: role.SchoolPortal,
	}
	return rs.RoleGrants.CanGrantRole(facts, permissions, append([]string(nil), role.Permissions...))
}
