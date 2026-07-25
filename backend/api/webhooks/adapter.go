package webhooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// maxEventsPerRequest bounds one delivery. Providers batch; nobody batches
// more than this, and an unbounded batch is a cheap way to hold a DB
// transaction open.
const maxEventsPerRequest = 100

// Adapter translates a provider's wire format into the neutral event shape.
// The {provider} path segment plus this interface is the extension point: a
// Postmark or SES adapter lands here without a route change or a schema
// change.
type Adapter interface {
	Name() string
	Parse(body []byte) ([]platformSvc.DeliveryEvent, error)
}

// adapters is the registry of supported providers.
var adapters = map[string]Adapter{
	genericAdapter{}.Name(): genericAdapter{},
}

// adapterFor resolves the provider path segment.
func adapterFor(provider string) (Adapter, bool) {
	a, ok := adapters[strings.ToLower(strings.TrimSpace(provider))]
	return a, ok
}

// genericPayload is the provider-neutral wire format. Documented in
// docs/email-delivery-webhook.md; a provider-specific adapter maps onto the
// same neutral events.
type genericPayload struct {
	Events []genericEvent `json:"events"`
}

type genericEvent struct {
	EventID           string         `json:"event_id"`
	Type              string         `json:"type"`
	OccurredAt        time.Time      `json:"occurred_at"`
	MessageID         string         `json:"message_id"`
	ProviderMessageID string         `json:"provider_message_id"`
	Recipient         string         `json:"recipient"`
	BounceClass       string         `json:"bounce_class"`
	ReasonCode        string         `json:"reason_code"`
	Reason            string         `json:"reason"`
	Raw               map[string]any `json:"raw"`
}

type genericAdapter struct{}

func (genericAdapter) Name() string { return "generic" }

func (a genericAdapter) Parse(body []byte) ([]platformSvc.DeliveryEvent, error) {
	var payload genericPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	if len(payload.Events) == 0 {
		return nil, errors.New("payload contains no events")
	}
	if len(payload.Events) > maxEventsPerRequest {
		return nil, fmt.Errorf("payload exceeds %d events", maxEventsPerRequest)
	}

	out := make([]platformSvc.DeliveryEvent, 0, len(payload.Events))
	for _, ev := range payload.Events {
		if ev.EventID == "" || ev.OccurredAt.IsZero() {
			return nil, errors.New("event_id and occurred_at are required")
		}
		if ev.MessageID == "" && ev.ProviderMessageID == "" {
			return nil, errors.New("event needs message_id or provider_message_id")
		}
		if !knownEventType(ev.Type) {
			// Skipped rather than rejected: an unknown event type from a
			// future provider version must not make the whole batch fail and
			// retry forever.
			continue
		}
		bounceClass, err := normalizeBounceClass(ev.BounceClass)
		if err != nil {
			return nil, err
		}
		out = append(out, platformSvc.DeliveryEvent{
			Provider:          a.Name(),
			EventID:           ev.EventID,
			Type:              ev.Type,
			OccurredAt:        ev.OccurredAt,
			MessageID:         ev.MessageID,
			ProviderMessageID: ev.ProviderMessageID,
			Recipient:         ev.Recipient,
			BounceClass:       bounceClass,
			ReasonCode:        ev.ReasonCode,
			Reason:            ev.Reason,
			Raw:               ev.Raw,
		})
	}
	return out, nil
}

func normalizeBounceClass(class string) (string, error) {
	class = strings.ToLower(strings.TrimSpace(class))
	switch class {
	case "", platformModels.BounceClassHard, platformModels.BounceClassSoft:
		return class, nil
	default:
		return "", fmt.Errorf("unsupported bounce_class %q", class)
	}
}

func knownEventType(t string) bool {
	switch t {
	case platformModels.DeliveryEventQueued, platformModels.DeliveryEventDelivered,
		platformModels.DeliveryEventDeferred, platformModels.DeliveryEventBounced,
		platformModels.DeliveryEventComplained, platformModels.DeliveryEventSuppressed:
		return true
	default:
		return false
	}
}
