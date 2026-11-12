package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/delivery/internal/domain"
)

const lettermintHeaderOutbox = "X-Moto-Outbox-Id"

// lettermintBlocklistAnswer recognizes a receiving server that refuses the provider's IP
// or its reputation, not the address. Then neither the address is at fault nor
// would a retry over the same provider help.
var lettermintBlocklistAnswer = regexp.MustCompile(`(?i)spamhaus|spamcop|barracuda|sorbs|block\s?list|black\s?list|blocked using|\b(rbl|dnsbl)\b|listed (on|in|by)\b|reputation|\b5\.7\.606\b|(banned|blocked)\s+(sending|sender)\s+ip`)

// lettermintDottedNumber takes every dotted number of an answer whole, so an IP like
// 5.1.1.4 does not read as a status.
var lettermintDottedNumber = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)*`)

// lettermintOutcome is what an event means for the mail.
type lettermintOutcome struct {
	state          domain.EmailDeliveryState
	keepsAddress   bool
	reroute        bool
	blocklistAware bool
}

// lettermintOutcomes maps Lettermint's event names, without their "message." prefix. Only
// a bounce whose answer blames the sender's IP changes its meaning: see
// answerBlamesTheSender.
var lettermintOutcomes = map[string]lettermintOutcome{
	"delivered":       {state: domain.EmailDeliveryDelivered},
	"soft_bounced":    {state: domain.EmailDeliveryDelayed, blocklistAware: true},
	"hard_bounced":    {state: domain.EmailDeliveryFailed, blocklistAware: true},
	"failed":          {state: domain.EmailDeliveryFailed, keepsAddress: true, reroute: true},
	"policy_rejected": {state: domain.EmailDeliveryFailed, keepsAddress: true, reroute: true},
	"suppressed":      {state: domain.EmailDeliverySuppressed, reroute: true},
	"spam_complaint":  {state: domain.EmailDeliveryComplained},
	"unsubscribed":    {state: domain.EmailDeliveryUnsubscribed},
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

var lettermintAddressStatuses = map[string]bool{"5.1.1": true, "5.1.2": true, "5.1.3": true, "5.1.4": true, "5.1.6": true, "5.1.10": true}

type lettermintEnvelope struct {
	ID        string    `json:"id"`
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Data      struct {
		MessageID  string          `json:"message_id"`
		Headers    json.RawMessage `json:"headers"`
		Reason     lettermintText  `json:"reason"`
		ReasonCode lettermintText  `json:"reason_code"`
		Response   *struct {
			StatusCode   int    `json:"status_code"`
			EnhancedCode string `json:"enhanced_status_code"`
			Content      string `json:"content"`
		} `json:"response"`
	} `json:"data"`
}

// lettermintText reads a text field the provider may also send as a number.
// Other forms say nothing moto could show, so they read as empty.
type lettermintText string

func (t *lettermintText) UnmarshalJSON(raw []byte) error {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		*t = lettermintText(text)
		return nil
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		*t = lettermintText(number.String())
		return nil
	}
	*t = ""
	return nil
}

// Parse reads one verified delivery. The name comes first: an event moto has no
// state for is acknowledged whatever its other fields look like. The recipient
// in the body stays unread: the mail is found by its identity, and the address
// is on the mail already.
func (*lettermintWebhook) Parse(body []byte) (domain.ReportedEmailEvent, error) {
	var name struct {
		Event string `json:"event"`
	}
	if err := json.Unmarshal(body, &name); err != nil {
		return domain.ReportedEmailEvent{}, fmt.Errorf("lettermint webhook: decode the event name: %w", err)
	}
	if name.Event == "" {
		return domain.ReportedEmailEvent{}, errors.New("lettermint webhook: event name is missing")
	}
	reported := domain.ReportedEmailEvent{Name: name.Event, Known: lettermintStateless[name.Event]}
	outcome, ok := lettermintOutcomes[strings.TrimPrefix(name.Event, "message.")]
	if !ok {
		return reported, nil
	}
	var envelope lettermintEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return domain.ReportedEmailEvent{}, fmt.Errorf("lettermint webhook: decode: %w", err)
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
	blocked := o.blocklistAware && answerBlamesTheSender(diagnostic)
	return domain.EmailDeliveryEvent{
		Provider: lettermintProviderName, State: o.state, Diagnostic: diagnostic,
		KeepsAddress: o.keepsAddress || blocked, Reroute: o.reroute || blocked,
	}
}

// answerBlamesTheSender is the rule for a bounce's answer, in this order:
//
//  1. An address status (5.1.1, 5.1.2, 5.1.3, 5.1.4, 5.1.6, 5.1.10, also as
//     4.x.x) proves the address is at fault, whatever words follow: "5.1.1 not
//     listed in Domino Directory" is no blocklist.
//  2. An answer naming a blocklist, the sending IP or a reputation blames moto's
//     way out. The address stays free and a security mail takes the fallback.
//  3. Anything else leaves the event's own meaning.
func answerBlamesTheSender(answer string) bool {
	for _, number := range lettermintDottedNumber.FindAllString(answer, -1) {
		if (number[0] == '4' || number[0] == '5') && lettermintAddressStatuses["5"+number[1:]] {
			return false
		}
	}
	return lettermintBlocklistAnswer.MatchString(answer)
}

// answerClearsAddress applies the rule of the school's list to a bounce: only
// the statuses that classifySendError also accepts prove an unusable address. An
// answer without an enhanced code leaves Lettermint's verdict standing.
func (e lettermintEnvelope) answerClearsAddress() bool {
	if e.Data.Response == nil {
		return false
	}
	enhanced := e.Data.Response.EnhancedCode
	return strings.HasPrefix(enhanced, "5.") && !lettermintAddressStatuses[enhanced]
}

func (e lettermintEnvelope) diagnostic() string {
	parts := []string{}
	if response := e.Data.Response; response != nil {
		if response.StatusCode != 0 {
			parts = append(parts, strconv.Itoa(response.StatusCode))
		}
		parts = append(parts, response.EnhancedCode, response.Content)
	}
	parts = append(parts, string(e.Data.ReasonCode), string(e.Data.Reason))
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
