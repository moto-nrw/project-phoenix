// Personal reminder notification tests. Pure in-memory: the batch computer,
// the consent filter, the three readers and the notifier are all fakes, and the
// tests drive runReminderNotificationsForTenant directly — the tenant/settings
// plumbing around it is exercised by the shared scheduler loop tests.
package scheduler

import (
	"context"
	"log/slog"
	"testing"
	"time"

	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/services/reminders"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The two people every test works with. Staff 1 (account 10) is a caregiver,
// staff 2 (account 20) an admin.
const (
	caregiverStaffID   int64 = 1
	caregiverAccountID int64 = 10
	adminStaffID       int64 = 2
	adminAccountID     int64 = 20
	testTenant         int64 = 7
)

type fakeBatchComputer struct {
	results map[int64]*reminders.Result
	err     error
	calls   int
	scopes  []reminders.BatchScope
}

func (f *fakeBatchComputer) ComputeBatch(_ context.Context, scopes []reminders.BatchScope) (map[int64]*reminders.Result, error) {
	f.calls++
	f.scopes = scopes
	if f.err != nil {
		return nil, f.err
	}
	return f.results, nil
}

// fakeConsent answers "who agreed to X" from a per-type account list.
type fakeConsent struct {
	byType map[string][]int64
	err    error
	// asked records the candidate list the producer offered, so a test can
	// assert that consent narrowed a relation instead of inventing one.
	asked []int64
}

func (f *fakeConsent) FilterOptedIn(_ context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.asked = accountIDs
	allowed := make(map[int64]struct{}, len(f.byType[notificationType]))
	for _, id := range f.byType[notificationType] {
		allowed[id] = struct{}{}
	}
	out := make([]int64, 0, len(allowed))
	for _, id := range accountIDs {
		if _, ok := allowed[id]; ok {
			out = append(out, id)
		}
	}
	return out, nil
}

type fakeStaffAccounts struct {
	accounts map[int64]int64
	err      error
}

func (f *fakeStaffAccounts) ListAllStaffAccountIDs(context.Context) (map[int64]int64, error) {
	return f.accounts, f.err
}

type fakeAdminAccounts struct {
	ids []int64
	err error
}

func (f *fakeAdminAccounts) ListEffectiveAdminAccountIDs(context.Context) ([]int64, error) {
	return f.ids, f.err
}

type fakeDuty struct {
	presence map[int64]string
	err      error
}

func (f *fakeDuty) GetTodayPresenceMap(context.Context) (map[int64]string, error) {
	return f.presence, f.err
}

type captureBatchNotifier struct {
	events []notifications.Event
	err    error
}

func (c *captureBatchNotifier) NotifyBatch(ctx context.Context, events []notifications.Event) error {
	if c.err != nil {
		return c.err
	}
	tenant.RegisterAfterCommit(ctx, func() {
		c.events = append(c.events, events...)
	})
	return nil
}

func str(s string) *string { return &s }

func pickupFixture(studentID, due string) reminders.Reminder {
	return reminders.Reminder{
		Type:      reminders.TypePickupUpcoming,
		StudentID: str(studentID),
		Title:     "Kind Name", // must never surface in a notification
		DueTime:   due,
	}
}

func activityFixture(instanceID, due string) reminders.Reminder {
	return reminders.Reminder{
		Type:               reminders.TypeActivityStart,
		ActivityInstanceID: str(instanceID),
		Title:              "Bastel-AG",
		DueTime:            due,
	}
}

func resultOf(rems ...reminders.Reminder) *reminders.Result {
	return &reminders.Result{Enabled: true, Reminders: rems, Count: len(rems)}
}

// reminderTestSetup is the wiring of one test, with every part reachable for
// assertions afterwards.
type reminderTestSetup struct {
	sched    *Scheduler
	computer *fakeBatchComputer
	consent  *fakeConsent
	staff    *fakeStaffAccounts
	admins   *fakeAdminAccounts
	duty     *fakeDuty
	notifier *captureBatchNotifier
}

// buildReminderSched wires a scheduler with the given batch results and consent
// map. The duty gate is off by default (empty presence map = the school does
// not clock in), so tests that do not care about it are unaffected.
func buildReminderSched(results map[int64]*reminders.Result, consent map[string][]int64) *reminderTestSetup {
	setup := &reminderTestSetup{
		computer: &fakeBatchComputer{results: results},
		consent:  &fakeConsent{byType: consent},
		staff: &fakeStaffAccounts{accounts: map[int64]int64{
			caregiverStaffID: caregiverAccountID,
			adminStaffID:     adminAccountID,
		}},
		admins:   &fakeAdminAccounts{ids: []int64{adminAccountID}},
		duty:     &fakeDuty{},
		notifier: &captureBatchNotifier{},
	}
	setup.sched = &Scheduler{
		tasks:  make(map[string]*ScheduledTask),
		logger: slog.Default(),
		reminderNotifications: ReminderNotificationDeps{
			Computer:     setup.computer,
			Notifier:     setup.notifier,
			Preferences:  setup.consent,
			Staff:        setup.staff,
			Accounts:     setup.admins,
			WorkSessions: setup.duty,
		},
	}
	return setup
}

func TestPersonalRemindersReachOnlyTheConsenting(t *testing.T) {
	setup := buildReminderSched(
		map[int64]*reminders.Result{
			caregiverStaffID: resultOf(pickupFixture("11", "14:00")),
			adminStaffID:     resultOf(pickupFixture("11", "14:00")),
		},
		// Only the caregiver switched pickups on.
		map[string][]int64{notifications.TypePickupUpcoming: {caregiverAccountID}},
	)

	setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())

	require.Len(t, setup.notifier.events, 1, "the admin never agreed, so nothing is addressed to them")
	event := setup.notifier.events[0]
	assert.Equal(t, notifications.TypePickupUpcoming, event.Type)
	assert.Equal(t, notifications.ScopeStaff, event.Audience.Scope)
	assert.Equal(t, []int64{caregiverAccountID}, event.Audience.StaffAccountIDs)
	assert.Equal(t, testTenant, event.Audience.TenantID)
	assert.Equal(t, "Abholung steht an", event.Title)
	assert.Equal(t, "Ein Kind aus Ihrer Aufsicht wird bald abgeholt.", event.Body)
	assert.Equal(t, "/reminders", event.DeepLink)
	assert.NotContains(t, event.Body, "Kind Name", "student names must never leave the reminder list")

	// The consent filter was offered the whole team, not just the one who said
	// yes: consent narrows a relation, it is never the source of the audience.
	assert.ElementsMatch(t, []int64{caregiverAccountID, adminAccountID}, setup.consent.asked)

	// The computation was scoped per person, with the admin marked as one.
	require.Len(t, setup.computer.scopes, 1, "only people with consent are computed")
	assert.Equal(t, caregiverStaffID, setup.computer.scopes[0].StaffID)
	assert.False(t, setup.computer.scopes[0].IsAdmin)
}

