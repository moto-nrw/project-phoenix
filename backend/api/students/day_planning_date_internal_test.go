package students

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

func TestResolvePlanningDate(t *testing.T) {
	now := time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)

	t.Run("empty_defaults_to_school_local_today", func(t *testing.T) {
		date, isToday, err := resolvePlanningDate("", now)
		require.NoError(t, err)
		assert.True(t, isToday)
		assert.Equal(t, timezone.NewDate(2026, time.June, 1), date)
	})

	t.Run("day_switches_at_berlin_midnight_not_utc", func(t *testing.T) {
		// 22:30 UTC on June 1 is already 00:30 on June 2 in Berlin (CEST).
		lateNow := time.Date(2026, time.June, 1, 22, 30, 0, 0, time.UTC)

		date, isToday, err := resolvePlanningDate("", lateNow)
		require.NoError(t, err)
		assert.True(t, isToday)
		assert.Equal(t, timezone.NewDate(2026, time.June, 2), date)

		// The UTC date is yesterday from the school's perspective.
		_, isToday, err = resolvePlanningDate("2026-06-01", lateNow)
		require.NoError(t, err)
		assert.False(t, isToday)
	})

	t.Run("explicit_today_is_today", func(t *testing.T) {
		date, isToday, err := resolvePlanningDate("2026-06-01", now)
		require.NoError(t, err)
		assert.True(t, isToday)
		assert.Equal(t, timezone.NewDate(2026, time.June, 1), date)
	})

	t.Run("tomorrow_is_not_today", func(t *testing.T) {
		date, isToday, err := resolvePlanningDate("2026-06-02", now)
		require.NoError(t, err)
		assert.False(t, isToday)
		assert.Equal(t, timezone.NewDate(2026, time.June, 2), date)
	})

	t.Run("invalid_value_is_an_error", func(t *testing.T) {
		_, _, err := resolvePlanningDate("02.06.2026", now)
		require.Error(t, err)
		_, _, err = resolvePlanningDate("junk", now)
		require.Error(t, err)
	})
}

func TestResolveDayPlanningForDateNonToday(t *testing.T) {
	checkInTime := time.Date(2026, time.June, 1, 8, 0, 0, 0, time.UTC)
	attendance := &activeService.AttendanceStatus{Status: "checked_in", CheckInTime: &checkInTime}
	arrivalTime := time.Date(2026, time.June, 2, 8, 0, 0, 0, time.UTC)

	t.Run("current_attendance_is_not_a_plan_for_another_day", func(t *testing.T) {
		status, reason, label := resolveDayPlanningForDate(
			StudentResponse{ID: 90}, nil, nil, attendance, map[int64]struct{}{}, false,
		)
		assert.Equal(t, DayPlanningStatusNotComingToday, status)
		assert.Equal(t, dayPlanningReasonNoPlan, reason)
		assert.Equal(t, "kein Plan für diesen Tag", label)
	})

	t.Run("explicit_absence_wins_over_planning_signal", func(t *testing.T) {
		status, reason, _ := resolveDayPlanningForDate(
			StudentResponse{ID: 90, Sick: true},
			&scheduleService.EffectiveArrivalTime{ArrivalTime: &arrivalTime},
			nil, nil, map[int64]struct{}{}, false,
		)
		assert.Equal(t, DayPlanningStatusNotComingToday, status)
		assert.Equal(t, dayPlanningReasonSick, reason)
	})

	t.Run("labels_avoid_heute_wording", func(t *testing.T) {
		status, reason, label := resolveDayPlanningForDate(
			StudentResponse{ID: 90},
			&scheduleService.EffectiveArrivalTime{ArrivalTime: &arrivalTime},
			nil, nil, map[int64]struct{}{}, false,
		)
		assert.Equal(t, DayPlanningStatusComesToday, status)
		assert.Equal(t, dayPlanningReasonArrivalSchedule, reason)
		assert.Equal(t, "Ankunftsplan", label)
		assert.NotContains(t, label, "heute")
	})

	t.Run("timetable_signal_counts_for_the_day", func(t *testing.T) {
		status, reason, label := resolveDayPlanningForDate(
			StudentResponse{ID: 90}, nil, nil, nil, map[int64]struct{}{90: {}}, false,
		)
		assert.Equal(t, DayPlanningStatusComesToday, status)
		assert.Equal(t, dayPlanningReasonTimetable, reason)
		assert.Equal(t, "Betreuungsplan", label)
	})
}

func TestResetScheduledStatusFlags(t *testing.T) {
	since := time.Date(2026, time.June, 1, 7, 0, 0, 0, time.UTC)
	responses := []StudentResponse{
		{ID: 90, Sick: true, SickSince: &since, Excused: true, ExcusedSince: &since, ClassTrip: true, ClassTripSince: &since},
		{ID: 91},
	}

	resetScheduledStatusFlags(responses)

	for _, response := range responses {
		assert.False(t, response.Sick)
		assert.Nil(t, response.SickSince)
		assert.False(t, response.Excused)
		assert.Nil(t, response.ExcusedSince)
		assert.False(t, response.ClassTrip)
		assert.Nil(t, response.ClassTripSince)
	}
}

func TestDailyStatusExportCellForDate(t *testing.T) {
	comes := StudentResponse{DayPlanningStatus: DayPlanningStatusComesToday}
	noPlan := StudentResponse{DayPlanningStatus: DayPlanningStatusNotComingToday}

	assert.Equal(t, "Kommt heute", dailyStatusExportCell(comes, true))
	assert.Equal(t, "Wird erwartet", dailyStatusExportCell(comes, false))
	assert.Equal(t, "Kommt heute nicht", dailyStatusExportCell(noPlan, true))
	assert.Equal(t, "Wird nicht erwartet", dailyStatusExportCell(noPlan, false))

	sick := StudentResponse{DayPlanningStatus: DayPlanningStatusNotComingToday, DayPlanningReason: dayPlanningReasonSick}
	assert.Equal(t, "Krank", dailyStatusExportCell(sick, false))
}

func TestExportFilterLabelsForDateAddsDateAndNeutralWording(t *testing.T) {
	planningDate := timezone.NewDate(2026, time.June, 2)

	labels := exportFilterLabelsForDate(studentExportFilters{DayStatus: DayPlanningStatusComesToday}, planningDate, false)

	assert.Contains(t, labels, "Datum: 02.06.2026")
	assert.Contains(t, labels, "Tagesplanung: Wird erwartet")
	for _, label := range labels {
		assert.NotContains(t, label, "heute")
	}
}
