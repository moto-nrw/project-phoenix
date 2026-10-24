package planning

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTargetOverrideDaysReader struct {
	days workforce.TargetOverrideDays
	err  error
}

func (f *fakeTargetOverrideDaysReader) StaffTargetOverrideDays(_ context.Context, _ []int64, _, _ string) (workforce.TargetOverrideDays, error) {
	return f.days, f.err
}

// A Sonderarbeitszeit corrects the Soll of a week the schedule or the model
// already priced. A week neither of them covers stays without a Soll: the
// overview must not present the override minutes as the whole week (#3259).
func TestAddOverrideDeltas_OnlyCorrectsResolvedWeeks(t *testing.T) {
	t.Parallel()

	member := &workforce.PlanStaff{}
	member.ID = 7
	resolved := calendar.NewDate(2026, time.October, 19)
	unresolved := calendar.NewDate(2026, time.October, 26)
	weekStarts := []calendar.Date{resolved, unresolved}
	days := map[string]int{
		resolved.String():              510,
		resolved.AddDays(1).String():   510,
		unresolved.String():            510,
		unresolved.AddDays(1).String(): 510,
	}
	base := func(_ *workforce.PlanStaff, _ calendar.Date) int { return 400 }

	targets := map[staffDateKey]int{{StaffID: member.ID, Date: resolved}: 2000}
	addOverrideDeltas(member, days, weekStarts, base, targets)

	assert.Equal(t, 2220, targets[staffDateKey{StaffID: member.ID, Date: resolved}])
	_, created := targets[staffDateKey{StaffID: member.ID, Date: unresolved}]
	assert.False(t, created, "a week without a resolved Soll must not gain one from an override")
}

// When outdated schedule rows leave a week to the work-time model, the
// override must replace the model's day, not add to it: both sides price the
// day the same way (#3259).
func TestWeeklyTargets_OverrideReplacesTheModelDayOfAnOutdatedSchedule(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.October, 19)
	modelID := int64(22)
	expired := calendar.NewDate(2026, time.January, 5)
	service := &staffScheduleOverviewService{deps: StaffScheduleOverviewDependencies{
		WorkModels: &fakeOverviewWorkModelReader{models: []workforce.WorkTimeModel{{
			ID:                 modelID,
			RotationLength:     1,
			RotationAnchorDate: monday.String(),
			Entries: []workforce.WorkTimeModelEntry{
				{WeekIndex: 0, DayOfWeek: dayMonday, TargetMinutes: 400},
				{WeekIndex: 0, DayOfWeek: dayTuesday, TargetMinutes: 400},
			},
		}}},
		TargetOverrides: &fakeTargetOverrideDaysReader{days: workforce.TargetOverrideDays{
			11: {monday.String(): 510},
		}},
	}}
	member := &workforce.PlanStaff{WorkTimeModelID: &modelID}
	member.ID = 11

	targets, err := service.weeklyTargets(
		context.Background(),
		[]*workforce.PlanStaff{member},
		[]*workforce.ScheduleRow{{
			StaffID: 11, WeekIndex: 0, RotationLength: 1,
			DayOfWeek: dayMonday, TargetMinutes: 120,
			ValidFrom: calendar.NewDate(2025, time.September, 1), ValidUntil: &expired,
		}},
		[]calendar.Date{monday},
	)
	require.NoError(t, err)

	assert.Equal(t, 910, targets[staffDateKey{StaffID: 11, Date: monday}],
		"the model week (800) keeps Tuesday and trades its Monday (400) for the override (510)")
}
