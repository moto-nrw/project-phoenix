package notifications

import (
	"context"
	"errors"
	"testing"

	deliveryModels "github.com/moto-ogs/project-phoenix/modules/delivery/subscriptions"
	"github.com/moto-ogs/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// guardianReadCounter counts the guardian device reads of a series.
type guardianReadCounter struct {
	*fakePushRepo
	reads [][]int64
}

func (c *guardianReadCounter) FindForGuardians(ctx context.Context, accountIDs, studentIDs []int64) ([]*deliveryModels.PushSubscription, error) {
	c.reads = append(c.reads, append([]int64(nil), accountIDs...))
	return c.fakePushRepo.FindForGuardians(ctx, accountIDs, studentIDs)
}

func guardianSub(id, accountID int64) *deliveryModels.PushSubscription {
	sub := testSub(id, 41, "https://fcm.googleapis.com/device")
	sub.AccountID = accountID
	sub.Portal = deliveryModels.PushPortalParent
	return sub
}

func seriesGuardianEvent(key, title string, studentIDs []int64, accountIDs ...int64) Event {
	return Event{Type: "parent_announcement", IdempotencyKey: key, Title: title, Priority: PriorityNormal,
		Audience: Audience{TenantID: 41, Scope: ScopeGuardian, GuardianAccountIDs: accountIDs, StudentIDs: studentIDs}}
}

func newSeriesTestChannel() (*webPushChannel, *guardianReadCounter) {
	repo := &guardianReadCounter{fakePushRepo: &fakePushRepo{guardians: map[int64][]*deliveryModels.PushSubscription{
		7: {guardianSub(1, 7)}, 8: {guardianSub(2, 8), guardianSub(3, 8)}, 9: {guardianSub(4, 9)},
	}}}
	channel := testChannel(repo.fakePushRepo, &fakeSender{})
	channel.repo = repo
	return channel, repo
}

func withinTenantTx(t *testing.T, channel *webPushChannel, fn func(context.Context)) {
	t.Helper()
	require.NoError(t, tenant.WithTenantTx(channel.withTenantRuntime(context.Background()), channel.db, 41, func(txCtx context.Context, _ bun.Tx) error {
		fn(txCtx)
		return nil
	}))
}

func TestWebPushSeriesReadsGuardianDevicesOncePerTransaction(t *testing.T) {
	t.Parallel()
	channel, repo := newSeriesTestChannel()
	events := []Event{seriesGuardianEvent("de", "Hallo", nil, 7), seriesGuardianEvent("en", "Hello", nil, 8), seriesGuardianEvent("tr", "Merhaba", nil, 9)}
	withinTenantTx(t, channel, func(ctx context.Context) {
		series, ok := channel.forSeries(ctx, events).(durableReceiptChannel)
		require.True(t, ok)
		require.NotSame(t, channel, series, "the series delivers through a channel of its own")
		for i, want := range []int{1, 2, 1} {
			count, err := series.DeliverDurably(ctx, events[i])
			require.NoError(t, err)
			assert.Equal(t, want, count)
		}
	})
	assert.Equal(t, [][]int64{{7, 8, 9}}, repo.reads)
	outbox := channel.outbox.(*fakePushOutbox)
	var got []string
	for _, intent := range outbox.intents {
		got = append(got, intent.Title+":"+intent.IdempotencyKey)
		assert.Equal(t, map[string]int64{"Hallo": 7, "Hello": 8, "Merhaba": 9}[intent.Title], intent.AccountID, "each variant reaches only its own guardians")
	}
	assert.Equal(t, []string{"Hallo:de:subscription:1", "Hello:en:subscription:2", "Hello:en:subscription:3", "Merhaba:tr:subscription:4"}, got)
}

func TestWebPushSeriesKeepsPlainReadsOutsideItsScope(t *testing.T) {
	t.Parallel()
	plain := []Event{seriesGuardianEvent("de", "Hallo", nil, 7), seriesGuardianEvent("en", "Hello", nil, 8)}
	t.Run("without a transaction", func(t *testing.T) {
		channel, _ := newSeriesTestChannel()
		assert.Same(t, channel, channel.forSeries(context.Background(), plain))
	})
	for name, events := range map[string][]Event{
		"one guardian event":    plain[:1],
		"different child scope": {seriesGuardianEvent("de", "Hallo", []int64{1}, 7), seriesGuardianEvent("en", "Hello", []int64{2}, 8)},
		"different tenant": {plain[0], func() Event {
			event := plain[1]
			event.Audience.TenantID = 42
			return event
		}()},
	} {
		t.Run(name, func(t *testing.T) {
			channel, _ := newSeriesTestChannel()
			withinTenantTx(t, channel, func(ctx context.Context) {
				assert.Same(t, channel, channel.forSeries(ctx, events))
			})
		})
	}
	t.Run("a staff event in a guardian series reads its own devices", func(t *testing.T) {
		channel, repo := newSeriesTestChannel()
		repo.staff = []*deliveryModels.PushSubscription{testSub(5, 41, "https://fcm.googleapis.com/staff")}
		staff := Event{Type: "test", IdempotencyKey: "staff", Title: "Team", Priority: PriorityNormal, Audience: Audience{TenantID: 41, Scope: ScopeTenant}}
		withinTenantTx(t, channel, func(ctx context.Context) {
			series := channel.forSeries(ctx, append(append([]Event{}, plain...), staff)).(durableReceiptChannel)
			count, err := series.DeliverDurably(ctx, staff)
			require.NoError(t, err)
			assert.Equal(t, 1, count)
		})
		assert.Empty(t, repo.reads, "the guardian read waits for a guardian event")
	})
}

// openGates admits every event: dispatch on, Eltern-App booked, no quiet hours.
type openGates struct{}

func (openGates) ResolveBoolForTenant(context.Context, int64, string) (bool, error) { return true, nil }
func (openGates) ResolveStringForTenant(context.Context, int64, string) (string, error) {
	return "00:00", nil
}

// refusingOutbox fails the intents of one title.
type refusingOutbox struct {
	fakePushOutbox
	refuse string
}

func (o *refusingOutbox) EnqueuePush(ctx context.Context, intent PushIntent) (bool, error) {
	if intent.Title == o.refuse {
		return false, errRefusedIntent
	}
	return o.fakePushOutbox.EnqueuePush(ctx, intent)
}

var errRefusedIntent = errors.New("refused intent")

func TestNotifyEachStopsAtTheFirstFailingVariant(t *testing.T) {
	t.Parallel()
	events := []Event{seriesGuardianEvent("de", "Hallo", nil, 7), seriesGuardianEvent("en", "Hello", nil, 8), seriesGuardianEvent("tr", "Merhaba", nil, 9)}
	for name, run := range map[string]func(context.Context, *router) (any, error){
		"notify":  func(ctx context.Context, r *router) (any, error) { return r.NotifyEach(ctx, events) },
		"durable": func(ctx context.Context, r *router) (any, error) { return r.NotifyEachDurably(ctx, events) },
	} {
		t.Run(name, func(t *testing.T) {
			channel, repo := newSeriesTestChannel()
			outbox := &refusingOutbox{refuse: "Hello"}
			channel.outbox = outbox
			r := &router{settings: openGates{}, channels: []Channel{channel}}
			var got any
			var err error
			withinTenantTx(t, channel, func(ctx context.Context) { got, err = run(ctx, r) })
			require.ErrorIs(t, err, errRefusedIntent)
			assert.Equal(t, map[string]any{"notify": 1, "durable": []int{1}}[name], got, "only the variants before the failure count")
			require.Len(t, outbox.intents, 1)
			assert.Equal(t, "Hallo", outbox.intents[0].Title, "nothing after the failing variant is enqueued")
			assert.Len(t, repo.reads, 1)
		})
	}
}
