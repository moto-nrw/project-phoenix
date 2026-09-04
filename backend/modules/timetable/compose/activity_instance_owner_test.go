package compose

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type ownedActivityInstanceFixture struct {
	roomID  int64
	groupID int64
}

func TestModuleOwnsActivityInstanceLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module, ctx := buildModule(t, db, log.record), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "lifecycle")

	first := createOwnedActivityInstance(t, module, ctx, fixture, "2027-09-06", "08:00:00", "Erste")
	second := createOwnedActivityInstance(t, module, ctx, fixture, "2027-09-07", "09:00:00", "Zweite")
	found, err := module.FindActivityInstance(ctx, first.ID)
	require.NoError(t, err)
	assert.Equal(t, "08:00:00", found.StartTime)

	listed, err := module.ListActivityInstances(ctx, timetable.ActivityInstanceFilter{
		ActivityGroupID: &fixture.groupID, FromDate: dateText("2027-09-06"), ToDate: dateText("2027-09-06"),
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{first.ID}, activityInstanceIDs(listed))

	input := ownedActivityInstanceInput(fixture, second.Date, "10:00:00", "Aktualisiert")
	updated, err := module.UpdateActivityInstance(ctx, second.ID, input)
	require.NoError(t, err)
	assert.Equal(t, "Aktualisiert", updated.Title)
	require.NoError(t, module.DeleteActivityInstance(ctx, second.ID))
	_, err = module.FindActivityInstance(ctx, second.ID)
	require.ErrorIs(t, err, timetable.ErrActivityInstanceNotFound)
	assert.EqualValues(t, 1, observedOperation(log.seen, "list_activity_instances").Stats.Queries)
}

func TestModuleOwnsActivityInstancePartialUpdates(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "patch")
	created := createOwnedActivityInstance(t, module, ctx, fixture, "2027-09-06", "08:00:00", "Patch")

	patch := timetable.ActivityInstanceInput{UnderstaffedAck: true}
	rows, err := module.PatchActivityInstance(ctx, created.ID, patch, []string{"understaffed_ack"})
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
	patched, err := module.FindActivityInstance(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, patched.UnderstaffedAck)
}

func TestModuleActivityInstanceConflictsKeepCallerTransactionUsable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module, ctx := buildModule(t, db, log.record), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "conflict")
	input := ownedActivityInstanceInput(fixture, "2027-09-08", "08:00:00", "Slot")

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, inserted, err := module.CreateTemplateBackedActivityInstanceIfAbsent(txCtx, input)
		if err != nil || !inserted {
			return fmt.Errorf("first insert: inserted=%t: %w", inserted, err)
		}
		_, inserted, err = module.CreateTemplateBackedActivityInstanceIfAbsent(txCtx, input)
		if err != nil || inserted {
			return fmt.Errorf("duplicate insert: inserted=%t: %w", inserted, err)
		}
		other := ownedActivityInstanceInput(fixture, input.Date, "10:00:00", "Transaction still usable")
		_, err = module.CreateActivityInstance(txCtx, other)
		return err
	})
	require.NoError(t, err)
	count, err := module.CountActivityInstances(ctx, dateText("2027-09-09"))
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.EqualValues(t, 1, lastObservedOperation(log.seen, "create_template_backed_activity_instance").Stats.DuplicatePreventionConflicts)
}

func TestModuleActivityInstanceIdempotencyUsesRequestKey(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module, ctx := buildModule(t, db, log.record), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "idempotency")
	input := ownedActivityInstanceInput(fixture, "2027-09-08", "12:00:00", "Idempotent")
	input.IsSpontaneous = true
	input.IdempotencyKey = stringText("request-1")

	first, inserted, err := module.CreateIdempotentActivityInstance(ctx, input)
	require.NoError(t, err)
	assert.True(t, inserted)
	assert.NotZero(t, first.ID)
	duplicate, inserted, err := module.CreateIdempotentActivityInstance(ctx, input)
	require.NoError(t, err)
	assert.False(t, inserted)
	assert.Zero(t, duplicate.ID)
	assert.EqualValues(t, 1, lastObservedOperation(log.seen, "create_idempotent_activity_instance").Stats.DuplicatePreventionConflicts)
}

