package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/webhooksig"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "webhook-test-secret"

// stubIngester records what reached the service layer. Behaviourally divergent
// from the real service on purpose (error injection), so it stays local.
type stubIngester struct {
	events []platformSvc.DeliveryEvent
	err    error
}

func (s *stubIngester) IngestBatch(_ context.Context, events []platformSvc.DeliveryEvent) (platformSvc.BatchResult, error) {
	if s.err != nil {
		return platformSvc.BatchResult{}, s.err
	}
	s.events = append(s.events, events...)
	return platformSvc.BatchResult{Accepted: len(events), Applied: len(events)}, nil
}

func newTestResource(t *testing.T, secret string) (*Resource, *stubIngester) {
	t.Helper()
	ingester := &stubIngester{}
	return NewResource(ResourceConfig{
		DeliveryService: ingester,
		Verifier:        webhooksig.NewVerifier(secret),
	}), ingester
}

func validPayload() []byte {
	return []byte(`{"events":[{
		"event_id":"evt_test_1",
		"type":"delivered",
		"occurred_at":"2026-07-25T10:31:02Z",
		"message_id":"ob.1.2.abc@mail.example.de",
		"recipient":"eltern@example.de"
	}]}`)
}

func signedRequest(t *testing.T, secret string, body []byte) *http.Request {
	t.Helper()
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	req := httptest.NewRequest(http.MethodPost, "/email/delivery/generic", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(webhooksig.HeaderTimestamp, ts)
	req.Header.Set(webhooksig.HeaderSignature, webhooksig.Sign(secret, ts, body))
	return req
}

func serve(rs *Resource, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	rs.Router().ServeHTTP(rec, req)
	return rec
}

// This is the regression guard for the whole design: the webhook must answer
// a correctly signed request that carries NO Authorization header. If someone
// ever moves the mount under an authenticated group, every provider
// integration breaks silently — this test fails loudly instead.
func TestWebhookIsNotBehindTenantJWT(t *testing.T) {
	rs, ingester := newTestResource(t, testSecret)

	req := signedRequest(t, testSecret, validPayload())
	require.Empty(t, req.Header.Get("Authorization"), "the test must not authenticate")

	rec := serve(rs, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	require.Len(t, ingester.events, 1)
	assert.Equal(t, "evt_test_1", ingester.events[0].EventID)
	assert.Equal(t, "generic", ingester.events[0].Provider)
}

func TestWebhookRejectsUnsignedRequest(t *testing.T) {
	rs, ingester := newTestResource(t, testSecret)

	req := httptest.NewRequest(http.MethodPost, "/email/delivery/generic", bytes.NewReader(validPayload()))
	rec := serve(rs, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Empty(t, ingester.events, "nothing may reach the service")
	// No detail in the body: distinguishing failure modes builds an oracle.
	assert.NotContains(t, rec.Body.String(), "timestamp")
}

func TestWebhookRejectsTamperedBody(t *testing.T) {
	rs, ingester := newTestResource(t, testSecret)

	req := signedRequest(t, testSecret, validPayload())
	req.Body = http.NoBody
	tampered := bytes.NewReader([]byte(`{"events":[{"event_id":"evt_forged","type":"bounced","occurred_at":"2026-07-25T10:31:02Z","message_id":"x@y"}]}`))
	req = httptest.NewRequest(http.MethodPost, "/email/delivery/generic", tampered)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	req.Header.Set(webhooksig.HeaderTimestamp, ts)
	req.Header.Set(webhooksig.HeaderSignature, webhooksig.Sign(testSecret, ts, validPayload()))

	rec := serve(rs, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Empty(t, ingester.events)
}

// 503, not 401: an operator must be able to tell a missing env var from an
// attack, and a webhook that silently accepts everything is unacceptable.
func TestWebhookReturns503WhenSecretUnset(t *testing.T) {
	rs, ingester := newTestResource(t, "")

	rec := serve(rs, signedRequest(t, testSecret, validPayload()))

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Empty(t, ingester.events)
}

func TestWebhookRejectsUnknownProvider(t *testing.T) {
	rs, _ := newTestResource(t, testSecret)

	body := validPayload()
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	req := httptest.NewRequest(http.MethodPost, "/email/delivery/some-other-provider", bytes.NewReader(body))
	req.Header.Set(webhooksig.HeaderTimestamp, ts)
	req.Header.Set(webhooksig.HeaderSignature, webhooksig.Sign(testSecret, ts, body))

	assert.Equal(t, http.StatusBadRequest, serve(rs, req).Code)
}

func TestWebhookRejectsOversizedBody(t *testing.T) {
	rs, ingester := newTestResource(t, testSecret)

	huge := []byte(`{"events":[{"event_id":"x","type":"delivered","occurred_at":"2026-07-25T10:31:02Z","message_id":"a@b","reason":"` +
		strings.Repeat("A", 2<<20) + `"}]}`)
	rec := serve(rs, signedRequest(t, testSecret, huge))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, ingester.events)
}

// A DB failure is the ONE case a provider should retry, so it is the only one
// that answers 5xx.
func TestWebhookReturns500OnIngestFailure(t *testing.T) {
	ingester := &stubIngester{err: assertAnError{}}
	rs := NewResource(ResourceConfig{
		DeliveryService: ingester,
		Verifier:        webhooksig.NewVerifier(testSecret),
	})

	rec := serve(rs, signedRequest(t, testSecret, validPayload()))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

type assertAnError struct{}

func (assertAnError) Error() string { return "database exploded" }

// An unmatched or stale event is a 202, never a 4xx: a shared provider account
// legitimately reports events about mail we never sent, and a 4xx would make
// the provider retry those forever.
func TestWebhookAcceptsUnmatchedEvents(t *testing.T) {
	ingester := &stubIngester{}
	rs := NewResource(ResourceConfig{
		DeliveryService: ingester,
		Verifier:        webhooksig.NewVerifier(testSecret),
	})

	rec := serve(rs, signedRequest(t, testSecret, validPayload()))
	require.Equal(t, http.StatusAccepted, rec.Code)

	var body platformSvc.BatchResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, 1, body.Accepted)
}

// An unknown event type from a future provider version is skipped, not
// rejected: rejecting would make the provider retry the batch forever.
func TestAdapterSkipsUnknownEventTypes(t *testing.T) {
	events, err := genericAdapter{}.Parse([]byte(`{"events":[
		{"event_id":"a","type":"teleported","occurred_at":"2026-07-25T10:31:02Z","message_id":"a@b"},
		{"event_id":"b","type":"delivered","occurred_at":"2026-07-25T10:31:02Z","message_id":"a@b"}
	]}`))

	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "b", events[0].EventID)
}

func TestAdapterRejectsIncompleteEvents(t *testing.T) {
	for _, payload := range []string{
		`{"events":[]}`,
		`{"events":[{"type":"delivered","occurred_at":"2026-07-25T10:31:02Z","message_id":"a@b"}]}`,
		`{"events":[{"event_id":"a","type":"delivered","message_id":"a@b"}]}`,
		`{"events":[{"event_id":"a","type":"delivered","occurred_at":"2026-07-25T10:31:02Z"}]}`,
	} {
		_, err := genericAdapter{}.Parse([]byte(payload))
		assert.Error(t, err, "payload %s must be rejected", payload)
	}
}

func TestWebhookRejectsUnknownBounceClassWithoutIngesting(t *testing.T) {
	rs, ingester := newTestResource(t, testSecret)
	body := []byte(`{"events":[{
		"event_id":"evt_unknown_bounce",
		"type":"bounced",
		"occurred_at":"2026-07-25T10:31:02Z",
		"message_id":"ob.1.2.abc@mail.example.de",
		"bounce_class":"permanent"
	}]}`)

	rec := serve(rs, signedRequest(t, testSecret, body))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, ingester.events)
}

func TestAdapterNormalizesKnownBounceClass(t *testing.T) {
	events, err := genericAdapter{}.Parse([]byte(`{"events":[{
		"event_id":"evt_soft_bounce",
		"type":"bounced",
		"occurred_at":"2026-07-25T10:31:02Z",
		"message_id":"ob.1.2.abc@mail.example.de",
		"bounce_class":" Soft "
	}]}`))

	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "soft", events[0].BounceClass)
}

