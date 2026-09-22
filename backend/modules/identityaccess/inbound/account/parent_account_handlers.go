package account

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// parentAccountResponse renders one parent account as the routes always did.
func parentAccountResponse(account identityaccess.ParentAccount) *ParentAccountResponse {
	return &ParentAccountResponse{
		ID:        account.ID,
		Email:     account.Email,
		Username:  account.Username,
		Active:    account.Active,
		CreatedAt: account.CreatedAt.Format(time.RFC3339),
		UpdatedAt: account.UpdatedAt.Format(time.RFC3339),
	}
}

// createParentAccount handles creating a parent account
func (rs *Resource) createParentAccount(w http.ResponseWriter, r *http.Request) {
	req := &CreateParentAccountRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	parentAccount, err := rs.AuthService.CreateParentAccount(r.Context(), req.Email, req.Username, req.Password)
	if err != nil {
		var authErr *identityaccess.AuthenticationError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, identityaccess.ErrEmailAlreadyExists):
				common.RenderError(w, r, common.ErrorInvalidRequest(identityaccess.ErrEmailAlreadyExists))
			case errors.Is(authErr.Err, identityaccess.ErrUsernameAlreadyExists):
				common.RenderError(w, r, common.ErrorInvalidRequest(identityaccess.ErrUsernameAlreadyExists))
			default:
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, parentAccountResponse(parentAccount), "Parent account created successfully")
}

// getParentAccountByID handles getting a parent account by ID
func (rs *Resource) getParentAccountByID(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidParentAccountID)
	if !ok {
		return
	}

	parentAccount, err := rs.AuthService.GetParentAccountByID(r.Context(), int64(id))
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("parent account not found")))
		return
	}

	common.Respond(w, r, http.StatusOK, parentAccountResponse(parentAccount), "Parent account retrieved successfully")
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

	parentAccount, err := rs.AuthService.GetParentAccountByID(r.Context(), int64(id))
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("parent account not found")))
		return
	}

	parentAccount.Email = req.Email
	if req.Username != "" {
		parentAccount.Username = req.Username
	}

	if err := rs.AuthService.UpdateParentAccount(r.Context(), parentAccount); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// listParentAccounts handles listing parent accounts
func (rs *Resource) listParentAccounts(w http.ResponseWriter, r *http.Request) {
	var filter identityaccess.ParentAccountFilter

	if email := r.URL.Query().Get("email"); email != "" {
		filter.Email = email
	}

	if active := r.URL.Query().Get("active"); active != "" {
		switch active {
		case "true":
			value := true
			filter.Active = &value
		case "false":
			value := false
			filter.Active = &value
		}
	}

	parentAccounts, err := rs.AuthService.ListParentAccounts(r.Context(), filter)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]*ParentAccountResponse, 0, len(parentAccounts))
	for _, account := range parentAccounts {
		responses = append(responses, parentAccountResponse(account))
	}

	common.Respond(w, r, http.StatusOK, responses, "Parent accounts retrieved successfully")
}
