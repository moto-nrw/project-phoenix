package compose

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// emptyCarePlanDirectory answers the care-plan reads a roster write re-applies
// (sick days, partial absences) with "nothing on file".
type emptyCarePlanDirectory struct{}

func (emptyCarePlanDirectory) FindPickupException(context.Context, int64) (*timetable.PickupException, error) {
	return nil, nil
}

func (emptyCarePlanDirectory) ListPickupExceptions(context.Context, timetable.PickupExceptionFilter) ([]timetable.PickupException, error) {
	return []timetable.PickupException{}, nil
}

func (emptyCarePlanDirectory) FindStudentStatusDay(context.Context, int64, bool) (*timetable.StudentStatusDay, error) {
	return nil, nil
}

func (emptyCarePlanDirectory) ListStudentStatusDays(context.Context, timetable.StudentStatusDayFilter) ([]timetable.StudentStatusDay, error) {
	return []timetable.StudentStatusDay{}, nil
}

func buildPickupExtensionModule(t *testing.T, db *bun.DB, observers ...func(Observation)) *timetable.Module {
	return buildPickupExtensionModuleWithStudents(t, db, StudentDirectoryFunc(func(context.Context) ([]TargetStudent, error) {
		return []TargetStudent{}, nil
	}), observers...)
}

func buildPickupExtensionModuleWithStudents(
	t *testing.T, db *bun.DB, students StudentDirectory, observers ...func(Observation),
) *timetable.Module {
	t.Helper()
	observe := func(Observation) {}
	if len(observers) > 0 {
		observe = observers[0]
	}
	module, err := New(Dependencies{
		DB: db, Students: students, Rooms: testRooms(), CareDays: testCareDays(), CarePlan: emptyCarePlanDirectory{},
		LockStaffAssignment: func(context.Context, int64) error { return nil }, Observe: observe,
	})
	require.NoError(t, err)
	return module
}

func pickupExtensionInstance(t *testing.T, module *timetable.Module, ctx context.Context, fixture ownedActivityInstanceFixture, date, start, end, title string) timetable.ActivityInstance {
	t.Helper()
	input := ownedActivityInstanceInput(fixture, date, start, title)
	input.EndTime = end
	value, err := module.CreateActivityInstance(ctx, input)
	require.NoError(t, err)
	return value
}

