package notifications_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/moto-nrw/project-phoenix/services/notifications"
)

// syncChannel is a channel that also reports acceptance synchronously — the
// shape NotifySynchronously singles out among the registered channels.
type syncChannel struct {
	recordingChannel
	syncErr    error
	syncEvents []notifications.Event
}

func (c *syncChannel) DeliverSynchronously(_ context.Context, event notifications.Event) error {
	c.syncEvents = append(c.syncEvents, event)
	return c.syncErr
}

// windowSettings resolves the dispatch flag plus an active window whose bounds
// are given as "HH:MM". Equal bounds mean an unrestricted 24h window.
func windowSettings(enabled bool, start, end string) *configtest.Mock {
	return &configtest.Mock{
		ResolveBoolForTenantFn: func(_ context.Context, _ int64, key string) (bool, error) {
			if key == configModel.KeyNotificationsDispatchEnabled {
				return enabled, nil
			}
			return false, nil
		},
		ResolveStringForTenantFn: func(_ context.Context, _ int64, key string) (string, error) {
			switch key {
			case configModel.KeyNotificationsActiveWindowStart:
				return start, nil
			case configModel.KeyNotificationsActiveWindowEnd:
				return end, nil
			}
			return "", nil
		},
	}
}

// openWindowSettings enables dispatch with an unrestricted delivery window.
func openWindowSettings() *configtest.Mock {
	return windowSettings(true, "00:00", "00:00")
}

// windowAfterNow renders an "HH:MM" window that starts startOffset minutes from
// now and ends endOffset minutes from now, so it never contains the moment the
// test runs. The window check reads the real clock, so a fixed window would be
// open for whoever runs the suite at the wrong time of day.
func windowAfterNow(startOffset, endOffset int) (start, end string) {
	now := timezone.Now()
	nowMin := now.Hour()*60 + now.Minute()
	clock := func(offset int) string {
		minute := (nowMin + offset) % (24 * 60)
		return fmt.Sprintf("%02d:%02d", minute/60, minute%60)
	}
	return clock(startOffset), clock(endOffset)
}

func syncEvent() notifications.Event {
	return notifications.Event{
		Type: notifications.TypeParentAppointmentReminder,
		Audience: notifications.Audience{
			TenantID:           41,
			Scope:              notifications.ScopeGuardian,
			GuardianAccountIDs: []int64{77},
		},
		Title:    "Terminerinnerung",
		Body:     "Ein Termin für Sie steht bald an.",
		DeepLink: "/calendar",
	}
}

// NotifySynchronously is the durable producers' entry point: unlike Notify it
// must report a delivery failure back to the caller, because the caller holds a
// claim it has to release when the push was not accepted.
func TestNotifySynchronouslyDeliversAndReportsAcceptance(t *testing.T) {
	channel := &syncChannel{recordingChannel: recordingChannel{name: "web_push"}}
	svc := notifications.NewService(openWindowSettings(), nil, channel)

	require.NoError(t, svc.NotifySynchronously(context.Background(), syncEvent()))

	require.Len(t, channel.syncEvents, 1)
	assert.Equal(t, notifications.PriorityNormal, channel.syncEvents[0].Priority,
		"an empty priority defaults to normal, exactly like Notify")
	assert.Empty(t, channel.events, "the async path must not run for a synchronous dispatch")
}

func TestNotifySynchronouslyReturnsChannelFailure(t *testing.T) {
	deliveryErr := errors.New("push service unavailable")
	channel := &syncChannel{recordingChannel: recordingChannel{name: "web_push"}, syncErr: deliveryErr}
	svc := notifications.NewService(openWindowSettings(), nil, channel)

	err := svc.NotifySynchronously(context.Background(), syncEvent())

	require.ErrorIs(t, err, deliveryErr,
		"a durable producer must see the failure so it can release its claim and retry")
	assert.ErrorContains(t, err, "web_push")
}

func TestNotifySynchronouslySkipsAsyncOnlyChannels(t *testing.T) {
	asyncOnly := &recordingChannel{name: "sse"}
	svc := notifications.NewService(openWindowSettings(), nil, asyncOnly)

	require.NoError(t, svc.NotifySynchronously(context.Background(), syncEvent()))
	assert.Empty(t, asyncOnly.events,
		"a channel without synchronous acceptance is skipped, not delivered to twice")
}

