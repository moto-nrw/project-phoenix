package webpush

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	provider "github.com/SherClockHolmes/webpush-go"

	"github.com/moto-nrw/project-phoenix/modules/delivery"
)

type Sender struct {
	config delivery.WebPushConfig
	client *http.Client
}

func New(config delivery.WebPushConfig) *Sender {
	return &Sender{
		config: config,
		client: &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (s *Sender) SendPush(ctx context.Context, intent delivery.ClaimedIntent) (delivery.ProviderResult, error) {
	if !s.config.Configured() {
		return delivery.ProviderResult{}, errors.New("delivery provider: web push is not configured")
	}
	payload, err := json.Marshal(intent.PushPayload)
	if err != nil {
		return delivery.ProviderResult{}, fmt.Errorf("delivery provider: encode push payload: %w", err)
	}
	response, err := provider.SendNotificationWithContext(ctx, payload, &provider.Subscription{
		Endpoint: intent.PushRecipient.Endpoint,
		Keys:     provider.Keys{P256dh: intent.PushRecipient.P256DH, Auth: intent.PushRecipient.Auth},
	}, &provider.Options{
		Subscriber: s.config.Subscriber, VAPIDPublicKey: s.config.PublicKey,
		VAPIDPrivateKey: s.config.PrivateKey, TTL: 300, HTTPClient: s.client,
	})
	if err != nil {
		return delivery.ProviderResult{}, fmt.Errorf("delivery provider: send push: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return delivery.ProviderResult{StatusCode: response.StatusCode}, fmt.Errorf("delivery provider: push service returned %s", response.Status)
	}
	return delivery.ProviderResult{StatusCode: response.StatusCode}, nil
}
