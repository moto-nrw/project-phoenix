package school

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-nrw/project-phoenix/api/common"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
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
				return errors.New("Passwörter stimmen nicht überein")
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
	if _, err := rs.AuthService.InitiateSchoolPasswordReset(r.Context(), req.Email); err != nil {
		var rateErr *authService.RateLimitError
		if errors.As(err, &rateErr) {
			if seconds := rateErr.RetryAfterSeconds(time.Now()); seconds > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(seconds))
			}
			common.RenderError(w, r, common.ErrorTooManyRequests(authService.ErrRateLimitExceeded))
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
	if err := rs.AuthService.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidToken), errors.Is(authErr.Err, sql.ErrNoRows):
				common.RenderError(w, r, common.ErrorGone(errors.New("Der Link ist ungültig oder abgelaufen")))
				return
			case errors.Is(authErr.Err, authService.ErrPasswordTooWeak):
				common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrPasswordTooWeak))
				return
			}
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Passwort wurde geändert")
}
