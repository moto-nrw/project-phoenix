package compose

import (
	"slices"
	"testing"
	"time"

	mealInputs "github.com/moto-nrw/project-phoenix/modules/mealplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func buildRulesModule(t *testing.T, db *bun.DB, settings testMealPlanSettings, calendar testCalendar, now time.Time) mealInputs.Capability {
	t.Helper()
	module, err := New(Dependencies{
		DB: db, Settings: settings, Calendar: calendar,
		Observe:      func(Observation) {},
		Now:          func() time.Time { return now },
		Participants: testParticipantFinder,
	})
	require.NoError(t, err)
	return module
}

func participantIDs(participants []mealInputs.DailyParticipant) []int64 {
	ids := make([]int64, 0, len(participants))
	for _, participant := range participants {
		ids = append(ids, participant.StudentID)
	}
	return ids
}

// #3164: a closing day is not a school day of the lead, and it has no meal.
func TestClosedDaysMoveTheLeadAndHaveNoMeal(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	settings := testMealPlanSettings{mealPlanEnabled: true, mealRegistrationEnabled: true, cutoff: "09:00", leadDays: 1}
	calendar := testCalendar{closed: map[string]bool{"2026-09-11": true}}
	student := testpkg.CreateTestStudent(t, db, "Frieda", "Feiertag", "2a")
	account := testpkg.CreateTestAccount(t, db, "meal-closed-day")
	ctx := testpkg.Ctx(t)

	early := buildRulesModule(t, db, settings, calendar, time.Date(2026, 9, 3, 8, 0, 0, 0, testBerlin))
	_, err := early.ReplaceParticipationSchedule(ctx, mealInputs.ReplaceParticipationSchedule{
		StudentID: student.ID, GuardianAccountID: account.ID,
		Weekdays: []mealInputs.Weekday{mealInputs.Monday, mealInputs.Tuesday, mealInputs.Wednesday, mealInputs.Thursday, mealInputs.Friday},
	})
	require.NoError(t, err)

	// Thursday 10:00: Monday's cutoff is Thursday 09:00, because the Friday
	// in between is closed.
	module := buildRulesModule(t, db, settings, calendar, time.Date(2026, 9, 10, 10, 0, 0, 0, testBerlin))
	err = module.SetParticipationForDay(ctx, mealInputs.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-14", Participating: false})
	require.ErrorIs(t, err, mealInputs.ErrParticipationCutoff)
	err = module.SetParticipationForDay(ctx, mealInputs.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-11", Participating: true})
	require.ErrorIs(t, err, mealInputs.ErrInvalidParticipation, "a closed day cannot be changed")

	plan, err := module.Participation(ctx, student.ID, "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	require.Len(t, plan.Days, 5)
	assert.Equal(t, mealInputs.ParticipationClosed, plan.Days[4].Source)
	assert.False(t, plan.Days[4].Participating)
	assert.False(t, plan.Days[4].Changeable)

	week, err := module.WeeklyList(ctx, "2026-09-07")
	require.NoError(t, err)
	assert.True(t, week.Days[4].Closed)
	assert.Empty(t, week.Days[4].Participants)
	assert.Contains(t, participantIDs(week.Days[3].Participants), student.ID)
}

// #3164: a fixed weekday closes the whole next week, and a later
// cancellation cutoff still lets parents take the child off a day.
func TestWeekdayCutoffWithLaterCancellation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	settings := testMealPlanSettings{
		mealPlanEnabled: true, mealRegistrationEnabled: true, cutoff: "16:00", weekday: 3,
		cancellationCutoff: "09:00", cancellationLeadDays: 1,
	}
	student := testpkg.CreateTestStudent(t, db, "Willi", "Woche", "3a")
	account := testpkg.CreateTestAccount(t, db, "meal-weekday-cutoff")
	ctx := testpkg.Ctx(t)

	wednesday := buildRulesModule(t, db, settings, testCalendar{}, time.Date(2026, 9, 9, 10, 0, 0, 0, testBerlin))
	effectiveFrom, err := wednesday.ReplaceParticipationSchedule(ctx, mealInputs.ReplaceParticipationSchedule{
		StudentID: student.ID, GuardianAccountID: account.ID,
		Weekdays: []mealInputs.Weekday{mealInputs.Monday, mealInputs.Tuesday, mealInputs.Wednesday, mealInputs.Thursday, mealInputs.Friday},
	})
	require.NoError(t, err)
	assert.Equal(t, mealInputs.Date("2026-09-14"), effectiveFrom, "next week is open until Wednesday 16:00")

	thursday := buildRulesModule(t, db, settings, testCalendar{}, time.Date(2026, 9, 10, 12, 0, 0, 0, testBerlin))
	effectiveFrom, err = thursday.ReplaceParticipationSchedule(ctx, mealInputs.ReplaceParticipationSchedule{
		StudentID: student.ID, GuardianAccountID: account.ID,
		Weekdays: []mealInputs.Weekday{mealInputs.Monday, mealInputs.Tuesday, mealInputs.Wednesday, mealInputs.Thursday, mealInputs.Friday},
	})
	require.NoError(t, err)
	assert.Equal(t, mealInputs.Date("2026-09-21"), effectiveFrom, "after Wednesday 16:00 the week after next is the first open one")

	require.NoError(t, thursday.SetParticipationForDay(ctx, mealInputs.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-15", Participating: false}),
		"Tuesday can be cancelled until Monday 09:00")
	err = thursday.SetParticipationForDay(ctx, mealInputs.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-15", Participating: true})
	require.ErrorIs(t, err, mealInputs.ErrParticipationCutoff, "signing up again needs the registration cutoff")
	err = thursday.ClearParticipationForDay(ctx, mealInputs.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-15"})
	require.ErrorIs(t, err, mealInputs.ErrParticipationCutoff, "returning to the fixed days signs the child up again")

	plan, err := thursday.Participation(ctx, student.ID, "2026-09-14", "2026-09-18")
	require.NoError(t, err)
	assert.Equal(t, mealInputs.Wednesday, plan.CutoffWeekday)
	assert.Equal(t, "09:00", plan.CancellationCutoffTime)
	assert.Equal(t, 1, plan.CancellationCutoffLeadDays)
	assert.False(t, plan.Days[1].Participating)
	assert.False(t, plan.Days[1].Changeable, "a cancelled day cannot be signed up again")
	assert.True(t, plan.Days[2].Participating)
	assert.True(t, plan.Days[2].Changeable, "a participating day can still be cancelled")

	// Wednesday's cancellation cutoff is Tuesday 09:00: a sick note before it
	// takes the child off the kitchen list, one after it does not.
	createSickStatusDay(t, db, student.TenantID, student.ID, "2026-09-16", time.Date(2026, 9, 14, 20, 0, 0, 0, testBerlin))
	createSickStatusDay(t, db, student.TenantID, student.ID, "2026-09-17", time.Date(2026, 9, 16, 10, 0, 0, 0, testBerlin))
	week, err := thursday.WeeklyList(ctx, "2026-09-14")
	require.NoError(t, err)
	listed := make([]bool, 0, len(week.Days))
	for _, day := range week.Days {
		listed = append(listed, slices.Contains(participantIDs(day.Participants), student.ID))
	}
	assert.Equal(t, []bool{true, false, false, true, true}, listed)
}

// #3164: the team can change a child's day after the cutoff, for example
// after a phone call, but not a past or closed day or a child who is not on
// the day's list.
func TestStaffChangesIgnoreTheCutoff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	settings := testMealPlanSettings{mealPlanEnabled: true, mealRegistrationEnabled: true, cutoff: "09:00"}
	module := buildRulesModule(t, db, settings, testCalendar{closed: map[string]bool{"2026-09-08": true}}, time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin))
	student := testpkg.CreateTestStudent(t, db, "Paula", "Telefon", "1b")
	staff := testpkg.CreateTestAccount(t, db, "meal-staff-change")
	ctx := testpkg.Ctx(t)

	before, err := module.DailyList(ctx, "2026-09-07")
	require.NoError(t, err)
	assert.Contains(t, participantIDs(before.Available), student.ID)
	assert.NotContains(t, participantIDs(before.Participants), student.ID)

	participating := true
	require.NoError(t, module.ChangeParticipationAsStaff(ctx, mealInputs.StaffParticipationChange{StudentID: student.ID, AccountID: staff.ID, Date: "2026-09-07", Participating: &participating}))
	after, err := module.DailyList(ctx, "2026-09-07")
	require.NoError(t, err)
	assert.Contains(t, participantIDs(after.Participants), student.ID)
	assert.NotContains(t, participantIDs(after.Available), student.ID)

	require.NoError(t, module.ChangeParticipationAsStaff(ctx, mealInputs.StaffParticipationChange{StudentID: student.ID, AccountID: staff.ID, Date: "2026-09-07"}))
	reset, err := module.DailyList(ctx, "2026-09-07")
	require.NoError(t, err)
	assert.NotContains(t, participantIDs(reset.Participants), student.ID)

	for name, change := range map[string]mealInputs.StaffParticipationChange{
		"past day":       {StudentID: student.ID, AccountID: staff.ID, Date: "2026-09-04", Participating: &participating},
		"closed day":     {StudentID: student.ID, AccountID: staff.ID, Date: "2026-09-08", Participating: &participating},
		"weekend":        {StudentID: student.ID, AccountID: staff.ID, Date: "2026-09-12", Participating: &participating},
		"unknown child":  {StudentID: student.ID + 1_000_000, AccountID: staff.ID, Date: "2026-09-07", Participating: &participating},
		"missing caller": {StudentID: student.ID, Date: "2026-09-07", Participating: &participating},
	} {
		err := module.ChangeParticipationAsStaff(ctx, change)
		require.ErrorIs(t, err, mealInputs.ErrInvalidParticipation, name)
	}
}
