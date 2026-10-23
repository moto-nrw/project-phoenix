package compose

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type recordedOrderChange struct {
	StudentID int64  `bun:"student_id"`
	Delta     int    `bun:"delta"`
	ChangedBy *int64 `bun:"changed_by_account_id"`
}

// orderChangesOf reads the recorded changes of a school directly, without the
// order view, which records changes itself.
func orderChangesOf(t *testing.T, db *bun.DB, tenantID int64) []recordedOrderChange {
	t.Helper()
	var changes []recordedOrderChange
	require.NoError(t, db.NewRaw(`SELECT student_id, delta, changed_by_account_id FROM schedule.meal_order_changes WHERE tenant_id = ? ORDER BY sequence`, tenantID).Scan(context.Background(), &changes))
	return changes
}

func orderCountOf(t *testing.T, db *bun.DB, tenantID int64) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM schedule.meal_order_days WHERE tenant_id = ?`, tenantID).Scan(context.Background(), &count))
	return count
}

// #3873: a change after the deadline is recorded by the write itself, with
// its author, even if nobody ever reads the orders that day.
func TestLateChangesAreRecordedAtTheWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Ida", "Sofort", "2a")
	parent := testpkg.CreateTestAccount(t, db, "meal-write-parent")
	staff := testpkg.CreateTestAccount(t, db, "meal-write-staff")
	ctx := testpkg.Ctx(t)
	require.NoError(t, buildModuleAt(t, db, true, nil, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin)).
		SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: parent.ID, Date: "2026-09-07", Participating: true}))

	late := buildModuleAt(t, db, true, nil, time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin))
	require.NoError(t, late.SetParticipationBySchool(ctx, mealplan.SchoolChange{StudentID: student.ID, StaffAccountID: staff.ID, Date: "2026-09-07", Participating: false}))

	changes := orderChangesOf(t, db, student.TenantID)
	require.Len(t, changes, 1)
	assert.Equal(t, -1, changes[0].Delta)
	require.NotNil(t, changes[0].ChangedBy)
	assert.Equal(t, staff.ID, *changes[0].ChangedBy)

	require.NoError(t, late.ClearParticipationBySchool(ctx, mealplan.SchoolChange{StudentID: student.ID, StaffAccountID: staff.ID, Date: "2026-09-07"}))
	assert.Len(t, orderChangesOf(t, db, student.TenantID), 1, "back to the regular days (none) changes nothing more")
}

// The worker fixes due orders and records what changed without an author:
// here a sick note after the deadline under "sick cancels".
func TestSettleOrdersFixesDueOrdersAndRecordsAbsences(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	settings := singleDeadlineSettings("09:00")
	settings.SickAfterDeadline = "cancelled"
	student := testpkg.CreateTestStudent(t, db, "Kai", "Arbeiter", "3c")
	parent := testpkg.CreateTestAccount(t, db, "meal-settle-parent")
	ctx := testpkg.Ctx(t)
	require.NoError(t, buildModuleWithRules(t, db, settings, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin)).
		SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: parent.ID, Date: "2026-09-08", Participating: true}))

	require.NoError(t, buildModuleWithRules(t, db, settings, time.Date(2026, 9, 7, 20, 0, 0, 0, testBerlin)).SettleOrders(ctx))
	assert.Equal(t, 1, orderCountOf(t, db, student.TenantID), "only Monday is due")

	worker := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 8, 10, 0, 0, 0, testBerlin))
	require.NoError(t, worker.SettleOrders(ctx))
	assert.Equal(t, 2, orderCountOf(t, db, student.TenantID), "Tuesday is fixed at its deadline")
	createSickStatusDay(t, db, student.TenantID, student.ID, "2026-09-08", time.Date(2026, 9, 8, 10, 30, 0, 0, testBerlin))
	require.NoError(t, buildModuleWithRules(t, db, settings, time.Date(2026, 9, 8, 10, 35, 0, 0, testBerlin)).SettleOrders(ctx))
	require.NoError(t, buildModuleWithRules(t, db, settings, time.Date(2026, 9, 8, 10, 40, 0, 0, testBerlin)).SettleOrders(ctx))

	changes := orderChangesOf(t, db, student.TenantID)
	require.Len(t, changes, 1, "settling twice records nothing twice")
	assert.Equal(t, -1, changes[0].Delta)
	assert.Nil(t, changes[0].ChangedBy, "a sick note names no author of the change")
}

func TestSettleOrdersDoesNothingWithoutRegistration(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Ole", "Aus", "1a")
	require.NoError(t, buildModuleAt(t, db, false, nil, time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin)).SettleOrders(testpkg.Ctx(t)))
	assert.Zero(t, orderCountOf(t, db, student.TenantID))
}

// A day already over is never ordered after the fact, so opening an old week
// or exporting the month on the first day bills nothing.
func TestOrdersNeverOrderAPastDay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Pia", "Früher", "2b")
	parent := testpkg.CreateTestAccount(t, db, "meal-past-day")
	ctx := testpkg.Ctx(t)
	early := buildModuleAt(t, db, true, nil, time.Date(2026, 9, 1, 8, 0, 0, 0, testBerlin))
	_, err := early.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{StudentID: student.ID, GuardianAccountID: parent.ID, Weekdays: []mealplan.Weekday{mealplan.Monday, mealplan.Wednesday}})
	require.NoError(t, err)

	week, err := buildModuleAt(t, db, true, nil, time.Date(2026, 9, 9, 10, 0, 0, 0, testBerlin)).Orders(ctx, "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	monday := orderDay(t, week, "2026-09-07")
	assert.Equal(t, mealplan.OrderUnordered, monday.Status)
	assert.Zero(t, monday.Total)
	assert.Equal(t, mealplan.OrderOrdered, orderDay(t, week, "2026-09-09").Status, "today after its deadline is ordered")
	assert.Equal(t, 1, orderCountOf(t, db, student.TenantID))
	billed := week.MealsPerChild()
	require.Len(t, billed, 1)
	assert.Equal(t, student.ID, billed[0].Child.StudentID)
	assert.Equal(t, 1, billed[0].Meals, "only Wednesday is billed, not the unordered Monday")
}

// A school switching from daily to weekly orders keeps the days already
// ordered; the week order takes only the days still open.
func TestSwitchingToWeeklyOrdersKeepsTheOrderedDays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Rita", "Rhythmus", "4b")
	parent := testpkg.CreateTestAccount(t, db, "meal-rhythm")
	ctx := testpkg.Ctx(t)
	daily := singleDeadlineSettings("09:00")
	_, err := buildModuleWithRules(t, db, daily, time.Date(2026, 9, 1, 8, 0, 0, 0, testBerlin)).
		ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{StudentID: student.ID, GuardianAccountID: parent.ID, Weekdays: []mealplan.Weekday{mealplan.Monday, mealplan.Tuesday, mealplan.Friday}})
	require.NoError(t, err)
	require.NoError(t, buildModuleWithRules(t, db, daily, time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin)).SettleOrders(ctx))

	weekly := daily
	weekly.OrderRhythm = "weekly"
	week, err := buildModuleWithRules(t, db, weekly, time.Date(2026, 9, 8, 10, 0, 0, 0, testBerlin)).Orders(ctx, "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	monday := orderDay(t, week, "2026-09-07")
	tuesday := orderDay(t, week, "2026-09-08")
	assert.NotEqual(t, monday.OrderID, tuesday.OrderID, "Monday keeps its daily order")
	for _, date := range []mealplan.Date{"2026-09-09", "2026-09-10", "2026-09-11"} {
		assert.Equal(t, tuesday.OrderID, orderDay(t, week, date).OrderID, "the rest of the week is one order")
	}
	assert.Equal(t, 1, orderDay(t, week, "2026-09-11").Total)
}

// The school changes only a child it knows, and a day only while the child
// is in care.
func TestSchoolChangesNeedAKnownChildInCare(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Uwe", "Ausgetreten", "3d")
	staff := testpkg.CreateTestAccount(t, db, "meal-enrollment")
	ctx := testpkg.Ctx(t)
	module := buildModuleAt(t, db, true, nil, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin))

	err := module.SetParticipationBySchool(ctx, mealplan.SchoolChange{StudentID: student.ID + 1_000_000, StaffAccountID: staff.ID, Date: "2026-09-08", Participating: true})
	require.ErrorIs(t, err, mealplan.ErrStudentNotFound)
	_, err = module.ReplaceScheduleBySchool(ctx, mealplan.SchoolSchedule{StudentID: student.ID + 1_000_000, StaffAccountID: staff.ID, Weekdays: []mealplan.Weekday{mealplan.Monday}, EffectiveFrom: "2026-09-14"})
	require.ErrorIs(t, err, mealplan.ErrStudentNotFound)

	_, err = db.NewRaw(`UPDATE users.student_school_memberships SET enrolled_until = '2026-09-07' WHERE student_profile_id = ?`, student.ID).Exec(context.Background())
	require.NoError(t, err)
	err = module.SetParticipationBySchool(ctx, mealplan.SchoolChange{StudentID: student.ID, StaffAccountID: staff.ID, Date: "2026-09-08", Participating: true})
	require.ErrorIs(t, err, mealplan.ErrChildNotEnrolled)
	require.NoError(t, module.SetParticipationBySchool(ctx, mealplan.SchoolChange{StudentID: student.ID, StaffAccountID: staff.ID, Date: "2026-09-07", Participating: true}), "the last day in care still changes")
}

// The newest regular days hold from their day on: regular days set earlier
// for a later day give way.
func TestNewRegularDaysReplaceLaterOnes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Vera", "Neuplan", "1c")
	staff := testpkg.CreateTestAccount(t, db, "meal-supersede")
	ctx := testpkg.Ctx(t)
	module := buildModuleAt(t, db, true, nil, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin))
	_, err := module.ReplaceScheduleBySchool(ctx, mealplan.SchoolSchedule{StudentID: student.ID, StaffAccountID: staff.ID, Weekdays: []mealplan.Weekday{mealplan.Monday, mealplan.Tuesday}, EffectiveFrom: "2026-09-21"})
	require.NoError(t, err)
	_, err = module.ReplaceScheduleBySchool(ctx, mealplan.SchoolSchedule{StudentID: student.ID, StaffAccountID: staff.ID, Weekdays: []mealplan.Weekday{mealplan.Friday}, EffectiveFrom: "2026-09-14"})
	require.NoError(t, err)

	plan, err := module.Participation(ctx, student.ID, "2026-09-21", "2026-09-25")
	require.NoError(t, err)
	assert.False(t, plan.Days[0].Participating, "Monday from the older plan is gone")
	assert.True(t, plan.Days[4].Participating, "Friday holds on")
}

type loggedRegistrationChange struct {
	AccountID     int64  `bun:"account_id"`
	BySchool      bool   `bun:"by_school"`
	Kind          string `bun:"kind"`
	Date          string `bun:"date"`
	Participating *bool  `bun:"participating"`
}

// Every change of a registration lands in the log, by the school or a
// parent; a change that changes nothing does not.
func TestRegistrationChangesAreLogged(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Wanda", "Protokoll", "2d")
	parent := testpkg.CreateTestAccount(t, db, "meal-log-parent")
	staff := testpkg.CreateTestAccount(t, db, "meal-log-staff")
	ctx := testpkg.Ctx(t)
	module := buildModuleAt(t, db, true, nil, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin))

	_, err := module.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{StudentID: student.ID, GuardianAccountID: parent.ID, Weekdays: []mealplan.Weekday{mealplan.Monday}})
	require.NoError(t, err)
	_, err = module.SetParticipationForWeek(ctx, mealplan.SetParticipationWeek{StudentID: student.ID, GuardianAccountID: parent.ID, WeekOf: "2026-09-14", Participating: true})
	require.NoError(t, err)
	require.NoError(t, module.SetParticipationBySchool(ctx, mealplan.SchoolChange{StudentID: student.ID, StaffAccountID: staff.ID, Date: "2026-09-09", Participating: true}))
	require.NoError(t, module.ClearParticipationBySchool(ctx, mealplan.SchoolChange{StudentID: student.ID, StaffAccountID: staff.ID, Date: "2026-09-09"}))
	require.NoError(t, module.ClearParticipationBySchool(ctx, mealplan.SchoolChange{StudentID: student.ID, StaffAccountID: staff.ID, Date: "2026-09-10"}))

	var entries []loggedRegistrationChange
	require.NoError(t, db.NewRaw(`SELECT account_id, by_school, kind, date::text AS date, participating FROM schedule.meal_registration_changes WHERE tenant_id = ? AND student_id = ? ORDER BY id`, student.TenantID, student.ID).Scan(context.Background(), &entries))
	kinds := make([]string, 0, len(entries))
	for _, entry := range entries {
		kinds = append(kinds, entry.Kind)
	}
	// The week change registers the four days that are not regular yet.
	assert.Equal(t, []string{"schedule", "day", "day", "day", "day", "day", "regular"}, kinds, "clearing a day without an exception logs nothing")
	assert.Equal(t, parent.ID, entries[0].AccountID)
	assert.False(t, entries[0].BySchool)
	school := entries[5]
	assert.Equal(t, staff.ID, school.AccountID)
	assert.True(t, school.BySchool)
	assert.Equal(t, "2026-09-09", school.Date)
	require.NotNil(t, school.Participating)
	assert.True(t, *school.Participating)
	assert.Nil(t, entries[6].Participating)
}

// A write records only its own changes under its author: a sick note that
// arrived since the last settling stays without one, even for the same child.
func TestAWriteDoesNotClaimAnEarlierAbsence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	settings := singleDeadlineSettings("09:00")
	settings.SickAfterDeadline = "cancelled"
	student := testpkg.CreateTestStudent(t, db, "Nora", "Autorin", "2c")
	parent := testpkg.CreateTestAccount(t, db, "meal-author-parent")
	staff := testpkg.CreateTestAccount(t, db, "meal-author-staff")
	ctx := testpkg.Ctx(t)
	require.NoError(t, buildModuleWithRules(t, db, settings, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin)).
		SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: parent.ID, Date: "2026-09-08", Participating: true}))
	require.NoError(t, buildModuleWithRules(t, db, settings, time.Date(2026, 9, 8, 10, 0, 0, 0, testBerlin)).SettleOrders(ctx))
	createSickStatusDay(t, db, student.TenantID, student.ID, "2026-09-08", time.Date(2026, 9, 8, 10, 30, 0, 0, testBerlin))

	school := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 8, 10, 32, 0, 0, testBerlin))
	_, err := school.ReplaceScheduleBySchool(ctx, mealplan.SchoolSchedule{StudentID: student.ID, StaffAccountID: staff.ID, Weekdays: []mealplan.Weekday{mealplan.Friday}, EffectiveFrom: "2026-09-14"})
	require.NoError(t, err)

	changes := orderChangesOf(t, db, student.TenantID)
	require.Len(t, changes, 1)
	assert.Equal(t, -1, changes[0].Delta)
	assert.Nil(t, changes[0].ChangedBy, "the sick note is nobody's change of the order")
}
