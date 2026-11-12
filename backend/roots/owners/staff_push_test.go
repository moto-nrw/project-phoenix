package owners

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
)

type staffPushPreferencesFunc func(context.Context, string, []int64) ([]int64, error)

func (f staffPushPreferencesFunc) FilterNotOptedOut(ctx context.Context, kind string, ids []int64) ([]int64, error) {
	return f(ctx, kind, ids)
}

type staffPushRouterFunc func(context.Context, deliveryCompose.NotificationEvent) error

func (f staffPushRouterFunc) Notify(ctx context.Context, event deliveryCompose.NotificationEvent) error {
	return f(ctx, event)
}

func TestStaffPushDropsDeclinesAndKeepsTheRest(t *testing.T) {
	t.Parallel()
	var sent deliveryCompose.NotificationEvent
	notifier := StaffPushNotifier{
		preferences: staffPushPreferencesFunc(func(_ context.Context, kind string, ids []int64) ([]int64, error) {
			assert.Equal(t, "staff_kind", kind)
			return ids[1:], nil // the first one switched it off
		}),
		router: staffPushRouterFunc(func(_ context.Context, event deliveryCompose.NotificationEvent) error {
			sent = event
			return nil
		}),
	}
	push := StaffPush{TenantID: 7, Type: "staff_kind", IdempotencyKey: "k", Title: "T", AccountIDs: []int64{1, 2, 3}}
	require.NoError(t, notifier.Notify(t.Context(), push))
	assert.Equal(t, []int64{2, 3}, sent.Audience.StaffAccountIDs)
	assert.Equal(t, push.TenantID, sent.Audience.TenantID)
	assert.Equal(t, deliveryCompose.NotificationAudienceScope("staff"), sent.Audience.Scope)
}

// A school without notifications, or a time outside its window, is no
// failure: the in-app record carries the information. Every other error
// reaches the caller, which rolls its write back.
func TestStaffPushReportsOnlyRealFailures(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		routerErr error
		wantErr   bool
	}{
		"disabled":       {routerErr: deliveryCompose.ErrNotificationsDisabled},
		"outside window": {routerErr: deliveryCompose.ErrOutsideNotificationWindow},
		"outbox failure": {routerErr: errors.New("enqueue failed"), wantErr: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			notifier := StaffPushNotifier{
				preferences: staffPushPreferencesFunc(func(_ context.Context, _ string, ids []int64) ([]int64, error) { return ids, nil }),
				router:      staffPushRouterFunc(func(context.Context, deliveryCompose.NotificationEvent) error { return tc.routerErr }),
			}
			err := notifier.Notify(t.Context(), StaffPush{Type: "staff_kind", IdempotencyKey: "k", AccountIDs: []int64{1}})
			if tc.wantErr {
				require.ErrorIs(t, err, tc.routerErr)
				return
			}
			require.NoError(t, err)
		})
	}
	unbound := StaffPushNotifier{}
	require.Error(t, unbound.Notify(t.Context(), StaffPush{AccountIDs: []int64{1}}), "missing wiring is a configuration error")
}
