package auth

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// StaffPreviewStartRequest selects the staff account to preview (#2893).
type StaffPreviewStartRequest struct {
	AccountID int64 `json:"account_id"`
}

// Bind validates the staff-preview start payload.
func (req *StaffPreviewStartRequest) Bind(_ *http.Request) error {
	if req.AccountID <= 0 {
		return errors.New("account_id is required")
	}
	return nil
}

// StaffPreviewEndRequest names the previewed account for the audit trail.
type StaffPreviewEndRequest struct {
	AccountID int64 `json:"account_id"`
}

// Bind validates the staff-preview end payload.
func (req *StaffPreviewEndRequest) Bind(_ *http.Request) error {
	if req.AccountID <= 0 {
		return errors.New("account_id is required")
	}
	return nil
}

// StaffPreviewStartResponse carries the read-only preview token. There is
// deliberately NO refresh token: the preview is re-minted with the admin's
// own session and dies with it.
type StaffPreviewStartResponse struct {
	AccessToken     string `json:"access_token"`
	ExpiresIn       int64  `json:"expires_in"`
	TargetAccountID int64  `json:"target_account_id"`
	TargetName      string `json:"target_name"`
}

// startStaffPreview handles POST /auth/staff-preview (admin only).
func (rs *Resource) startStaffPreview(w http.ResponseWriter, r *http.Request) {
	req := &StaffPreviewStartRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	session, err := rs.AuthService.StartStaffPreview(
		r.Context(), int64(claims.ID), claims.TenantID, req.AccountID,
		getClientIP(r), r.Header.Get(headerUserAgent),
	)
	if err != nil {
		renderStaffPreviewError(w, r, err)
		return
	}

	render.JSON(w, r, StaffPreviewStartResponse{
		AccessToken:     session.AccessToken,
		ExpiresIn:       session.ExpiresIn,
		TargetAccountID: session.TargetAccountID,
		TargetName:      session.TargetName,
	})
}

// endStaffPreview handles POST /auth/staff-preview/end (admin only). Runs
// with the RESTORED admin session and only writes the audit trail.
func (rs *Resource) endStaffPreview(w http.ResponseWriter, r *http.Request) {
	req := &StaffPreviewEndRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	rs.AuthService.EndStaffPreview(
		r.Context(), int64(claims.ID), req.AccountID,
		getClientIP(r), r.Header.Get(headerUserAgent),
	)
	common.Respond(w, r, http.StatusOK, nil, "Preview ended")
}

// listStaffPreviewCandidates handles GET /auth/staff-preview/candidates
// (admin only, tenant transaction).
func (rs *Resource) listStaffPreviewCandidates(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	candidates, err := rs.AuthService.ListStaffPreviewCandidates(r.Context(), claims.TenantID, int64(claims.ID))
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, candidates, "Preview candidates retrieved")
}

// renderStaffPreviewError maps the preview-start service errors onto wire
// responses. Mirrors the switch-tenant mapping; the not-previewable cases
// carry stable codes so the frontend can explain them.
func renderStaffPreviewError(w http.ResponseWriter, r *http.Request, err error) {
	var authErr *authService.AuthError
	if errors.As(err, &authErr) {
		switch {
		case errors.Is(authErr.Err, authService.ErrAccountNotFound):
			common.RenderError(w, r, common.ErrorNotFound(authService.ErrAccountNotFound))
		case errors.Is(authErr.Err, authService.ErrAccountInactive),
			errors.Is(authErr.Err, authService.ErrTenantAccessDenied),
			errors.Is(authErr.Err, authService.ErrPreviewTargetNotStaff):
			common.RenderError(w, r, common.ErrorForbiddenWithCode(authErr.Err, "preview_target_not_previewable"))
		case errors.Is(authErr.Err, authService.ErrMustUseSchoolPortal):
			common.RenderError(w, r, common.ErrorForbiddenWithCode(authErr.Err, "preview_target_school_portal"))
		case errors.Is(authErr.Err, authService.ErrPreviewSelf):
			common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrPreviewSelf))
		case errors.Is(authErr.Err, authService.ErrTenantNotFound):
			common.RenderError(w, r, common.ErrorNotFound(authService.ErrTenantNotFound))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	common.RenderError(w, r, common.ErrorInternalServer(err))
}
