package auth

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// getAccount returns the current user's account details
func (rs *Resource) getAccount(w http.ResponseWriter, r *http.Request) {
	// Get user ID and permissions from JWT claims
	claims := jwt.ClaimsFromCtx(r.Context())

	account, err := rs.AuthService.GetAccountByID(r.Context(), claims.ID)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			if errors.Is(authErr.Err, authService.ErrAccountNotFound) {
				common.RenderError(w, r, common.ErrorNotFound(authService.ErrAccountNotFound))
				return
			}
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
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

// Role Management Request/Response Types

func (rs *Resource) getCaregiverCapability(w http.ResponseWriter, r *http.Request) {
	if rs.CaregiverCapabilityService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("caregiver capability service is not configured")))
		return
	}

	accountID, ok := common.ParseInt64IDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	state, err := rs.CaregiverCapabilityService.GetCaregiverCapability(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, caregiverCapabilityErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, state, "Caregiver capability retrieved successfully")
}

func (rs *Resource) enableCaregiverCapability(w http.ResponseWriter, r *http.Request) {
	if rs.CaregiverCapabilityService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("caregiver capability service is not configured")))
		return
	}

	accountID, ok := common.ParseInt64IDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	req := &EnableCaregiverCapabilityRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	state, err := rs.CaregiverCapabilityService.EnableCaregiverCapability(r.Context(), accountID, userModel.EnableCaregiverCapabilityInput{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Position:  req.Position,
	})
	if err != nil {
		common.RenderError(w, r, caregiverCapabilityErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, state, "Caregiver capability enabled successfully")
}

func (rs *Resource) disableCaregiverCapability(w http.ResponseWriter, r *http.Request) {
	if rs.CaregiverCapabilityService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("caregiver capability service is not configured")))
		return
	}

	accountID, ok := common.ParseInt64IDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	state, err := rs.CaregiverCapabilityService.DisableCaregiverCapability(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, caregiverCapabilityErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, state, "Caregiver capability disabled successfully")
}

func caregiverCapabilityErrorRenderer(err error) render.Renderer {
	var blockedErr *usersService.CaregiverCapabilityBlockedError
	var accountTenantErr *usersService.AccountNotAssignedToTenantError
	var usersErr *usersService.UsersError
	var validationErr *usersService.ValidationError

	switch {
	case errors.As(err, &blockedErr):
		return common.NewCaregiverCapabilityBlockedResponse(
			http.StatusConflict,
			blockedErr.Error(),
			blockedErr.Reasons,
		)
	case errors.Is(err, authService.ErrAccountNotFound), errors.As(err, &accountTenantErr):
		return common.ErrorNotFound(errors.New("account not found"))
	case errors.As(err, &usersErr) && errors.As(err, &validationErr):
		return common.ErrorInvalidRequest(validationErr)
	case errors.As(err, &usersErr):
		return common.ErrorInternalServer(usersErr.Err)
	default:
		return common.ErrorInternalServer(err)
	}
}

// Permission Management Endpoints

// updateAccount handles updating an account
func (rs *Resource) updateAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	req := &UpdateAccountRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	account, err := rs.AuthService.GetAccountByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("account not found")))
		return
	}

	account.Email = req.Email
	if req.Username != "" {
		username := req.Username
		account.Username = &username
	}

	if err := rs.AuthService.UpdateAccount(r.Context(), account); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
		common.RenderError(w, r, common.ErrorInternalServer(err))
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

// Password Reset Endpoints
