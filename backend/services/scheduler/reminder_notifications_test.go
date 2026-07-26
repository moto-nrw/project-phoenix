// Reminder → notification bridge tests (#1624 follow-up). Pure in-memory:
// the reminders computer and the notifications service are faked, and the
// tests drive runReminderNotificationsForTenant directly — the tenant/settings
// plumbing around it is exercised by the shared scheduler loop tests.
package scheduler

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/services/reminders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeReminderComputer struct {
	result *reminders.Result
	err    error
}

func (f *fakeReminderComputer) Compute(context.Context, reminders.Scope) (*reminders.Result, error) {
	return f.result, f.err
}

type captureNotifier struct {
	events []notifications.Event
	err    error
}

func (c *captureNotifier) Notify(_ context.Context, event notifications.Event) error {
	if c.err != nil {
		return c.err
	}
	c.events = append(c.events, event)
	return nil
}

func str(s string) *string { return &s }

func reminderFixture(typ, studentID, due string) reminders.Reminder {
	return reminders.Reminder{
		Type:      typ,
		StudentID: str(studentID),
		Title:     "Kind Name", // must never surface in the notification
		DueTime:   due,
	}
}

func buildReminderSched(computer reminders.Service, notifier notifications.Service) *Scheduler {
	return &Scheduler{
		tasks:            make(map[string]*ScheduledTask),
		logger:           slog.Default(),
		reminderComputer: computer,
		reminderNotifier: notifier,
	}
}

func TestReminderNotificationsAggregateAndDedup(t *testing.T) {
	notifier := &captureNotifier{}
	computer := &fakeReminderComputer{result: &reminders.Result{
		Enabled: true,
		Reminders: []reminders.Reminder{
			reminderFixture(reminders.TypePickupUpcoming, "11", "14:00"),
			reminderFixture(reminders.TypePickupUpcoming, "12", "14:05"),
			reminderFixture(reminders.TypeActivityStart, "13", "14:10"),
		},
	}}
	sched := buildReminderSched(computer, notifier)
	now := time.Now()

	sched.runReminderNotificationsForTenant(context.Background(), 7, now)
	require.Len(t, notifier.events, 1, "fresh reminders aggregate into ONE event")

	event := notifier.events[0]
	assert.Equal(t, "reminders_due", event.Type)
	assert.Equal(t, int64(7), event.Audience.TenantID)
	assert.Equal(t, notifications.ScopeAdmin, event.Audience.Scope)
	assert.Equal(t, notifications.PriorityNormal, event.Priority)
	assert.Equal(t, "Erinnerungen", event.Title)
	assert.Equal(t, "2 anstehende Abholungen, 1 beginnende Aktivität.", event.Body)
	assert.Equal(t, "/reminders", event.DeepLink)
	assert.Equal(t, map[string]string{"pickup_upcoming": "2", "activity_start": "1"}, event.Data)
	assert.NotContains(t, event.Body, "Kind Name", "student names must never leave the reminder list")

	// Second tick with the identical list: everything already notified.
	sched.runReminderNotificationsForTenant(context.Background(), 7, now)
	assert.Len(t, notifier.events, 1, "already-notified reminders must not re-fire")

	// A new occurrence joins: only IT is dispatched.
	computer.result.Reminders = append(computer.result.Reminders,
		reminderFixture(reminders.TypePickupOverdue, "14", "13:30"))
	sched.runReminderNotificationsForTenant(context.Background(), 7, now)
	require.Len(t, notifier.events, 2)
	assert.Equal(t, "1 überfällige Abholung.", notifier.events[1].Body)
	assert.Equal(t, notifications.PriorityHigh, notifier.events[1].Priority, "overdue raises priority")
}

func TestReminderNotificationsSkipConditions(t *testing.T) {
	notifier := &captureNotifier{}

	disabled := buildReminderSched(&fakeReminderComputer{result: &reminders.Result{
		Enabled:   false,
		Reminders: []reminders.Reminder{reminderFixture(reminders.TypePickupUpcoming, "11", "14:00")},
	}}, notifier)
	disabled.runReminderNotificationsForTenant(context.Background(), 7, time.Now())
	assert.Empty(t, notifier.events, "reminder feature off → nothing dispatched")

	empty := buildReminderSched(&fakeReminderComputer{result: &reminders.Result{Enabled: true}}, notifier)
	empty.runReminderNotificationsForTenant(context.Background(), 7, time.Now())
	assert.Empty(t, notifier.events, "no reminders → nothing dispatched")

	failing := buildReminderSched(&fakeReminderComputer{err: assert.AnError}, notifier)
	failing.runReminderNotificationsForTenant(context.Background(), 7, time.Now())
	assert.Empty(t, notifier.events, "compute error → skip, no dispatch")
}

func TestReminderNotificationsNotifyFailureLeavesUnmarked(t *testing.T) {
	notifier := &captureNotifier{err: notifications.ErrDisabled}
	computer := &fakeReminderComputer{result: &reminders.Result{
		Enabled:   true,
		Reminders: []reminders.Reminder{reminderFixture(reminders.TypePickupUpcoming, "11", "14:00")},
	}}
	sched := buildReminderSched(computer, notifier)

	sched.runReminderNotificationsForTenant(context.Background(), 7, time.Now())
	assert.Empty(t, notifier.events)

	// Dispatch becomes possible again → the SAME occurrence fires now.
	notifier.err = nil
	sched.runReminderNotificationsForTenant(context.Background(), 7, time.Now())
	assert.Len(t, notifier.events, 1, "a failed dispatch must not consume the occurrence")
}

func TestReminderNotificationsTenantIsolationAndDayRotation(t *testing.T) {
	notifier := &captureNotifier{}
	computer := &fakeReminderComputer{result: &reminders.Result{
		Enabled:   true,
		Reminders: []reminders.Reminder{reminderFixture(reminders.TypePickupUpcoming, "11", "14:00")},
	}}
	sched := buildReminderSched(computer, notifier)
	now := time.Now()

	// Production rotates at the top of every tick; initialize the day first so
	// the later same-day rotation call is a genuine no-op check.
	sched.rotateReminderNotificationCacheIfNewDay(now)
	sched.runReminderNotificationsForTenant(context.Background(), 7, now)
	sched.runReminderNotificationsForTenant(context.Background(), 8, now)
	assert.Len(t, notifier.events, 2, "the guard is per tenant")

	// Same civil date → rotation is a no-op, guard intact.
	sched.rotateReminderNotificationCacheIfNewDay(now)
	sched.runReminderNotificationsForTenant(context.Background(), 7, now)
	assert.Len(t, notifier.events, 2)

	// Next civil date → guard cleared, identical schedule fires again.
	sched.rotateReminderNotificationCacheIfNewDay(now.Add(24 * time.Hour))
	sched.runReminderNotificationsForTenant(context.Background(), 7, now)
	assert.Len(t, notifier.events, 3)
}

func TestScheduleReminderNotificationTaskRequiresBothDeps(t *testing.T) {
	sched := buildReminderSched(nil, nil)
	sched.scheduleReminderNotificationTask()
	assert.Empty(t, sched.tasks, "missing deps → task must not register")

	sched = buildReminderSched(&fakeReminderComputer{}, nil)
	sched.scheduleReminderNotificationTask()
	assert.Empty(t, sched.tasks)

	sched = buildReminderSched(&fakeReminderComputer{}, &captureNotifier{})
	sched.scheduleReminderNotificationTask()
	assert.Contains(t, sched.tasks, "reminder-notifications")
}
