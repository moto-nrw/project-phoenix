package notifications_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/notifications"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// moduleSettings dispatches every tenant and cancels Eltern-App at the
// listed schools (#4895).
func moduleSettings(withoutElternApp ...int64) *settingsDouble {
	return &settingsDouble{
		ResolveBoolForTenantFn: func(_ context.Context, tenantID int64, key string) (bool, error) {
			switch key {
			case "notifications.dispatch_enabled":
				return true, nil
			case "module.eltern_app":
				for _, cancelled := range withoutElternApp {
					if cancelled == tenantID {
						return false, nil
					}
				}
				return true, nil
			}
			return key == "module.unterricht_vormittag", nil
		},
	}
}

func guardianEvent(tenantID, guardianID int64) notifications.Event {
	event := tenantEvent(tenantID)
	event.Audience.Scope = notifications.ScopeGuardian
	event.Audience.GuardianAccountID = guardianID
	return event
}

func TestParentPushNeedsElternApp(t *testing.T) {
	t.Parallel()

	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := notifications.NewService(moduleSettings(42), nil, notifications.NewSSEChannel(broadcaster))

	err := svc.Notify(context.Background(), guardianEvent(42, 77))
	require.ErrorIs(t, err, notifications.ErrDisabled, "producers treat it as a silent no-op")
	require.NoError(t, svc.Notify(context.Background(), guardianEvent(41, 88)))

	calls := broadcaster.CallsByMethod("guardian")
	require.Len(t, calls, 1, "only the school with Eltern-App reaches its parents")
	assert.Equal(t, int64(88), calls[0].GuardianID)
}

func TestParentPushNeedsElternAppOnEveryDispatchPath(t *testing.T) {
	t.Parallel()

	svc := notifications.NewService(moduleSettings(42), nil)
	ctx := context.Background()

	_, err := svc.NotifyDurably(ctx, guardianEvent(42, 77))
	assert.ErrorIs(t, err, notifications.ErrDisabled)
	assert.ErrorIs(t, svc.NotifySynchronously(ctx, guardianEvent(42, 77)), notifications.ErrDisabled)
	err = dispatch(t, func(ctx context.Context) error {
		return svc.NotifyBatch(ctx, []notifications.Event{guardianEvent(42, 77)})
	})
	assert.ErrorIs(t, err, notifications.ErrDisabled)
}

func TestStaffPushIgnoresElternApp(t *testing.T) {
	t.Parallel()

	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := notifications.NewService(moduleSettings(42), nil, notifications.NewSSEChannel(broadcaster))

	require.NoError(t, svc.Notify(context.Background(), tenantEvent(42)), "the team keeps its notifications")
}

func TestUnreadableModuleSwitchFailsTheDispatch(t *testing.T) {
	t.Parallel()

	readErr := errors.New("settings unavailable")
	svc := notifications.NewService(&settingsDouble{
		ResolveBoolForTenantFn: func(_ context.Context, _ int64, key string) (bool, error) {
			if key == "module.eltern_app" {
				return false, readErr
			}
			return true, nil
		},
	}, nil)

	err := svc.Notify(context.Background(), guardianEvent(41, 77))
	require.ErrorIs(t, err, readErr)
}
