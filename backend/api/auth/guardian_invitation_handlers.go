package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const errGuardianInvitationServiceUnavailable = "guardian invitation service unavailable"

// GuardianInvitationValidateResponse is the public-safe view of a guardian
// invitation. Pre-fills the accept page so the parent doesn't have to re-enter
// their email or name.
type GuardianInvitationValidateResponse struct {
	Email         string    `json:"email"`
	FirstName     string    `json:"first_name,omitempty"`
	LastName      string    `json:"last_name,omitempty"`
	ExpiresAt     time.Time `json:"expires_at"`
	SchoolName    string    `json:"school_name,omitempty"`
	TenantSlug    string    `json:"tenant_slug,omitempty"`
	SchoolLogoURL string    `json:"school_logo_url,omitempty"`
}

// AcceptGuardianInvitationRequest is the form payload submitted on the
// accept-guardian-invite page. First/last name are NOT collected. They come
// from the GuardianProfile and are shown read-only on the page.
type AcceptGuardianInvitationRequest struct {
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (req *AcceptGuardianInvitationRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Password, validation.Required),
		validation.Field(&req.ConfirmPassword, validation.Required),
	)
}

// AcceptGuardianInvitationResponse mirrors AcceptInvitationResponse so the
// frontend can reuse mapping helpers.
type AcceptGuardianInvitationResponse struct {
	AccountID  int64  `json:"account_id"`
	Email      string `json:"email"`
	TenantSlug string `json:"tenant_slug,omitempty"`
}

func (rs *Resource) validateGuardianInvitation(w http.ResponseWriter, r *http.Request) {
	if rs.GuardianInvitationService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New(errGuardianInvitationServiceUnavailable)))
		return
	}

	token := strings.TrimSpace(chi.URLParam(r, "token"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("token is required")))
		return
	}

	var result *authService.GuardianInvitationValidation
	var err error
	if rs.db != nil {
		err = tenant.WithAdminTx(r.Context(), rs.db, func(txCtx context.Context, _ bun.Tx) error {
			var txErr error
			result, txErr = rs.GuardianInvitationService.Validate(txCtx, token)
			return txErr
		})
	} else {
		result, err = rs.GuardianInvitationService.Validate(r.Context(), token)
	}
	if err != nil {
		if renderInvitationError(w, r, err) {
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	resp := GuardianInvitationValidateResponse{
		Email:         result.Email,
		FirstName:     result.FirstName,
		LastName:      result.LastName,
		ExpiresAt:     result.ExpiresAt,
		SchoolName:    result.SchoolName,
		TenantSlug:    result.TenantSlug,
		SchoolLogoURL: result.SchoolLogoURL,
	}
	common.Respond(w, r, http.StatusOK, resp, "Guardian invitation validated successfully")
}

func (rs *Resource) acceptGuardianInvitation(w http.ResponseWriter, r *http.Request) {
	if rs.GuardianInvitationService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New(errGuardianInvitationServiceUnavailable)))
		return
	}

	token := strings.TrimSpace(chi.URLParam(r, "token"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("token is required")))
		return
	}

	req := &AcceptGuardianInvitationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	data := authService.GuardianInvitationAcceptData{
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
	}

	var account *authModels.Account
	var err error
	if rs.db != nil {
		err = tenant.WithAdminTx(r.Context(), rs.db, func(txCtx context.Context, _ bun.Tx) error {
			var txErr error
			account, txErr = rs.GuardianInvitationService.Accept(txCtx, token, data)
			return txErr
		})
	} else {
		account, err = rs.GuardianInvitationService.Accept(r.Context(), token, data)
	}
	if err != nil {
		if !renderAcceptError(w, r, err) {
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}

	slog.Default().Info("guardian invitation accepted",
		slog.Int64("account_id", account.ID))

	resp := AcceptGuardianInvitationResponse{
		AccountID: account.ID,
		Email:     account.Email,
	}
	if slug := rs.lookupTenantSlugForGuardianInvitation(r.Context(), token); slug != "" {
		resp.TenantSlug = slug
	}
	common.Respond(w, r, http.StatusCreated, resp, "Guardian invitation accepted successfully")
}

// lookupTenantSlugForGuardianInvitation resolves the tenant slug from a
// guardian invitation token. Best-effort; returns "" on error.
func (rs *Resource) lookupTenantSlugForGuardianInvitation(ctx context.Context, token string) string {
	if rs.GuardianInvitationService == nil {
		return ""
	}
	return rs.GuardianInvitationService.GetTenantSlugForToken(ctx, token)
}

func (rs *Resource) resendGuardianInvitation(w http.ResponseWriter, r *http.Request) {
	if rs.GuardianInvitationService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New(errGuardianInvitationServiceUnavailable)))
		return
	}
	rs.resendInvitationHandler(w, r,
		rs.GuardianInvitationService.Resend,
		"guardian invitation resend requested", "Guardian invitation resent", "Guardian invitation resent successfully")
}
