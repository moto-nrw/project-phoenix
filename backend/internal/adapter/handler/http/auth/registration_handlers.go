package auth

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	adaptermiddleware "github.com/moto-nrw/project-phoenix/internal/adapter/middleware"
	authModel "github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	authService "github.com/moto-nrw/project-phoenix/internal/core/service/auth"
)

// register handles user registration
func (rs *Resource) register(w http.ResponseWriter, r *http.Request) {
	req := &RegisterRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Authorize role assignment (if role_id specified)
	roleID, shouldReturn := rs.authorizeRoleAssignment(w, r, req.RoleID)
	if shouldReturn {
		return
	}

	account, err := rs.AuthService.Register(r.Context(), req.Email, req.Username, req.Password, roleID)
	if err != nil {
		rs.handleRegistrationError(w, r, err)
		return
	}

	resp := buildAccountResponse(account)
	common.Respond(w, r, http.StatusCreated, resp, "Account registered successfully")
}

// authorizeRoleAssignment checks if the caller is authorized to assign a role during registration.
// Returns the authorized role ID and a boolean indicating if the handler should return early.
func (rs *Resource) authorizeRoleAssignment(w http.ResponseWriter, r *http.Request, requestedRoleID *int64) (*int64, bool) {
	if requestedRoleID == nil {
		return nil, false
	}

	authHeader := r.Header.Get("Authorization")
	if !isValidAuthHeader(authHeader) {
		recordRoleAssignmentWarning(r, "missing_auth", "unauthenticated register attempt with role_id")
		return nil, false
	}

	token := authHeader[7:]
	callerAccount, err := rs.AuthService.ValidateToken(r.Context(), token)
	if err != nil {
		recordRoleAssignmentWarning(r, "invalid_token", "invalid token in register attempt with role_id")
		return nil, false
	}

	if !hasAdminRole(callerAccount.Roles) {
		event := recordRoleAssignmentWarning(r, "non_admin", "non-admin attempted to set role_id")
		if event != nil && event.AccountID == "" {
			event.AccountID = strconv.FormatInt(callerAccount.ID, 10)
		}
		common.RenderError(w, r, ErrorUnauthorized(errors.New("only administrators can assign roles")))
		return nil, true
	}

	return requestedRoleID, false
}

// handleRegistrationError handles authentication errors during registration
func (rs *Resource) handleRegistrationError(w http.ResponseWriter, r *http.Request, err error) {
	var authErr *authService.AuthError
	if !errors.As(err, &authErr) {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	switch {
	case errors.Is(authErr.Err, authService.ErrEmailAlreadyExists):
		common.RenderError(w, r, ErrorInvalidRequest(authService.ErrEmailAlreadyExists))
	case errors.Is(authErr.Err, authService.ErrUsernameAlreadyExists):
		common.RenderError(w, r, ErrorInvalidRequest(authService.ErrUsernameAlreadyExists))
	case errors.Is(authErr.Err, authService.ErrPasswordTooWeak):
		common.RenderError(w, r, ErrorInvalidRequest(authService.ErrPasswordTooWeak))
	default:
		common.RenderError(w, r, ErrorInternalServer(err))
	}
}

// isValidAuthHeader checks if the Authorization header contains a valid Bearer token format
func isValidAuthHeader(authHeader string) bool {
	return authHeader != "" && len(authHeader) >= 8 && authHeader[:7] == "Bearer "
}

// recordRoleAssignmentWarning records a warning for role assignment attempts
func recordRoleAssignmentWarning(r *http.Request, code string, message string) *adaptermiddleware.WideEvent {
	if r == nil {
		return nil
	}
	event := adaptermiddleware.GetWideEvent(r.Context())
	if event == nil || event.Timestamp.IsZero() || event.WarningType != "" {
		return event
	}
	event.WarningType = "role_assignment"
	event.WarningCode = code
	event.WarningMessage = message
	return event
}

// hasAdminRole checks if any of the roles has the "admin" name
func hasAdminRole(roles []*authModel.Role) bool {
	for _, role := range roles {
		if role.Name == "admin" {
			return true
		}
	}
	return false
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
