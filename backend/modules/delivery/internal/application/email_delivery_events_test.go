package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// eventWorkerStore is the worker store with a suppression list.
type eventWorkerStore struct {
	workerStore
	suppressed bool
	stopped    bool
	// traegerListed answers the read of a Träger's mail, so a test tells it apart
	// from the school's.
	traegerListed bool
	events        []domain.EmailDeliveryEvent
}

func (s *eventWorkerStore) RecordEmailDeliveryEvent(_ context.Context, event domain.EmailDeliveryEvent) (domain.EmailDeliveryEventResult, error) {
	s.events = append(s.events, event)
	return domain.EmailDeliveryEventResult{Matched: true, Recorded: true, State: event.State}, nil
}

func (s *eventWorkerStore) SchoolEmailHolds(context.Context, int64, string) (bool, bool, error) {
	return s.suppressed, s.stopped, nil
}

func (s *eventWorkerStore) TraegerEmailHolds(context.Context, int64, string) (bool, bool, error) {
	return s.traegerListed, false, nil
}

func eventMail(security bool) domain.Intent {
	intent := claimedEmail(0)
	intent.Template = "parent_announcement"
	intent.Security = security
	intent.DeliveryToken = "token-1"
	intent.Recipient = json.RawMessage(`{"address":"guardian@example.com"}`)
	return intent
}

func TestWorkerHoldsBackAMailToASuppressedAddress(t *testing.T) {
	t.Parallel()
	store := &eventWorkerStore{suppressed: true}
	store.claimed, store.renewed, store.finalizeCancelled = []domain.Intent{eventMail(false)}, true, true
	provider := &workerProvider{}

	stats, err := NewWorker(store, provider, func(domain.Observation) {}).RunOnce(context.Background(), 1, 6)

	require.NoError(t, err)
	assert.Zero(t, provider.calls)
	assert.Equal(t, 1, stats.Cancelled)
	require.Len(t, store.events, 1)
	assert.Equal(t, domain.EmailDeliverySuppressed, store.events[0].State)
	assert.Equal(t, "token-1", store.events[0].Token)
	assert.Equal(t, "lease-token", store.cancelledToken, "the mail ends before the dispatch fence")
}

func TestWorkerHoldsBackAMailToAStoppedGuardianWithoutMarkingTheAddress(t *testing.T) {
	t.Parallel()
	store := &eventWorkerStore{stopped: true}
	store.claimed, store.renewed, store.finalizeCancelled = []domain.Intent{eventMail(false)}, true, true
	provider := &workerProvider{}

	stats, err := NewWorker(store, provider, func(domain.Observation) {}).RunOnce(context.Background(), 1, 6)

	require.NoError(t, err)
	assert.Zero(t, provider.calls)
	assert.Equal(t, 1, stats.Cancelled)
	require.Len(t, store.events, 1)
	assert.False(t, store.events[0].State.BlocksAddress(), "the stop is the school's decision, not a problem of the address")
}

func TestWorkerAsksTheTraegersSchoolsForATraegerMail(t *testing.T) {
	t.Parallel()
	store := &eventWorkerStore{traegerListed: true}
	mail := eventMail(false)
	mail.TenantID, mail.OrganizationID = 0, 9
	mail.Template = "care_contract"
	store.claimed, store.renewed, store.finalizeCancelled = []domain.Intent{mail}, true, true
	provider := &workerProvider{}

	stats, err := NewWorker(store, provider, func(domain.Observation) {}).RunOnce(context.Background(), 1, 6)

	require.NoError(t, err)
	assert.Zero(t, provider.calls)
	assert.Equal(t, 1, stats.Cancelled)
}

func TestWorkerSendsSecurityMailToASuppressedAddress(t *testing.T) {
	t.Parallel()
	store := &eventWorkerStore{suppressed: true}
	store.claimed, store.renewed, store.finalizeSent = []domain.Intent{eventMail(true)}, true, true
	provider := &workerProvider{}

	stats, err := NewWorker(store, provider, func(domain.Observation) {}).RunOnce(context.Background(), 1, 6)

	require.NoError(t, err)
	assert.Equal(t, 1, provider.calls)
	assert.Equal(t, 1, stats.Sent)
}

