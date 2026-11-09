package api

import (
	"context"
	"errors"
	"testing"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type communicationDeliveryProbe func(context.Context, devices.NotificationEvent) (int, error)

func (send communicationDeliveryProbe) NotifyEvent(ctx context.Context, event devices.NotificationEvent) error {
	_, err := send(ctx, event)
	return err
}
func (send communicationDeliveryProbe) NotifyEventDurably(ctx context.Context, event devices.NotificationEvent) (int, error) {
	return send(ctx, event)
}

// NotifyEvents and NotifyEventsDurably answer each event of a series on its own.
func (send communicationDeliveryProbe) NotifyEvents(ctx context.Context, events []devices.NotificationEvent) (int, error) {
	for i, event := range events {
		if _, err := send(ctx, event); err != nil {
			return i, err
		}
	}
	return len(events), nil
}
func (send communicationDeliveryProbe) NotifyEventsDurably(ctx context.Context, events []devices.NotificationEvent) ([]int, error) {
	receipts := []int{}
	for _, event := range events {
		count, err := send(ctx, event)
		if err != nil {
			return receipts, err
		}
		receipts = append(receipts, count)
	}
	return receipts, nil
}

func TestCommunicationDeliveryPreservesRecipientSelectorsAndPartialReceipt(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	failure := errors.New("delivery refused after partial receipt")
	event := communication.NotificationEvent{
		Type: "kind", IdempotencyKey: "idempotency", RelatedType: "entity", RelatedID: 19,
		Audience: communication.NotificationAudience{TenantID: 23, Scope: "guardian", GuardianAccountID: 29, GuardianAccountIDs: []int64{31}, ActiveGroupID: "group", StaffAccountIDs: []int64{37}, StudentIDs: []int64{41}},
		Priority: "high", Title: "title", Body: "body", DeepLink: "/children", SchoolDeepLink: "/staff", Data: map[string]string{"key": "value"}, Portal: "parents",
	}
	expected := devices.NotificationEvent{
		Type: "kind", IdempotencyKey: "idempotency", RelatedType: "entity", RelatedID: 19,
		Audience: devices.NotificationAudience{TenantID: 23, Scope: "guardian", GuardianAccountID: 29, GuardianAccountIDs: []int64{31}, ActiveGroupID: "group", StaffAccountIDs: []int64{37}, StudentIDs: []int64{41}},
		Priority: "high", Title: "title", Body: "body", DeepLink: "/children", SchoolDeepLink: "/staff", Data: map[string]string{"key": "value"}, Portal: "parents",
	}
	var receivedContext context.Context
	var receivedEvent devices.NotificationEvent
	source := communicationDeliveryProbe(func(ctx context.Context, event devices.NotificationEvent) (int, error) {
		receivedContext, receivedEvent = ctx, event
		return 7, failure
	})
	require.Same(t, failure, owners.BindCommunicationNotificationSender(source)(ctx, event))
	assert.Same(t, ctx, receivedContext)
	assert.Equal(t, expected, receivedEvent)
	series := owners.BindCommunicationNotificationSeries(source)
	accepted, err := series.Notify(ctx, []communication.NotificationEvent{event})
	assert.Zero(t, accepted)
	assert.Same(t, failure, err)
	assert.Same(t, ctx, receivedContext)
	assert.Equal(t, expected, receivedEvent)
	receipts, err := series.NotifyDurably(ctx, []communication.NotificationEvent{event})
	assert.Empty(t, receipts, "the failing variant's partial receipt is not an accepted variant")
	assert.Same(t, failure, err)
	assert.Equal(t, expected, receivedEvent)
	accepting := communicationDeliveryProbe(func(context.Context, devices.NotificationEvent) (int, error) { return 7, nil })
	receipts, err = owners.BindCommunicationNotificationSeries(accepting).NotifyDurably(ctx, []communication.NotificationEvent{event, event})
	require.NoError(t, err)
	assert.Equal(t, []int{7, 7}, receipts)
	assert.Nil(t, owners.BindCommunicationNotificationSender(nil))
	assert.Equal(t, communication.NotificationSeries{}, owners.BindCommunicationNotificationSeries(nil))
}

func TestCommunicationDeliveryPreservesAbsentAndEmptySelectors(t *testing.T) {
	t.Parallel()
	for _, empty := range []bool{false, true} {
		event := communication.NotificationEvent{}
		if empty {
			event.Data = map[string]string{}
			event.Audience.GuardianAccountIDs = []int64{}
			event.Audience.StaffAccountIDs = []int64{}
			event.Audience.StudentIDs = []int64{}
		}
		var receivedEvent devices.NotificationEvent
		source := communicationDeliveryProbe(func(_ context.Context, event devices.NotificationEvent) (int, error) {
			receivedEvent = event
			return 0, nil
		})
		require.NoError(t, owners.BindCommunicationNotificationSender(source)(context.Background(), event))
		assert.Equal(t, empty, receivedEvent.Data != nil)
		assert.Equal(t, empty, receivedEvent.Audience.GuardianAccountIDs != nil)
		assert.Equal(t, empty, receivedEvent.Audience.StaffAccountIDs != nil)
		assert.Equal(t, empty, receivedEvent.Audience.StudentIDs != nil)
	}
}
