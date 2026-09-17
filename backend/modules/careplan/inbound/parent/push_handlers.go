package parent

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	notificationsService "github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
)

// pushSubscriptionRequest mirrors the browser's PushSubscription JSON.
type pushSubscriptionRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// getPushPublicKey returns the VAPID public key the browser subscribes with.
func (rs *Resource) getPushPublicKey(w http.ResponseWriter, r *http.Request) {
	key, err := rs.PushService.PublicKey()
	if errors.Is(err, notificationsService.ErrWebPushNotConfigured) {
		common.RenderError(w, r, common.ErrorNotFound(err))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"public_key": key}, "")
}

// subscribePush registers the parent's device across every school the
// account is actively linked to.
func (rs *Resource) subscribePush(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(jwt.ErrTokenUnauthorized))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	req := pushSubscriptionRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if req.Endpoint == "" || req.Keys.P256dh == "" || req.Keys.Auth == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("endpoint and keys are required")))
		return
	}

	err := rs.PushService.SubscribeParent(r.Context(), int64(claims.ID), notificationsService.PushSubscriptionInput{
		Endpoint:      req.Endpoint,
		P256dh:        req.Keys.P256dh,
		Auth:          req.Keys.Auth,
		UserAgent:     r.UserAgent(),
		TokenFamilyID: claims.FamilyID,
	})
	switch {
	case errors.Is(err, notificationsService.ErrWebPushNotConfigured):
		common.RenderError(w, r, common.ErrorConflict(err))
		return
	case errors.Is(err, notificationsService.ErrInvalidPushSubscription):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	case err != nil:
		common.RenderError(w, r, common.ErrorInternalServerWrap("Push-Registrierung konnte nicht gespeichert werden.", err))
		return
	}
	common.Respond(w, r, http.StatusNoContent, nil, "")
}

// unsubscribePush removes the parent's device registration (best effort).
func (rs *Resource) unsubscribePush(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(jwt.ErrTokenUnauthorized))
		return
	}

	endpoint := r.URL.Query().Get("endpoint")
	if endpoint == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("endpoint query parameter is required")))
		return
	}

	if err := rs.PushService.UnsubscribeParent(r.Context(), int64(claims.ID), endpoint); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("Push-Registrierung konnte nicht entfernt werden.", err))
		return
	}
	common.Respond(w, r, http.StatusNoContent, nil, "")
}