func TestWorkerSendsStaffMailToASuppressedAddress(t *testing.T) {
	t.Parallel()
	staffMail := eventMail(false)
	staffMail.Template = "payroll_dispatch"
	store := &eventWorkerStore{suppressed: true}
	store.claimed, store.renewed, store.finalizeSent = []domain.Intent{staffMail}, true, true
	provider := &workerProvider{}

	stats, err := NewWorker(store, provider, func(domain.Observation) {}).RunOnce(context.Background(), 1, 6)

	require.NoError(t, err)
	assert.Equal(t, 1, provider.calls)
	assert.Equal(t, 1, stats.Sent)
}

func TestWorkerSuppressesAnAddressTheServerRefused(t *testing.T) {
	t.Parallel()
	store := &eventWorkerStore{}
	store.claimed, store.renewed = []domain.Intent{eventMail(false)}, true
	store.failureResult = domain.FinalizeResult{Finalized: true, State: string(domain.StateDeadLetter)}
	provider := &workerProvider{err: fmt.Errorf("%w: %w: 550 <guardian@example.com> unknown", domain.ErrPermanent, domain.ErrRecipientRejected)}

	stats, err := NewWorker(store, provider, func(domain.Observation) {}).RunOnce(context.Background(), 1, 6)

	require.NoError(t, err)
	assert.Equal(t, 1, stats.DeadLettered)
	assert.Equal(t, 1, store.failureMaxAttempts, "no retry can change the answer")
	require.Len(t, store.events, 1)
	assert.Equal(t, domain.EmailDeliveryFailed, store.events[0].State)
	assert.NotContains(t, store.events[0].Diagnostic, "guardian@example.com")
}

func TestWorkerKeepsTheAddressWhenTheMailItselfIsRefused(t *testing.T) {
	t.Parallel()
	store := &eventWorkerStore{}
	store.claimed, store.renewed = []domain.Intent{eventMail(false)}, true
	store.failureResult = domain.FinalizeResult{Finalized: true, State: string(domain.StateDeadLetter)}
	provider := &workerProvider{err: fmt.Errorf("%w: 554 content rejected", domain.ErrPermanent)}

	stats, err := NewWorker(store, provider, func(domain.Observation) {}).RunOnce(context.Background(), 1, 6)

	require.NoError(t, err)
	assert.Equal(t, 1, stats.DeadLettered)
	assert.Empty(t, store.events)
}

func TestRecordEmailDeliveryEventValidatesAndCleans(t *testing.T) {
	t.Parallel()
	now := time.Now()
	valid := domain.EmailDeliveryEvent{Provider: "lettermint", ProviderEventID: "e1", State: domain.EmailDeliveryFailed, OccurredAt: now}
	for name, change := range map[string]func(*domain.EmailDeliveryEvent){
		"no provider": func(e *domain.EmailDeliveryEvent) { e.Provider = " " },
		"no event id": func(e *domain.EmailDeliveryEvent) { e.ProviderEventID = "" },
		"bad state":   func(e *domain.EmailDeliveryEvent) { e.State = "opened" },
		"no time":     func(e *domain.EmailDeliveryEvent) { e.OccurredAt = time.Time{} },
	} {
		event := valid
		change(&event)
		store := &eventWorkerStore{}
		_, err := RecordEmailDeliveryEvent(context.Background(), store, event)
		require.Error(t, err, name)
		assert.Empty(t, store.events, name)
	}

	store := &eventWorkerStore{}
	event := valid
	event.MessageID = "<token-9@mail.example>"
	event.Diagnostic = "550 <jane@example.com> " + strings.Repeat("ü", maxDiagnosticLen)
	_, err := RecordEmailDeliveryEvent(context.Background(), store, event)
	require.NoError(t, err)
	require.Len(t, store.events, 1)
	assert.Equal(t, "token-9@mail.example", store.events[0].MessageID)
	assert.Equal(t, "token-9", store.events[0].Token, "an event can overtake the stored Message-ID")
	assert.NotContains(t, store.events[0].Diagnostic, "jane@example.com")
	assert.LessOrEqual(t, len(store.events[0].Diagnostic), maxDiagnosticLen)
	assert.True(t, strings.HasPrefix(store.events[0].Diagnostic, "550 <[address]>"))
}

func TestRetentionCountsEventsAndSuppressions(t *testing.T) {
	t.Parallel()
	result := domain.RetentionResult{EmailIntents: 1, DeliveryEvents: 2, Suppressions: 3}
	assert.Equal(t, 6, result.Total())
}
