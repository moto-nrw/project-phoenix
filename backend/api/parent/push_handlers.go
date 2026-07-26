package parent

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	notificationsAPI "github.com/moto-nrw/project-phoenix/api/notifications"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// The push endpoints share their implementation with the staff portal
// (api/notifications): same wire format, same error classification. Only the
// parent claims guard and the parent-specific service methods differ.

// getPushPublicKey returns the VAPID public key the browser subscribes with.
func (rs *Resource) getPushPublicKey(w http.ResponseWriter, r *http.Request) {
	notificationsAPI.HandlePushPublicKey(w, r, rs.PushService)
}

// subscribePush registers the parent's device across every school the
// account is actively linked to.
func (rs *Resource) subscribePush(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(jwt.ErrTokenUnauthorized))
		return
	}
	notificationsAPI.HandleSubscribePush(w, r, int64(claims.ID), rs.PushService.SubscribeParent)
}

// unsubscribePush removes the parent's device registration (best effort).
func (rs *Resource) unsubscribePush(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(jwt.ErrTokenUnauthorized))
		return
	}
	notificationsAPI.HandleUnsubscribePush(w, r, int64(claims.ID), rs.PushService.UnsubscribeParent)
}