func TestPersonalRemindersMarkAdminScope(t *testing.T) {
	setup := buildReminderSched(
		map[int64]*reminders.Result{adminStaffID: resultOf(pickupFixture("11", "14:00"))},
		map[string][]int64{notifications.TypePickupUpcoming: {adminAccountID}},
	)

	setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())

	require.Len(t, setup.computer.scopes, 1)
	assert.Equal(t, adminStaffID, setup.computer.scopes[0].StaffID)
	assert.True(t, setup.computer.scopes[0].IsAdmin,
		"effective admin scope decides what the batch may see")
}

func TestPersonalRemindersSkipComputationWithoutConsent(t *testing.T) {
	setup := buildReminderSched(
		map[int64]*reminders.Result{caregiverStaffID: resultOf(pickupFixture("11", "14:00"))},
		nil, // nobody agreed to anything
	)

	setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())

	assert.Empty(t, setup.notifier.events)
	assert.Zero(t, setup.computer.calls,
		"with nobody to address, the tick must not compute reminders at all")
}

func TestPersonalRemindersAggregatePerTypeAndDedupPerPerson(t *testing.T) {
	setup := buildReminderSched(
		map[int64]*reminders.Result{
			caregiverStaffID: resultOf(
				pickupFixture("11", "14:00"),
				pickupFixture("12", "14:05"),
				activityFixture("31", "14:10"),
			),
			adminStaffID: resultOf(pickupFixture("11", "14:00")),
		},
		map[string][]int64{
			notifications.TypePickupUpcoming: {caregiverAccountID, adminAccountID},
			notifications.TypeActivityStart:  {caregiverAccountID},
		},
	)
	now := time.Now()

	setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, now)

	require.Len(t, setup.notifier.events, 3, "two kinds for the caregiver, one for the admin")
	assert.Equal(t, "2 Kinder aus Ihrer Aufsicht werden bald abgeholt.", setup.notifier.events[0].Body)
	assert.Equal(t, notifications.TypeActivityStart, setup.notifier.events[1].Type)
	assert.Equal(t, []int64{adminAccountID}, setup.notifier.events[2].Audience.StaffAccountIDs)

	// The same pickup reached both people — and each of them only once.
	setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, now)
	assert.Len(t, setup.notifier.events, 3, "already-notified occurrences must not re-fire")

	// A newly due overdue pickup for the caregiver alone.
	setup.computer.results[caregiverStaffID] = resultOf(
		pickupFixture("11", "14:00"),
		reminders.Reminder{Type: reminders.TypePickupOverdue, StudentID: str("14"), DueTime: "13:30"},
	)
	setup.consent.byType[notifications.TypePickupOverdue] = []int64{caregiverAccountID}

	setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, now)
	require.Len(t, setup.notifier.events, 4)
	assert.Equal(t, notifications.PriorityHigh, setup.notifier.events[3].Priority,
		"overdue raises the priority")
}

