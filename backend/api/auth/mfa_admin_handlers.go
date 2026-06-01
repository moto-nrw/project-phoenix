package auth

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// MFAAdminOverrideRequest is the body for the admin-override endpoint —
// the `reason` field is mandatory and ends up in the audit row so
// revoked-by-admin events can be traced back to a human decision (e.g.
// "Postfach gesperrt, Nutzer hat sein 2FA-Gerät verloren").
type MFAAdminOverrideRequest struct {
	Reason string `json:"reason"`
}

// Bind enforces a non-empty reason. We intentionally don't cap the length
// here — the audit JSONB column can hold whatever a human types and the
// frontend already limits the textarea size.
func (req *MFAAdminOverrideRequest) Bind(_ *http.Request) error {
	req.Reason = strings.TrimSpace(req.Reason)
	return validation.ValidateStruct(req,
		validation.Field(&req.Reason, validation.Required, validation.Length(3, 0)),
	)
}

// adminOverrideContext bundles the per-request state every admin handler
// needs (target ID from URL, actor identity + tenant + permissions from
// JWT, parsed reason). Returning a single struct keeps the handlers thin
// and below the gocognit cap.
type adminOverrideContext struct {
	targetAccountID  int64
	actorAccountID   int64
	actorTenantID    int64
	actorPermissions []string
	reason           string
}

// resolveAdminOverrideContext extracts and validates everything an admin
// override needs. On error it writes the response itself and returns nil
// — callers just exit early.
func (rs *Resource) resolveAdminOverrideContext(w http.ResponseWriter, r *http.Request) *adminOverrideContext {
	idStr := chi.URLParam(r, "accountId")
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || targetID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidAccountID)))
		return nil
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return nil
	}

	req := &MFAAdminOverrideRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return nil
	}

	return &adminOverrideContext{
		targetAccountID:  targetID,
		actorAccountID:   int64(claims.ID),
		actorTenantID:    claims.TenantID,
		actorPermissions: claims.Permissions,
		reason:           req.Reason,
	}
}

// MFAAdminOverrideSetRequest is the body for PUT /auth/accounts/{id}/mfa/override.
// The override allow-list mirrors authService.IsValidMFAAdminOverride.
type MFAAdminOverrideSetRequest struct {
	Override string `json:"override"`
	Reason   string `json:"reason"`
}

// Bind enforces a known override value + non-empty reason. Length floor on
// reason matches MFAAdminOverrideRequest (3 chars) so the UI textarea
// validation message stays consistent across both admin paths.
func (req *MFAAdminOverrideSetRequest) Bind(_ *http.Request) error {
	req.Override = strings.TrimSpace(req.Override)
	req.Reason = strings.TrimSpace(req.Reason)
	if !authService.IsValidMFAAdminOverride(req.Override) {
		return errors.New("override must be one of: none, force_off, force_on")
	}
	return validation.ValidateStruct(req,
		validation.Field(&req.Reason, validation.Required, validation.Length(3, 0)),
	)
}

// MFAAdminStateResponse is the read-side payload the modal uses to decide
// which buttons to render. enrolled tells the modal whether a Reset is
// even meaningful; override is the current per-account flag.
type MFAAdminStateResponse struct {
	Enrolled bool   `json:"enrolled"`
	Override string `json:"override"`
}

// mfaAdminGetState returns the current MFA admin-facing snapshot for an
// account so the modal can render the right action set without inferring
// state from /accounts list views.
//
// Membership-gated: the read path runs the same actor-permissions +
// account-belongs-to-tenant check as the write paths via
// MFAService.GetAdminState. Without that gate a tenant admin with
// users:manage could probe account_ids across tenants and learn MFA
// state for accounts they don't administer. (#1430 review round 2.)
func (rs *Resource) mfaAdminGetState(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	idStr := chi.URLParam(r, "accountId")
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || targetID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidAccountID)))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	state, err := rs.MFAService.GetAdminState(
		r.Context(),
		int64(claims.ID),
		claims.TenantID,
		targetID,
		claims.Permissions,
	)
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	render.JSON(w, r, MFAAdminStateResponse{
		Enrolled: state.Enrolled,
		Override: state.Override,
	})
}

// mfaAdminSetOverride flips the per-account override (force_off / force_on /
// none). MFAService.SetMFAOverride handles trusted-device revocation and
// audit logging.
func (rs *Resource) mfaAdminSetOverride(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	idStr := chi.URLParam(r, "accountId")
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || targetID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidAccountID)))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	req := &MFAAdminOverrideSetRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.MFAService.SetMFAOverride(
		r.Context(),
		int64(claims.ID),
		claims.TenantID,
		targetID,
		req.Override,
		req.Reason,
		claims.Permissions,
	); err != nil {
		mapMFAError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// mfaAdminDisable wipes the target's MFA enrollment and trusted devices.
// The MFAService's AdminDisable runs the cascade and records the audit
// event with the actor's identity + reason.
func (rs *Resource) mfaAdminDisable(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	octx := rs.resolveAdminOverrideContext(w, r)
	if octx == nil {
		return
	}
	err := rs.MFAService.AdminDisable(
		r.Context(),
		octx.actorAccountID,
		octx.actorTenantID,
		octx.targetAccountID,
		octx.reason,
		octx.actorPermissions,
	)
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
