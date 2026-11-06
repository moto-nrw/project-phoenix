package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var reconcileNow = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

type reconcileStore struct {
	mails     []domain.ProviderMail
	query     domain.ProviderMailQuery
	checked   []int64
	recorded  []domain.EmailDeliveryEvent
	reason    domain.EmailDeliveryState
	listed    bool
	deleted   int
	recordErr error
}

func (s *reconcileStore) OpenProviderMails(_ context.Context, query domain.ProviderMailQuery) ([]domain.ProviderMail, error) {
	s.query = query
	return s.mails, nil
}

func (s *reconcileStore) MarkEmailDeliveryChecked(_ context.Context, id int64, _ time.Time) error {
	s.checked = append(s.checked, id)
	return nil
}

func (s *reconcileStore) EmailSuppressionReason(context.Context, int64, string) (domain.EmailDeliveryState, bool, error) {
	return s.reason, s.listed, nil
}

func (s *reconcileStore) DeleteEmailSuppression(context.Context, int64, string) error {
	s.deleted++
	return nil
}

func (s *reconcileStore) RecordEmailDeliveryEvent(_ context.Context, event domain.EmailDeliveryEvent) (domain.EmailDeliveryEventResult, error) {
	s.recorded = append(s.recorded, event)
	if s.recordErr != nil {
		return domain.EmailDeliveryEventResult{}, s.recordErr
	}
	return domain.EmailDeliveryEventResult{Matched: true, Recorded: true, State: event.State}, nil
}

func (*reconcileStore) EmailAddressSuppressed(context.Context, int64, string) (bool, error) {
	return false, nil
}

type stubProviderAPI struct {
	enabled    bool
	enabledErr error
	events     map[string][]domain.EmailDeliveryEvent
	eventErrs  map[string]error
	asked      []string
	liftErr    error
	lifted     []string
}

func (a *stubProviderAPI) WebhookEnabled(context.Context) (bool, error) {
	return a.enabled, a.enabledErr
}

func (a *stubProviderAPI) MessageEvents(_ context.Context, id string) ([]domain.EmailDeliveryEvent, error) {
	a.asked = append(a.asked, id)
	return a.events[id], a.eventErrs[id]
}

func (a *stubProviderAPI) LiftSuppression(_ context.Context, address string) error {
	a.lifted = append(a.lifted, address)
	return a.liftErr
}

func reconciler(store *reconcileStore, api *stubProviderAPI) *EmailReconciliation {
	reconciliation := NewEmailReconciliation(store, api, true, slog.New(slog.DiscardHandler), func(domain.Observation) {})
	reconciliation.now = func() time.Time { return reconcileNow }
	return reconciliation
}

func providerEvent(id string, state domain.EmailDeliveryState) domain.EmailDeliveryEvent {
	return domain.EmailDeliveryEvent{ProviderMessageID: id, Provider: "lettermint", ProviderEventID: "api:" + string(state), State: state, OccurredAt: reconcileNow}
}

func TestReconciliationWithoutProviderDoesNothing(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{mails: []domain.ProviderMail{{ID: 1, ProviderMessageID: "m1"}}}
	reconciliation := NewEmailReconciliation(store, nil, false, slog.New(slog.DiscardHandler), func(domain.Observation) {})

	require.NoError(t, reconciliation.ReconcileEmailDelivery(context.Background()))

	assert.Empty(t, store.checked)
}

func TestReconciliationRestoresTheStateAndAlarmsOnWhatTheWebhookMissed(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{mails: []domain.ProviderMail{
		{ID: 1, ProviderMessageID: "m1", SentAt: reconcileNow.Add(-time.Hour)},
		{ID: 2, ProviderMessageID: "m2", State: domain.EmailDeliveryDelayed, SentAt: reconcileNow.Add(-time.Hour)},
	}}
	api := &stubProviderAPI{enabled: true, events: map[string][]domain.EmailDeliveryEvent{
		"m1": {providerEvent("m1", domain.EmailDeliveryDelivered), providerEvent("m1", domain.EmailDeliveryComplained)},
		"m2": {providerEvent("m2", domain.EmailDeliveryDelayed)},
	}}

	err := reconciler(store, api).ReconcileEmailDelivery(context.Background())

	assert.ErrorIs(t, err, ErrEmailEventsMissed)
	assert.Len(t, store.recorded, 3)
	assert.Equal(t, []int64{1, 2}, store.checked)
	assert.True(t, store.query.SentAfter.Equal(reconcileNow.Add(-emailProviderRetention)))
	assert.True(t, store.query.SentBefore.Equal(reconcileNow.Add(-emailWebhookGrace)))
}

func TestReconciliationStaysQuietWhenTheWebhookWasFaster(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{mails: []domain.ProviderMail{{ID: 2, ProviderMessageID: "m2", State: domain.EmailDeliveryDelayed, SentAt: reconcileNow.Add(-time.Hour)}}}
	api := &stubProviderAPI{enabled: true, events: map[string][]domain.EmailDeliveryEvent{"m2": {providerEvent("m2", domain.EmailDeliveryDelayed)}}}

	require.NoError(t, reconciler(store, api).ReconcileEmailDelivery(context.Background()), "the delay was known, so nothing was missed")
}

