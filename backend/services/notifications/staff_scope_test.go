package notifications_test

import (
	"context"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// settingsWithWindow builds a settings double with the dispatch flag on and an
// explicit delivery window.
func settingsWithWindow(start, end string) *configtest.Mock {
	return &configtest.Mock{
		ResolveBoolForTenantFn: func(_ context.Context, _ int64, key string) (bool, error) {
			return key == configModel.KeyNotificationsDispatchEnabled, nil
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

// testTenantID is the school every event in this file belongs to. These tests
// are pure in-memory (recording broadcaster, settings double, no database), so
// there is no fixture to derive an ID from.
const testTenantID int64 = 7

// dispatch runs fn inside an after-commit scope and then drains the hooks, so
// the assertions see what delivery actually did rather than what was queued.
func dispatch(t *testing.T, fn func(ctx context.Context) error) error {
	t.Helper()
	ctx, drain := tenant.WithAfterCommitHooksForTest(context.Background())
	err := fn(ctx)
	drain()
	return err
}

func staffEvent(tenantID int64, accountIDs ...int64) notifications.Event {
	return notifications.Event{
		Type: notifications.TypePickupUpcoming,
		Audience: notifications.Audience{
			TenantID:        tenantID,
			Scope:           notifications.ScopeStaff,
			StaffAccountIDs: accountIDs,
		},
		Title:    "Abholung steht an",
		Body:     "Ein Kind aus Ihrer Gruppe wird bald abgeholt.",
		DeepLink: "/reminders",
	}
}

// TestStaffScopeValidation pins that a staff-scoped event must name real
// recipients. An event that addresses nobody must be rejected rather than
// delivered to a fallback audience.
func TestStaffScopeValidation(t *testing.T) {
	svc := notifications.NewService(settingsWithWindow("", ""), nil,
		notifications.NewSSEChannel(testpkg.NewRecordingBroadcaster()))

	t.Run("no recipients is rejected", func(t *testing.T) {
		err := svc.Notify(context.Background(), staffEvent(7))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one staff account id")
	})

	t.Run("a non-positive recipient is rejected", func(t *testing.T) {
		err := svc.Notify(context.Background(), staffEvent(7, 11, 0))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "positive staff account ids")
	})
}

// TestStaffScopeRoutesToNamedAccounts pins that the SSE channel addresses
// exactly the named accounts and no other audience.
func TestStaffScopeRoutesToNamedAccounts(t *testing.T) {
	bc := testpkg.NewRecordingBroadcaster()
	svc := notifications.NewService(settingsWithWindow("", ""), nil,
		notifications.NewSSEChannel(bc))

	err := dispatch(t, func(ctx context.Context) error {
		return svc.Notify(ctx, staffEvent(7, 11, 22))
	})
	require.NoError(t, err)

	calls := bc.CallsByMethod("staff")
	require.Len(t, calls, 1)
	assert.Equal(t, testTenantID, calls[0].TenantID)
	assert.Equal(t, []int64{11, 22}, calls[0].AccountIDs)

	// A personal notification must not also reach the tenant, the admins or a
	// group: those are the audiences a mis-routed staff event would fall back
	// to, and each of them is a disclosure.
	assert.Empty(t, bc.CallsByMethod("tenant"))
	assert.Empty(t, bc.CallsByMethod("admin"))
	assert.Empty(t, bc.CallsByMethod("group"))
	assert.Empty(t, bc.CallsByMethod("guardian"))
}

// TestStaffAccountIDsAreSnapshotted pins that the recipient list survives the
// after-commit boundary unchanged. Delivery runs after Notify returns, so a
// caller reusing its slice would otherwise redirect a notification.
func TestStaffAccountIDsAreSnapshotted(t *testing.T) {
	bc := testpkg.NewRecordingBroadcaster()
	svc := notifications.NewService(settingsWithWindow("", ""), nil,
		notifications.NewSSEChannel(bc))

	recipients := []int64{11, 22}
	event := staffEvent(7, recipients...)

	err := dispatch(t, func(ctx context.Context) error {
		if nerr := svc.Notify(ctx, event); nerr != nil {
			return nerr
		}
		// Mutate the caller's slice before the hook runs.
		recipients[0] = 999
		return nil
	})
	require.NoError(t, err)

	calls := bc.CallsByMethod("staff")
	require.Len(t, calls, 1)
	assert.Equal(t, []int64{11, 22}, calls[0].AccountIDs,
		"the recipient list must be snapshotted, not aliased")
}

// TestDeliveryWindow pins the quiet-hours gate that lives in the router, so it
// holds for every producer rather than each carrying its own.
func TestDeliveryWindow(t *testing.T) {
	t.Run("outside the window nothing is dispatched", func(t *testing.T) {
		bc := testpkg.NewRecordingBroadcaster()
		// A window that has already closed for every possible current time.
		svc := notifications.NewService(settingsWithWindow("00:00", "00:01"), nil,
			notifications.NewSSEChannel(bc))

		err := dispatch(t, func(ctx context.Context) error {
			return svc.Notify(ctx, staffEvent(7, 11))
		})
		require.ErrorIs(t, err, notifications.ErrOutsideActiveWindow)
		assert.Empty(t, bc.Calls())
	})

	t.Run("the test notification ignores the window", func(t *testing.T) {
		bc := testpkg.NewRecordingBroadcaster()
		svc := notifications.NewService(settingsWithWindow("00:00", "00:01"), nil,
			notifications.NewSSEChannel(bc))

		// An admin verifying the setup in the evening must get an answer, not
		// silence that looks like a broken configuration.
		err := dispatch(t, func(ctx context.Context) error {
			return svc.Notify(ctx, tenantEvent(7))
		})
		require.NoError(t, err)
		assert.NotEmpty(t, bc.Calls())
	})

	t.Run("a malformed window does not silence the school", func(t *testing.T) {
		bc := testpkg.NewRecordingBroadcaster()
		svc := notifications.NewService(settingsWithWindow("nonsense", "18:00"), nil,
			notifications.NewSSEChannel(bc))

		err := dispatch(t, func(ctx context.Context) error {
			return svc.Notify(ctx, staffEvent(7, 11))
		})
		require.NoError(t, err)
		assert.NotEmpty(t, bc.Calls())
	})
}

// TestNotifyBatch pins the batched entry point.
func TestNotifyBatch(t *testing.T) {
	t.Run("delivers one event per recipient", func(t *testing.T) {
		bc := testpkg.NewRecordingBroadcaster()
		svc := notifications.NewService(settingsWithWindow("", ""), nil,
			notifications.NewSSEChannel(bc))

		err := dispatch(t, func(ctx context.Context) error {
			return svc.NotifyBatch(ctx, []notifications.Event{
				staffEvent(7, 11),
				staffEvent(7, 22),
			})
		})
		require.NoError(t, err)

		calls := bc.CallsByMethod("staff")
		require.Len(t, calls, 2)
		assert.Equal(t, []int64{11}, calls[0].AccountIDs)
		assert.Equal(t, []int64{22}, calls[1].AccountIDs)
	})

	t.Run("test notifications ignore the delivery window", func(t *testing.T) {
		bc := testpkg.NewRecordingBroadcaster()
		svc := notifications.NewService(settingsWithWindow("00:00", "00:01"), nil,
			notifications.NewSSEChannel(bc))

		err := dispatch(t, func(ctx context.Context) error {
			return svc.NotifyBatch(ctx, []notifications.Event{tenantEvent(7)})
		})
		require.NoError(t, err)
		assert.NotEmpty(t, bc.Calls())
	})

	t.Run("a mixed-tenant batch is rejected", func(t *testing.T) {
		bc := testpkg.NewRecordingBroadcaster()
		svc := notifications.NewService(settingsWithWindow("", ""), nil,
			notifications.NewSSEChannel(bc))

		err := svc.NotifyBatch(context.Background(), []notifications.Event{
			staffEvent(7, 11),
			staffEvent(8, 22),
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must not mix tenants")
		assert.Empty(t, bc.Calls())
	})

	t.Run("one malformed event fails the whole batch", func(t *testing.T) {
		bc := testpkg.NewRecordingBroadcaster()
		svc := notifications.NewService(settingsWithWindow("", ""), nil,
			notifications.NewSSEChannel(bc))

		// Dropping the bad event silently would lose a recipient without a
		// trace; failing loudly lets the caller retry the whole tick.
		err := svc.NotifyBatch(context.Background(), []notifications.Event{
			staffEvent(7, 11),
			staffEvent(7),
		})
		require.Error(t, err)
		assert.Empty(t, bc.Calls())
	})

	t.Run("an empty batch is a no-op", func(t *testing.T) {
		bc := testpkg.NewRecordingBroadcaster()
		svc := notifications.NewService(settingsWithWindow("", ""), nil,
			notifications.NewSSEChannel(bc))

		require.NoError(t, svc.NotifyBatch(context.Background(), nil))
		assert.Empty(t, bc.Calls())
	})
}