// An activity someone is personally planned on is their own slot, not "an
// activity in the room I am watching". The two switches must not overlap.
func TestPersonalRemindersClassifyAssignedActivities(t *testing.T) {
	assigned := resultOf(activityFixture("31", "14:10"))
	assigned.AssignedActivityInstanceIDs = map[string]struct{}{"31": {}}

	t.Run("assigned slot needs the assignment consent", func(t *testing.T) {
		setup := buildReminderSched(
			map[int64]*reminders.Result{caregiverStaffID: assigned},
			map[string][]int64{notifications.TypeMyActivityStarting: {caregiverAccountID}},
		)

		setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())

		require.Len(t, setup.notifier.events, 1)
		assert.Equal(t, notifications.TypeMyActivityStarting, setup.notifier.events[0].Type)
		assert.Equal(t, "Ihr Einsatz beginnt", setup.notifier.events[0].Title)
	})

	t.Run("the room consent does not cover an own slot", func(t *testing.T) {
		setup := buildReminderSched(
			map[int64]*reminders.Result{caregiverStaffID: assigned},
			map[string][]int64{notifications.TypeActivityStart: {caregiverAccountID}},
		)

		setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())

		assert.Empty(t, setup.notifier.events,
			"somebody who switched their own slots off must not receive them under another name")
	})

	t.Run("an unassigned activity stays a room activity", func(t *testing.T) {
		setup := buildReminderSched(
			map[int64]*reminders.Result{caregiverStaffID: resultOf(activityFixture("31", "14:10"))},
			map[string][]int64{notifications.TypeActivityStart: {caregiverAccountID}},
		)

		setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())

		require.Len(t, setup.notifier.events, 1)
		assert.Equal(t, notifications.TypeActivityStart, setup.notifier.events[0].Type)
	})
}

func TestPersonalRemindersOnDutyGate(t *testing.T) {
	results := map[int64]*reminders.Result{
		caregiverStaffID: resultOf(pickupFixture("11", "14:00")),
		adminStaffID:     resultOf(pickupFixture("11", "14:00")),
	}
	consent := map[string][]int64{
		notifications.TypePickupUpcoming: {caregiverAccountID, adminAccountID},
	}

	t.Run("only stamped-in people are addressed", func(t *testing.T) {
		setup := buildReminderSched(results, consent)
		setup.duty.presence = map[int64]string{
			caregiverStaffID: activeModel.WorkSessionStatusPresent,
			adminStaffID:     "checked_out",
		}

		setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())

		require.Len(t, setup.notifier.events, 1)
		assert.Equal(t, []int64{caregiverAccountID}, setup.notifier.events[0].Audience.StaffAccountIDs)
	})

	t.Run("home office counts as on duty", func(t *testing.T) {
		setup := buildReminderSched(results, consent)
		setup.duty.presence = map[int64]string{
			caregiverStaffID: activeModel.WorkSessionStatusHomeOffice,
		}

		setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())

		require.Len(t, setup.notifier.events, 1)
		assert.Equal(t, []int64{caregiverAccountID}, setup.notifier.events[0].Audience.StaffAccountIDs)
	})

	t.Run("a school without time tracking is not restricted", func(t *testing.T) {
		setup := buildReminderSched(results, consent)
		setup.duty.presence = nil

		setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())

		assert.Len(t, setup.notifier.events, 2,
			"no work sessions at all means the school does not clock in, not that nobody works")
	})

	t.Run("everybody stamped out stays quiet", func(t *testing.T) {
		setup := buildReminderSched(results, consent)
		setup.duty.presence = map[int64]string{
			caregiverStaffID: "checked_out",
			adminStaffID:     "checked_out",
		}

		setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())

		assert.Empty(t, setup.notifier.events)
		assert.Zero(t, setup.computer.calls)
	})
}

