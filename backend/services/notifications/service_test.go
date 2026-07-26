package notifications_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type recordingChannel struct {
	name   string
	err    error
	events []notifications.Event
}

func (c *recordingChannel) Name() string {
	return c.name
}

func (c *recordingChannel) Deliver(_ context.Context, event notifications.Event) error {
	c.events = append(c.events, event)
	return c.err
}

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

	event := tenantEvent(41)
	event.Data = map[string]string{"reminder_id": "123"}
	require.NoError(t, svc.Notify(context.Background(), event))
	event.Data["reminder_id"] = "mutated-after-notify"

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
	assert.Equal(t, map[string]string{"reminder_id": "123"}, call.Event.Data.NotificationData)
}

func TestNotifyDefersDeliveryUntilCommit(t *testing.T) {
	channel := &recordingChannel{name: "recording"}
	svc := notifications.NewService(enabledSettings(true), nil, channel)
	ctx, commit := tenant.WithAfterCommitHooksForTest(context.Background())

	event := tenantEvent(41)
	event.Data = map[string]string{"reminder_id": "123"}
	event.Audience = notifications.Audience{
		TenantID:           41,
		Scope:              notifications.ScopeGuardian,
		GuardianAccountIDs: []int64{77, 88},
	}
	require.NoError(t, svc.Notify(ctx, event))
	assert.Empty(t, channel.events, "notification must not be delivered before commit")

	event.Data["reminder_id"] = "mutated-before-commit"
	event.Audience.GuardianAccountIDs[0] = 999
	commit()

	require.Len(t, channel.events, 1)
	assert.Equal(t, map[string]string{"reminder_id": "123"}, channel.events[0].Data,
		"queued notification must use the event snapshot taken by Notify")
	assert.Equal(t, []int64{77, 88}, channel.events[0].Audience.GuardianAccountIDs,
		"queued notification must snapshot batched guardian ids")
}

func TestNotifyDropsQueuedDeliveryOnRollback(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		_ = db.Close()
		_ = sqlDB.Close()
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_tenant").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SELECT set_config").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	channel := &recordingChannel{name: "recording"}
	svc := notifications.NewService(enabledSettings(true), nil, channel)
	rollbackErr := errors.New("force rollback")
	err = tenant.WithTenantTx(context.Background(), db, 41, func(ctx context.Context, _ bun.Tx) error {
		require.NoError(t, svc.Notify(ctx, tenantEvent(41)))
		assert.Empty(t, channel.events, "notification must remain queued inside the transaction")
		return rollbackErr
	})

	require.ErrorIs(t, err, rollbackErr)
	assert.Empty(t, channel.events, "rollback must discard the queued notification")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNotifyAdminGuardianAndGroupRouting(t *testing.T) {
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := notifications.NewService(enabledSettings(true), nil, notifications.NewSSEChannel(broadcaster))

	admin := tenantEvent(41)
	admin.Audience.Scope = notifications.ScopeAdmin
	require.NoError(t, svc.Notify(context.Background(), admin))

	guardian := tenantEvent(41)
	guardian.Audience.Scope = notifications.ScopeGuardian
	guardian.Audience.GuardianAccountID = 77
	require.NoError(t, svc.Notify(context.Background(), guardian))

	guardianBatch := tenantEvent(41)
	guardianBatch.Audience.Scope = notifications.ScopeGuardian
	guardianBatch.Audience.GuardianAccountIDs = []int64{88, 99}
	require.NoError(t, svc.Notify(context.Background(), guardianBatch))

	group := tenantEvent(41)
	group.Audience.Scope = notifications.ScopeGroup
	group.Audience.ActiveGroupID = "g-9"
	require.NoError(t, svc.Notify(context.Background(), group))

	adminCalls := broadcaster.CallsByMethod("admin")
	require.Len(t, adminCalls, 1)
	assert.Equal(t, int64(41), adminCalls[0].TenantID)

	guardianCalls := broadcaster.CallsByMethod("guardian")
	require.Len(t, guardianCalls, 3)
	assert.Equal(t, int64(77), guardianCalls[0].GuardianID)
	assert.Equal(t, int64(88), guardianCalls[1].GuardianID)
	assert.Equal(t, int64(99), guardianCalls[2].GuardianID)

	groupCalls := broadcaster.CallsByMethod("group")
	require.Len(t, groupCalls, 1)
	assert.Equal(t, "g-9", groupCalls[0].Topic)
}