func pickupExtensionTask(t *testing.T, module *timetable.Module, ctx context.Context, studentID int64) timetable.PickupExtensionTask {
	t.Helper()
	tasks, err := module.ListOpenPickupExtensions(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	return tasks[0]
}

func pickupExtensionBlockIDs(blocks []timetable.PickupExtensionBlock) []int64 {
	result := make([]int64, 0, len(blocks))
	for _, block := range blocks {
		result = append(result, block.ID)
	}
	return result
}

// TestModuleOwnsPickupDayExtension covers the call's incident for one day:
// the child stays until 16:00, "Freies Spiel" runs 14:45-16:00 with other
// children, and an office appointment without children overlaps too.
func TestModuleOwnsPickupDayExtension(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module, ctx := buildPickupExtensionModule(t, db, log.record), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "pickup-day")
	suffix := time.Now().UnixNano()
	child := testpkg.CreateTestStudent(t, db, "Mia", fmt.Sprintf("Later-%d", suffix), "1a")
	other := testpkg.CreateTestStudent(t, db, "Ben", fmt.Sprintf("Other-%d", suffix), "1a")
	staff := testpkg.CreateTestStaff(t, db, "Swantje", fmt.Sprintf("Lead-%d", suffix))
	const date = "2099-03-03"
	exception := testpkg.CreateTestPickupException(t, db, child.ID, testpkg.Date(2099, time.March, 3), staff.ID, "16:00", "")

	freePlay := pickupExtensionInstance(t, module, ctx, fixture, date, "14:45:00", "16:00:00", "Freies Spiel")
	office := pickupExtensionInstance(t, module, ctx, fixture, date, "15:00:00", "15:30:00", "Bürotermin")
	notScheduled := pickupExtensionInstance(t, module, ctx, fixture, date, "15:31:00", "16:00:00", "Abgemeldet")
	lunch := pickupExtensionInstance(t, module, ctx, fixture, date, "12:00:00", "13:00:00", "Mittagessen")
	createOwnedInstanceStudent(t, module, ctx, freePlay.ID, other.ID, timetable.InstanceAttendanceExpected)
	createOwnedInstanceStudent(t, module, ctx, notScheduled.ID, other.ID, timetable.InstanceAttendanceExpected)
	createOwnedInstanceStudent(t, module, ctx, lunch.ID, other.ID, timetable.InstanceAttendanceExpected)
	createOwnedInstanceStudent(t, module, ctx, lunch.ID, child.ID, timetable.InstanceAttendanceExpected)
	require.NoError(t, module.MarkNotScheduled(ctx, []timetable.StudentInstanceRef{{
		StudentID: other.ID, InstanceID: notScheduled.ID,
	}}))

	require.NoError(t, module.RecordPickupDayExtension(ctx, timetable.PickupDayExtension{
		StudentID: child.ID, PickupExceptionID: exception.ID, Date: date, PreviousPickup: "14:45", Pickup: "16:00",
	}))

	task := pickupExtensionTask(t, module, ctx, child.ID)
	assert.Equal(t, timetable.PickupExtensionKindDay, task.Kind)
	assert.Equal(t, date, task.Date)
	assert.Equal(t, "14:45", task.PreviousPickup)
	assert.Equal(t, "16:00", task.Pickup)
	assert.Equal(t, []int64{freePlay.ID}, pickupExtensionBlockIDs(task.Blocks), "only blocks with children in the extra time are choices")
	assert.Equal(t, timetable.PickupExtensionBlock{ID: freePlay.ID, Title: "Freies Spiel", StartTime: "14:45", EndTime: "16:00"}, task.Blocks[0])
	assert.EqualValues(t, 2, observedOperation(log.seen, "list_open_pickup_extensions").Stats.Queries)

	all, err := module.ListOpenPickupExtensions(ctx, 0)
	require.NoError(t, err)
	assert.Contains(t, pickupExtensionTaskIDs(all), task.ID)

	_, err = module.ResolvePickupExtension(ctx, task.ID, []int64{office.ID})
	require.ErrorIs(t, err, timetable.ErrPickupExtensionBlockGone, "an office appointment is never a choice")
	_, err = module.ResolvePickupExtension(ctx, task.ID, []int64{notScheduled.ID})
	require.ErrorIs(t, err, timetable.ErrPickupExtensionBlockGone, "an instance without scheduled children is never a choice")

	resolved, err := module.ResolvePickupExtension(ctx, task.ID, []int64{freePlay.ID})
	require.NoError(t, err)
	assert.Equal(t, child.ID, resolved.StudentID)
	assert.Equal(t, []int64{freePlay.ID}, resolved.InstanceIDs)
	roster, err := module.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{freePlay.ID}})
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{other.ID, child.ID}, instanceStudentStudentIDs(roster))
	for _, row := range roster {
		if row.StudentID == child.ID {
			assert.Equal(t, timetable.InstanceAttendanceExpected, row.Status)
			assert.False(t, row.IsUnplanned, "the child is planned, not an unplanned walk-in")
		}
	}

	tasks, err := module.ListOpenPickupExtensions(ctx, child.ID)
	require.NoError(t, err)
	assert.Empty(t, tasks, "a resolved task disappears")
	_, err = module.ResolvePickupExtension(ctx, task.ID, nil)
	require.ErrorIs(t, err, timetable.ErrPickupExtensionNotFound)

	// Recording the same change again opens nothing: the child is now on a
	// block that runs until the new pickup time.
	require.NoError(t, module.RecordPickupDayExtension(ctx, timetable.PickupDayExtension{
		StudentID: child.ID, PickupExceptionID: exception.ID, Date: date, PreviousPickup: "14:45", Pickup: "16:00",
	}))
	tasks, err = module.ListOpenPickupExtensions(ctx, child.ID)
	require.NoError(t, err)
	assert.Empty(t, tasks)

	require.NoError(t, module.ClearPickupDayExtension(ctx, child.ID, date))
	assert.Zero(t, countPickupExtensionRows(t, db, child.ID))
}

