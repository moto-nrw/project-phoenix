package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// AccountResponse represents the account response payload
type AccountResponse struct {
	ID          int64    `json:"id"`
	Email       string   `json:"email"`
	Username    string   `json:"username,omitempty"`
	Active      bool     `json:"active"`
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

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

// getAccount returns the current user's account details
func (rs *Resource) getAccount(w http.ResponseWriter, r *http.Request) {
	// Get user ID and permissions from JWT claims
	claims := jwt.ClaimsFromCtx(r.Context())

	account, err := rs.AuthService.GetAccountByID(r.Context(), claims.ID)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			if errors.Is(authErr.Err, authService.ErrAccountNotFound) {
				common.RenderError(w, r, ErrorNotFound(authService.ErrAccountNotFound))
				return
			}
		}
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Convert account to response
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

	// Include permissions from JWT claims
	resp.Permissions = claims.Permissions

	common.Respond(w, r, http.StatusOK, resp, "Account information retrieved successfully")
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
