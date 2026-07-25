package notifications_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// enabledSettings returns a settings mock with the dispatch flag set as given.
func enabledSettings(enabled bool) *configtest.Mock {
	return &configtest.Mock{
		ResolveBoolForTenantFn: func(_ context.Context, _ int64, key string) (bool, error) {
			if key == configModel.KeyNotificationsDispatchEnabled {
				return enabled, nil
			}
			return false, nil
		},
	}
}

func tenantEvent(tenantID int64) notifications.Event {
	return notifications.Event{
		Type:     "test",
		Audience: notifications.Audience{TenantID: tenantID, Scope: notifications.ScopeTenant},
		Title:    "Testbenachrichtigung",
		Body:     "Die Benachrichtigungen funktionieren.",
		DeepLink: "/dashboard",
	}
}

func TestNotifyDisabledByDefault(t *testing.T) {
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := notifications.NewService(enabledSettings(false), nil, notifications.NewSSEChannel(broadcaster))

	err := svc.Notify(context.Background(), tenantEvent(41))
	require.ErrorIs(t, err, notifications.ErrDisabled)
	assert.Empty(t, broadcaster.Calls(), "no channel may deliver while the feature flag is off")
}

func TestNotifyDeliversTenantScopedSSE(t *testing.T) {
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := notifications.NewService(enabledSettings(true), nil, notifications.NewSSEChannel(broadcaster))

	require.NoError(t, svc.Notify(context.Background(), tenantEvent(41)))

	calls := broadcaster.CallsByMethod("tenant")
	require.Len(t, calls, 1)
	call := calls[0]
	assert.Equal(t, int64(41), call.TenantID)
	assert.Equal(t, realtime.EventNotification, call.Event.Type)
	require.NotNil(t, call.Event.Data.Title)
	assert.Equal(t, "Testbenachrichtigung", *call.Event.Data.Title)
	require.NotNil(t, call.Event.Data.Priority)
	assert.Equal(t, notifications.PriorityNormal, *call.Event.Data.Priority, "empty priority defaults to normal")
	require.NotNil(t, call.Event.Data.DeepLink)
	assert.Equal(t, "/dashboard", *call.Event.Data.DeepLink)
}

func TestNotifyGuardianAndGroupRouting(t *testing.T) {
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := notifications.NewService(enabledSettings(true), nil, notifications.NewSSEChannel(broadcaster))

	guardian := tenantEvent(41)
	guardian.Audience.Scope = notifications.ScopeGuardian
	guardian.Audience.GuardianAccountID = 77
	require.NoError(t, svc.Notify(context.Background(), guardian))

	group := tenantEvent(41)
	group.Audience.Scope = notifications.ScopeGroup
	group.Audience.ActiveGroupID = "g-9"
	require.NoError(t, svc.Notify(context.Background(), group))

	guardianCalls := broadcaster.CallsByMethod("guardian")
	require.Len(t, guardianCalls, 1)
	assert.Equal(t, int64(77), guardianCalls[0].GuardianID)

	groupCalls := broadcaster.CallsByMethod("group")
	require.Len(t, groupCalls, 1)
	assert.Equal(t, "g-9", groupCalls[0].Topic)
}

func TestNotifyValidation(t *testing.T) {
	svc := notifications.NewService(enabledSettings(true), nil)
	ctx := context.Background()

	missingTitle := tenantEvent(41)
	missingTitle.Title = ""
	assert.Error(t, svc.Notify(ctx, missingTitle))

	missingTenant := tenantEvent(0)
	assert.Error(t, svc.Notify(ctx, missingTenant))

	badScope := tenantEvent(41)
	badScope.Audience.Scope = "everyone"
	assert.Error(t, svc.Notify(ctx, badScope))

	guardianWithoutAccount := tenantEvent(41)
	guardianWithoutAccount.Audience.Scope = notifications.ScopeGuardian
	assert.Error(t, svc.Notify(ctx, guardianWithoutAccount))

	externalLink := tenantEvent(41)
	externalLink.DeepLink = "https://example.com/phish"
	assert.Error(t, svc.Notify(ctx, externalLink))

	protocolRelative := tenantEvent(41)
	protocolRelative.DeepLink = "//example.com"
	assert.Error(t, svc.Notify(ctx, protocolRelative))

	badPriority := tenantEvent(41)
	badPriority.Priority = "urgent"
	assert.Error(t, svc.Notify(ctx, badPriority))
}

func TestNotifyChannelErrorDoesNotPropagate(t *testing.T) {
	broadcaster := testpkg.NewRecordingBroadcaster()
	broadcaster.Err = assert.AnError
	svc := notifications.NewService(enabledSettings(true), nil, notifications.NewSSEChannel(broadcaster))

	assert.NoError(t, svc.Notify(context.Background(), tenantEvent(41)),
		"channel failures are fire-and-forget and must not reach the caller")
}