func TestModuleClosesPickupDayExtensionWithoutBlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildPickupExtensionModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "pickup-none")
	suffix := time.Now().UnixNano()
	child := testpkg.CreateTestStudent(t, db, "Lina", fmt.Sprintf("None-%d", suffix), "2b")
	other := testpkg.CreateTestStudent(t, db, "Tom", fmt.Sprintf("None-%d", suffix), "2b")
	staff := testpkg.CreateTestStaff(t, db, "Team", fmt.Sprintf("None-%d", suffix))
	exception := testpkg.CreateTestPickupException(t, db, child.ID, testpkg.Date(2099, time.March, 4), staff.ID, "16:30", "")
	block := pickupExtensionInstance(t, module, ctx, fixture, "2099-03-04", "15:00:00", "16:30:00", "Hausaufgaben")
	createOwnedInstanceStudent(t, module, ctx, block.ID, other.ID, timetable.InstanceAttendanceExpected)

	require.NoError(t, module.RecordPickupDayExtension(ctx, timetable.PickupDayExtension{
		StudentID: child.ID, PickupExceptionID: exception.ID, Date: "2099-03-04", PreviousPickup: "15:00", Pickup: "16:30",
	}))
	task := pickupExtensionTask(t, module, ctx, child.ID)

	resolved, err := module.ResolvePickupExtension(ctx, task.ID, nil)
	require.NoError(t, err)
	assert.Empty(t, resolved.AssignedBlocks)
	assert.Zero(t, countPickupExtensionRows(t, db, child.ID), "\"keinem Block zuordnen\" closes the task")
	roster, err := module.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{block.ID}})
	require.NoError(t, err)
	assert.Equal(t, []int64{other.ID}, instanceStudentStudentIDs(roster), "no roster changes without a choice")

	// Past days never open a task.
	pastException := testpkg.CreateTestPickupException(t, db, child.ID, testpkg.Date(2020, time.January, 7), staff.ID, "16:30", "")
	require.NoError(t, module.RecordPickupDayExtension(ctx, timetable.PickupDayExtension{
		StudentID: child.ID, PickupExceptionID: pastException.ID, Date: "2020-01-07", PreviousPickup: "15:00", Pickup: "16:30",
	}))
	assert.Zero(t, countPickupExtensionRows(t, db, child.ID))

	err = module.RecordPickupDayExtension(ctx, timetable.PickupDayExtension{
		StudentID: child.ID, PickupExceptionID: exception.ID, Date: "2099-03-04", PreviousPickup: "16:30", Pickup: "15:00",
	})
	require.ErrorIs(t, err, timetable.ErrInvalidPickupExtension, "an earlier pickup is the partial-absence path")
}

// TestModuleOwnsPickupWeekdayExtension covers the lasting change from the
// call: Tuesday moves from 14:45 to 16:00, and the choice applies to every
// future Tuesday of the template.
func TestModuleOwnsPickupWeekdayExtension(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildPickupExtensionModule(t, db), testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	category := createCategory(t, ctx, module, fmt.Sprintf("Pickup weekday %d", suffix))
	child := testpkg.CreateTestStudent(t, db, "Emil", fmt.Sprintf("Weekday-%d", suffix), "1a")
	other := testpkg.CreateTestStudent(t, db, "Ida", fmt.Sprintf("Weekday-%d", suffix), "1a")
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Pickup weekday room %d", suffix))

	freePlay := pickupExtensionTemplate(t, module, ctx, category.ID, "Freies Spiel", "14:45:00", "16:00:00", timetable.WeekdayTuesday)
	office := pickupExtensionTemplate(t, module, ctx, category.ID, "Teamsitzung", "15:00:00", "15:45:00", timetable.WeekdayTuesday)
	_, err := module.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: other.ID, ActivityGroupID: freePlay.ID, ValidFrom: "2020-01-01",
	})
	require.NoError(t, err)
	_, err = module.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: other.ID, ActivityGroupID: office.ID, ValidFrom: "2100-01-01",
	})
	require.NoError(t, err)
	// Future attendance does not make the otherwise empty Teamsitzung a
	// selectable block at the effective date.
	fixture := ownedActivityInstanceFixture{roomID: room.ID, groupID: freePlay.ID}
	tuesday := pickupExtensionInstance(t, module, ctx, fixture, "2099-03-10", "14:45:00", "16:00:00", "Freies Spiel")
	earlierTuesday := pickupExtensionInstance(t, module, ctx, fixture, "2099-03-03", "14:45:00", "16:00:00", "Freies Spiel")
	wednesday := pickupExtensionInstance(t, module, ctx, fixture, "2099-03-11", "14:45:00", "16:00:00", "Freies Spiel")

	record := func(previous, pickup string) {
		t.Helper()
		require.NoError(t, module.RecordPickupWeekdayExtension(ctx, timetable.PickupWeekdayExtension{
			StudentID: child.ID, Weekday: timetable.WeekdayTuesday, EffectiveFrom: "2099-03-05",
			PreviousPickup: previous, Pickup: pickup,
		}))
	}

	// Two changes in a row keep the earliest previous time.
	record("14:45", "16:00")
	record("16:00", "15:30")
	task := pickupExtensionTask(t, module, ctx, child.ID)
	assert.Equal(t, timetable.PickupExtensionKindWeekday, task.Kind)
	assert.Equal(t, timetable.WeekdayTuesday, task.Weekday)
	assert.Equal(t, "2099-03-05", task.EffectiveFrom)
	assert.Equal(t, "14:45", task.PreviousPickup)
	assert.Equal(t, "15:30", task.Pickup)
	assert.Equal(t, []int64{freePlay.ID}, pickupExtensionBlockIDs(task.Blocks), "a template without children is no choice")
	assert.NotContains(t, pickupExtensionBlockIDs(task.Blocks), office.ID)

	// A change back to the old time closes the task.
	record("15:30", "14:45")
	tasks, err := module.ListOpenPickupExtensions(ctx, child.ID)
	require.NoError(t, err)
	assert.Empty(t, tasks)

	record("14:45", "16:00")
	task = pickupExtensionTask(t, module, ctx, child.ID)
	resolved, err := module.ResolvePickupExtension(ctx, task.ID, []int64{freePlay.ID})
	require.NoError(t, err)
	assert.Equal(t, timetable.PickupExtensionKindWeekday, resolved.Kind)
	assert.Equal(t, []int64{tuesday.ID}, resolved.InstanceIDs, "only planned Tuesdays from the effective date on")

	enrollments, err := module.ListStudentEnrollments(ctx, timetable.StudentEnrollmentFilter{
		StudentIDs: []int64{child.ID}, ActivityGroupIDs: []int64{freePlay.ID},
	})
	require.NoError(t, err)
	require.Len(t, enrollments, 1)
	require.NotNil(t, enrollments[0].Weekday)
	assert.Equal(t, timetable.WeekdayTuesday, *enrollments[0].Weekday, "the roster change is limited to Tuesday")
	assert.Equal(t, "2099-03-05", enrollments[0].ValidFrom)

	for instanceID, want := range map[int64]bool{tuesday.ID: true, earlierTuesday.ID: false, wednesday.ID: false} {
		roster, listErr := module.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{instanceID}})
		require.NoError(t, listErr)
		assert.Equal(t, want, containsStudent(roster, child.ID), "instance %d", instanceID)
	}

	// The template roster now covers the new pickup time.
	record("14:45", "16:00")
	tasks, err = module.ListOpenPickupExtensions(ctx, child.ID)
	require.NoError(t, err)
	assert.Empty(t, tasks)

	require.NoError(t, module.ClearPickupWeekdayExtension(ctx, child.ID, timetable.WeekdayTuesday))
	assert.Zero(t, countPickupExtensionRows(t, db, child.ID))
}

