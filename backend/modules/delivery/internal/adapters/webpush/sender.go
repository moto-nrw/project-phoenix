package webpush

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	provider "github.com/SherClockHolmes/webpush-go"
)

type Sender struct {
	config  deliveryContract.WebPushConfig
	client  *http.Client
	cleanup func(context.Context, deliveryContract.ClaimedIntent) error
}

func New(config deliveryContract.WebPushConfig, cleanup func(context.Context, deliveryContract.ClaimedIntent) error) *Sender {
	return &Sender{
		config:  config,
		cleanup: cleanup,
		client: &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (s *Sender) SendPush(ctx context.Context, intent deliveryContract.ClaimedIntent) (deliveryContract.ProviderResult, error) {
	if !s.config.Configured() {
		return deliveryContract.ProviderResult{}, errors.New("delivery provider: web push is not configured")
	}
	if err := deliveryContract.ValidatePushEndpoint(intent.PushRecipient.Endpoint); err != nil {
		return deliveryContract.ProviderResult{}, fmt.Errorf("delivery provider: invalid push endpoint: %w", err)
	}
	payload, err := json.Marshal(intent.PushPayload)
	if err != nil {
		return deliveryContract.ProviderResult{}, fmt.Errorf("delivery provider: encode push payload: %w", err)
	}
	response, err := provider.SendNotificationWithContext(ctx, payload, &provider.Subscription{
		Endpoint: intent.PushRecipient.Endpoint,
		Keys:     provider.Keys{P256dh: intent.PushRecipient.P256DH, Auth: intent.PushRecipient.Auth},
	}, &provider.Options{
		Subscriber: s.config.Subscriber, VAPIDPublicKey: s.config.PublicKey,
		VAPIDPrivateKey: s.config.PrivateKey, TTL: pushTTL(intent.PushPayload.Priority), Urgency: pushUrgency(intent.PushPayload.Priority), HTTPClient: s.client,
	})
	if err != nil {
		return deliveryContract.ProviderResult{}, fmt.Errorf("delivery provider: send push: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusGone {
			if s.cleanup == nil {
				return deliveryContract.ProviderResult{StatusCode: response.StatusCode}, errors.New("delivery provider: expired push subscription cleanup is not configured")
			}
			if err := s.cleanup(ctx, intent); err != nil {
				return deliveryContract.ProviderResult{StatusCode: response.StatusCode}, fmt.Errorf("delivery provider: remove expired push subscription: %w", err)
			}
			return deliveryContract.ProviderResult{StatusCode: response.StatusCode}, deliveryContract.ErrCancelled
		}
		return deliveryContract.ProviderResult{StatusCode: response.StatusCode}, fmt.Errorf("delivery provider: push service returned %s", response.Status)
	}
	return deliveryContract.ProviderResult{StatusCode: response.StatusCode}, nil
}

func pushTTL(priority string) int {
	if priority == "high" {
		return 3600
	}
	return 86400
}

func pushUrgency(priority string) provider.Urgency {
	switch priority {
	case "high":
		return provider.UrgencyHigh
	case "low":
		return provider.UrgencyLow
	default:
		return provider.UrgencyNormal
	}
}
