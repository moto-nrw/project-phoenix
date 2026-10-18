package notifications

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
)

// pushSubscriptionRequest mirrors the browser's PushSubscription JSON.
type pushSubscriptionRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// Bind implements render.Binder.
func (req *pushSubscriptionRequest) Bind(_ *http.Request) error {
	if req.Endpoint == "" {
		return errors.New("endpoint is required")
	}
	if req.Keys.P256dh == "" || req.Keys.Auth == "" {
		return errors.New("subscription keys are required")
	}
	return nil
}

// getPushPublicKey returns the VAPID public key the browser subscribes with.
func (rs *Resource) getPushPublicKey(w http.ResponseWriter, r *http.Request) {
	key, err := rs.PushService.PublicKey()
	if rs.PushService.IsNotConfigured(err) {
		rs.Runtime.NotFound(w, r, err)
		return
	}
	rs.Runtime.Respond(w, r, http.StatusOK, map[string]string{"public_key": key}, "")
}

// subscribePush registers (or refreshes) the caller's device for Web Push.
func (rs *Resource) subscribePush(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	req := &pushSubscriptionRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.Runtime.Invalid(w, r, err)
		return
	}
	claims := rs.Runtime.Caller(r.Context())
	input := PushSubscriptionInput{
		Endpoint:      req.Endpoint,
		P256dh:        req.Keys.P256dh,
		Auth:          req.Keys.Auth,
		UserAgent:     r.UserAgent(),
		TokenFamilyID: claims.FamilyID,
	}
	var err error
	if requestPortal(r) == PortalSchool {
		err = rs.PushService.SubscribeSchool(r.Context(), int64(claims.ID), input)
	} else {
		err = rs.PushService.Subscribe(r.Context(), int64(claims.ID), input)
	}
	switch {
	case rs.PushService.IsNotConfigured(err):
		rs.Runtime.Conflict(w, r, err)
		return
	case rs.PushService.IsInvalidSubscription(err):
		rs.Runtime.Invalid(w, r, err)
		return
	case err != nil:
		rs.Runtime.Wrapped(w, r, "Push-Registrierung konnte nicht gespeichert werden.", err)
		return
	}
	rs.Runtime.Respond(w, r, http.StatusNoContent, nil, "")
}

// unsubscribePush removes the caller's device registration (best effort).
// The endpoint travels as a query parameter because DELETE bodies don't
// survive every proxy layer.
func (rs *Resource) unsubscribePush(w http.ResponseWriter, r *http.Request) {
	endpoint := r.URL.Query().Get("endpoint")
	if endpoint == "" {
		rs.Runtime.Invalid(w, r, errors.New("endpoint query parameter is required"))
		return
	}
	claims := rs.Runtime.Caller(r.Context())
	var err error
	if requestPortal(r) == PortalSchool {
		err = rs.PushService.UnsubscribeSchool(r.Context(), int64(claims.ID), endpoint)
	} else {
		err = rs.PushService.Unsubscribe(r.Context(), int64(claims.ID), endpoint)
	}
	if err != nil {
		rs.Runtime.Wrapped(w, r, "Push-Registrierung konnte nicht entfernt werden.", err)
		return
	}
	rs.Runtime.Respond(w, r, http.StatusNoContent, nil, "")
}
