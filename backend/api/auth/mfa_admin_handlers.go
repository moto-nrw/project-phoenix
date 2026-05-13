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
// needs (target ID from URL, actor identity + permissions from JWT, parsed
// reason). Returning a single struct keeps the handlers thin and below the
// gocognit cap.
type adminOverrideContext struct {
	targetAccountID  int64
	actorAccountID   int64
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
		actorPermissions: claims.Permissions,
		reason:           req.Reason,
	}
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
