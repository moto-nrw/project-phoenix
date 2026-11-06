package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"
)

const maxLoggedEventName = 64

// EmailWebhooks takes in what mail providers report. The signature is the only
// credential, and the school of an event comes from the mail it matches, never
// from the request.
type EmailWebhooks struct {
	store     ports.EmailDeliveryEventStore
	providers map[string]ports.EmailWebhookProvider
	fallback  bool
	logger    *slog.Logger
	observe   ports.Observer
	now       func() time.Time
}

// NewEmailWebhooks registers the providers under their path segment. Without a
// fallback transport no event may send a mail again: it would go out over the
// same provider.
func NewEmailWebhooks(store ports.EmailDeliveryEventStore, providers []ports.EmailWebhookProvider, fallback bool, logger *slog.Logger, observe ports.Observer) *EmailWebhooks {
	byName := make(map[string]ports.EmailWebhookProvider, len(providers))
	for _, provider := range providers {
		byName[provider.Name()] = provider
	}
	return &EmailWebhooks{store: store, providers: byName, fallback: fallback, logger: logger, observe: observe, now: time.Now}
}

// IngestEmailWebhook verifies one delivery on its raw body, before anything
// decodes it, and records the event. An event moto has no state for, or for a
// mail it never sent, is no error: the provider must not retry it.
func (w *EmailWebhooks) IngestEmailWebhook(ctx context.Context, providerName string, header func(string) string, body []byte) error {
	provider, ok := w.providers[strings.ToLower(strings.TrimSpace(providerName))]
	if !ok {
		return deliveryContract.ErrEmailWebhookUnknownProvider
	}
	if !provider.Configured() {
		return deliveryContract.ErrEmailWebhookNotConfigured
	}
	if err := provider.Verify(header, body, w.now()); err != nil {
		w.logger.Warn("email webhook delivery rejected",
			slog.String("provider", provider.Name()),
			slog.String("reason", err.Error()))
		return deliveryContract.ErrEmailWebhookSignature
	}
	reported, err := provider.Parse(body)
	if err != nil {
		w.logger.Warn("email webhook payload rejected",
			slog.String("provider", provider.Name()),
			slog.String("reason", err.Error()))
		return fmt.Errorf("%w: %v", deliveryContract.ErrEmailWebhookPayload, err)
	}
	if reported.Event == nil {
		w.acknowledge(provider.Name(), reported)
		return nil
	}
	event := *reported.Event
	event.Reroute = event.Reroute && w.fallback
	result, err := RecordEmailDeliveryEvent(ctx, w.store, event)
	w.observe(domain.Observation{Operation: "email_webhook_event", Count: boolCount(result.Recorded), Err: err})
	if err != nil {
		return fmt.Errorf("record webhook event: %w", err)
	}
	return nil
}

func (w *EmailWebhooks) acknowledge(provider string, reported domain.ReportedEmailEvent) {
	if reported.Known {
		return
	}
	name := reported.Name
	if len(name) > maxLoggedEventName {
		name = name[:maxLoggedEventName]
	}
	w.logger.Warn("unknown email webhook event",
		slog.String("provider", provider),
		slog.String("event", name))
}
