package application

import (
	"bytes"
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
	openErr   error
	markErr   error
	mailed    bool
	query     domain.ProviderMailQuery
	checked   []int64
	recorded  []domain.EmailDeliveryEvent
	reason    domain.EmailDeliveryState
	listed    bool
	deleted   int
	recordErr error
	deleteErr error
}

func (s *reconcileStore) OpenProviderMails(_ context.Context, query domain.ProviderMailQuery) ([]domain.ProviderMail, error) {
	s.query = query
	return s.mails, s.openErr
}

func (s *reconcileStore) MarkEmailDeliveryChecked(_ context.Context, id int64, _ time.Time) error {
	s.checked = append(s.checked, id)
	return s.markErr
}

func (s *reconcileStore) EmailSuppressionReason(context.Context, int64, string) (domain.EmailDeliveryState, bool, error) {
	return s.reason, s.listed, nil
}

func (s *reconcileStore) DeleteEmailSuppression(context.Context, int64, string) error {
	s.deleted++
	return s.deleteErr
}

func (s *reconcileStore) SchoolMailedAddress(context.Context, int64, string) (bool, error) {
	return s.mailed, nil
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

	err := reconciler(store, api).ReconcileEmailDelivery(context.Background())

	assert.ErrorIs(t, err, domain.ErrProviderRateLimited, "a run cut short is reported; the next one continues")
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

func TestLiftSuppressionReportsAComplaintRecordedDuringTheProviderCall(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{listed: true, reason: domain.EmailDeliveryFailed, deleteErr: domain.ErrSuppressionNeedsReview}
	api := &stubProviderAPI{}
	err := reconciler(store, api).LiftEmailSuppression(context.Background(), 7, "eltern@example.com")
	assert.ErrorIs(t, err, deliveryContract.ErrEmailSuppressionNeedsReview)
	assert.Equal(t, []string{"eltern@example.com"}, api.lifted)
	assert.Equal(t, 1, store.deleted)
}

func alarmingReconciler(store *reconcileStore, api *stubProviderAPI) (*EmailReconciliation, *bytes.Buffer) {
	var logs bytes.Buffer
	reconciliation := reconciler(store, api)
	reconciliation.logger = slog.New(slog.NewTextHandler(&logs, nil))
	return reconciliation, &logs
}

// The Grafana rule reads this log line, so every failed run has to write it.
func TestReconciliationWritesTheAlarmLineWhenTheOpenMailsCannotBeRead(t *testing.T) {
	t.Parallel()
	failure := errors.New("database is down")
	store := &reconcileStore{openErr: failure}
	reconciliation, logs := alarmingReconciler(store, &stubProviderAPI{enabled: false})

	err := reconciliation.ReconcileEmailDelivery(context.Background())

	assert.ErrorIs(t, err, failure)
	assert.ErrorIs(t, err, ErrEmailWebhookDisabled, "what the run already knew goes out too")
	assert.Contains(t, logs.String(), `msg="email delivery alarm"`)
	assert.Contains(t, logs.String(), "database is down")
	assert.Contains(t, logs.String(), "disabled at the provider")
}

func TestReconciliationWritesTheAlarmLineWhenTheRateLimitEndsTheRun(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{mails: []domain.ProviderMail{{ID: 1, ProviderMessageID: "m1", SentAt: reconcileNow.Add(-time.Hour)}}}
	api := &stubProviderAPI{enabled: false, eventErrs: map[string]error{"m1": domain.ErrProviderRateLimited}}
	reconciliation, logs := alarmingReconciler(store, api)

	err := reconciliation.ReconcileEmailDelivery(context.Background())

	assert.ErrorIs(t, err, domain.ErrProviderRateLimited)
	assert.ErrorIs(t, err, ErrEmailWebhookDisabled)
	assert.Contains(t, logs.String(), `msg="email delivery alarm"`)
}

// A mail stays open for up to 28 days. Its silence is one finding, not one per hour.
func TestReconciliationReportsASilentMailOnlyOnce(t *testing.T) {
	t.Parallel()
	sent := reconcileNow.Add(-emailEventSilence - 2*time.Hour)
	beforeTheLimit := sent.Add(emailEventSilence - time.Hour)
	afterTheLimit := sent.Add(emailEventSilence + time.Hour)
	for name, test := range map[string]struct {
		checked *time.Time
		alarm   bool
	}{
		"never asked":                 {checked: nil, alarm: true},
		"last asked before the limit": {checked: &beforeTheLimit, alarm: true},
		"already reported":            {checked: &afterTheLimit, alarm: false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := &reconcileStore{mails: []domain.ProviderMail{{ID: 1, ProviderMessageID: "m1", SentAt: sent, CheckedAt: test.checked}}}

			err := reconciler(store, &stubProviderAPI{enabled: true}).ReconcileEmailDelivery(context.Background())

			if test.alarm {
				assert.ErrorIs(t, err, ErrEmailEventsSilent)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, []int64{1}, store.checked, "the mail stays in the rotation")
		})
	}
}