func TestModuleIgnoresFuturePickupWeekdayEnrollment(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildPickupExtensionModule(t, db), testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	category := createCategory(t, ctx, module, fmt.Sprintf("Future pickup enrollment %d", suffix))
	child := testpkg.CreateTestStudent(t, db, "Emil", fmt.Sprintf("Future pickup-%d", suffix), "1a")
	other := testpkg.CreateTestStudent(t, db, "Ida", fmt.Sprintf("Future pickup-%d", suffix), "1a")
	freePlay := pickupExtensionTemplate(t, module, ctx, category.ID, "Freies Spiel", "14:45:00", "16:00:00", timetable.WeekdayTuesday)

	_, err := module.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: other.ID, ActivityGroupID: freePlay.ID, ValidFrom: "2020-01-01",
	})
	require.NoError(t, err)
	weekday := timetable.WeekdayTuesday
	futureEnrollment, err := module.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: child.ID, ActivityGroupID: freePlay.ID, ValidFrom: "2100-01-01", Weekday: &weekday,
	})
	require.NoError(t, err)

	// The child's enrollment starts after the changed pickup time, so it must
	// not make the child appear to be on the otherwise suitable template.
	require.NoError(t, module.RecordPickupWeekdayExtension(ctx, timetable.PickupWeekdayExtension{
		StudentID: child.ID, Weekday: timetable.WeekdayTuesday, EffectiveFrom: "2099-03-05",
		PreviousPickup: "14:45", Pickup: "16:00",
	}))
	task := pickupExtensionTask(t, module, ctx, child.ID)
	assert.Equal(t, []int64{freePlay.ID}, pickupExtensionBlockIDs(task.Blocks))
	_, err = module.ResolvePickupExtension(ctx, task.ID, []int64{freePlay.ID})
	require.NoError(t, err)
	enrollments, err := module.ListStudentEnrollments(ctx, timetable.StudentEnrollmentFilter{
		StudentIDs: []int64{child.ID}, ActivityGroupIDs: []int64{freePlay.ID},
	})
	require.NoError(t, err)
	require.Len(t, enrollments, 1)
	assert.Equal(t, futureEnrollment.ID, enrollments[0].ID)
	assert.Equal(t, "2099-03-05", enrollments[0].ValidFrom)
}

