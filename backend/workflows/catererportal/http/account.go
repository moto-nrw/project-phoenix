package catererportalhttp

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

var (
	errPasswordFieldsRequired = errors.New("current_password and new_password are required")
	errCurrentPasswordWrong   = errors.New("current password is incorrect")
	errPasswordMismatch       = errors.New("passwords do not match")
)

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// changePassword replaces the signed-in caterer's password (#4496). A wrong
// current password answers on its field, like the school portal's change,
// so the form keeps the session instead of signing out. The change ends
// every session of the account, this one included.
func (rs *Resource) changePassword(w http.ResponseWriter, r *http.Request) {
	var request changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.CurrentPassword == "" || request.NewPassword == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errPasswordFieldsRequired))
		return
	}
	if request.NewPassword != request.ConfirmPassword {
		common.RenderError(w, r, common.ErrorInvalidOnField(errPasswordMismatch, common.CodeIdentityPasswordMismatch, "confirm_password"))
		return
	}
	err := rs.portal.ChangeCatererPassword(r.Context(), rs.currentAccountID(r.Context()), request.CurrentPassword, request.NewPassword, common.GetClientIPString(r), r.Header.Get("User-Agent"))
	switch {
	case err == nil:
		common.RespondNoContent(w, r)
	case errors.Is(err, identityaccess.ErrInvalidCredentials):
		common.RenderError(w, r, common.ErrorUnauthorizedOnField(errCurrentPasswordWrong, common.CodeIdentityCurrentPasswordWrong, "current_password"))
	case errors.Is(err, identityaccess.ErrPasswordTooWeak):
		common.RenderError(w, r, common.ErrorInvalidOnField(identityaccess.ErrPasswordTooWeak, common.CodeIdentityPasswordTooWeak, "new_password"))
	default:
		renderFailure(w, r, err)
	}
}
