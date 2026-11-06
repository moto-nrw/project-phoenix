package lettermint

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func webhookBody(event, dataExtra string) []byte {
	return []byte(`{"id":"9b0c4a4e-4e29-4d8b-8b3a-3f0f3e6d2f9b","event":"` + event + `","timestamp":"2026-11-06T12:00:30.000Z",` +
		`"data":{"message_id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","recipient":"eltern@example.com",` +
		`"headers":[{"name":"x-moto-outbox-id","value":"token-1"},{"name":"X-Other","value":"z"}]` + dataExtra + `}}`)
}

func TestParseFindsIdentityAndTime(t *testing.T) {
	t.Parallel()
	reported, err := NewWebhook(testSecret).Parse(webhookBody("message.delivered", ""))

	require.NoError(t, err)
	require.True(t, reported.Known)
	event := reported.Event
	assert.Equal(t, "lettermint", event.Provider)
	assert.Equal(t, "9b0c4a4e-4e29-4d8b-8b3a-3f0f3e6d2f9b", event.ProviderEventID)
	assert.Equal(t, "f47ac10b-58cc-4372-a567-0e02b2c3d479", event.ProviderMessageID)
	assert.Equal(t, "token-1", event.Token, "header names are compared without case")
	assert.True(t, event.OccurredAt.Equal(time.Date(2026, 11, 6, 12, 0, 30, 0, time.UTC)))
}

func TestParseMapsEveryEvent(t *testing.T) {
	t.Parallel()
	bounced := `,"response":{"status_code":550,"enhanced_status_code":"5.1.1","content":"user unknown"}`
	blocked := `,"response":{"status_code":554,"enhanced_status_code":"5.7.1","content":"Service unavailable; client host blocked using zen.spamhaus.org"}`
	softBlocked := `,"response":{"status_code":451,"enhanced_status_code":"4.7.1","content":"listed on a DNSBL, try later"}`
	for name, test := range map[string]struct {
		event, extra string
		state        domain.EmailDeliveryState
		keepsAddress bool
		reroute      bool
	}{
		"delivered":           {event: "message.delivered", state: domain.EmailDeliveryDelivered},
		"soft bounce":         {event: "message.soft_bounced", extra: `,"response":{"status_code":451,"content":"mailbox busy"}`, state: domain.EmailDeliveryDelayed},
		"blocked soft bounce": {event: "message.soft_bounced", extra: softBlocked, state: domain.EmailDeliveryDelayed, keepsAddress: true, reroute: true},
		"hard bounce":         {event: "message.hard_bounced", extra: bounced, state: domain.EmailDeliveryFailed},
		"policy hard bounce":  {event: "message.hard_bounced", extra: `,"response":{"status_code":550,"enhanced_status_code":"5.7.26","content":"unauthenticated"}`, state: domain.EmailDeliveryFailed, keepsAddress: true},
		"blocked hard bounce": {event: "message.hard_bounced", extra: blocked, state: domain.EmailDeliveryFailed, keepsAddress: true, reroute: true},
		"failed":              {event: "message.failed", extra: `,"reason":"TLS","reason_code":"enforced_tls_failed"`, state: domain.EmailDeliveryFailed, keepsAddress: true, reroute: true},
		"suppressed":          {event: "message.suppressed", extra: `,"reason":"hard_bounce","scope":"team"`, state: domain.EmailDeliverySuppressed, reroute: true},
		"complaint":           {event: "message.spam_complaint", state: domain.EmailDeliveryComplained},
		"policy rejected":     {event: "message.policy_rejected", extra: `,"reason":"spam score"`, state: domain.EmailDeliveryFailed, keepsAddress: true, reroute: true},
		"unsubscribed":        {event: "message.unsubscribed", state: domain.EmailDeliverySuppressed},
		"name without prefix": {event: "delivered", state: domain.EmailDeliveryDelivered},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			reported, err := NewWebhook(testSecret).Parse(webhookBody(test.event, test.extra))

			require.NoError(t, err)
			require.True(t, reported.Known)
			assert.Equal(t, test.state, reported.Event.State)
			assert.Equal(t, test.keepsAddress, reported.Event.KeepsAddress, "keeps address")
			assert.Equal(t, test.reroute, reported.Event.Reroute, "reroute")
		})
	}
}

func TestParseKeepsTheServersAnswer(t *testing.T) {
	t.Parallel()
	reported, err := NewWebhook(testSecret).Parse(webhookBody("message.hard_bounced",
		`,"response":{"status_code":550,"enhanced_status_code":"5.1.1","content":"user unknown"}`))

	require.NoError(t, err)
	assert.Equal(t, "550 5.1.1 user unknown", reported.Event.Diagnostic)
}

func TestParseAcknowledgesEventsWithoutAState(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"message.sent", "message.created", "message.opened", "suppression.added", "webhook.test", "message.something_new"} {
		reported, err := NewWebhook(testSecret).Parse(webhookBody(name, ""))

		require.NoError(t, err)
		assert.Nil(t, reported.Event, name)
		assert.Equal(t, name, reported.Name)
	}
	known, err := NewWebhook(testSecret).Parse(webhookBody("webhook.test", ""))
	require.NoError(t, err)
	assert.True(t, known.Known, "moto knows this event and has no state for it")
	unknown, err := NewWebhook(testSecret).Parse(webhookBody("message.something_new", ""))
	require.NoError(t, err)
	assert.False(t, unknown.Known)
}

func TestParseSurvivesAMissingHeaderSnapshot(t *testing.T) {
	t.Parallel()
	for name, data := range map[string]string{
		"none":       `{"message_id":"m1"}`,
		"empty":      `{"message_id":"m1","headers":[]}`,
		"other form": `{"message_id":"m1","headers":{"X-Moto-Outbox-Id":"t"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			reported, err := NewWebhook(testSecret).Parse([]byte(`{"id":"e1","event":"message.delivered","timestamp":"2026-11-06T12:00:30Z","data":` + data + `}`))

			require.NoError(t, err)
			assert.Equal(t, "m1", reported.Event.ProviderMessageID, "the provider's ID still finds the mail")
		})
	}
}

func TestParseRejectsWhatItCannotUse(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"no json":  `not json`,
		"no event": `{"id":"e1","timestamp":"2026-11-06T12:00:30Z"}`,
		"no id":    `{"event":"message.delivered","timestamp":"2026-11-06T12:00:30Z","data":{"message_id":"m1"}}`,
		"no time":  `{"id":"e1","event":"message.delivered","data":{"message_id":"m1"}}`,
		"no mail":  `{"id":"e1","event":"message.delivered","timestamp":"2026-11-06T12:00:30Z","data":{}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := NewWebhook(testSecret).Parse([]byte(body))
			assert.Error(t, err)
		})
	}
}