func TestModulePreservesBoundedPickupWeekdayEnrollment(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildPickupExtensionModule(t, db), testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	category := createCategory(t, ctx, module, fmt.Sprintf("Bounded pickup enrollment %d", suffix))
	child := testpkg.CreateTestStudent(t, db, "Emil", fmt.Sprintf("Bounded pickup-%d", suffix), "1a")
	other := testpkg.CreateTestStudent(t, db, "Ida", fmt.Sprintf("Bounded pickup-%d", suffix), "1a")
	freePlay := pickupExtensionTemplate(t, module, ctx, category.ID, "Freies Spiel", "14:45:00", "16:00:00", timetable.WeekdayTuesday)

	_, err := module.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: other.ID, ActivityGroupID: freePlay.ID, ValidFrom: "2020-01-01",
	})
	require.NoError(t, err)
	wednesday := timetable.WeekdayWednesday
	tuesday := timetable.WeekdayTuesday
	validUntil := "2099-04-01"
	existing, err := module.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: child.ID, ActivityGroupID: freePlay.ID, ValidFrom: "2099-03-01", ValidUntil: &validUntil,
		SelectedWeekdays: []int{wednesday}, Weekday: &tuesday,
	})
	require.NoError(t, err)

	require.NoError(t, module.RecordPickupWeekdayExtension(ctx, timetable.PickupWeekdayExtension{
		StudentID: child.ID, Weekday: timetable.WeekdayTuesday, EffectiveFrom: "2099-03-05",
		PreviousPickup: "14:45", Pickup: "16:00",
	}))
	task := pickupExtensionTask(t, module, ctx, child.ID)
	_, err = module.ResolvePickupExtension(ctx, task.ID, []int64{freePlay.ID})
	require.NoError(t, err)

	enrollments, err := module.ListStudentEnrollments(ctx, timetable.StudentEnrollmentFilter{
		StudentIDs: []int64{child.ID}, ActivityGroupIDs: []int64{freePlay.ID},
	})
	require.NoError(t, err)
	require.Len(t, enrollments, 1)
	assert.Equal(t, existing.ID, enrollments[0].ID)
	assert.Equal(t, &validUntil, enrollments[0].ValidUntil)
	assert.ElementsMatch(t, []int{timetable.WeekdayTuesday, timetable.WeekdayWednesday}, enrollments[0].SelectedWeekdays)
}

func TestModuleReturnsPickupWeekdayTaskWithoutMatchingBlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildPickupExtensionModule(t, db), testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	category := createCategory(t, ctx, module, fmt.Sprintf("Pickup weekday roster %d", suffix))
	child := testpkg.CreateTestStudent(t, db, "Lina", fmt.Sprintf("Pickup weekday-%d", suffix), "1a")
	other := testpkg.CreateTestStudent(t, db, "Noah", fmt.Sprintf("Pickup weekday-%d", suffix), "1a")
	weekdayOnly := pickupExtensionTemplate(t, module, ctx, category.ID, "Mittwoch", "14:45:00", "16:00:00", timetable.WeekdayTuesday)
	selectedOnly := pickupExtensionTemplate(t, module, ctx, category.ID, "Auswahl Mittwoch", "14:45:00", "16:00:00", timetable.WeekdayTuesday)
	wednesday := timetable.WeekdayWednesday
	for _, input := range []timetable.StudentEnrollmentInput{
		{StudentID: other.ID, ActivityGroupID: weekdayOnly.ID, ValidFrom: "2020-01-01", Weekday: &wednesday},
		{StudentID: other.ID, ActivityGroupID: selectedOnly.ID, ValidFrom: "2020-01-01", SelectedWeekdays: []int{wednesday}},
	} {
		_, err := module.CreateStudentEnrollment(ctx, input)
		require.NoError(t, err)
	}

	require.NoError(t, module.RecordPickupWeekdayExtension(ctx, timetable.PickupWeekdayExtension{
		StudentID: child.ID, Weekday: timetable.WeekdayTuesday, EffectiveFrom: "2099-03-05",
		PreviousPickup: "14:45", Pickup: "16:00",
	}))
	tasks, err := module.ListOpenPickupExtensions(ctx, child.ID)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Empty(t, tasks[0].Blocks, "a roster restricted to Wednesday offers no Tuesday block")
	_, err = module.ResolvePickupExtension(ctx, tasks[0].ID, nil)
	require.NoError(t, err)
	tasks, err = module.ListOpenPickupExtensions(ctx, child.ID)
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestModuleUsesEffectivePickupWeekdaySchedules(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildPickupExtensionModule(t, db), testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	category := createCategory(t, ctx, module, fmt.Sprintf("Pickup schedule versions %d", suffix))
	child := testpkg.CreateTestStudent(t, db, "Lina", fmt.Sprintf("Schedule-%d", suffix), "1a")
	other := testpkg.CreateTestStudent(t, db, "Noah", fmt.Sprintf("Schedule-%d", suffix), "1a")

	createTemplate := func(name string) timetable.Group {
		t.Helper()
		group, err := module.CreateGroup(ctx, timetable.GroupInput{
			Name: fmt.Sprintf("%s %d", name, time.Now().UnixNano()), CategoryID: category.ID,
			Type: timetable.GroupTypeCare, IsTemplate: true,
		})
		require.NoError(t, err)
		return group
	}
	current := createTemplate("Freies Spiel")
	future := createTemplate("Späteres Angebot")
	oldFrame := createOwnedTimeframe(t, module, ctx, "14:45:00", "15:30:00", true, "Alte Zeit")
	currentFrame := createOwnedTimeframe(t, module, ctx, "14:45:00", "17:00:00", true, "Neue Zeit")
	futureFrame := createOwnedTimeframe(t, module, ctx, "14:45:00", "18:00:00", true, "Zukünftige Zeit")
	oldFrom, currentFrom, futureFrom := "2020-01-01", "2020-02-01", time.Now().AddDate(0, 0, 7).Format(time.DateOnly)
	for _, input := range []timetable.ScheduleInput{
		{ActivityGroupID: current.ID, Weekday: timetable.WeekdayTuesday, TimeframeID: &oldFrame.ID, ValidFrom: &oldFrom},
		{ActivityGroupID: current.ID, Weekday: timetable.WeekdayTuesday, TimeframeID: &currentFrame.ID, ValidFrom: &currentFrom},
		{ActivityGroupID: future.ID, Weekday: timetable.WeekdayTuesday, TimeframeID: &futureFrame.ID, ValidFrom: &futureFrom},
	} {
		_, err := module.CreateSchedule(ctx, input)
		require.NoError(t, err)
	}
	for _, groupID := range []int64{current.ID, future.ID} {
		_, err := module.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
			StudentID: other.ID, ActivityGroupID: groupID, ValidFrom: "2020-01-01",
		})
		require.NoError(t, err)
	}

	require.NoError(t, module.RecordPickupWeekdayExtension(ctx, timetable.PickupWeekdayExtension{
		StudentID: child.ID, Weekday: timetable.WeekdayTuesday, EffectiveFrom: "2099-03-05",
		PreviousPickup: "14:45", Pickup: "16:00",
	}))
	task := pickupExtensionTask(t, module, ctx, child.ID)
	require.Len(t, task.Blocks, 2)
	assert.ElementsMatch(t, []int64{current.ID, future.ID}, pickupExtensionBlockIDs(task.Blocks))
}

