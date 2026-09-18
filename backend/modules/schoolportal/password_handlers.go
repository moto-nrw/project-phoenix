package schoolportal

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-nrw/project-phoenix/api/common"
)

type passwordResetRequest struct {
	Email string `json:"email"`
}

func (req *passwordResetRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	return validation.ValidateStruct(req, validation.Field(&req.Email, validation.Required, is.Email))
}

type passwordResetConfirmRequest struct {
	Token           string `json:"token"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (req *passwordResetConfirmRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Token, validation.Required),
		validation.Field(&req.NewPassword, validation.Required, validation.Length(8, 0)),
		validation.Field(&req.ConfirmPassword, validation.Required, validation.By(func(_ interface{}) error {
			if req.NewPassword != req.ConfirmPassword {
				return errors.New("Passwörter stimmen nicht überein") //nolint:staticcheck // ST1005: user-facing German message
			}
			return nil
		})),
	)
}

func (rs *Resource) initiatePasswordReset(w http.ResponseWriter, r *http.Request) {
	req := &passwordResetRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !rs.Resets.complete() {
		common.RenderError(w, r, common.ErrorInternalServer(ErrPasswordResetUnavailable))
		return
	}
	if err := rs.Resets.Initiate(r.Context(), req.Email); err != nil {
		if rs.renderPasswordResetRateLimit(w, r, err) {
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Wenn die E-Mail-Adresse bekannt ist, wurde ein Link zum Zurücksetzen gesendet")
}

func (rs *Resource) resetPassword(w http.ResponseWriter, r *http.Request) {
	req := &passwordResetConfirmRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !rs.Resets.complete() {
		common.RenderError(w, r, common.ErrorInternalServer(ErrPasswordResetUnavailable))
		return
	}
	if err := rs.Resets.Reset(r.Context(), req.Token, req.NewPassword); err != nil {
		switch {
		case rs.Resets.LinkUnusable(err):
			common.RenderError(w, r, common.ErrorGone(errors.New("Der Link ist ungültig oder abgelaufen"))) //nolint:staticcheck // ST1005: user-facing German message
			return
		case rs.Resets.TooWeak(err):
			common.RenderError(w, r, common.ErrorInvalidRequest(ErrPasswordTooWeak))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Passwort wurde geändert")
}