func TestReconciliationAlarmsOnADisabledWebhookAndStillReconciles(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{mails: []domain.ProviderMail{{ID: 1, ProviderMessageID: "m1", SentAt: reconcileNow.Add(-time.Hour)}}}
	api := &stubProviderAPI{enabled: false}

	err := reconciler(store, api).ReconcileEmailDelivery(context.Background())

	assert.ErrorIs(t, err, ErrEmailWebhookDisabled)
	assert.Equal(t, []string{"m1"}, api.asked)
}

func TestReconciliationAlarmsOnAWebhookStateItCannotRead(t *testing.T) {
	t.Parallel()
	failure := errors.New("answered 502")

	err := reconciler(&reconcileStore{}, &stubProviderAPI{enabledErr: failure}).ReconcileEmailDelivery(context.Background())

	assert.ErrorIs(t, err, failure)
}

func TestReconciliationAlarmsOnMailsWithoutAnyEvent(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{mails: []domain.ProviderMail{
		{ID: 1, ProviderMessageID: "old", SentAt: reconcileNow.Add(-emailEventSilence - time.Minute)},
		{ID: 2, ProviderMessageID: "new", SentAt: reconcileNow.Add(-time.Hour)},
	}}

	err := reconciler(store, &stubProviderAPI{enabled: true}).ReconcileEmailDelivery(context.Background())

	assert.ErrorIs(t, err, ErrEmailEventsSilent)
}

func TestReconciliationStopsAtTheRateLimitWithoutMarkingTheRest(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{mails: []domain.ProviderMail{
		{ID: 1, ProviderMessageID: "m1", SentAt: reconcileNow.Add(-time.Hour)},
		{ID: 2, ProviderMessageID: "m2", SentAt: reconcileNow.Add(-time.Hour)},
	}}
	api := &stubProviderAPI{enabled: true, eventErrs: map[string]error{"m1": domain.ErrProviderRateLimited}}

	require.NoError(t, reconciler(store, api).ReconcileEmailDelivery(context.Background()), "the next run continues")

	assert.Equal(t, []string{"m1"}, api.asked)
	assert.Empty(t, store.checked)
}

func TestReconciliationMarksAMailTheProviderNoLongerKeeps(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{mails: []domain.ProviderMail{{ID: 1, ProviderMessageID: "gone", SentAt: reconcileNow.Add(-time.Hour)}}}
	api := &stubProviderAPI{enabled: true, eventErrs: map[string]error{"gone": domain.ErrProviderNotFound}}

	require.NoError(t, reconciler(store, api).ReconcileEmailDelivery(context.Background()))

	assert.Equal(t, []int64{1}, store.checked)
}

func TestReconciliationKeepsGoingAfterOneFailedMail(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{mails: []domain.ProviderMail{
		{ID: 1, ProviderMessageID: "m1", SentAt: reconcileNow.Add(-time.Hour)},
		{ID: 2, ProviderMessageID: "m2", SentAt: reconcileNow.Add(-time.Hour)},
	}}
	failure := errors.New("answered 500")
	api := &stubProviderAPI{enabled: true, eventErrs: map[string]error{"m1": failure}}

	err := reconciler(store, api).ReconcileEmailDelivery(context.Background())

	assert.ErrorIs(t, err, failure)
	assert.Equal(t, []int64{2}, store.checked, "the failed mail is asked about again")
}

func TestLiftSuppressionFreesTheAddress(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{listed: true, reason: domain.EmailDeliveryFailed}
	api := &stubProviderAPI{}

	require.NoError(t, reconciler(store, api).LiftEmailSuppression(context.Background(), 7, " eltern@example.com "))

	assert.Equal(t, []string{"eltern@example.com"}, api.lifted)
	assert.Equal(t, 1, store.deleted)
}

func TestLiftSuppressionRefusesAComplaint(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{listed: true, reason: domain.EmailDeliveryComplained}
	api := &stubProviderAPI{}

	err := reconciler(store, api).LiftEmailSuppression(context.Background(), 7, "eltern@example.com")

	assert.ErrorIs(t, err, deliveryContract.ErrEmailSuppressionNeedsReview)
	assert.Empty(t, api.lifted)
	assert.Zero(t, store.deleted)
}

func TestLiftSuppressionHonorsTheProvidersRefusal(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{listed: true, reason: domain.EmailDeliveryFailed}

	err := reconciler(store, &stubProviderAPI{liftErr: domain.ErrSuppressionNeedsReview}).LiftEmailSuppression(context.Background(), 7, "eltern@example.com")

	assert.ErrorIs(t, err, deliveryContract.ErrEmailSuppressionNeedsReview)
	assert.Zero(t, store.deleted, "the local list follows the provider, not the other way round")
}

func TestLiftSuppressionKeepsTheListWhenTheProviderFails(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{listed: true, reason: domain.EmailDeliveryFailed}

	err := reconciler(store, &stubProviderAPI{liftErr: errors.New("answered 500")}).LiftEmailSuppression(context.Background(), 7, "eltern@example.com")

	require.Error(t, err)
	assert.Zero(t, store.deleted)
}

func TestLiftSuppressionWithoutProviderFreesTheSchoolsList(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{listed: true, reason: domain.EmailDeliveryFailed}
	reconciliation := NewEmailReconciliation(store, nil, false, slog.New(slog.DiscardHandler), func(domain.Observation) {})

	require.NoError(t, reconciliation.LiftEmailSuppression(context.Background(), 7, "eltern@example.com"))

	assert.Equal(t, 1, store.deleted)
}
