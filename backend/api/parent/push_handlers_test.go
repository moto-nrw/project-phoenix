package parent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	notificationsService "github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/stretchr/testify/assert"
)

type parentPushSubscriptionServiceStub struct {
	notificationsService.PushSubscriptionService
	subscribeErr   error
	unsubscribeErr error
}

func (s parentPushSubscriptionServiceStub) SubscribeParent(context.Context, int64, notificationsService.PushSubscriptionInput) error {
	return s.subscribeErr
}

func (s parentPushSubscriptionServiceStub) UnsubscribeParent(context.Context, int64, string) error {
	return s.unsubscribeErr
}

func TestSubscribePushClassifiesServiceErrors(t *testing.T) {
	tests := []struct {
		name          string
		serviceErr    error
		wantStatus    int
		wantBody      string
		forbiddenBody string
	}{
		{
			name:       "invalid browser subscription",
			serviceErr: notificationsService.ErrInvalidPushSubscription,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:          "tenant persistence failure",
			serviceErr:    errors.New("tenant registration failed: internal detail"),
			wantStatus:    http.StatusInternalServerError,
			wantBody:      "Push-Registrierung konnte nicht gespeichert werden.",
			forbiddenBody: "internal detail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := &Resource{PushService: parentPushSubscriptionServiceStub{subscribeErr: tt.serviceErr}}
			req := withClaims(
				httptest.NewRequest(
					http.MethodPost,
					"/parent/me/push/subscriptions",
					strings.NewReader(`{"endpoint":"https://fcm.googleapis.com/fcm/send/device","keys":{"p256dh":"key","auth":"auth"}}`),
				),
				42,
			)
			rec := httptest.NewRecorder()

			rs.subscribePush(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantBody != "" {
				assert.Contains(t, rec.Body.String(), tt.wantBody)
			}
			if tt.forbiddenBody != "" {
				assert.NotContains(t, rec.Body.String(), tt.forbiddenBody)
			}
		})
	}
}

func TestUnsubscribePushHidesServiceErrors(t *testing.T) {
	rs := &Resource{PushService: parentPushSubscriptionServiceStub{
		unsubscribeErr: errors.New("delete parent subscription: pq: internal detail"),
	}}
	req := withClaims(
		httptest.NewRequest(
			http.MethodDelete,
			"/parent/me/push/subscriptions?endpoint=https%3A%2F%2Ffcm.googleapis.com%2Ffcm%2Fsend%2Fdevice",
			nil,
		),
		42,
	)
	rec := httptest.NewRecorder()

	rs.unsubscribePush(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "Push-Registrierung konnte nicht entfernt werden.")
	assert.NotContains(t, rec.Body.String(), "internal detail")
}
