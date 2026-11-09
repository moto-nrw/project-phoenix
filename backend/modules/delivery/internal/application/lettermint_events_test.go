package application

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lettermintBody(event, dataExtra string) []byte {
	return []byte(`{"id":"9b0c4a4e-4e29-4d8b-8b3a-3f0f3e6d2f9b","event":"` + event + `","timestamp":"2026-11-06T12:00:30.000Z",` +
		`"data":{"message_id":"f47ac10b-58cc-4372-a567-0e02b2c3d479","recipient":"eltern@example.com",` +
		`"headers":[{"name":"x-moto-outbox-id","value":"token-1"},{"name":"X-Other","value":"z"}]` + dataExtra + `}}`)
}

func TestLettermintParseFindsIdentityAndTime(t *testing.T) {
	t.Parallel()
	reported, err := newLettermintWebhook(lettermintTestSecret).Parse(lettermintBody("message.delivered", ""))

	require.NoError(t, err)
	require.True(t, reported.Known)
	event := reported.Event
	assert.Equal(t, "lettermint", event.Provider)
	assert.Equal(t, "9b0c4a4e-4e29-4d8b-8b3a-3f0f3e6d2f9b", event.ProviderEventID)
	assert.Equal(t, "f47ac10b-58cc-4372-a567-0e02b2c3d479", event.ProviderMessageID)
	assert.Equal(t, "token-1", event.Token, "header names are compared without case")
	assert.True(t, event.OccurredAt.Equal(time.Date(2026, 11, 6, 12, 0, 30, 0, time.UTC)))
}

func TestLettermintParseMapsEveryEvent(t *testing.T) {
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
		"address status beats the blocklist words": {event: "message.hard_bounced", state: domain.EmailDeliveryFailed,
			extra: `,"response":{"status_code":550,"enhanced_status_code":"5.1.1","content":"user not listed in Domino Directory"}`},
		"address status in the text only": {event: "message.hard_bounced", state: domain.EmailDeliveryFailed,
			extra: `,"response":{"status_code":550,"content":"5.1.1 user not listed in Domino Directory."}`},
		"address status 5.1.10": {event: "message.hard_bounced", state: domain.EmailDeliveryFailed,
			extra: `,"response":{"status_code":550,"enhanced_status_code":"5.1.10","content":"recipient not found by SMTP address lookup, low reputation"}`},
		"address status after another one": {event: "message.hard_bounced", state: domain.EmailDeliveryFailed,
			extra: `,"response":{"status_code":550,"content":"5.7.1 5.1.1 not listed in the directory"}`},
		"temporary address status": {event: "message.soft_bounced", state: domain.EmailDeliveryDelayed,
			extra: `,"response":{"status_code":450,"enhanced_status_code":"4.1.1","content":"user not listed in directory, try later"}`},
		"banned sending IP": {event: "message.hard_bounced", state: domain.EmailDeliveryFailed, keepsAddress: true, reroute: true,
			extra: `,"response":{"status_code":550,"enhanced_status_code":"5.7.606","content":"Access denied, banned sending IP [192.0.2.1]"}`},
		"low reputation": {event: "message.hard_bounced", state: domain.EmailDeliveryFailed, keepsAddress: true, reroute: true,
			extra: `,"response":{"status_code":554,"enhanced_status_code":"5.7.1","content":"rejected due to low reputation of the sending host"}`},
		"reputation on a soft bounce": {event: "message.soft_bounced", state: domain.EmailDeliveryDelayed, keepsAddress: true, reroute: true,
			extra: `,"response":{"status_code":421,"enhanced_status_code":"4.7.0","content":"try later, IP reputation too low"}`},
		"an IP that looks like a status": {event: "message.hard_bounced", state: domain.EmailDeliveryFailed, keepsAddress: true, reroute: true,
			extra: `,"response":{"status_code":554,"enhanced_status_code":"5.7.1","content":"client 5.1.1.4 blocked using zen.spamhaus.org"}`},
		"failed":              {event: "message.failed", extra: `,"reason":"TLS","reason_code":"enforced_tls_failed"`, state: domain.EmailDeliveryFailed, keepsAddress: true, reroute: true},
		"suppressed":          {event: "message.suppressed", extra: `,"reason":"hard_bounce","scope":"team"`, state: domain.EmailDeliverySuppressed, reroute: true},
		"complaint":           {event: "message.spam_complaint", state: domain.EmailDeliveryComplained},
		"policy rejected":     {event: "message.policy_rejected", extra: `,"reason":"spam score"`, state: domain.EmailDeliveryFailed, keepsAddress: true, reroute: true},
		"unsubscribed":        {event: "message.unsubscribed", state: domain.EmailDeliveryUnsubscribed},
		"name without prefix": {event: "delivered", state: domain.EmailDeliveryDelivered},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			reported, err := newLettermintWebhook(lettermintTestSecret).Parse(lettermintBody(test.event, test.extra))

			require.NoError(t, err)
			require.True(t, reported.Known)
			assert.Equal(t, test.state, reported.Event.State)
			assert.Equal(t, test.keepsAddress, reported.Event.KeepsAddress, "keeps address")
			assert.Equal(t, test.reroute, reported.Event.Reroute, "reroute")
		})
	}
}