func TestPersonalRemindersSkipConditions(t *testing.T) {
	consent := map[string][]int64{notifications.TypePickupUpcoming: {caregiverAccountID}}
	reminder := pickupFixture("11", "14:00")

	t.Run("reminder feature off", func(t *testing.T) {
		setup := buildReminderSched(map[int64]*reminders.Result{
			caregiverStaffID: {Enabled: false, Reminders: []reminders.Reminder{reminder}},
		}, consent)
		setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())
		assert.Empty(t, setup.notifier.events)
	})

	t.Run("nothing due", func(t *testing.T) {
		setup := buildReminderSched(map[int64]*reminders.Result{
			caregiverStaffID: {Enabled: true},
		}, consent)
		setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())
		assert.Empty(t, setup.notifier.events)
	})

	t.Run("compute error", func(t *testing.T) {
		setup := buildReminderSched(nil, consent)
		setup.computer.err = assert.AnError
		setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())
		assert.Empty(t, setup.notifier.events)
	})

	t.Run("no staff at all", func(t *testing.T) {
		setup := buildReminderSched(nil, consent)
		setup.staff.accounts = nil
		setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())
		assert.Empty(t, setup.notifier.events)
		assert.Zero(t, setup.computer.calls)
	})

	t.Run("a failing reader aborts the tick", func(t *testing.T) {
		setup := buildReminderSched(nil, consent)
		setup.admins.err = assert.AnError
		setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())
		assert.Empty(t, setup.notifier.events)
		assert.Zero(t, setup.computer.calls,
			"an unknown admin set would silently change what people are allowed to see")
	})
}

func TestPersonalRemindersNotifyFailureLeavesUnmarked(t *testing.T) {
	setup := buildReminderSched(
		map[int64]*reminders.Result{caregiverStaffID: resultOf(pickupFixture("11", "14:00"))},
		map[string][]int64{notifications.TypePickupUpcoming: {caregiverAccountID}},
	)
	setup.notifier.err = notifications.ErrDisabled

	setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())
	assert.Empty(t, setup.notifier.events)

	// Dispatch becomes possible again → the SAME occurrence fires now.
	setup.notifier.err = nil
	setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())
	assert.Len(t, setup.notifier.events, 1, "a failed dispatch must not consume the occurrence")
}

func TestPersonalRemindersMarkOccurrencesAfterCommit(t *testing.T) {
	reminder := pickupFixture("11", "14:00")
	setup := buildReminderSched(
		map[int64]*reminders.Result{caregiverStaffID: resultOf(reminder)},
		map[string][]int64{notifications.TypePickupUpcoming: {caregiverAccountID}},
	)
	key := reminderNotificationKey{
		tenantID:  testTenant,
		accountID: caregiverAccountID,
		reminder:  reminderIdentity(reminder),
	}
	ctx, commit := tenant.WithAfterCommitHooksForTest(context.Background())

	setup.sched.runReminderNotificationsForTenant(ctx, testTenant, time.Now())

	assert.Empty(t, setup.notifier.events, "delivery must wait for commit")
	_, markedBeforeCommit := setup.sched.reminderNotified.Load(key)
	assert.False(t, markedBeforeCommit, "dedup key must wait for commit")

	commit()

	assert.Len(t, setup.notifier.events, 1)
	_, markedAfterCommit := setup.sched.reminderNotified.Load(key)
	assert.True(t, markedAfterCommit, "successful commit must consume the occurrence")

	setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())
	assert.Len(t, setup.notifier.events, 1, "committed occurrence must not re-fire")
}

func TestPersonalRemindersRollbackLeavesOccurrencesUnmarked(t *testing.T) {
	reminder := pickupFixture("11", "14:00")
	setup := buildReminderSched(
		map[int64]*reminders.Result{caregiverStaffID: resultOf(reminder)},
		map[string][]int64{notifications.TypePickupUpcoming: {caregiverAccountID}},
	)
	key := reminderNotificationKey{
		tenantID:  testTenant,
		accountID: caregiverAccountID,
		reminder:  reminderIdentity(reminder),
	}
	rolledBackCtx, _ := tenant.WithAfterCommitHooksForTest(context.Background())

	setup.sched.runReminderNotificationsForTenant(rolledBackCtx, testTenant, time.Now())

	assert.Empty(t, setup.notifier.events, "rollback must drop queued delivery")
	_, marked := setup.sched.reminderNotified.Load(key)
	assert.False(t, marked, "rollback must leave the occurrence eligible")

	setup.sched.runReminderNotificationsForTenant(context.Background(), testTenant, time.Now())
	assert.Len(t, setup.notifier.events, 1, "the next successful tick must retry the occurrence")
}

