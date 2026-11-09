package notifications

import (
	"context"
	"slices"

	deliveryModels "github.com/moto-nrw/project-phoenix/modules/delivery/subscriptions"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// A series is several events a producer would otherwise notify one after the
// other, such as one announcement in each language of its audience (#5869,
// #5876). Each event keeps the validation, tenant gates, durable intents,
// after-commit dispatch and error of its own Notify or NotifyDurably call, in
// the given order; the first error ends the series. Only the device read is
// shared: the guardians of all events are read once, in the transaction the
// series runs in, and each event gets the devices of its own guardians.

// seriesChannel is a channel that can serve a series better than one event
// at a time. forSeries returns the channel to use for exactly these events.
type seriesChannel interface {
	forSeries(ctx context.Context, events []Event) Channel
}

// NotifyEach notifies the events in order, as consecutive Notify calls
// would. It returns how many were accepted before the first error.
func (r *router) NotifyEach(ctx context.Context, events []Event) (int, error) {
	series := r.forSeries(ctx, events)
	for i, event := range events {
		if err := series.Notify(ctx, event); err != nil {
			return i, err
		}
	}
	return len(events), nil
}

// NotifyEachDurably notifies the events in order, as consecutive
// NotifyDurably calls would. It returns the receipt of every event accepted
// before the first error; the failing event's partial receipt is dropped.
func (r *router) NotifyEachDurably(ctx context.Context, events []Event) ([]int, error) {
	series := r.forSeries(ctx, events)
	receipts := make([]int, 0, len(events))
	for _, event := range events {
		count, err := series.NotifyDurably(ctx, event)
		if err != nil {
			return receipts, err
		}
		receipts = append(receipts, count)
	}
	return receipts, nil
}

func (r *router) forSeries(ctx context.Context, events []Event) *router {
	series := *r
	series.channels = make([]Channel, len(r.channels))
	for i, channel := range r.channels {
		if shared, ok := channel.(seriesChannel); ok {
			channel = shared.forSeries(ctx, events)
		}
		series.channels[i] = channel
	}
	return &series
}

// guardianDevices is the shared device read of a series' guardian events.
// It is only used inside the transaction the series runs in, so every event
// reads the state an event of its own would have read there (E2 of the
// roundtrip decisions, no lock in between).
type guardianDevices struct {
	tenantID   int64
	studentIDs []int64
	accountIDs []int64
	loaded     bool
	subs       []*deliveryModels.PushSubscription
}

// forSeries shares the device read when at least two guardian events of one
// tenant and one child filter run in an open transaction. Anything else is
// delivered by the plain channel, one read per event.
func (c *webPushChannel) forSeries(ctx context.Context, events []Event) Channel {
	if _, active := tenant.TransactionFromContext(c.withTenantRuntime(ctx)); !active {
		return c
	}
	var devices *guardianDevices
	shared := 0
	seen := make(map[int64]bool)
	for _, event := range events {
		if event.Audience.Scope != ScopeGuardian {
			continue
		}
		if devices == nil {
			devices = &guardianDevices{tenantID: event.Audience.TenantID, studentIDs: event.Audience.StudentIDs}
		}
		if !devices.covers(event) {
			return c
		}
		shared++
		for _, accountID := range guardianAccountIDs(event.Audience) {
			if !seen[accountID] {
				seen[accountID] = true
				devices.accountIDs = append(devices.accountIDs, accountID)
			}
		}
	}
	if shared < 2 {
		return c
	}
	series := *c
	series.devices = devices
	return &series
}

func (d *guardianDevices) covers(event Event) bool {
	return d != nil && event.Audience.Scope == ScopeGuardian && event.Audience.TenantID == d.tenantID &&
		slices.Equal(event.Audience.StudentIDs, d.studentIDs)
}

// resolveSubscriptions reads an event's devices: from the series' shared read
// when the event belongs to it, otherwise on its own.
func (c *webPushChannel) resolveSubscriptions(ctx context.Context, event Event, devices *guardianDevices) ([]*deliveryModels.PushSubscription, error) {
	if !devices.covers(event) {
		return c.resolveEventSubscriptions(ctx, event)
	}
	if !devices.loaded {
		subs, err := c.repo.FindForGuardians(ctx, devices.accountIDs, devices.studentIDs)
		if err != nil {
			return nil, err
		}
		devices.subs, devices.loaded = subs, true
	}
	wanted := make(map[int64]bool)
	for _, accountID := range guardianAccountIDs(event.Audience) {
		wanted[accountID] = true
	}
	var subs []*deliveryModels.PushSubscription
	for _, sub := range devices.subs {
		if wanted[sub.AccountID] {
			subs = append(subs, sub)
		}
	}
	return subs, nil
}