func TestModuleUsesCurrentPickupWeekdaySchedules(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildPickupExtensionModule(t, db), testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	category := createCategory(t, ctx, module, fmt.Sprintf("Current pickup schedule %d", suffix))
	child := testpkg.CreateTestStudent(t, db, "Lina", fmt.Sprintf("Current schedule-%d", suffix), "1a")
	other := testpkg.CreateTestStudent(t, db, "Noah", fmt.Sprintf("Current schedule-%d", suffix), "1a")

	createTemplate := func(name string) timetable.Group {
		t.Helper()
		group, err := module.CreateGroup(ctx, timetable.GroupInput{
			Name: fmt.Sprintf("%s %d", name, time.Now().UnixNano()), CategoryID: category.ID,
			Type: timetable.GroupTypeCare, IsTemplate: true,
		})
		require.NoError(t, err)
		return group
	}
	oldTemplate := createTemplate("Altes Angebot")
	currentTemplate := createTemplate("Aktuelles Angebot")
	oldFrame := createOwnedTimeframe(t, module, ctx, "14:45:00", "16:30:00", true, "Alte Zeit")
	currentFrame := createOwnedTimeframe(t, module, ctx, "14:45:00", "17:00:00", true, "Neue Zeit")
	today := time.Now()
	effectiveFrom := today.AddDate(0, 0, -7).Format(time.DateOnly)
	validUntil := today.Format(time.DateOnly)
	currentFrom := today.Format(time.DateOnly)
	for _, input := range []timetable.ScheduleInput{
		{ActivityGroupID: oldTemplate.ID, Weekday: timetable.WeekdayTuesday, TimeframeID: &oldFrame.ID, ValidFrom: &effectiveFrom, ValidUntil: &validUntil},
		{ActivityGroupID: currentTemplate.ID, Weekday: timetable.WeekdayTuesday, TimeframeID: &currentFrame.ID, ValidFrom: &currentFrom},
	} {
		_, err := module.CreateSchedule(ctx, input)
		require.NoError(t, err)
	}
	for _, groupID := range []int64{oldTemplate.ID, currentTemplate.ID} {
		_, err := module.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
			StudentID: other.ID, ActivityGroupID: groupID, ValidFrom: "2020-01-01",
		})
		require.NoError(t, err)
	}

	require.NoError(t, module.RecordPickupWeekdayExtension(ctx, timetable.PickupWeekdayExtension{
		StudentID: child.ID, Weekday: timetable.WeekdayTuesday, EffectiveFrom: effectiveFrom,
		PreviousPickup: "14:45", Pickup: "16:00",
	}))
	task := pickupExtensionTask(t, module, ctx, child.ID)
	require.Len(t, task.Blocks, 1)
	assert.Equal(t, currentTemplate.ID, task.Blocks[0].ID)
}

