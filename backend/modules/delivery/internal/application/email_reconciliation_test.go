package application

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	"github.com/moto-ogs/project-phoenix/modules/delivery/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var reconcileNow = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

type reconcileStore struct {
	mails     []domain.ProviderMail
	openErr   error
	markErr   error
	query     domain.ProviderMailQuery
	checked   []int64
	recorded  []domain.EmailDeliveryEvent
	reason    domain.EmailSuppressionReason
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

func (s *reconcileStore) EmailSuppressionReason(context.Context, int64, string) (domain.EmailSuppressionReason, bool, error) {
	return s.reason, s.listed, nil
}

func (s *reconcileStore) DeleteEmailSuppression(context.Context, int64, string) error {
	s.deleted++
	return s.deleteErr
}

func (s *reconcileStore) RecordEmailDeliveryEvent(_ context.Context, event domain.EmailDeliveryEvent) (domain.EmailDeliveryEventResult, error) {
	s.recorded = append(s.recorded, event)
	if s.recordErr != nil {
		return domain.EmailDeliveryEventResult{}, s.recordErr
	}
	return domain.EmailDeliveryEventResult{Matched: true, Recorded: true, State: event.State}, nil
}

func (*reconcileStore) SchoolEmailHolds(context.Context, int64, string) (domain.SchoolMailHolds, error) {
	return domain.SchoolMailHolds{}, nil
}

func (*reconcileStore) TraegerEmailHolds(context.Context, int64, string) (bool, bool, error) {
	return false, false, nil
}

type stubProviderAPI struct {
	enabled    bool
	enabledErr error
	events     map[string][]domain.EmailDeliveryEvent
	eventErrs  map[string]error
	asked      []string
	liftErr    error
	lifted     []string
	blocks     map[string]bool
	blockErrs  map[string]error
	blockAsked []string
}

func (a *stubProviderAPI) SuppressionActive(_ context.Context, address string) (bool, error) {
	a.blockAsked = append(a.blockAsked, address)
	return a.blocks[address], a.blockErrs[address]
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

func reconcilerWithoutProvider(store *reconcileStore) *EmailReconciliation {
	return NewEmailReconciliation(store, nil, false, slog.New(slog.DiscardHandler), func(domain.Observation) {})
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
	store := &reconcileStore{listed: true, reason: domain.EmailSuppressionFailed}
	api := &stubProviderAPI{}

	require.NoError(t, reconciler(store, api).LiftEmailSuppression(context.Background(), 7, " eltern@example.com "))

	assert.Equal(t, []string{"eltern@example.com"}, api.lifted)
	assert.Equal(t, 1, store.deleted)
}

func TestLiftSuppressionRefusesAComplaint(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{listed: true, reason: domain.EmailSuppressionComplained}
	api := &stubProviderAPI{}

	err := reconciler(store, api).LiftEmailSuppression(context.Background(), 7, "eltern@example.com")

	assert.ErrorIs(t, err, deliveryContract.ErrEmailSuppressionNeedsReview)
	assert.Empty(t, api.lifted)
	assert.Zero(t, store.deleted)
}

func TestLiftSuppressionKeepsTheListWhenTheProviderFails(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{listed: true, reason: domain.EmailSuppressionFailed}

	err := reconciler(store, &stubProviderAPI{liftErr: errors.New("answered 500")}).LiftEmailSuppression(context.Background(), 7, "eltern@example.com")

	require.Error(t, err)
	assert.Zero(t, store.deleted)
}

func TestLiftSuppressionWithoutProviderFreesTheSchoolsList(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{listed: true, reason: domain.EmailSuppressionFailed}
	reconciliation := NewEmailReconciliation(store, nil, false, slog.New(slog.DiscardHandler), func(domain.Observation) {})

	require.NoError(t, reconciliation.LiftEmailSuppression(context.Background(), 7, "eltern@example.com"))

	assert.Equal(t, 1, store.deleted)
}

// Strato and Brevo have no provider list to ask: the school's list is the only
// block, so it is what lifting clears. The next refusal of the server puts the
// address back (#5934).
func TestLiftSuppressionWorksWithoutAProvider(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{listed: true, reason: domain.EmailSuppressionFailed}
	require.NoError(t, reconcilerWithoutProvider(store).LiftEmailSuppression(context.Background(), 7, "eltern@example.com"))
	assert.Equal(t, 1, store.deleted)
}

func TestLiftSuppressionReportsAComplaintRecordedDuringTheProviderCall(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{listed: true, reason: domain.EmailSuppressionFailed, deleteErr: domain.ErrSuppressionNeedsReview}
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
// A block the school did not cause is not the school's to lift, whatever mail it
// sent to the address. Nothing is asked of the provider, so the answer says nothing
// about the address either (#5934).
func TestLiftSuppressionWithoutOwnListEntryLeavesTheProviderAlone(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{}
	api := &stubProviderAPI{}
	err := reconciler(store, api).LiftEmailSuppression(context.Background(), 7, "eltern@example.com")
	assert.ErrorIs(t, err, deliveryContract.ErrEmailSuppressionNotFound)
	assert.Empty(t, api.lifted)
	assert.Zero(t, store.deleted)
}

// The provider's word about a complaint may come from another school: the school
// only learns that the block stays.
func TestLiftSuppressionHidesAComplaintAtTheProvider(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{listed: true, reason: domain.EmailSuppressionFailed}
	err := reconciler(store, &stubProviderAPI{liftErr: domain.ErrSuppressionNeedsReview}).LiftEmailSuppression(context.Background(), 7, "eltern@example.com")
	assert.ErrorIs(t, err, deliveryContract.ErrEmailSuppressionStaysAtProvider)
	assert.NotErrorIs(t, err, deliveryContract.ErrEmailSuppressionNeedsReview)
	assert.Zero(t, store.deleted)
}
func TestLiftSuppressionReportsABlockThatStaysAndKeepsTheList(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{listed: true, reason: domain.EmailSuppressionFailed}
	api := &stubProviderAPI{liftErr: domain.ErrSuppressionStaysAtProvider}

	err := reconciler(store, api).LiftEmailSuppression(context.Background(), 7, "eltern@example.com")

	require.ErrorIs(t, err, deliveryContract.ErrEmailSuppressionStaysAtProvider)
	assert.Zero(t, store.deleted, "an error means nothing changed")
}

func TestLiftSuppressionWithoutProviderReportsAnAddressThatIsNotListed(t *testing.T) {
	t.Parallel()
	store := &reconcileStore{}
	reconciliation := NewEmailReconciliation(store, nil, false, slog.New(slog.DiscardHandler), func(domain.Observation) {})

	require.ErrorIs(t, reconciliation.LiftEmailSuppression(context.Background(), 7, "eltern@example.com"), deliveryContract.ErrEmailSuppressionNotFound)
}

func TestReconciliationAlarmsOnADelayPastTheLimit(t *testing.T) {
	t.Parallel()
	delayed := reconcileNow.Add(-deliveryContract.EmailDelayLimit - time.Hour)
	beforeTheLimit := delayed.Add(time.Hour)
	afterTheLimit := delayed.Add(deliveryContract.EmailDelayLimit + time.Minute)
	recently := reconcileNow.Add(-time.Hour)
	for name, test := range map[string]struct {
		state   domain.EmailDeliveryState
		stateAt *time.Time
		checked *time.Time
		alarm   bool
	}{
		"past the limit":     {state: domain.EmailDeliveryDelayed, stateAt: &delayed, checked: &beforeTheLimit, alarm: true},
		"never asked":        {state: domain.EmailDeliveryDelayed, stateAt: &delayed, alarm: true},
		"already reported":   {state: domain.EmailDeliveryDelayed, stateAt: &delayed, checked: &afterTheLimit},
		"within the limit":   {state: domain.EmailDeliveryDelayed, stateAt: &recently},
		"not delayed at all": {stateAt: &delayed},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := &reconcileStore{mails: []domain.ProviderMail{{
				ID: 1, ProviderMessageID: "m1", State: test.state, StateAt: test.stateAt,
				SentAt: reconcileNow.Add(-2 * time.Hour), CheckedAt: test.checked,
			}}}

			err := reconciler(store, &stubProviderAPI{enabled: true}).ReconcileEmailDelivery(context.Background())

			if test.alarm {
				assert.ErrorIs(t, err, ErrEmailDelayOverdue)
			} else {
				assert.NotErrorIs(t, err, ErrEmailDelayOverdue)
			}
		})
	}
}

func TestProviderBlocksReadEachAddressOnceAndNameEveryState(t *testing.T) {
	t.Parallel()
	api := &stubProviderAPI{
		blocks:    map[string]bool{"a@example.com": true},
		blockErrs: map[string]error{"c@example.com": errors.New("down")},
	}

	blocks := reconciler(&reconcileStore{}, api).ProviderEmailBlocks(context.Background(), []string{" A@example.com", "a@example.com", "b@example.com", "c@example.com"})

	assert.Equal(t, []string{"a@example.com", "b@example.com", "c@example.com"}, api.blockAsked)
	assert.Equal(t, deliveryContract.EmailProviderBlockActive, blocks["a@example.com"])
	assert.Equal(t, deliveryContract.EmailProviderBlockNone, blocks["b@example.com"])
	assert.Equal(t, deliveryContract.EmailProviderBlockUnreadable, blocks["c@example.com"])
}

func TestProviderBlocksStayWithinTheirLimit(t *testing.T) {
	t.Parallel()
	addresses := make([]string, 0, deliveryContract.ProviderBlockReads+3)
	for i := range deliveryContract.ProviderBlockReads + 3 {
		addresses = append(addresses, fmt.Sprintf("a%d@example.com", i))
	}
	api := &stubProviderAPI{}

	blocks := reconciler(&reconcileStore{}, api).ProviderEmailBlocks(context.Background(), addresses)

	assert.Len(t, api.blockAsked, deliveryContract.ProviderBlockReads)
	assert.Equal(t, deliveryContract.EmailProviderBlockUnreadable, blocks[addresses[deliveryContract.ProviderBlockReads]], "what it left out is not offered")
}

// hangingProviderAPI answers only when the caller's context ends.
type hangingProviderAPI struct{ stubProviderAPI }

func (a *hangingProviderAPI) SuppressionActive(ctx context.Context, address string) (bool, error) {
	a.blockAsked = append(a.blockAsked, address)
	<-ctx.Done()
	return false, ctx.Err()
}

// Callers read this inside their transaction (see ProviderEmailBlocks). The budget is
// what bounds that: a provider that hangs costs one call its budget, and the addresses
// after it are not asked.
func TestProviderBlocksHoldTheCallerForAtMostTheBudget(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		api := &hangingProviderAPI{}
		reconciliation := NewEmailReconciliation(&reconcileStore{}, api, true, slog.New(slog.DiscardHandler), func(domain.Observation) {})
		start := time.Now()

		blocks := reconciliation.ProviderEmailBlocks(context.Background(), []string{"a@example.com", "b@example.com"})

		assert.Equal(t, deliveryContract.ProviderBlockTime, time.Since(start))
		assert.Equal(t, []string{"a@example.com"}, api.blockAsked)
		assert.Equal(t, deliveryContract.EmailProviderBlockUnreadable, blocks["a@example.com"])
		assert.Equal(t, deliveryContract.EmailProviderBlockUnreadable, blocks["b@example.com"], "nothing is offered on a guess")
	})
}

