package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
)

const lettermintHeaderOutbox = "X-Moto-Outbox-Id"

// lettermintBlocklistAnswer recognizes a receiving server that refuses the provider's IP,
// not the address. Then neither the address is at fault nor would a retry over
// the same provider help.
var lettermintBlocklistAnswer = regexp.MustCompile(`(?i)spamhaus|spamcop|barracuda|sorbs|block\s?list|black\s?list|blocked using|\b(rbl|dnsbl)\b|listed (on|in|by)\b|bad reputation`)

// lettermintOutcome is what an event means for the mail.
type lettermintOutcome struct {
	state          domain.EmailDeliveryState
	keepsAddress   bool
	reroute        bool
	blocklistAware bool
}

// lettermintOutcomes maps Lettermint's event names, without their "message." prefix. Only
// a bounce whose answer names a blocklist changes its meaning.
var lettermintOutcomes = map[string]lettermintOutcome{
	"delivered":       {state: domain.EmailDeliveryDelivered},
	"soft_bounced":    {state: domain.EmailDeliveryDelayed, blocklistAware: true},
	"hard_bounced":    {state: domain.EmailDeliveryFailed, blocklistAware: true},
	"failed":          {state: domain.EmailDeliveryFailed, keepsAddress: true, reroute: true},
	"policy_rejected": {state: domain.EmailDeliveryFailed, keepsAddress: true, reroute: true},
	"suppressed":      {state: domain.EmailDeliverySuppressed, reroute: true},
	"spam_complaint":  {state: domain.EmailDeliveryComplained},
	"unsubscribed":    {state: domain.EmailDeliverySuppressed},
}

// lettermintStateless are documented events that say nothing about the outcome. They need
// no log line, unlike an event moto has never heard of.
var lettermintStateless = map[string]bool{
	"message.created": true, "message.sent": true, "message.auto_replied": true,
	"message.opened": true, "message.clicked": true, "message.inbound": true,
	"inbound.rejected": true, "suppression.added": true, "suppression.removed": true,
	"webhook.test": true,
	// The names of the message events API before delivery.
	"scheduled": true, "rescheduled": true, "canceled": true, "released": true,
	"queued": true, "processed": true, "sent": true, "created": true,
}

type lettermintEnvelope struct {
	ID        string    `json:"id"`
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Data      struct {
		MessageID  string          `json:"message_id"`
		Headers    json.RawMessage `json:"headers"`
		Reason     string          `json:"reason"`
		ReasonCode string          `json:"reason_code"`
		Response   *struct {
			StatusCode   int    `json:"status_code"`
			EnhancedCode string `json:"enhanced_status_code"`
			Content      string `json:"content"`
		} `json:"response"`
	} `json:"data"`
}

// Parse reads one verified delivery. The recipient in the body stays unread:
// the mail is found by its identity, and the address is on the mail already.
func (*lettermintWebhook) Parse(body []byte) (domain.ReportedEmailEvent, error) {
	var envelope lettermintEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return domain.ReportedEmailEvent{}, fmt.Errorf("lettermint webhook: decode: %w", err)
	}
	if envelope.Event == "" {
		return domain.ReportedEmailEvent{}, errors.New("lettermint webhook: event name is missing")
	}
	reported := domain.ReportedEmailEvent{Name: envelope.Event, Known: lettermintStateless[envelope.Event]}
	outcome, ok := lettermintOutcomes[strings.TrimPrefix(envelope.Event, "message.")]
	if !ok {
		return reported, nil
	}
	if envelope.ID == "" || envelope.Timestamp.IsZero() || envelope.Data.MessageID == "" {
		return domain.ReportedEmailEvent{}, errors.New("lettermint webhook: event id, timestamp or message id is missing")
	}
	diagnostic := envelope.diagnostic()
	event := outcome.event(diagnostic)
	event.KeepsAddress = event.KeepsAddress || envelope.answerClearsAddress()
	event.ProviderEventID = envelope.ID
	event.OccurredAt = envelope.Timestamp
	event.ProviderMessageID = envelope.Data.MessageID
	event.Token = lettermintOutboxToken(envelope.Data.Headers)
	reported.Known, reported.Event = true, &event
	return reported, nil
}

func (o lettermintOutcome) event(diagnostic string) domain.EmailDeliveryEvent {
	blocked := o.blocklistAware && lettermintBlocklistAnswer.MatchString(diagnostic)
	return domain.EmailDeliveryEvent{
		Provider: lettermintProviderName, State: o.state, Diagnostic: diagnostic,
		KeepsAddress: o.keepsAddress || blocked, Reroute: o.reroute || blocked,
	}
}

// answerClearsAddress applies the rule of the school's list to a bounce: only
// 5.1.x proves a wrong address. An answer without an enhanced code leaves
// Lettermint's verdict standing.
func (e lettermintEnvelope) answerClearsAddress() bool {
	if e.Data.Response == nil {
		return false
	}
	enhanced := e.Data.Response.EnhancedCode
	return strings.HasPrefix(enhanced, "5.") && !strings.HasPrefix(enhanced, "5.1.")
}

func (e lettermintEnvelope) diagnostic() string {
	parts := []string{}
	if response := e.Data.Response; response != nil {
		if response.StatusCode != 0 {
			parts = append(parts, strconv.Itoa(response.StatusCode))
		}
		parts = append(parts, response.EnhancedCode, response.Content)
	}
	parts = append(parts, e.Data.ReasonCode, e.Data.Reason)
	kept := parts[:0]
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " ")
}

// lettermintOutboxToken reads moto's header from the snapshot. Lettermint documents a
// list and an empty one for old mail; any other form falls back to its ID.
func lettermintOutboxToken(raw json.RawMessage) string {
	var headers []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &headers) != nil {
		return ""
	}
	for _, header := range headers {
		if strings.EqualFold(header.Name, lettermintHeaderOutbox) {
			return strings.TrimSpace(header.Value)
		}
	}
	return ""
}