func TestNotifyValidation(t *testing.T) {
	svc := notifications.NewService(enabledSettings(true), nil)
	ctx := context.Background()

	missingType := tenantEvent(41)
	missingType.Type = ""
	assert.EqualError(t, svc.Notify(ctx, missingType), "notification event requires a type")

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

	guardianWithMixedIDs := tenantEvent(41)
	guardianWithMixedIDs.Audience.Scope = notifications.ScopeGuardian
	guardianWithMixedIDs.Audience.GuardianAccountID = 77
	guardianWithMixedIDs.Audience.GuardianAccountIDs = []int64{88}
	assert.Error(t, svc.Notify(ctx, guardianWithMixedIDs))

	guardianWithInvalidBatch := tenantEvent(41)
	guardianWithInvalidBatch.Audience.Scope = notifications.ScopeGuardian
	guardianWithInvalidBatch.Audience.GuardianAccountIDs = []int64{77, 0}
	assert.Error(t, svc.Notify(ctx, guardianWithInvalidBatch))

	groupWithoutID := tenantEvent(41)
	groupWithoutID.Audience.Scope = notifications.ScopeGroup
	assert.EqualError(t, svc.Notify(ctx, groupWithoutID), "group-scoped notification requires an active group id")

	externalLink := tenantEvent(41)
	externalLink.DeepLink = "https://example.com/phish"
	assert.Error(t, svc.Notify(ctx, externalLink))

	protocolRelative := tenantEvent(41)
	protocolRelative.DeepLink = "//example.com"
	assert.Error(t, svc.Notify(ctx, protocolRelative))

	backslashRelative := tenantEvent(41)
	backslashRelative.DeepLink = `/\evil.example/path`
	assert.EqualError(t, svc.Notify(ctx, backslashRelative), "deep link must be an app-relative path")

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

func TestNotifyConfigurationErrors(t *testing.T) {
	ctx := context.Background()

	withoutSettings := notifications.NewService(nil, nil)
	assert.EqualError(t, withoutSettings.Notify(ctx, tenantEvent(41)),
		"notifications service has no settings service configured")

	settingsErr := errors.New("settings unavailable")
	failingSettings := &configtest.Mock{
		ResolveBoolForTenantFn: func(_ context.Context, _ int64, _ string) (bool, error) {
			return false, settingsErr
		},
	}
	svc := notifications.NewService(failingSettings, nil)
	err := svc.Notify(ctx, tenantEvent(41))
	require.ErrorContains(t, err, "resolving notification feature flag")
	require.ErrorIs(t, err, settingsErr)
}

func TestNotifyContinuesAfterChannelFailureAndLogs(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	failing := &recordingChannel{name: "failing", err: assert.AnError}
	succeeding := &recordingChannel{name: "succeeding"}
	svc := notifications.NewService(enabledSettings(true), logger, failing, succeeding)

	event := tenantEvent(41)
	event.Priority = notifications.PriorityHigh
	require.NoError(t, svc.Notify(context.Background(), event))

	require.Len(t, failing.events, 1)
	require.Len(t, succeeding.events, 1, "one failed channel must not block later channels")
	assert.Equal(t, notifications.PriorityHigh, succeeding.events[0].Priority)
	assert.Contains(t, logs.String(), "notification channel delivery failed")
	assert.Contains(t, logs.String(), "channel=failing")
}

func TestSSEChannelRejectsUnknownAudienceScope(t *testing.T) {
	channel := notifications.NewSSEChannel(testpkg.NewRecordingBroadcaster())
	event := tenantEvent(41)
	event.Audience.Scope = "unknown"

	err := channel.Deliver(context.Background(), event)
	require.EqualError(t, err, `sse channel cannot route audience scope "unknown" (tenant 41)`)
}

// TestWebPushChannelUnconfigured replaces the former TestWebPushChannelStub:
// with #2003 the channel is real, and the guaranteed no-op case is now
// "VAPID keys not configured" (the behavior every keyless environment gets).
func TestWebPushChannelUnconfigured(t *testing.T) {
	event := tenantEvent(41)

	defaultLoggerChannel := notifications.NewWebPushChannel(nil, nil, notifications.VAPIDConfig{}, nil)
	assert.Equal(t, "web_push", defaultLoggerChannel.Name())
	require.NoError(t, defaultLoggerChannel.Deliver(context.Background(), event))

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	channel := notifications.NewWebPushChannel(nil, nil, notifications.VAPIDConfig{}, logger)
	require.NoError(t, channel.Deliver(context.Background(), event))
	assert.Contains(t, logs.String(), "no VAPID keys configured")
	assert.Contains(t, logs.String(), "notification_type=test")
	assert.Contains(t, logs.String(), "tenant_id=41")
}