func TestPersonalRemindersTenantIsolationAndDayRotation(t *testing.T) {
	setup := buildReminderSched(
		map[int64]*reminders.Result{caregiverStaffID: resultOf(pickupFixture("11", "14:00"))},
		map[string][]int64{notifications.TypePickupUpcoming: {caregiverAccountID}},
	)
	now := time.Now()

	// Production rotates at the top of every tick; initialize the day first so
	// the later same-day rotation call is a genuine no-op check.
	setup.sched.rotateReminderNotificationCacheIfNewDay(now)
	setup.sched.runReminderNotificationsForTenant(context.Background(), 7, now)
	setup.sched.runReminderNotificationsForTenant(context.Background(), 8, now)
	assert.Len(t, setup.notifier.events, 2, "the guard is per tenant")

	// Same civil date → rotation is a no-op, guard intact.
	setup.sched.rotateReminderNotificationCacheIfNewDay(now)
	setup.sched.runReminderNotificationsForTenant(context.Background(), 7, now)
	assert.Len(t, setup.notifier.events, 2)

	// Next civil date → guard cleared, identical schedule fires again.
	setup.sched.rotateReminderNotificationCacheIfNewDay(now.Add(24 * time.Hour))
	setup.sched.runReminderNotificationsForTenant(context.Background(), 7, now)
	assert.Len(t, setup.notifier.events, 3)
}

func TestScheduleReminderNotificationTaskRequiresEveryDep(t *testing.T) {
	complete := buildReminderSched(nil, nil).sched.reminderNotifications

	t.Run("nothing wired", func(t *testing.T) {
		sched := &Scheduler{tasks: make(map[string]*ScheduledTask), logger: slog.Default()}
		sched.scheduleReminderNotificationTask()
		assert.Empty(t, sched.tasks)
	})

	// Every single missing dependency must disable the task: a producer that
	// resolves a narrower audience because one reader is absent would look like
	// it works.
	partials := map[string]ReminderNotificationDeps{
		"no computer":     {Notifier: complete.Notifier, Preferences: complete.Preferences, Staff: complete.Staff, Accounts: complete.Accounts, WorkSessions: complete.WorkSessions},
		"no notifier":     {Computer: complete.Computer, Preferences: complete.Preferences, Staff: complete.Staff, Accounts: complete.Accounts, WorkSessions: complete.WorkSessions},
		"no preferences":  {Computer: complete.Computer, Notifier: complete.Notifier, Staff: complete.Staff, Accounts: complete.Accounts, WorkSessions: complete.WorkSessions},
		"no staff reader": {Computer: complete.Computer, Notifier: complete.Notifier, Preferences: complete.Preferences, Accounts: complete.Accounts, WorkSessions: complete.WorkSessions},
		"no admin reader": {Computer: complete.Computer, Notifier: complete.Notifier, Preferences: complete.Preferences, Staff: complete.Staff, WorkSessions: complete.WorkSessions},
		"no duty reader":  {Computer: complete.Computer, Notifier: complete.Notifier, Preferences: complete.Preferences, Staff: complete.Staff, Accounts: complete.Accounts},
	}
	for name, deps := range partials {
		t.Run(name, func(t *testing.T) {
			sched := &Scheduler{tasks: make(map[string]*ScheduledTask), logger: slog.Default()}
			sched.SetReminderNotificationDeps(deps)
			sched.scheduleReminderNotificationTask()
			assert.Empty(t, sched.tasks)
		})
	}

	t.Run("fully wired", func(t *testing.T) {
		sched := &Scheduler{tasks: make(map[string]*ScheduledTask), logger: slog.Default()}
		sched.SetReminderNotificationDeps(complete)
		sched.scheduleReminderNotificationTask()
		assert.Contains(t, sched.tasks, "reminder-notifications")
	})
}

// The copy table and the type list are two halves of one contract: a type
// without wording would dispatch the generic fallback, which reads like a bug.
func TestEveryPersonalReminderTypeHasCopy(t *testing.T) {
	for _, notificationType := range personalReminderTypes {
		copyFor, ok := reminderCopyByType[notificationType]
		require.True(t, ok, "missing copy for %s", notificationType)
		assert.NotEmpty(t, copyFor.title)
		assert.NotEmpty(t, copyFor.singular)
		assert.Contains(t, copyFor.plural, "%d")

		// And the type has to exist in the catalogue the profile page renders,
		// or people could never agree to it.
		_, known := notifications.GetType(notificationType)
		assert.True(t, known, "%s is not in the notification catalogue", notificationType)
	}
}
