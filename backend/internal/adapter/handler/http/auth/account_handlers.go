package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
)

// UpdateAccountRequest represents the update account request payload
type UpdateAccountRequest struct {
	Email    string `json:"email"`
	Username string `json:"username,omitempty"`
}

// Bind validates the update account request
func (req *UpdateAccountRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Username = strings.TrimSpace(req.Username)

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
		validation.Field(&req.Username, validation.Length(3, 30)),
	)
}

// activateAccount handles activating an account
func (rs *Resource) activateAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	if err := rs.AuthService.ActivateAccount(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// deactivateAccount handles deactivating an account
func (rs *Resource) deactivateAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	if err := rs.AuthService.DeactivateAccount(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// updateAccount handles updating an account
func (rs *Resource) updateAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	req := &UpdateAccountRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	account, err := rs.AuthService.GetAccountByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("account not found")))
		return
	}

	account.Email = req.Email
	if req.Username != "" {
		username := req.Username
		account.Username = &username
	}

	if err := rs.AuthService.UpdateAccount(r.Context(), account); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// listAccounts handles listing accounts
func (rs *Resource) listAccounts(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters for filtering
	filters := make(map[string]interface{})

	if email := r.URL.Query().Get("email"); email != "" {
		filters["email"] = email
	}

	if active := r.URL.Query().Get("active"); active != "" {
		switch active {
		case "true":
			filters["active"] = true
		case "false":
			filters["active"] = false
		}
	}

	accounts, err := rs.AuthService.ListAccounts(r.Context(), filters)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	responses := make([]*AccountResponse, 0, len(accounts))
	for _, account := range accounts {
		resp := &AccountResponse{
			ID:     account.ID,
			Email:  account.Email,
			Active: account.Active,
		}

		if account.Username != nil {
			resp.Username = *account.Username
		}

		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Accounts retrieved successfully")
}

// getAccountsByRole handles getting accounts by role
func (rs *Resource) getAccountsByRole(w http.ResponseWriter, r *http.Request) {
	roleName := chi.URLParam(r, "roleName")

	accounts, err := rs.AuthService.GetAccountsByRole(r.Context(), roleName)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	responses := make([]*AccountResponse, 0, len(accounts))
	for _, account := range accounts {
		resp := &AccountResponse{
			ID:     account.ID,
			Email:  account.Email,
			Active: account.Active,
		}

		if account.Username != nil {
			resp.Username = *account.Username
		}

		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Accounts retrieved successfully")
}

// getAccountRoles handles getting roles for an account
func (rs *Resource) getAccountRoles(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	roles, err := rs.AuthService.GetAccountRoles(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	responses := make([]*RoleResponse, 0, len(roles))
	for _, role := range roles {
		resp := &RoleResponse{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			CreatedAt:   role.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   role.UpdatedAt.Format(time.RFC3339),
		}
		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Account roles retrieved successfully")
}

// assignRoleToAccount handles assigning a role to an account
func (rs *Resource) assignRoleToAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	roleID, ok := common.ParseIntIDWithError(w, r, "roleId", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	if err := rs.AuthService.AssignRoleToAccount(r.Context(), accountID, roleID); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// removeRoleFromAccount handles removing a role from an account
func (rs *Resource) removeRoleFromAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	roleID, ok := common.ParseIntIDWithError(w, r, "roleId", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	if err := rs.AuthService.RemoveRoleFromAccount(r.Context(), accountID, roleID); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// grantPermissionToAccount handles granting a permission to an account
func (rs *Resource) grantPermissionToAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	permissionID, ok := common.ParseIntIDWithError(w, r, "permissionId", common.MsgInvalidPermissionID)
	if !ok {
		return
	}

	if err := rs.AuthService.GrantPermissionToAccount(r.Context(), accountID, permissionID); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// denyPermissionToAccount handles denying a permission to an account
func (rs *Resource) denyPermissionToAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	permissionID, ok := common.ParseIntIDWithError(w, r, "permissionId", common.MsgInvalidPermissionID)
	if !ok {
		return
	}

	if err := rs.AuthService.DenyPermissionToAccount(r.Context(), accountID, permissionID); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// removePermissionFromAccount handles removing a permission from an account
func (rs *Resource) removePermissionFromAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	permissionID, ok := common.ParseIntIDWithError(w, r, "permissionId", common.MsgInvalidPermissionID)
	if !ok {
		return
	}

	if err := rs.AuthService.RemovePermissionFromAccount(r.Context(), accountID, permissionID); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// getAccountPermissions handles getting permissions for an account
func (rs *Resource) getAccountPermissions(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	permissions, err := rs.AuthService.GetAccountPermissions(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	responses := make([]*PermissionResponse, 0, len(permissions))
	for _, permission := range permissions {
		resp := &PermissionResponse{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
			Resource:    permission.Resource,
			Action:      permission.Action,
			CreatedAt:   permission.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   permission.UpdatedAt.Format(time.RFC3339),
		}
		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Account permissions retrieved successfully")
}

// getAccountDirectPermissions handles getting only direct permissions for an account (not role-based)
func (rs *Resource) getAccountDirectPermissions(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	permissions, err := rs.AuthService.GetAccountDirectPermissions(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	responses := make([]*PermissionResponse, 0, len(permissions))
	for _, permission := range permissions {
		resp := &PermissionResponse{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
			Resource:    permission.Resource,
			Action:      permission.Action,
			CreatedAt:   permission.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   permission.UpdatedAt.Format(time.RFC3339),
		}
		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Account direct permissions retrieved successfully")
}

// revokeAllTokens handles revoking all tokens for an account
func (rs *Resource) revokeAllTokens(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	if err := rs.AuthService.RevokeAllTokens(r.Context(), accountID); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// getActiveTokens handles getting active tokens for an account
func (rs *Resource) getActiveTokens(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	tokens, err := rs.AuthService.GetActiveTokens(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	type TokenResponse struct {
		ID         int64  `json:"id"`
		Token      string `json:"token"`
		Expiry     string `json:"expiry"`
		Mobile     bool   `json:"mobile"`
		Identifier string `json:"identifier,omitempty"`
		CreatedAt  string `json:"created_at"`
	}

	responses := make([]*TokenResponse, 0, len(tokens))
	for _, token := range tokens {
		resp := &TokenResponse{
			ID:        token.ID,
			Token:     token.Token,
			Expiry:    token.Expiry.Format(time.RFC3339),
			Mobile:    token.Mobile,
			CreatedAt: token.CreatedAt.Format(time.RFC3339),
		}

		if token.Identifier != nil {
			resp.Identifier = *token.Identifier
		}

		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Active tokens retrieved successfully")
}