func TestLettermintParseKeepsTheServersAnswer(t *testing.T) {
	t.Parallel()
	reported, err := newLettermintWebhook(lettermintTestSecret).Parse(lettermintBody("message.hard_bounced",
		`,"response":{"status_code":550,"enhanced_status_code":"5.1.1","content":"user unknown"}`))

	require.NoError(t, err)
	assert.Equal(t, "550 5.1.1 user unknown", reported.Event.Diagnostic)
}

func TestLettermintParseAcknowledgesEventsWithoutAState(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"message.sent", "message.created", "message.opened", "suppression.added", "webhook.test", "message.something_new"} {
		reported, err := newLettermintWebhook(lettermintTestSecret).Parse(lettermintBody(name, ""))

		require.NoError(t, err)
		assert.Nil(t, reported.Event, name)
		assert.Equal(t, name, reported.Name)
	}
	known, err := newLettermintWebhook(lettermintTestSecret).Parse(lettermintBody("webhook.test", ""))
	require.NoError(t, err)
	assert.True(t, known.Known, "moto knows this event and has no state for it")
	unknown, err := newLettermintWebhook(lettermintTestSecret).Parse(lettermintBody("message.something_new", ""))
	require.NoError(t, err)
	assert.False(t, unknown.Known)
}

func TestLettermintParseSurvivesAMissingHeaderSnapshot(t *testing.T) {
	t.Parallel()
	for name, data := range map[string]string{
		"none":       `{"message_id":"m1"}`,
		"empty":      `{"message_id":"m1","headers":[]}`,
		"other form": `{"message_id":"m1","headers":{"X-Moto-Outbox-Id":"t"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			reported, err := newLettermintWebhook(lettermintTestSecret).Parse([]byte(`{"id":"e1","event":"message.delivered","timestamp":"2026-11-06T12:00:30Z","data":` + data + `}`))

			require.NoError(t, err)
			assert.Equal(t, "m1", reported.Event.ProviderMessageID, "the provider's ID still finds the mail")
		})
	}
}

func TestLettermintParseRejectsWhatItCannotUse(t *testing.T) {
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
			_, err := newLettermintWebhook(lettermintTestSecret).Parse([]byte(body))
			assert.Error(t, err)
		})
	}
}

// The issue asks for 2xx on unknown events, so nothing but the name is read.
func TestLettermintParseAcknowledgesUnknownEventsOfAnyShape(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`{"id":7,"event":"message.something_new","timestamp":1762430400,"data":"text"}`,
		`{"event":"message.something_new","data":{"message_id":12,"headers":"none","response":[1,2]}}`,
		`{"id":{"a":1},"event":"webhook.test","data":[]}`,
	} {
		reported, err := newLettermintWebhook(lettermintTestSecret).Parse([]byte(body))

		require.NoError(t, err, body)
		assert.Nil(t, reported.Event, body)
	}
}

func TestLettermintParseStillRefusesABodyWithoutAName(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`not json`, `{"id":"e1"}`, `{"event":7}`, `[]`} {
		_, err := newLettermintWebhook(lettermintTestSecret).Parse([]byte(body))

		assert.Error(t, err, body)
	}
}

func TestLettermintParseReadsANumericReasonCode(t *testing.T) {
	t.Parallel()
	reported, err := newLettermintWebhook(lettermintTestSecret).Parse(lettermintBody("message.failed", `,"reason":"no route","reason_code":421`))

	require.NoError(t, err)
	require.NotNil(t, reported.Event)
	assert.Equal(t, "421 no route", reported.Event.Diagnostic)

	reported, err = newLettermintWebhook(lettermintTestSecret).Parse(lettermintBody("message.failed", `,"reason":"no route","reason_code":null`))
	require.NoError(t, err)
	assert.Equal(t, "no route", reported.Event.Diagnostic)
}
