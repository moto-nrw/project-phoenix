package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	"github.com/moto-ogs/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/delivery/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubWebhookProvider answers as configured and records what reached it.
type stubWebhookProvider struct {
	unconfigured bool
	verifyErr    error
	parsed       domain.ReportedEmailEvent
	parseErr     error
	parseCalls   int
}

func (*stubWebhookProvider) Name() string       { return "lettermint" }
func (p *stubWebhookProvider) Configured() bool { return !p.unconfigured }
func (p *stubWebhookProvider) Verify(func(string) string, []byte, time.Time) error {
	return p.verifyErr
}
func (p *stubWebhookProvider) Parse([]byte) (domain.ReportedEmailEvent, error) {
	p.parseCalls++
	return p.parsed, p.parseErr
}

type failingEventStore struct {
	eventWorkerStore
	err error
}

func (s *failingEventStore) RecordEmailDeliveryEvent(context.Context, domain.EmailDeliveryEvent) (domain.EmailDeliveryEventResult, error) {
	return domain.EmailDeliveryEventResult{}, s.err
}

func webhookEvent(reroute bool) *domain.EmailDeliveryEvent {
	return &domain.EmailDeliveryEvent{
		ProviderMessageID: "m1", Provider: "lettermint", ProviderEventID: "e1",
		State: domain.EmailDeliverySuppressed, OccurredAt: time.Now(), Reroute: reroute,
	}
}

func noHeaders(string) string { return "" }

func ingestFor(store *eventWorkerStore, provider *stubWebhookProvider, fallback bool) *EmailWebhooks {
	return NewEmailWebhooks(store, []ports.EmailWebhookProvider{provider}, fallback, slog.New(slog.DiscardHandler), func(domain.Observation) {})
}

func TestWebhookRefusalsWriteNothing(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		providerName string
		provider     *stubWebhookProvider
		want         error
	}{
		"unknown provider": {"other", &stubWebhookProvider{}, deliveryContract.ErrEmailWebhookUnknownProvider},
		"no secret":        {"lettermint", &stubWebhookProvider{unconfigured: true}, deliveryContract.ErrEmailWebhookNotConfigured},
		"bad signature":    {"lettermint", &stubWebhookProvider{verifyErr: errors.New("mismatch")}, deliveryContract.ErrEmailWebhookSignature},
		"bad payload":      {"lettermint", &stubWebhookProvider{parseErr: errors.New("no json")}, deliveryContract.ErrEmailWebhookPayload},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := &eventWorkerStore{}

			err := ingestFor(store, test.provider, true).IngestEmailWebhook(context.Background(), test.providerName, noHeaders, []byte(`{}`))

			assert.ErrorIs(t, err, test.want)
			assert.Empty(t, store.events)
		})
	}
}

func TestWebhookDoesNotReadABodyItCouldNotVerify(t *testing.T) {
	t.Parallel()
	provider := &stubWebhookProvider{verifyErr: errors.New("mismatch")}

	err := ingestFor(&eventWorkerStore{}, provider, true).IngestEmailWebhook(context.Background(), "lettermint", noHeaders, []byte(`{}`))

	require.Error(t, err)
	assert.Zero(t, provider.parseCalls)
}

func TestWebhookRecordsTheEvent(t *testing.T) {
	t.Parallel()
	store := &eventWorkerStore{}
	provider := &stubWebhookProvider{parsed: domain.ReportedEmailEvent{Name: "message.suppressed", Known: true, Event: webhookEvent(true)}}

	require.NoError(t, ingestFor(store, provider, true).IngestEmailWebhook(context.Background(), "Lettermint", noHeaders, []byte(`{}`)))

	require.Len(t, store.events, 1)
	assert.True(t, store.events[0].Reroute)
}

func TestWebhookWithoutFallbackTransportNeverReroutes(t *testing.T) {
	t.Parallel()
	store := &eventWorkerStore{}
	provider := &stubWebhookProvider{parsed: domain.ReportedEmailEvent{Name: "message.suppressed", Known: true, Event: webhookEvent(true)}}

	require.NoError(t, ingestFor(store, provider, false).IngestEmailWebhook(context.Background(), "lettermint", noHeaders, []byte(`{}`)))

	require.Len(t, store.events, 1)
	assert.False(t, store.events[0].Reroute, "the mail would go out over the same provider again")
}

func TestWebhookAcknowledgesEventsWithoutAState(t *testing.T) {
	t.Parallel()
	for _, known := range []bool{true, false} {
		store := &eventWorkerStore{}
		provider := &stubWebhookProvider{parsed: domain.ReportedEmailEvent{Name: "message.future", Known: known}}

		require.NoError(t, ingestFor(store, provider, true).IngestEmailWebhook(context.Background(), "lettermint", noHeaders, []byte(`{}`)))

		assert.Empty(t, store.events)
	}
}

func TestWebhookReportsAStoreFailureForARetry(t *testing.T) {
	t.Parallel()
	failure := errors.New("database down")
	store := &failingEventStore{err: failure}
	provider := &stubWebhookProvider{parsed: domain.ReportedEmailEvent{Name: "message.suppressed", Known: true, Event: webhookEvent(false)}}
	webhooks := NewEmailWebhooks(store, []ports.EmailWebhookProvider{provider}, true, slog.New(slog.DiscardHandler), func(domain.Observation) {})

	err := webhooks.IngestEmailWebhook(context.Background(), "lettermint", noHeaders, []byte(`{}`))

	assert.ErrorIs(t, err, failure)
}