func TestNotifySynchronouslyHonorsTenantGates(t *testing.T) {
	ctx := context.Background()

	t.Run("validation rejects a malformed event before any channel sees it", func(t *testing.T) {
		channel := &syncChannel{recordingChannel: recordingChannel{name: "web_push"}}
		svc := notifications.NewService(openWindowSettings(), nil, channel)

		event := syncEvent()
		event.Title = ""
		require.Error(t, svc.NotifySynchronously(ctx, event))
		assert.Empty(t, channel.syncEvents)
	})

	t.Run("an unwired settings service is a configuration error", func(t *testing.T) {
		svc := notifications.NewService(nil, nil)
		assert.EqualError(t, svc.NotifySynchronously(ctx, syncEvent()),
			"notifications service has no settings service configured")
	})

	t.Run("the tenant feature flag off returns ErrDisabled", func(t *testing.T) {
		channel := &syncChannel{recordingChannel: recordingChannel{name: "web_push"}}
		svc := notifications.NewService(windowSettings(false, "00:00", "00:00"), nil, channel)

		require.ErrorIs(t, svc.NotifySynchronously(ctx, syncEvent()), notifications.ErrDisabled)
		assert.Empty(t, channel.syncEvents)
	})

	t.Run("a feature flag read failure is wrapped, not swallowed", func(t *testing.T) {
		settingsErr := errors.New("settings unavailable")
		svc := notifications.NewService(&configtest.Mock{
			ResolveBoolForTenantFn: func(context.Context, int64, string) (bool, error) {
				return false, settingsErr
			},
		}, nil)

		err := svc.NotifySynchronously(ctx, syncEvent())
		require.ErrorIs(t, err, settingsErr)
		assert.ErrorContains(t, err, "resolving notification feature flag")
	})

	t.Run("outside the delivery window nothing is sent", func(t *testing.T) {
		channel := &syncChannel{recordingChannel: recordingChannel{name: "web_push"}}
		// A window that opens an hour from now cannot contain now, whatever the
		// wall clock happens to be while the suite runs.
		start, end := windowAfterNow(60, 120)
		svc := notifications.NewService(windowSettings(true, start, end), nil, channel)

		require.ErrorIs(t, svc.NotifySynchronously(ctx, syncEvent()), notifications.ErrOutsideActiveWindow,
			"a reminder must not wake a family outside the school's delivery window")
		assert.Empty(t, channel.syncEvents)
	})

	t.Run("a blank window bound fails closed", func(t *testing.T) {
		channel := &syncChannel{recordingChannel: recordingChannel{name: "web_push"}}
		svc := notifications.NewService(windowSettings(true, "", ""), nil, channel)

		require.ErrorIs(t, svc.NotifySynchronously(ctx, syncEvent()), notifications.ErrOutsideActiveWindow)
		assert.Empty(t, channel.syncEvents)
	})

	t.Run("a window read failure is returned", func(t *testing.T) {
		windowErr := errors.New("window unavailable")
		svc := notifications.NewService(&configtest.Mock{
			ResolveBoolForTenantFn: func(_ context.Context, _ int64, key string) (bool, error) {
				return key == configModel.KeyNotificationsDispatchEnabled, nil
			},
			ResolveStringForTenantFn: func(context.Context, int64, string) (string, error) {
				return "", windowErr
			},
		}, nil)

		require.ErrorIs(t, svc.NotifySynchronously(ctx, syncEvent()), windowErr)
	})

	t.Run("the test notification is exempt from the window", func(t *testing.T) {
		channel := &syncChannel{recordingChannel: recordingChannel{name: "web_push"}}
		svc := notifications.NewService(windowSettings(true, "", ""), nil, channel)

		event := syncEvent()
		event.Type = notifications.TypeTest
		require.NoError(t, svc.NotifySynchronously(ctx, event))
		assert.Len(t, channel.syncEvents, 1)
	})
}