// One request shares one budget, however many mails ask: a bulk send of contracts
// costs the transaction at most one budget, and the rest is not sent on a guess.
func TestProviderBlocksShareTheBudgetOfTheRequest(t *testing.T) {
	t.Parallel()
	t.Run("a hanging provider", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			api := &hangingProviderAPI{}
			reconciliation := NewEmailReconciliation(&reconcileStore{}, api, true, slog.New(slog.DiscardHandler), func(domain.Observation) {})
			ctx := deliveryContract.WithProviderBlockBudget(context.Background())
			start := time.Now()

			first := reconciliation.ProviderEmailBlocks(ctx, []string{"a@example.com"})
			second := reconciliation.ProviderEmailBlocks(ctx, []string{"b@example.com"})

			assert.Equal(t, deliveryContract.ProviderBlockTime, time.Since(start), "not once per call")
			assert.Equal(t, []string{"a@example.com"}, api.blockAsked)
			assert.Equal(t, deliveryContract.EmailProviderBlockUnreadable, first["a@example.com"])
			assert.Equal(t, deliveryContract.EmailProviderBlockUnreadable, second["b@example.com"])
		})
	})
	t.Run("the number of reads", func(t *testing.T) {
		t.Parallel()
		api := &stubProviderAPI{}
		ctx := deliveryContract.WithProviderBlockBudget(context.Background())
		var last map[string]deliveryContract.EmailProviderBlock
		for i := range deliveryContract.ProviderBlockReads + 1 {
			last = reconciler(&reconcileStore{}, api).ProviderEmailBlocks(ctx, []string{fmt.Sprintf("a%d@example.com", i)})
		}

		assert.Len(t, api.blockAsked, deliveryContract.ProviderBlockReads)
		assert.Equal(t, deliveryContract.EmailProviderBlockUnreadable, last[fmt.Sprintf("a%d@example.com", deliveryContract.ProviderBlockReads)])
	})
	t.Run("a rate limit", func(t *testing.T) {
		t.Parallel()
		api := &stubProviderAPI{blockErrs: map[string]error{"a@example.com": domain.ErrProviderRateLimited}}
		ctx := deliveryContract.WithProviderBlockBudget(context.Background())

		reconciler(&reconcileStore{}, api).ProviderEmailBlocks(ctx, []string{"a@example.com"})
		blocks := reconciler(&reconcileStore{}, api).ProviderEmailBlocks(ctx, []string{"b@example.com"})

		assert.Equal(t, []string{"a@example.com"}, api.blockAsked)
		assert.Equal(t, deliveryContract.EmailProviderBlockUnreadable, blocks["b@example.com"])
	})
}