func TestModuleActivityInstancesAreTenantIsolated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	ownedFixture := newOwnedActivityInstanceFixture(t, db, "owned")
	owned := createOwnedActivityInstance(t, module, ctx, ownedFixture, "2027-09-09", "08:00:00", "Owned")

	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)
	foreignRoom := testpkg.CreateTestRoomForTenant(t, db, foreignTenantID, "Foreign instance room")
	foreignGroup := testpkg.CreateTestActivityGroupForTenant(t, db, foreignTenantID, "Foreign instance group")
	foreignFixture := ownedActivityInstanceFixture{roomID: foreignRoom.ID, groupID: foreignGroup.ID}
	foreign := createOwnedActivityInstance(t, module, foreignCtx, foreignFixture, "2027-09-09", "08:00:00", "Foreign")

	_, err := module.CreateActivityInstance(ctx, ownedActivityInstanceInput(foreignFixture, "2027-09-10", "08:00:00", "Cross tenant"))
	require.Error(t, err)
	_, err = module.FindActivityInstance(foreignCtx, owned.ID)
	require.ErrorIs(t, err, timetable.ErrActivityInstanceNotFound)
	listed, err := module.ListActivityInstances(ctx, timetable.ActivityInstanceFilter{})
	require.NoError(t, err)
	assert.Contains(t, activityInstanceIDs(listed), owned.ID)
	assert.NotContains(t, activityInstanceIDs(listed), foreign.ID)
	_, err = module.UpdateActivityInstance(foreignCtx, owned.ID, ownedActivityInstanceInput(foreignFixture, "2027-09-11", "08:00:00", "No"))
	require.ErrorIs(t, err, timetable.ErrActivityInstanceNotFound)
	require.NoError(t, module.DeleteActivityInstance(foreignCtx, owned.ID))
	_, err = module.FindActivityInstance(ctx, owned.ID)
	require.NoError(t, err)
}

func TestModuleActivityInstanceBulkLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "bulk")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, fixture.groupID, fixture.roomID)
	input := ownedActivityInstanceInput(fixture, "2027-09-12", "08:00:00", "Active")
	input.Status, input.ActiveGroupID = timetable.InstanceStatusActive, &activeGroup.ID
	active, err := module.CreateActivityInstance(ctx, input)
	require.NoError(t, err)

	completedAt := time.Date(2027, 9, 12, 10, 0, 0, 0, time.UTC)
	rows, err := module.CompleteActiveActivityInstances(ctx, []int64{activeGroup.ID}, completedAt)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
	stored, err := module.FindActivityInstance(ctx, active.ID)
	require.NoError(t, err)
	assert.Equal(t, timetable.InstanceStatusCompleted, stored.Status)
	require.NotNil(t, stored.CompletedAt)
	assert.True(t, completedAt.Equal(*stored.CompletedAt))
}

func TestModuleActivityInstanceReplanPreservesDeviations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "deviation")
	plain := createOwnedActivityInstance(t, module, ctx, fixture, "2027-09-13", "08:00:00", "Plain")
	absent := createOwnedActivityInstance(t, module, ctx, fixture, "2027-09-13", "10:00:00", "Absent")
	ack := ownedActivityInstanceInput(fixture, "2027-09-13", "12:00:00", "Acknowledged")
	ack.UnderstaffedAck = true
	acknowledged, err := module.CreateActivityInstance(ctx, ack)
	require.NoError(t, err)
	staff := testpkg.CreateTestStaff(t, db, "Owner deviation", fmt.Sprintf("%d", time.Now().UnixNano()))
	testpkg.CreateTestInstanceStaff(t, db, absent.ID, staff.ID, testpkg.InstanceStaffOpts{IsAbsent: true})

	to := "2027-09-13"
	rows, err := module.DeletePlannedActivityInstances(ctx, to, &to, nil, true)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
	_, err = module.FindActivityInstance(ctx, plain.ID)
	require.ErrorIs(t, err, timetable.ErrActivityInstanceNotFound)
	_, err = module.FindActivityInstance(ctx, absent.ID)
	require.NoError(t, err)
	_, err = module.FindActivityInstance(ctx, acknowledged.ID)
	require.NoError(t, err)

	rows, err = module.DeletePlannedActivityInstances(ctx, to, &to, &fixture.groupID, false)
	require.NoError(t, err)
	assert.EqualValues(t, 2, rows)
}

func TestModuleActivityInstanceWeekendCleanup(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "weekend")
	const saturday = "2099-01-03"
	periodID := createOwnerCalendarPeriod(t, db, saturday, "2099-01-04")

	materializedInput := ownedActivityInstanceInput(fixture, saturday, "08:00:00", "Materialized")
	materializedInput.CalendarPeriodID = &periodID
	materialized, err := module.CreateActivityInstance(ctx, materializedInput)
	require.NoError(t, err)
	manual := createOwnedActivityInstance(t, module, ctx, fixture, saturday, "10:00:00", "Manual")

	rows, err := module.DeleteRemovedWeekendActivityInstances(ctx, fixture.groupID, []int{6})
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
	_, err = module.FindActivityInstance(ctx, materialized.ID)
	require.ErrorIs(t, err, timetable.ErrActivityInstanceNotFound)
	_, err = module.FindActivityInstance(ctx, manual.ID)
	require.NoError(t, err)
}

func TestModuleActivityInstanceListKindPropagation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "list-kind")
	const today, future = "2099-01-05", "2099-01-12"
	futureRow := createOwnedActivityInstance(t, module, ctx, fixture, future, "08:00:00", "Future")
	todayRow := createOwnedActivityInstance(t, module, ctx, fixture, today, "08:00:00", "Today")
	overrideInput := ownedActivityInstanceInput(fixture, future, "10:00:00", "Override")
	overrideInput.ListKind = stringText(timetable.ListKindMensa)
	override, err := module.CreateActivityInstance(ctx, overrideInput)
	require.NoError(t, err)

	newKind := stringText(timetable.ListKindLearningTime)
	rows, err := module.PropagateActivityInstanceListKind(ctx, fixture.groupID, nil, newKind, today)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)
	assertActivityInstanceListKind(t, module, ctx, futureRow.ID, newKind)
	assertActivityInstanceListKind(t, module, ctx, todayRow.ID, nil)
	assertActivityInstanceListKind(t, module, ctx, override.ID, overrideInput.ListKind)
}

func TestModuleActivityInstanceRetentionAndFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "retention")
	createOwnedActivityInstance(t, module, ctx, fixture, "2027-09-14", "08:00:00", "Old")
	createOwnedActivityInstance(t, module, ctx, fixture, "2027-09-15", "08:00:00", "New")

	oldest, err := module.OldestActivityInstanceBefore(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "2027-09-14", *oldest)
	rows, err := module.DeleteActivityInstancesBefore(ctx, "2027-09-15")
	require.NoError(t, err)
	assert.EqualValues(t, 1, rows)

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = module.FindActivityInstance(cancelled, 1)
	require.ErrorIs(t, err, context.Canceled)
	_, err = module.ListActivityInstances(cancelled, timetable.ActivityInstanceFilter{})
	require.ErrorIs(t, err, context.Canceled)
}

func TestModuleActivityInstanceWritesRollBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "rollback")
	wantErr := errors.New("abort activity instance write")
	var rolledBackID int64

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		created, createErr := module.CreateActivityInstance(txCtx, ownedActivityInstanceInput(fixture, "2027-09-16", "08:00:00", "Rollback"))
		rolledBackID = created.ID
		if createErr != nil {
			return createErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	_, err = module.FindActivityInstance(ctx, rolledBackID)
	require.ErrorIs(t, err, timetable.ErrActivityInstanceNotFound)
	created := createOwnedActivityInstance(t, module, ctx, fixture, "2027-09-16", "08:00:00", "Retry")
	assert.NotZero(t, created.ID)
}

func TestActivityInstanceListQueryBudgetStaysFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "budget")
	for day := 17; day <= 24; day++ {
		createOwnedActivityInstance(t, module, ctx, fixture, fmt.Sprintf("2027-09-%02d", day), "08:00:00", "Budget")
	}
	counter := testpkg.CaptureQueries(t, db)
	_, err := module.ListActivityInstances(ctx, timetable.ActivityInstanceFilter{ActivityGroupID: &fixture.groupID})
	require.NoError(t, err)
	testpkg.AssertQueryBudget(t, "modules.timetable.activity_instances.list", counter.Queries())
}

func newOwnedActivityInstanceFixture(t *testing.T, db *bun.DB, suffix string) ownedActivityInstanceFixture {
	t.Helper()
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Owner instance room %s %d", suffix, time.Now().UnixNano()))
	group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Owner instance group %s %d", suffix, time.Now().UnixNano()))
	return ownedActivityInstanceFixture{roomID: room.ID, groupID: group.ID}
}

func createOwnedActivityInstance(t *testing.T, module *timetable.Module, ctx context.Context, fixture ownedActivityInstanceFixture, date, start, title string) timetable.ActivityInstance {
	t.Helper()
	value, err := module.CreateActivityInstance(ctx, ownedActivityInstanceInput(fixture, date, start, title))
	require.NoError(t, err)
	return value
}

func ownedActivityInstanceInput(fixture ownedActivityInstanceFixture, date, start, title string) timetable.ActivityInstanceInput {
	parsed, _ := time.Parse("15:04:05", start)
	end := parsed.Add(time.Hour).Format("15:04:05")
	return timetable.ActivityInstanceInput{
		Date: date, ActivityGroupID: &fixture.groupID, Title: title, StartTime: start, EndTime: end,
		RoomID: fixture.roomID, Status: timetable.InstanceStatusPlanned,
	}
}

func activityInstanceIDs(values []timetable.ActivityInstance) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		result = append(result, value.ID)
	}
	return result
}

func lastObservedOperation(observations []Observation, operation string) Observation {
	for index := len(observations) - 1; index >= 0; index-- {
		if observations[index].Operation == operation {
			return observations[index]
		}
	}
	return Observation{}
}

func createOwnerCalendarPeriod(t *testing.T, db *bun.DB, start, end string) int64 {
	t.Helper()
	ctx := testpkg.Ctx(t)
	var id int64
	err := db.NewRaw(`INSERT INTO schedule.calendar_periods
		(tenant_id, name, period_type, start_date, end_date, week_cycle_length, is_active)
		VALUES (?, ?, 'custom', ?::date, ?::date, 1, FALSE) RETURNING id`,
		testpkg.Tenant(t), fmt.Sprintf("Owner instance period %d", time.Now().UnixNano()), start, end).Scan(ctx, &id)
	require.NoError(t, err)
	return id
}

func assertActivityInstanceListKind(t *testing.T, module *timetable.Module, ctx context.Context, id int64, want *string) {
	t.Helper()
	value, err := module.FindActivityInstance(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, want, value.ListKind)
}

func stringText(value string) *string { return &value }