func TestAdapterExtractsPhoenixOutboxHeader(t *testing.T) {
	events, err := genericAdapter{}.Parse([]byte(`{"events":[{
		"event_id":"evt_rewritten",
		"type":"delivered",
		"occurred_at":"2026-07-25T10:31:02Z",
		"message_id":"rewritten@relay.example",
		"headers":{"x-phoenix-outbox-id":" <ob.1.2.abc@mail.example.de> "}
	}]}`))

	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "<ob.1.2.abc@mail.example.de>", events[0].FallbackMessageID)
}

func TestAdapterAcceptsPhoenixOutboxHeaderAsOnlyCorrelation(t *testing.T) {
	events, err := genericAdapter{}.Parse([]byte(`{"events":[{
		"event_id":"evt_header_only",
		"type":"delivered",
		"occurred_at":"2026-07-25T10:31:02Z",
		"headers":{"X-Phoenix-Outbox-Id":"ob.1.2.abc@mail.example.de"}
	}]}`))

	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "ob.1.2.abc@mail.example.de", events[0].FallbackMessageID)
}

func TestAdapterRejectsOversizedBatch(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`{"events":[`)
	for i := 0; i <= maxEventsPerRequest; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"event_id":"e` + strconv.Itoa(i) +
			`","type":"delivered","occurred_at":"2026-07-25T10:31:02Z","message_id":"a@b"}`)
	}
	sb.WriteString(`]}`)

	_, err := genericAdapter{}.Parse([]byte(sb.String()))
	assert.Error(t, err)
}