func TestReconciliationDoesNotCountASilentMailItCouldNotMark(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{markErr: errors.New("database is down"), mails: []domain.ProviderMail{
		{ID: 1, ProviderMessageID: "m1", SentAt: reconcileNow.Add(-emailEventSilence - time.Hour)},
	}}

	err := reconciler(store, &stubProviderAPI{enabled: true}).ReconcileEmailDelivery(context.Background())

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrEmailEventsSilent, "the next run reports it, once")
}

// A mail the provider does not know says nothing about the event stream.
func TestReconciliationNotesAnUnknownMailWithoutTheSilenceAlarm(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{mails: []domain.ProviderMail{{ID: 1, ProviderMessageID: "gone", SentAt: reconcileNow.Add(-emailEventSilence - time.Hour)}}}
	api := &stubProviderAPI{enabled: true, eventErrs: map[string]error{"gone": domain.ErrProviderNotFound}}
	reconciliation, logs := alarmingReconciler(store, api)

	require.NoError(t, reconciliation.ReconcileEmailDelivery(context.Background()))

	assert.Contains(t, logs.String(), "the provider does not know an accepted mail")
	assert.NotContains(t, logs.String(), "email delivery alarm")
	assert.Equal(t, []int64{1}, store.checked)
}

// A team-wide block of a foreign address is not the school's to lift.
func TestLiftSuppressionWithoutAnyLinkToTheSchoolTouchesNothing(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{}
	api := &stubProviderAPI{}

	err := reconciler(store, api).LiftEmailSuppression(context.Background(), 7, "fremd@example.com")

	require.ErrorIs(t, err, deliveryContract.ErrEmailSuppressionNotFound)
	assert.Empty(t, api.lifted)
	assert.Zero(t, store.deleted)
}

func TestLiftSuppressionFollowsTheSchoolsOwnMail(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{mailed: true}
	api := &stubProviderAPI{}

	require.NoError(t, reconciler(store, api).LiftEmailSuppression(context.Background(), 7, " Eltern@Example.com "))

	assert.Equal(t, []string{"eltern@example.com"}, api.lifted)
}

func TestLiftSuppressionReportsABlockThatStaysAndKeepsTheList(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{listed: true, reason: domain.EmailDeliveryFailed}
	api := &stubProviderAPI{liftErr: domain.ErrSuppressionStaysAtProvider}

	err := reconciler(store, api).LiftEmailSuppression(context.Background(), 7, "eltern@example.com")

	require.ErrorIs(t, err, deliveryContract.ErrEmailSuppressionStaysAtProvider)
	assert.Zero(t, store.deleted, "an error means nothing changed")
}

func TestLiftSuppressionWithoutProviderReportsAnAddressThatIsNotListed(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{mailed: true}
	reconciliation := NewEmailReconciliation(store, nil, false, slog.New(slog.DiscardHandler), func(domain.Observation) {})

	require.ErrorIs(t, reconciliation.LiftEmailSuppression(context.Background(), 7, "eltern@example.com"), deliveryContract.ErrEmailSuppressionNotFound)
}
