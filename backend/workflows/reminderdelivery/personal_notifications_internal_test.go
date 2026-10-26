// Personal reminder notification tests. Pure in-memory: the batch computer,
// the consent filter, the readers, the notifier and the transaction are all
// fakes, and the tests drive NotifyDue directly — the Worker's tick around it
// is tested in services/scheduler.
package reminderdelivery

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	stafflessAdminID   int64 = 30
	testTenant         int64 = 7
)

// errDispatchPaused stands in for the notification service's ordinary
// refusals (feature flag off, outside the delivery window).
var errDispatchPaused = errors.New("notifications are disabled for this tenant")

type fakeBatchComputer struct {
	results map[int64]*Result
	err     error
	calls   int
	scopes  []BatchScope
}

func (f *fakeBatchComputer) ComputeBatch(_ context.Context, scopes []BatchScope) (map[int64]*Result, error) {
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
	asked       []int64
	askedTypes  []string
	hasCalls    int
	filterCalls int
}

func (f *fakeConsent) HasAnyOptedIn(_ context.Context, notificationTypes []string) (bool, error) {
	f.hasCalls++
	if f.err != nil {
		return false, f.err
	}
	for _, notificationType := range notificationTypes {
		if len(f.byType[notificationType]) > 0 {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeConsent) FilterOptedInByType(
	_ context.Context,
	notificationTypes []string,
	accountIDs []int64,
) (map[string][]int64, error) {
	f.filterCalls++
	if f.err != nil {
		return nil, f.err
	}
	f.asked = accountIDs
	f.askedTypes = notificationTypes
	result := make(map[string][]int64)
	for _, notificationType := range notificationTypes {
		allowed := make(map[int64]struct{}, len(f.byType[notificationType]))
		for _, id := range f.byType[notificationType] {
			allowed[id] = struct{}{}
		}
		for _, id := range accountIDs {
			if _, ok := allowed[id]; ok {
				result[notificationType] = append(result[notificationType], id)
			}
		}
	}
	return result, nil
}

type fakeStaffAccounts struct {
	accounts map[int64]int64
	err      error
	calls    int
}

func (f *fakeStaffAccounts) ListAllStaffAccountIDs(context.Context) (map[int64]int64, error) {
	f.calls++
	return f.accounts, f.err
}

type fakeAdminAccounts struct {
	ids   []int64
	err   error
	calls int
}

func (f *fakeAdminAccounts) ListEffectiveAdminAccountIDs(context.Context) ([]int64, error) {
	f.calls++
	return f.ids, f.err
}

// fakeDuty answers the staff members who are working right now.
type fakeDuty struct {
	onDuty map[int64]struct{}
	err    error
	calls  int
}

func (f *fakeDuty) OnDutyStaff(context.Context) (map[int64]struct{}, error) {
	f.calls++
	return f.onDuty, f.err
}

// withFakeTx opens a tenant transaction: its after-commit hooks run on
// commit, and outside a transaction a hook runs at once.
func withFakeTx(ctx context.Context) (context.Context, func()) {
	hooks := &[]func(){}
	commit := func() {
		queued := *hooks
		*hooks = nil
		for _, hook := range queued {
			hook()
		}
	}
	return context.WithValue(ctx, fakeTxKey{}, hooks), commit
}

type fakeTxKey struct{}

func fakeAfterCommit(ctx context.Context, fn func()) {
	if hooks, ok := ctx.Value(fakeTxKey{}).(*[]func()); ok {
		*hooks = append(*hooks, fn)
		return
	}
	fn()
}

type captureNotifier struct {
	notices []PersonalReminderNotice
	err     error
}

func (c *captureNotifier) Notify(ctx context.Context, notices []PersonalReminderNotice) error {
	if c.err != nil {
		return c.err
	}
	fakeAfterCommit(ctx, func() {
		c.notices = append(c.notices, notices...)
	})
	return nil
}

func str(s string) *string { return &s }

func pickupFixture(studentID, due string) Reminder {
	return Reminder{
		Type:      TypePickupUpcoming,
		StudentID: str(studentID),
		Title:     "Kind Name", // must never surface in a notification
		DueTime:   due,
	}
}

func activityFixture(instanceID, due string) Reminder {
	return Reminder{
		Type:               TypeActivityStart,
		ActivityInstanceID: str(instanceID),
		Title:              "Bastel-AG",
		DueTime:            due,
	}
}

func resultOf(rems ...Reminder) *Result {
	return &Result{Enabled: true, Reminders: rems, Count: len(rems)}
}

// reminderTestSetup is the wiring of one test, with every part reachable for
// assertions afterwards.
type reminderTestSetup struct {
	producer *PersonalReminderNotifications
	computer *fakeBatchComputer
	consent  *fakeConsent
	staff    *fakeStaffAccounts
	admins   *fakeAdminAccounts
	duty     *fakeDuty
	notifier *captureNotifier
}

func (s *reminderTestSetup) dependencies() PersonalReminderDependencies {
	return PersonalReminderDependencies{
		Computer:    s.computer,
		Consent:     s.consent,
		Staff:       s.staff,
		Admins:      s.admins,
		OnDutyStaff: s.duty.OnDutyStaff,
		Notify:      s.notifier.Notify,
		Paused:      func(err error) bool { return errors.Is(err, errDispatchPaused) },
		AfterCommit: fakeAfterCommit,
		Logger:      slog.Default(),
	}
}

// buildReminderProducer wires a producer with the given batch results and
// consent map. Both staff are on duty, so tests that do not care about the
// duty gate are unaffected.
func buildReminderProducer(t *testing.T, results map[int64]*Result, consent map[string][]int64) *reminderTestSetup {
	t.Helper()
	setup := &reminderTestSetup{
		computer: &fakeBatchComputer{results: results},
		consent:  &fakeConsent{byType: consent},
		staff: &fakeStaffAccounts{accounts: map[int64]int64{
			caregiverStaffID: caregiverAccountID,
			adminStaffID:     adminAccountID,
		}},
		admins:   &fakeAdminAccounts{ids: []int64{adminAccountID}},
		duty:     &fakeDuty{onDuty: map[int64]struct{}{caregiverStaffID: {}, adminStaffID: {}}},
		notifier: &captureNotifier{},
	}
	producer, err := NewPersonalReminderNotifications(setup.dependencies())
	require.NoError(t, err)
	setup.producer = producer
	return setup
}

// notify runs one tick of the producer for the test tenant without the
// on-duty restriction (#3736: off unless a school switches it on).
func (s *reminderTestSetup) notify(ctx context.Context, now time.Time) {
	s.producer.NotifyDue(ctx, testTenant, now, false)
}

func TestPersonalRemindersReachOnlyTheConsenting(t *testing.T) {
	t.Parallel()

	setup := buildReminderProducer(t,
		map[int64]*Result{
			caregiverStaffID: resultOf(pickupFixture("11", "14:00")),
			adminStaffID:     resultOf(pickupFixture("11", "14:00")),
		},
		// Only the caregiver switched pickups on.
		map[string][]int64{TypePickupUpcoming: {caregiverAccountID}},
	)

	setup.notify(context.Background(), time.Now())

	require.Len(t, setup.notifier.notices, 1, "the admin never agreed, so nothing is addressed to them")
	notice := setup.notifier.notices[0]
	assert.Equal(t, TypePickupUpcoming, notice.Type)
	assert.Equal(t, NoticeScopeStaff, notice.Scope)
	assert.Equal(t, []int64{caregiverAccountID}, notice.StaffAccountIDs)
	assert.Equal(t, testTenant, notice.TenantID)
	assert.Equal(t, "Abholung steht an", notice.Title)
	assert.Equal(t, "Ein Kind aus Ihrer Aufsicht wird bald abgeholt.", notice.Body)
	assert.Equal(t, "/reminders", notice.DeepLink)
	assert.NotContains(t, notice.Body, "Kind Name", "student names must never leave the reminder list")

	// The consent filter was offered the whole team, not just the one who said
	// yes: consent narrows a relation, it is never the source of the audience.
	assert.ElementsMatch(t, []int64{caregiverAccountID, adminAccountID}, setup.consent.asked)
	assert.Equal(t, 1, setup.consent.filterCalls, "all reminder types must share one consent filter call")

	// The computation was scoped per person, with the admin marked as one.
	require.Len(t, setup.computer.scopes, 1, "only people with consent are computed")
	assert.Equal(t, caregiverStaffID, setup.computer.scopes[0].StaffID)
	assert.False(t, setup.computer.scopes[0].IsAdmin)
	assert.False(t, setup.computer.scopes[0].IncludeAssignedActivityStart)
}

func TestPersonalRemindersMarkAdminScope(t *testing.T) {
	t.Parallel()

	setup := buildReminderProducer(t,
		map[int64]*Result{adminStaffID: resultOf(pickupFixture("11", "14:00"))},
		map[string][]int64{TypePickupUpcoming: {adminAccountID}},
	)

	setup.notify(context.Background(), time.Now())

	require.Len(t, setup.computer.scopes, 1)
	assert.Equal(t, adminStaffID, setup.computer.scopes[0].StaffID)
	assert.True(t, setup.computer.scopes[0].IsAdmin,
		"effective admin scope decides what the batch may see")
}

func TestPersonalRemindersIncludeStafflessEffectiveAdminsWhenDutyGateDisabled(t *testing.T) {
	t.Parallel()

	setup := buildReminderProducer(t,
		map[int64]*Result{-stafflessAdminID: resultOf(pickupFixture("11", "14:00"))},
		map[string][]int64{TypePickupUpcoming: {stafflessAdminID}},
	)
	setup.staff.accounts = map[int64]int64{caregiverStaffID: caregiverAccountID}
	setup.admins.ids = []int64{stafflessAdminID}
	setup.duty.onDuty = map[int64]struct{}{}

	setup.producer.NotifyDue(context.Background(), testTenant, time.Now(), false)

	require.Len(t, setup.computer.scopes, 1)
	assert.Zero(t, setup.computer.scopes[0].StaffID)
	assert.True(t, setup.computer.scopes[0].IsAdmin)
	assert.Equal(t, -stafflessAdminID, setup.computer.scopes[0].ResultKey)
	require.Len(t, setup.notifier.notices, 1)
	assert.Equal(t, []int64{stafflessAdminID}, setup.notifier.notices[0].StaffAccountIDs)
	assert.Zero(t, setup.duty.calls, "without the duty gate nobody's presence is read")
}

func TestPersonalRemindersExcludeStafflessEffectiveAdminsWhenDutyGateEnabled(t *testing.T) {
	t.Parallel()

	setup := buildReminderProducer(t,
		map[int64]*Result{-stafflessAdminID: resultOf(pickupFixture("11", "14:00"))},
		map[string][]int64{TypePickupUpcoming: {stafflessAdminID}},
	)
	setup.staff.accounts = map[int64]int64{caregiverStaffID: caregiverAccountID}
	setup.admins.ids = []int64{stafflessAdminID}

	setup.producer.NotifyDue(context.Background(), testTenant, time.Now(), true)

	assert.Zero(t, setup.computer.calls,
		"an account without a staff identity cannot satisfy the duty gate")
	assert.Empty(t, setup.notifier.notices)
}

func TestPersonalRemindersSkipComputationWithoutConsent(t *testing.T) {
	t.Parallel()

	setup := buildReminderProducer(t,
		map[int64]*Result{caregiverStaffID: resultOf(pickupFixture("11", "14:00"))},
		nil, // nobody agreed to anything
	)

	setup.notify(context.Background(), time.Now())

	assert.Empty(t, setup.notifier.notices)
	assert.Zero(t, setup.computer.calls,
		"with nobody to address, the tick must not compute reminders at all")
	assert.Zero(t, setup.staff.calls, "the tenant-wide consent gate must run before staff resolution")
	assert.Zero(t, setup.admins.calls, "the tenant-wide consent gate must run before admin resolution")
	assert.Equal(t, 1, setup.consent.hasCalls)
	assert.Zero(t, setup.consent.filterCalls)
}

func TestPersonalRemindersTreatStoredConsentAsGateNotAudience(t *testing.T) {
	t.Parallel()

	const staleAccountID int64 = 999
	setup := buildReminderProducer(t,
		map[int64]*Result{caregiverStaffID: resultOf(pickupFixture("11", "14:00"))},
		map[string][]int64{TypePickupUpcoming: {staleAccountID}},
	)

	setup.notify(context.Background(), time.Now())

	assert.Equal(t, 1, setup.staff.calls, "a positive gate justifies canonical audience resolution")
	assert.Equal(t, 1, setup.admins.calls)
	assert.Equal(t, 1, setup.consent.filterCalls)
	assert.Zero(t, setup.computer.calls,
		"a stale preference row must not turn its account into a recipient")
	assert.Empty(t, setup.notifier.notices)
}

func TestPersonalRemindersAggregatePerTypeAndDedupPerPerson(t *testing.T) {
	t.Parallel()

	setup := buildReminderProducer(t,
		map[int64]*Result{
			caregiverStaffID: resultOf(
				pickupFixture("11", "14:00"),
				pickupFixture("12", "14:05"),
				activityFixture("31", "14:10"),
			),
			adminStaffID: resultOf(pickupFixture("11", "14:00")),
		},
		map[string][]int64{
			TypePickupUpcoming: {caregiverAccountID, adminAccountID},
			TypeActivityStart:  {caregiverAccountID},
		},
	)
	now := time.Now()

	setup.notify(context.Background(), now)

	require.Len(t, setup.notifier.notices, 3, "two kinds for the caregiver, one for the admin")
	assert.Equal(t, "2 Kinder aus Ihrer Aufsicht werden bald abgeholt.", setup.notifier.notices[0].Body)
	assert.Equal(t, TypeActivityStart, setup.notifier.notices[1].Type)
	assert.Equal(t, []int64{adminAccountID}, setup.notifier.notices[2].StaffAccountIDs)

	// The same pickup reached both people — and each of them only once.
	setup.notify(context.Background(), now)
	assert.Len(t, setup.notifier.notices, 3, "already-notified occurrences must not re-fire")

	// A newly due overdue pickup for the caregiver alone.
	setup.computer.results[caregiverStaffID] = resultOf(
		pickupFixture("11", "14:00"),
		Reminder{Type: TypePickupOverdue, StudentID: str("14"), DueTime: "13:30"},
	)
	setup.consent.byType[TypePickupOverdue] = []int64{caregiverAccountID}

	setup.notify(context.Background(), now)
	require.Len(t, setup.notifier.notices, 4)
	assert.Equal(t, NoticePriorityHigh, setup.notifier.notices[3].Priority,
		"overdue raises the priority")
}

// An activity someone is personally planned on is their own slot, not "an
// activity in the room I am watching". The two switches must not overlap.
func TestPersonalRemindersClassifyAssignedActivities(t *testing.T) {
	t.Parallel()

	assigned := resultOf(activityFixture("31", "14:10"))
	assigned.AssignedActivityInstanceIDs = map[string]struct{}{"31": {}}

	t.Run("assigned slot needs the assignment consent", func(t *testing.T) {
		setup := buildReminderProducer(t,
			map[int64]*Result{caregiverStaffID: assigned},
			map[string][]int64{TypeMyActivityStarting: {caregiverAccountID}},
		)

		setup.notify(context.Background(), time.Now())

		require.Len(t, setup.notifier.notices, 1)
		require.Len(t, setup.computer.scopes, 1)
		assert.True(t, setup.computer.scopes[0].IncludeAssignedActivityStart)
		assert.Equal(t, TypeMyActivityStarting, setup.notifier.notices[0].Type)
		assert.Equal(t, "Ihr Einsatz beginnt", setup.notifier.notices[0].Title)
	})

	t.Run("the room consent does not cover an own slot", func(t *testing.T) {
		setup := buildReminderProducer(t,
			map[int64]*Result{caregiverStaffID: assigned},
			map[string][]int64{TypeActivityStart: {caregiverAccountID}},
		)

		setup.notify(context.Background(), time.Now())

		assert.Empty(t, setup.notifier.notices,
			"somebody who switched their own slots off must not receive them under another name")
		require.Len(t, setup.computer.scopes, 1)
		assert.False(t, setup.computer.scopes[0].IncludeAssignedActivityStart)
	})

	t.Run("an unassigned activity stays a room activity", func(t *testing.T) {
		setup := buildReminderProducer(t,
			map[int64]*Result{caregiverStaffID: resultOf(activityFixture("31", "14:10"))},
			map[string][]int64{TypeActivityStart: {caregiverAccountID}},
		)

		setup.notify(context.Background(), time.Now())

		require.Len(t, setup.notifier.notices, 1)
		assert.Equal(t, TypeActivityStart, setup.notifier.notices[0].Type)
	})
}

// Which statuses count as on duty is Workforce's answer
// (workforce.OnDutyStaffIDs); the producer only applies it.
func TestPersonalRemindersOnDutyGate(t *testing.T) {
	t.Parallel()

	results := map[int64]*Result{
		caregiverStaffID: resultOf(pickupFixture("11", "14:00")),
		adminStaffID:     resultOf(pickupFixture("11", "14:00")),
	}
	consent := map[string][]int64{
		TypePickupUpcoming: {caregiverAccountID, adminAccountID},
	}

	t.Run("only stamped-in people are addressed", func(t *testing.T) {
		setup := buildReminderProducer(t, results, consent)
		setup.duty.onDuty = map[int64]struct{}{caregiverStaffID: {}}

		setup.producer.NotifyDue(context.Background(), testTenant, time.Now(), true)

		require.Len(t, setup.notifier.notices, 1)
		assert.Equal(t, []int64{caregiverAccountID}, setup.notifier.notices[0].StaffAccountIDs)
	})

	t.Run("nobody clocked in fails closed", func(t *testing.T) {
		setup := buildReminderProducer(t, results, consent)
		setup.duty.onDuty = nil

		setup.producer.NotifyDue(context.Background(), testTenant, time.Now(), true)

		assert.Empty(t, setup.notifier.notices)
		assert.Zero(t, setup.computer.calls,
			"an empty work-session day must not widen the recipient set")
	})

	t.Run("everybody stamped out stays quiet", func(t *testing.T) {
		setup := buildReminderProducer(t, results, consent)
		setup.duty.onDuty = map[int64]struct{}{}

		setup.producer.NotifyDue(context.Background(), testTenant, time.Now(), true)

		assert.Empty(t, setup.notifier.notices)
		assert.Zero(t, setup.computer.calls)
	})

	t.Run("a failing presence read aborts the tick", func(t *testing.T) {
		setup := buildReminderProducer(t, results, consent)
		setup.duty.err = assert.AnError

		setup.producer.NotifyDue(context.Background(), testTenant, time.Now(), true)

		assert.Empty(t, setup.notifier.notices)
		assert.Zero(t, setup.computer.calls)
	})
}

// Without an own value the duty gate is off (#3736): a school without time
// tracking reaches its team even though nobody is clocked in.
func TestPersonalRemindersDutyGateOffByDefault(t *testing.T) {
	t.Parallel()

	setup := buildReminderProducer(t,
		map[int64]*Result{caregiverStaffID: resultOf(pickupFixture("11", "14:00"))},
		map[string][]int64{TypePickupUpcoming: {caregiverAccountID}},
	)
	setup.duty.onDuty = nil

	setup.notify(context.Background(), time.Now())

	require.Len(t, setup.notifier.notices, 1)
	assert.Equal(t, []int64{caregiverAccountID}, setup.notifier.notices[0].StaffAccountIDs)
}

func TestPersonalRemindersSkipConditions(t *testing.T) {
	t.Parallel()

	consent := map[string][]int64{TypePickupUpcoming: {caregiverAccountID}}
	reminder := pickupFixture("11", "14:00")

	t.Run("reminder feature off", func(t *testing.T) {
		setup := buildReminderProducer(t, map[int64]*Result{
			caregiverStaffID: {Enabled: false, Reminders: []Reminder{reminder}},
		}, consent)
		setup.notify(context.Background(), time.Now())
		assert.Empty(t, setup.notifier.notices)
	})

	t.Run("nothing due", func(t *testing.T) {
		setup := buildReminderProducer(t, map[int64]*Result{
			caregiverStaffID: {Enabled: true},
		}, consent)
		setup.notify(context.Background(), time.Now())
		assert.Empty(t, setup.notifier.notices)
	})

	t.Run("compute error", func(t *testing.T) {
		setup := buildReminderProducer(t, nil, consent)
		setup.computer.err = assert.AnError
		setup.notify(context.Background(), time.Now())
		assert.Empty(t, setup.notifier.notices)
	})

	t.Run("no staff at all", func(t *testing.T) {
		setup := buildReminderProducer(t, nil, consent)
		setup.staff.accounts = nil
		setup.notify(context.Background(), time.Now())
		assert.Empty(t, setup.notifier.notices)
		assert.Zero(t, setup.computer.calls)
	})

	t.Run("a failing reader aborts the tick", func(t *testing.T) {
		setup := buildReminderProducer(t, nil, consent)
		setup.admins.err = assert.AnError
		setup.notify(context.Background(), time.Now())
		assert.Empty(t, setup.notifier.notices)
		assert.Zero(t, setup.computer.calls,
			"an unknown admin set would silently change what people are allowed to see")
	})
}

func TestPersonalRemindersNotifyFailureLeavesUnmarked(t *testing.T) {
	t.Parallel()

	setup := buildReminderProducer(t,
		map[int64]*Result{caregiverStaffID: resultOf(pickupFixture("11", "14:00"))},
		map[string][]int64{TypePickupUpcoming: {caregiverAccountID}},
	)
	setup.notifier.err = errDispatchPaused

	setup.notify(context.Background(), time.Now())
	assert.Empty(t, setup.notifier.notices)

	// Dispatch becomes possible again → the SAME occurrence fires now.
	setup.notifier.err = nil
	setup.notify(context.Background(), time.Now())
	assert.Len(t, setup.notifier.notices, 1, "a failed dispatch must not consume the occurrence")
}

func TestPersonalRemindersMarkOccurrencesAfterCommit(t *testing.T) {
	t.Parallel()

	reminder := pickupFixture("11", "14:00")
	setup := buildReminderProducer(t,
		map[int64]*Result{caregiverStaffID: resultOf(reminder)},
		map[string][]int64{TypePickupUpcoming: {caregiverAccountID}},
	)
	key := reminderNotificationKey{
		tenantID:  testTenant,
		accountID: caregiverAccountID,
		reminder:  reminderIdentity(reminder),
	}
	ctx, commit := withFakeTx(context.Background())

	setup.notify(ctx, time.Now())

	assert.Empty(t, setup.notifier.notices, "delivery must wait for commit")
	_, markedBeforeCommit := setup.producer.notified.Load(key)
	assert.False(t, markedBeforeCommit, "dedup key must wait for commit")

	commit()

	assert.Len(t, setup.notifier.notices, 1)
	_, markedAfterCommit := setup.producer.notified.Load(key)
	assert.True(t, markedAfterCommit, "successful commit must consume the occurrence")

	setup.notify(context.Background(), time.Now())
	assert.Len(t, setup.notifier.notices, 1, "committed occurrence must not re-fire")
}

func TestPersonalRemindersRollbackLeavesOccurrencesUnmarked(t *testing.T) {
	t.Parallel()

	reminder := pickupFixture("11", "14:00")
	setup := buildReminderProducer(t,
		map[int64]*Result{caregiverStaffID: resultOf(reminder)},
		map[string][]int64{TypePickupUpcoming: {caregiverAccountID}},
	)
	key := reminderNotificationKey{
		tenantID:  testTenant,
		accountID: caregiverAccountID,
		reminder:  reminderIdentity(reminder),
	}
	rolledBackCtx, _ := withFakeTx(context.Background())

	setup.notify(rolledBackCtx, time.Now())

	assert.Empty(t, setup.notifier.notices, "rollback must drop queued delivery")
	_, marked := setup.producer.notified.Load(key)
	assert.False(t, marked, "rollback must leave the occurrence eligible")

	setup.notify(context.Background(), time.Now())
	assert.Len(t, setup.notifier.notices, 1, "the next successful tick must retry the occurrence")
}

func TestPersonalRemindersTenantIsolationAndDayRotation(t *testing.T) {
	t.Parallel()

	setup := buildReminderProducer(t,
		map[int64]*Result{caregiverStaffID: resultOf(pickupFixture("11", "14:00"))},
		map[string][]int64{TypePickupUpcoming: {caregiverAccountID}},
	)
	// 00:30 on the night the clocks go back: that day has 25 hours, so
	// now+24h would still fall on it. The next civil date comes from the
	// calendar, never from a fixed duration (#4027).
	day := calendar.NewDate(2026, time.October, 25)
	now := day.BerlinMidnight().Add(30 * time.Minute)
	nextDay := day.AddDays(1).BerlinMidnight().Add(30 * time.Minute)

	setup.producer.NotifyDue(context.Background(), 7, now, false)
	setup.producer.NotifyDue(context.Background(), 8, now, false)
	assert.Len(t, setup.notifier.notices, 2, "the guard is per tenant")

	// Same civil date → rotation is a no-op, guard intact.
	setup.producer.NotifyDue(context.Background(), 7, now, false)
	assert.Len(t, setup.notifier.notices, 2)

	// Next civil date → guard cleared, identical schedule fires again.
	setup.producer.NotifyDue(context.Background(), 7, nextDay, false)
	assert.Len(t, setup.notifier.notices, 3)
}

// Every single missing dependency must refuse construction: a producer that
// resolves a narrower audience because one reader is absent would look like
// it works.
func TestPersonalReminderNotificationsRequireEveryDependency(t *testing.T) {
	t.Parallel()

	complete := buildReminderProducer(t, nil, nil).dependencies()
	partials := map[string]func(*PersonalReminderDependencies){
		"no computer":     func(d *PersonalReminderDependencies) { d.Computer = nil },
		"no consent":      func(d *PersonalReminderDependencies) { d.Consent = nil },
		"no staff reader": func(d *PersonalReminderDependencies) { d.Staff = nil },
		"no admin reader": func(d *PersonalReminderDependencies) { d.Admins = nil },
		"no duty reader":  func(d *PersonalReminderDependencies) { d.OnDutyStaff = nil },
		"no notifier":     func(d *PersonalReminderDependencies) { d.Notify = nil },
		"no pause check":  func(d *PersonalReminderDependencies) { d.Paused = nil },
		"no after-commit": func(d *PersonalReminderDependencies) { d.AfterCommit = nil },
		"no logger":       func(d *PersonalReminderDependencies) { d.Logger = nil },
	}
	for name, drop := range partials {
		t.Run(name, func(t *testing.T) {
			deps := complete
			drop(&deps)
			producer, err := NewPersonalReminderNotifications(deps)
			require.Error(t, err)
			assert.Nil(t, producer)
		})
	}

	t.Run("fully wired", func(t *testing.T) {
		producer, err := NewPersonalReminderNotifications(complete)
		require.NoError(t, err)
		assert.NotNil(t, producer)
	})
}

// The copy table and the type list are two halves of one contract: a type
// without wording would dispatch the generic fallback, which reads like a bug.
// That every type is in the notification catalogue is pinned beside the
// catalogue (modules/delivery/application/notifications); both sides pin the
// wire vocabulary independently, without a platform-to-workflow import.
func TestEveryPersonalReminderTypeHasCopy(t *testing.T) {
	t.Parallel()

	for _, notificationType := range personalReminderTypes {
		copyFor, ok := reminderCopyByType[notificationType]
		require.True(t, ok, "missing copy for %s", notificationType)
		assert.NotEmpty(t, copyFor.title)
		assert.NotEmpty(t, copyFor.singular)
		assert.Contains(t, copyFor.plural, "%d")
	}
}

// Pin the notice vocabulary where this workflow hands it to Delivery.
func TestPersonalReminderNoticeWireVocabulary(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"pickup_upcoming", "pickup_overdue", "activity_start", "activity_overdue", "activity_running_over", "my_activity_starting"}, personalReminderTypes)
	assert.Equal(t, "normal", NoticePriorityNormal)
	assert.Equal(t, "high", NoticePriorityHigh)
	assert.Equal(t, "staff", NoticeScopeStaff)
}