func TestModuleLimitsPickupWeekdayResolutionToCalendarPeriod(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildPickupExtensionModule(t, db), testpkg.Ctx(t)
	suffix := time.Now().UnixNano()
	category := createCategory(t, ctx, module, fmt.Sprintf("Pickup periods %d", suffix))
	child := testpkg.CreateTestStudent(t, db, "Mia", fmt.Sprintf("Pickup period-%d", suffix), "1a")
	other := testpkg.CreateTestStudent(t, db, "Noah", fmt.Sprintf("Pickup period-%d", suffix), "1a")
	group, err := module.CreateGroup(ctx, timetable.GroupInput{
		Name: fmt.Sprintf("Freies Spiel %d", suffix), CategoryID: category.ID, Type: timetable.GroupTypeCare, IsTemplate: true,
	})
	require.NoError(t, err)
	frame := createOwnedTimeframe(t, module, ctx, "14:45:00", "16:00:00", true, "Freies Spiel")
	today := time.Now()
	selectedPeriod := createOwnerCalendarPeriod(t, db, today.Format(time.DateOnly), today.AddDate(0, 0, 30).Format(time.DateOnly))
	otherPeriod := createOwnerCalendarPeriod(t, db, today.Format(time.DateOnly), today.AddDate(0, 0, 30).Format(time.DateOnly))
	_, err = module.CreateSchedule(ctx, timetable.ScheduleInput{
		ActivityGroupID: group.ID, Weekday: timetable.WeekdayTuesday, TimeframeID: &frame.ID, CalendarPeriodID: &selectedPeriod,
	})
	require.NoError(t, err)
	_, err = module.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: other.ID, ActivityGroupID: group.ID, ValidFrom: "2020-01-01",
	})
	require.NoError(t, err)
	nextTuesday := nextPickupExtensionWeekday(today, time.Tuesday)
	fixture := newOwnedActivityInstanceFixture(t, db, "pickup-period")
	selectedInput := ownedActivityInstanceInput(fixture, nextTuesday.Format(time.DateOnly), "14:45:00", "Ausgewählter Zeitraum")
	selectedInput.ActivityGroupID = &group.ID
	selectedInput.CalendarPeriodID = &selectedPeriod
	selected, err := module.CreateActivityInstance(ctx, selectedInput)
	require.NoError(t, err)
	otherInput := ownedActivityInstanceInput(fixture, nextTuesday.AddDate(0, 0, 7).Format(time.DateOnly), "14:45:00", "Anderer Zeitraum")
	otherInput.ActivityGroupID = &group.ID
	otherInput.CalendarPeriodID = &otherPeriod
	otherInstance, err := module.CreateActivityInstance(ctx, otherInput)
	require.NoError(t, err)

	require.NoError(t, module.RecordPickupWeekdayExtension(ctx, timetable.PickupWeekdayExtension{
		StudentID: child.ID, Weekday: timetable.WeekdayTuesday, EffectiveFrom: today.Format(time.DateOnly),
		PreviousPickup: "14:45", Pickup: "16:00",
	}))
	task := pickupExtensionTask(t, module, ctx, child.ID)
	_, err = module.ResolvePickupExtension(ctx, task.ID, []int64{group.ID})
	require.NoError(t, err)

	selectedStudents, err := module.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{selected.ID}})
	require.NoError(t, err)
	assert.True(t, containsStudent(selectedStudents, child.ID))
	otherStudents, err := module.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{otherInstance.ID}})
	require.NoError(t, err)
	assert.False(t, containsStudent(otherStudents, child.ID))
}

func nextPickupExtensionWeekday(from time.Time, weekday time.Weekday) time.Time {
	days := (int(weekday) - int(from.Weekday()) + 7) % 7
	if days == 0 {
		days = 7
	}
	return from.AddDate(0, 0, days)
}

