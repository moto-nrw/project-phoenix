package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// getAccount returns the current user's account details
func (rs *Resource) getAccount(w http.ResponseWriter, r *http.Request) {
	// Get user ID and permissions from JWT claims
	claims := jwt.ClaimsFromCtx(r.Context())

	account, err := rs.Sessions.FindOwnAccount(r.Context(), int64(claims.ID))
	if err != nil {
		if errors.Is(err, identityaccess.ErrAccountNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(identityaccess.ErrAccountNotFound))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert account to response. The role assignment is not part of the
	// account row, so the list stays empty here as it was; the caller's
	// roles and permissions travel in their session.
	resp := &AccountResponse{
		ID:          account.ID,
		Email:       account.Email,
		Username:    account.Username,
		Active:      account.Active,
		Roles:       []string{},
		Permissions: claims.Permissions,
	}

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
	// The caregiver capability is People Directory's and reports its own
	// sentinel; the account administration reports Identity & Access's.
	case errors.Is(err, usersService.ErrAccountNotFound),
		errors.Is(err, identityaccess.ErrAccountNotFound),
		errors.As(err, &accountTenantErr):
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

// updateAccount handles updating an account. The write carries the
// account-management boundary itself, so an account the caller may not
// administer is reported as missing without a separate read.
func (rs *Resource) updateAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	req := &UpdateAccountRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	update := identityaccess.AccountIdentityUpdate{AccountID: id, Email: req.Email}
	if req.Username != "" {
		username := req.Username
		update.Username = &username
	}

	if err := rs.Sessions.UpdateManageableAccount(r.Context(), update); err != nil {
		common.RenderError(w, r, accountManagementErrorRenderer(err))
		return
	}

	common.RespondNoContent(w, r)
}

var accountManagementErrorRenderer = common.UnwrapRenderer[*identityaccess.AuthenticationError](
	[]common.ErrorRule{
		{Target: identityaccess.ErrAccountNotFound, Render: common.ErrorNotFound},
	},
	common.ErrorInternalServer,
)

// listAccounts handles listing accounts
func (rs *Resource) listAccounts(w http.ResponseWriter, r *http.Request) {
	filter := identityaccess.AccountListFilter{Email: r.URL.Query().Get("email")}

	switch r.URL.Query().Get("active") {
	case "true":
		active := true
		filter.Active = &active
	case "false":
		active := false
		filter.Active = &active
	}

	accounts, err := rs.Sessions.ListManageableAccounts(r.Context(), filter)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, accountResponses(accounts), "Accounts retrieved successfully")
}

// getAccountsByRole handles getting accounts by role
func (rs *Resource) getAccountsByRole(w http.ResponseWriter, r *http.Request) {
	roleName := chi.URLParam(r, "roleName")

	accounts, err := rs.Sessions.ListManageableAccountsByRole(r.Context(), roleName)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, accountResponses(accounts), "Accounts retrieved successfully")
}

// activateAccount and deactivateAccount re-enable and disable an account
// the caller may administer. A deactivation also revokes the account's
// sessions, which is why it is its own command and not a field of the
// account update.
func (rs *Resource) activateAccount(ctx context.Context, accountID int64) error {
	return rs.Sessions.ActivateAccount(ctx, accountID)
}

func (rs *Resource) deactivateAccount(ctx context.Context, accountID int64) error {
	return rs.Sessions.DeactivateAccount(ctx, accountID)
}

// accountResponses is the account listing's wire shape: the identifying
// fields only, never a role or a credential of another school.
func accountResponses(accounts []identityaccess.ManagedAccount) []*AccountResponse {
	responses := make([]*AccountResponse, 0, len(accounts))
	for _, account := range accounts {
		responses = append(responses, &AccountResponse{
			ID:       account.ID,
			Email:    account.Email,
			Username: account.Username,
			Active:   account.Active,
		})
	}
	return responses
}

// Password Reset Endpoints
