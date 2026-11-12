package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/communication"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationProducerPreservesNativeAudienceAndFailureReceipt(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	failure := errors.New("dispatch refused")
	original := NotificationEvent{
		Type: "kind", IdempotencyKey: "key", RelatedType: "related", RelatedID: 13,
		Audience: NotificationAudience{TenantID: 17, Scope: "guardian", GuardianAccountID: 19, GuardianAccountIDs: []int64{23}, ActiveGroupID: "group", StaffAccountIDs: []int64{29}, StudentIDs: []int64{31}},
		Priority: "high", Title: "title", Body: "body", DeepLink: "/parent", SchoolDeepLink: "/staff", Data: map[string]string{"key": "value"}, Portal: "parents",
	}
	expected := communication.NotificationEvent{
		Type: "kind", IdempotencyKey: "key", RelatedType: "related", RelatedID: 13,
		Audience: communication.NotificationAudience{TenantID: 17, Scope: "guardian", GuardianAccountID: 19, GuardianAccountIDs: []int64{23}, ActiveGroupID: "group", StaffAccountIDs: []int64{29}, StudentIDs: []int64{31}},
		Priority: "high", Title: "title", Body: "body", DeepLink: "/parent", SchoolDeepLink: "/staff", Data: map[string]string{"key": "value"}, Portal: "parents",
	}
	var calls int
	send := NewNotificationSender(func(actual context.Context, event communication.NotificationEvent) error {
		calls++
		assert.Same(t, ctx, actual)
		assert.Equal(t, expected, event)
		return failure
	})
	assert.Same(t, failure, send.Notify(ctx, original.Public()))
	assert.Equal(t, 1, calls)
	assert.Nil(t, NewNotificationSender(nil))
	require.NoError(t, NewNotificationSender(func(_ context.Context, event communication.NotificationEvent) error {
		assert.Nil(t, event.Data)
		assert.Nil(t, event.Audience.GuardianAccountIDs)
		assert.Nil(t, event.Audience.StaffAccountIDs)
		assert.Nil(t, event.Audience.StudentIDs)
		return nil
	}).Notify(ctx, NotificationEvent{}.Public()))
}
