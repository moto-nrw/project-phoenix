package notifications

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	notificationsService "github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// emailSubscriptionResponse is the caller's own decision for one opt-in
// e-mail type (#3780).
type emailSubscriptionResponse struct {
	Enabled bool `json:"enabled"`
}

// getEmailSubscription returns the caller's decision for an e-mail type.
func (rs *Resource) getEmailSubscription(w http.ResponseWriter, r *http.Request) {
	if !rs.preferencesConfigured(w, r) {
		return
	}
	ctx := r.Context()
	enabled, err := rs.PreferenceService.EmailSubscribed(ctx, int64(jwt.ClaimsFromCtx(ctx).ID),
		jwt.PermissionsFromCtx(ctx), chi.URLParam(r, "type"))
	if err != nil {
		renderEmailSubscriptionError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, emailSubscriptionResponse{Enabled: enabled}, "E-mail subscription retrieved successfully")
}

// setEmailSubscription records the caller's decision for an e-mail type.
func (rs *Resource) setEmailSubscription(w http.ResponseWriter, r *http.Request) {
	if !rs.preferencesConfigured(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	req := &preferenceRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	ctx := r.Context()
	if err := rs.PreferenceService.SetEmailSubscribed(ctx, int64(jwt.ClaimsFromCtx(ctx).ID),
		jwt.PermissionsFromCtx(ctx), chi.URLParam(r, "type"), *req.Enabled); err != nil {
		renderEmailSubscriptionError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusNoContent, nil, "")
}

func (rs *Resource) preferencesConfigured(w http.ResponseWriter, r *http.Request) bool {
	if rs.PreferenceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("notification preferences are not configured")))
		return false
	}
	return true
}

// renderEmailSubscriptionError maps the service's refusals: a push or unknown
// type is not found here, a missing permission is forbidden.
func renderEmailSubscriptionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, notificationsService.ErrUnknownNotificationType):
		common.RenderError(w, r, common.ErrorNotFound(errors.New("unknown e-mail notification type")))
	case errors.Is(err, notificationsService.ErrEmailPermissionRequired):
		common.RenderError(w, r, common.ErrorForbidden(errors.New("insufficient permissions")))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