func TestModuleMatchesPickupTargetsAtEffectiveDate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	suffix := time.Now().UnixNano()
	child := testpkg.CreateTestStudent(t, db, "Mia", fmt.Sprintf("Target-%d", suffix), "1a")
	students := StudentDirectoryFunc(func(context.Context) ([]TargetStudent, error) {
		return []TargetStudent{{
			ID: child.ID, SchoolClass: child.SchoolClass, EnrolledUntil: "2099-03-04",
		}}, nil
	})
	module, ctx := buildPickupExtensionModuleWithStudents(t, db, students), testpkg.Ctx(t)
	category := createCategory(t, ctx, module, fmt.Sprintf("Pickup target date %d", suffix))
	class := "1a"
	group, err := module.CreateGroup(ctx, timetable.GroupInput{
		Name: fmt.Sprintf("Klassenangebot %d", suffix), CategoryID: category.ID,
		Type: timetable.GroupTypeCare, IsTemplate: true,
		TargetGroupType: timetable.TargetGroupTypeSchoolClass, TargetSchoolClass: &class,
	})
	require.NoError(t, err)
	require.NoError(t, module.ReplaceGroupTargets(ctx, group.ID, []timetable.GroupTargetInput{{
		TargetGroupType: timetable.TargetGroupTypeSchoolClass, TargetSchoolClass: &class,
	}}))
	frame := createOwnedTimeframe(t, module, ctx, "14:45:00", "16:00:00", true, "Klassenangebot")
	_, err = module.CreateSchedule(ctx, timetable.ScheduleInput{
		ActivityGroupID: group.ID, Weekday: timetable.WeekdayTuesday, TimeframeID: &frame.ID,
	})
	require.NoError(t, err)

	require.NoError(t, module.RecordPickupWeekdayExtension(ctx, timetable.PickupWeekdayExtension{
		StudentID: child.ID, Weekday: timetable.WeekdayTuesday, EffectiveFrom: "2099-03-05",
		PreviousPickup: "14:45", Pickup: "16:00",
	}))
	task := pickupExtensionTask(t, module, ctx, child.ID)
	assert.Equal(t, []int64{group.ID}, pickupExtensionBlockIDs(task.Blocks))
}

func TestModuleRejectsInvalidPickupExtensions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildPickupExtensionModule(t, db), testpkg.Ctx(t)
	cases := map[string]error{
		"day without exception": module.RecordPickupDayExtension(ctx, timetable.PickupDayExtension{StudentID: 9, Date: "2099-03-03", PreviousPickup: "14:00", Pickup: "15:00"}),
		"day with bad date":     module.RecordPickupDayExtension(ctx, timetable.PickupDayExtension{StudentID: 9, PickupExceptionID: 9, Date: "03.03.2099", PreviousPickup: "14:00", Pickup: "15:00"}),
		"weekday on saturday":   module.RecordPickupWeekdayExtension(ctx, timetable.PickupWeekdayExtension{StudentID: 9, Weekday: 6, EffectiveFrom: "2099-03-03", PreviousPickup: "14:00", Pickup: "15:00"}),
		"weekday bad clock":     module.RecordPickupWeekdayExtension(ctx, timetable.PickupWeekdayExtension{StudentID: 9, Weekday: 2, EffectiveFrom: "2099-03-03", PreviousPickup: "14:00", Pickup: "25:00"}),
		"negative student":      func() error { _, err := module.ListOpenPickupExtensions(ctx, -1); return err }(),
		"non-positive block":    func() error { _, err := module.ResolvePickupExtension(ctx, 9, []int64{0}); return err }(),
	}
	for name, err := range cases {
		assert.ErrorIs(t, err, timetable.ErrInvalidPickupExtension, name)
	}
	assert.Equal(t, "invalid_pickup_extension", timetable.ErrorCode(timetable.ErrInvalidPickupExtension))
	assert.Equal(t, "pickup_extension_block_gone", timetable.ErrorCode(timetable.ErrPickupExtensionBlockGone))
}

func pickupExtensionTemplate(t *testing.T, module *timetable.Module, ctx context.Context, categoryID int64, name, start, end string, weekday int) timetable.Group {
	t.Helper()
	group, err := module.CreateGroup(ctx, timetable.GroupInput{
		Name: fmt.Sprintf("%s %d", name, time.Now().UnixNano()), CategoryID: categoryID,
		Type: timetable.GroupTypeCare, IsTemplate: true,
	})
	require.NoError(t, err)
	timeframe := createOwnedTimeframe(t, module, ctx, start, end, true, name)
	_, err = module.CreateSchedule(ctx, timetable.ScheduleInput{
		ActivityGroupID: group.ID, Weekday: weekday, TimeframeID: &timeframe.ID,
	})
	require.NoError(t, err)
	return group
}

func pickupExtensionTaskIDs(tasks []timetable.PickupExtensionTask) []int64 {
	result := make([]int64, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, task.ID)
	}
	return result
}

func instanceStudentStudentIDs(values []timetable.InstanceStudent) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		result = append(result, value.StudentID)
	}
	return result
}

func containsStudent(values []timetable.InstanceStudent, studentID int64) bool {
	for _, value := range values {
		if value.StudentID == studentID {
			return true
		}
	}
	return false
}

func countPickupExtensionRows(t *testing.T, db *bun.DB, studentID int64) int {
	t.Helper()
	count, err := db.NewSelect().TableExpr("schedule.pickup_extension_tasks").
		Where("tenant_id = ?", testpkg.Tenant(t)).Where("student_id = ?", studentID).
		Count(testpkg.WithPackageTenantRuntime(context.Background()))
	require.NoError(t, err)
	return count
}
