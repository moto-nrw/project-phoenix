// Personal reminder notification tick tests. The producer is a recording
// fake; its own behaviour is tested in workflows/reminderdelivery.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tickCall is one call of the producer.
type tickCall struct {
	tenantID   int64
	onDutyOnly bool
}

type recordingTick struct {
	calls []tickCall
}

func (r *recordingTick) notify(_ context.Context, tenantID int64, _ time.Time, onDutyOnly bool) {
	r.calls = append(r.calls, tickCall{tenantID: tenantID, onDutyOnly: onDutyOnly})
}

func reminderTickScheduler(settings ports.SettingsResolver) (*Scheduler, *recordingTick) {
	tick := &recordingTick{}
	sched := unitScheduler(&unitSchedulerDependencies{
		tasks:                 make(map[string]*ScheduledTask),
		logger:                slog.Default(),
		settings:              settings,
		reminderNotifications: tick.notify,
	})
	return sched, tick
}

// registryDefaultSettings behaves like the real settings service for a
// school that never touched a setting: no tenant override, and Resolve*
// answers the registered default. The defaults it carries are the
// registry's, pinned there by services/config/defaults; the other doubles in
// this package invent their own "no value" behaviour, which is exactly what
// would let a registry-default regression pass unnoticed.
type registryDefaultSettings struct {
	bools map[string]bool
}

func (registryDefaultSettings) HasTenantOverride(context.Context, string) (bool, error) {
	return false, nil
}

func (r registryDefaultSettings) ResolveBool(_ context.Context, key string) (bool, error) {
	value, ok := r.bools[key]
	if !ok {
		return false, fmt.Errorf("no definition registered for %s", key)
	}
	return value, nil
}

func (registryDefaultSettings) ResolveInt(_ context.Context, key string) (int, error) {
	return 0, fmt.Errorf("no definition registered for %s", key)
}

func (registryDefaultSettings) ResolveString(_ context.Context, key string) (string, error) {
	return "", fmt.Errorf("no definition registered for %s", key)
}

// The tick's feature gate has to answer what the notification router answers.
// While the gate carried its own literal default, every school without an
// explicit override was silently skipped even though the setting reads "on" in
// the UI and the router would have delivered.
func TestReminderNotificationTickHonoursRegistryDefault(t *testing.T) {
	t.Parallel()

	// Premise: the registry default of the dispatch flag is on, pinned by
	// services/config/defaults.TestNotificationsDispatchDefaultsOn.

	t.Run("no tenant override runs the tick", func(t *testing.T) {
		sched, tick := reminderTickScheduler(registryDefaultSettings{bools: map[string]bool{settingNotificationsDispatchEnabled: true}})

		// No db/schoolRepo wired, so forEachTenantSettings runs the body once
		// without tenant context, which is enough to exercise the gate.
		sched.checkAndRunReminderNotifications(context.Background(), &ScheduledTask{Name: "reminder-notifications"})

		require.Len(t, tick.calls, 1,
			"a school that never touched the setting must be served the registry default")
		assert.False(t, tick.calls[0].onDutyOnly, "the duty gate is off unless a school switches it on (#3736)")
	})

	t.Run("an explicit off override still stops the tick", func(t *testing.T) {
		sched, tick := reminderTickScheduler(&stubSettingsResolver{hasOverride: true, boolVal: false})

		sched.checkAndRunReminderNotifications(context.Background(), &ScheduledTask{Name: "reminder-notifications"})

		assert.Empty(t, tick.calls,
			"a school that switched notifications off must not even resolve an audience")
	})

	t.Run("no settings service fails closed", func(t *testing.T) {
		sched, tick := reminderTickScheduler(nil)

		sched.checkAndRunReminderNotifications(context.Background(), &ScheduledTask{Name: "reminder-notifications"})

		assert.Empty(t, tick.calls,
			"without a settings service the router could not dispatch either")
	})
}

// A school that switched notifications.on_duty_only on restricts the
// audience to staff members who are working right now.
func TestReminderNotificationTickPassesTheDutyGate(t *testing.T) {
	t.Parallel()

	sched, tick := reminderTickScheduler(&fakeSettingsResolver{boolValues: map[string]bool{
		settingNotificationsDispatchEnabled: true,
		settingNotificationsOnDutyOnly:      true,
	}})

	sched.checkAndRunReminderNotifications(context.Background(), &ScheduledTask{Name: "reminder-notifications"})

	require.Len(t, tick.calls, 1)
	assert.True(t, tick.calls[0].onDutyOnly)
}

func TestScheduleReminderNotificationTaskRequiresTheProducer(t *testing.T) {
	t.Parallel()

	t.Run("nothing wired", func(t *testing.T) {
		sched := unitScheduler(&unitSchedulerDependencies{tasks: make(map[string]*ScheduledTask), logger: slog.Default()})
		sched.scheduleReminderNotificationTask()
		assert.Empty(t, sched.tasks)
	})

	t.Run("fully wired", func(t *testing.T) {
		sched, _ := reminderTickScheduler(nil)
		sched.scheduleReminderNotificationTask()
		assert.Contains(t, sched.tasks, "reminder-notifications")
	})
}
