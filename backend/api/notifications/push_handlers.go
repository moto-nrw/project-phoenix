package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	notificationsService "github.com/moto-nrw/project-phoenix/services/notifications"
)

// The Handle* functions below are shared with the parent portal
// (api/parent/push_handlers.go): both portals speak the same wire format and
// differ only in which PushSubscriptionService methods they bind.

// pushSubscriptionRequest mirrors the browser's PushSubscription JSON.
type pushSubscriptionRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func (req *pushSubscriptionRequest) validate() error {
	if req.Endpoint == "" {
		return errors.New("endpoint is required")
	}
	if req.Keys.P256dh == "" || req.Keys.Auth == "" {
		return errors.New("subscription keys are required")
	}
	return nil
}

// HandlePushPublicKey serves the VAPID public key the browser subscribes with.
func HandlePushPublicKey(w http.ResponseWriter, r *http.Request, service notificationsService.PushSubscriptionService) {
	key, err := service.PublicKey()
	if errors.Is(err, notificationsService.ErrWebPushNotConfigured) {
		common.RenderError(w, r, common.ErrorNotFound(err))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"public_key": key}, "")
}

// HandleSubscribePush parses the browser subscription and persists it via the
// portal-specific subscribe function (staff Subscribe / parent SubscribeParent).
func HandleSubscribePush(
	w http.ResponseWriter,
	r *http.Request,
	accountID int64,
	subscribe func(ctx context.Context, accountID int64, input notificationsService.PushSubscriptionInput) error,
) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	req := pushSubscriptionRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := req.validate(); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	err := subscribe(r.Context(), accountID, notificationsService.PushSubscriptionInput{
		Endpoint:  req.Endpoint,
		P256dh:    req.Keys.P256dh,
		Auth:      req.Keys.Auth,
		UserAgent: r.UserAgent(),
	})
	switch {
	case errors.Is(err, notificationsService.ErrWebPushNotConfigured):
		common.RenderError(w, r, common.ErrorConflict(err))
	case errors.Is(err, notificationsService.ErrInvalidPushSubscription):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case err != nil:
		common.RenderError(w, r, common.ErrorInternalServerWrap("Push-Registrierung konnte nicht gespeichert werden.", err))
	default:
		common.Respond(w, r, http.StatusNoContent, nil, "")
	}
}

// HandleUnsubscribePush removes the caller's device registration (best
// effort). The endpoint travels as a query parameter because DELETE bodies
// don't survive every proxy layer.
func HandleUnsubscribePush(
	w http.ResponseWriter,
	r *http.Request,
	accountID int64,
	unsubscribe func(ctx context.Context, accountID int64, endpoint string) error,
) {
	endpoint := r.URL.Query().Get("endpoint")
	if endpoint == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("endpoint query parameter is required")))
		return
	}
	if err := unsubscribe(r.Context(), accountID, endpoint); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("Push-Registrierung konnte nicht entfernt werden.", err))
		return
	}
	common.Respond(w, r, http.StatusNoContent, nil, "")
}

func (rs *Resource) getPushPublicKey(w http.ResponseWriter, r *http.Request) {
	HandlePushPublicKey(w, r, rs.PushService)
}

func (rs *Resource) subscribePush(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	HandleSubscribePush(w, r, int64(claims.ID), rs.PushService.Subscribe)
}

func (rs *Resource) unsubscribePush(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	HandleUnsubscribePush(w, r, int64(claims.ID), rs.PushService.Unsubscribe)
}
