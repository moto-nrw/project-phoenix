package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// createParentAccount handles creating a parent account
func (rs *Resource) createParentAccount(w http.ResponseWriter, r *http.Request) {
	req := &CreateParentAccountRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	parentAccount, err := rs.AuthService.CreateParentAccount(r.Context(), req.Email, req.Username, req.Password)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrEmailAlreadyExists):
				common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrEmailAlreadyExists))
			case errors.Is(authErr.Err, authService.ErrUsernameAlreadyExists):
				common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrUsernameAlreadyExists))
			default:
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	resp := &ParentAccountResponse{
		ID:        parentAccount.ID,
		Email:     parentAccount.Email,
		Active:    parentAccount.Active,
		CreatedAt: parentAccount.CreatedAt.Format(time.RFC3339),
		UpdatedAt: parentAccount.UpdatedAt.Format(time.RFC3339),
	}

	if parentAccount.Username != nil {
		resp.Username = *parentAccount.Username
	}

	common.Respond(w, r, http.StatusCreated, resp, "Parent account created successfully")
}

// getParentAccountByID handles getting a parent account by ID
func (rs *Resource) getParentAccountByID(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidParentAccountID)
	if !ok {
		return
	}

	parentAccount, err := rs.AuthService.GetParentAccountByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("parent account not found")))
		return
	}

	resp := &ParentAccountResponse{
		ID:        parentAccount.ID,
		Email:     parentAccount.Email,
		Active:    parentAccount.Active,
		CreatedAt: parentAccount.CreatedAt.Format(time.RFC3339),
		UpdatedAt: parentAccount.UpdatedAt.Format(time.RFC3339),
	}

	if parentAccount.Username != nil {
		resp.Username = *parentAccount.Username
	}

	common.Respond(w, r, http.StatusOK, resp, "Parent account retrieved successfully")
}

// updateParentAccount handles updating a parent account
func (rs *Resource) updateParentAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidParentAccountID)
	if !ok {
		return
	}

	req := &UpdateAccountRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	parentAccount, err := rs.AuthService.GetParentAccountByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("parent account not found")))
		return
	}

	parentAccount.Email = req.Email
	if req.Username != "" {
		username := req.Username
		parentAccount.Username = &username
	}

	if err := rs.AuthService.UpdateParentAccount(r.Context(), parentAccount); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// listParentAccounts handles listing parent accounts
func (rs *Resource) listParentAccounts(w http.ResponseWriter, r *http.Request) {
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

	parentAccounts, err := rs.AuthService.ListParentAccounts(r.Context(), filters)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]*ParentAccountResponse, 0, len(parentAccounts))
	for _, account := range parentAccounts {
		resp := &ParentAccountResponse{
			ID:        account.ID,
			Email:     account.Email,
			Active:    account.Active,
			CreatedAt: account.CreatedAt.Format(time.RFC3339),
			UpdatedAt: account.UpdatedAt.Format(time.RFC3339),
		}

		if account.Username != nil {
			resp.Username = *account.Username
		}

		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Parent accounts retrieved successfully")
}