// Without a shared budget each call has its own, as a single read needs nothing else.
func TestProviderBlocksWithoutASharedBudgetStartAnew(t *testing.T) {
	t.Parallel()
	api := &stubProviderAPI{}
	for i := range deliveryContract.ProviderBlockReads + 1 {
		reconciler(&reconcileStore{}, api).ProviderEmailBlocks(context.Background(), []string{fmt.Sprintf("a%d@example.com", i)})
	}
	assert.Len(t, api.blockAsked, deliveryContract.ProviderBlockReads+1)
}

func TestProviderBlocksStopAtTheRateLimit(t *testing.T) {
	t.Parallel()
	api := &stubProviderAPI{blockErrs: map[string]error{"a@example.com": domain.ErrProviderRateLimited}}

	blocks := reconciler(&reconcileStore{}, api).ProviderEmailBlocks(context.Background(), []string{"a@example.com", "b@example.com"})

	assert.Equal(t, []string{"a@example.com"}, api.blockAsked)
	assert.Equal(t, deliveryContract.EmailProviderBlockUnreadable, blocks["b@example.com"])
}

func TestWithoutAProviderNoAddressIsBlockedThere(t *testing.T) {
	t.Parallel()
	blocks := reconcilerWithoutProvider(&reconcileStore{}).ProviderEmailBlocks(context.Background(), []string{"a@example.com"})
	assert.Equal(t, deliveryContract.EmailProviderBlockNone, blocks["a@example.com"])
}
