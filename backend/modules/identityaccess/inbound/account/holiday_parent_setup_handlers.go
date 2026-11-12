package account

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/inbound/portalauth"
)

// The setup link of a parent account at a Träger (ADR 0216 points 5 and 6).
// Both routes are public and token-proved like the guardian invitation:
// the signed link is the credential.

// HolidayParentSetupResponse is the setup page's view of the link.
type HolidayParentSetupResponse struct {
	Email         string `json:"email"`
	TraegerName   string `json:"traeger_name"`
	AccountExists bool   `json:"account_exists"`
}

// CompleteHolidayParentSetupRequest carries the password of a new account;
// an existing account keeps its own, so the fields may stay empty then.
type CompleteHolidayParentSetupRequest struct {
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (*CompleteHolidayParentSetupRequest) Bind(*http.Request) error { return nil }

// CompleteHolidayParentSetupResponse reports whether the address already had
// an account, whose password stayed unchanged.
type CompleteHolidayParentSetupResponse struct {
	AccountExists bool `json:"account_exists"`
}

func (rs *Resource) previewHolidayParentSetup(w http.ResponseWriter, r *http.Request) {
	if rs.HolidayParentSetups == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("holiday parent setup is not composed")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	// The capability opens its own administrative transaction.
	preview, err := rs.HolidayParentSetups.PreviewHolidayParentSetup(r.Context(), token)
	if err != nil {
		renderHolidayParentSetupError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, HolidayParentSetupResponse{Email: preview.Email, TraegerName: preview.TraegerName, AccountExists: preview.AccountExists}, "Holiday parent setup validated")
}

func (rs *Resource) completeHolidayParentSetup(w http.ResponseWriter, r *http.Request) {
	if rs.HolidayParentSetups == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("holiday parent setup is not composed")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	req := &CompleteHolidayParentSetupRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	existed, err := rs.HolidayParentSetups.CompleteHolidayParentSetup(r.Context(), token, identityaccess.GuardianRegistration{Password: req.Password, ConfirmPassword: req.ConfirmPassword})
	if err != nil {
		renderHolidayParentSetupError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, CompleteHolidayParentSetupResponse{AccountExists: existed}, "Holiday parent account set up")
}

// renderHolidayParentSetupError answers an invalid link with 404 and the
// password refusals as the guardian invitation does.
func renderHolidayParentSetupError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(portalauth.Cause(err), identityaccess.ErrHolidayParentSetupInvalid) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(identityaccess.ErrHolidayParentSetupInvalid, common.CodeIdentityHolidayParentSetupInvalid))
		return
	}
	if !renderAcceptError(w, r, err) {
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
