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

		// The UTC date is yesterday from the school's perspective, so it is
		// rejected as a past date rather than treated as another planning day.
		_, _, err = resolvePlanningDate("2026-06-01", lateNow)
		require.Error(t, err)
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

	t.Run("past_date_is_rejected", func(t *testing.T) {
		// Planning is future-only (#1939): a syntactically valid past date must
		// not slip past the boundary and yield a misleading historical plan.
		_, _, err := resolvePlanningDate("2026-05-31", now)
		require.Error(t, err)
	})

	t.Run("horizon_ends_with_current_calendar_week", func(t *testing.T) {
		// 2026-06-01 is a Monday, so the supported horizon runs through the
		// Sunday closing the CURRENT week: 2026-06-07 (see maxPlanningDate).
		// The next week is out: with the default Friday cadence it is only
		// materialized by the run later this week, so a timetable-only child
		// would answer "Kommt nicht" for it today.
		date, isToday, err := resolvePlanningDate("2026-06-07", now)
		require.NoError(t, err)
		assert.False(t, isToday)
		assert.Equal(t, timezone.NewDate(2026, time.June, 7), date)

		_, _, err = resolvePlanningDate("2026-06-08", now)
		require.Error(t, err)
	})

	t.Run("horizon_collapses_to_today_on_a_sunday", func(t *testing.T) {
		// A Sunday closes its own week, so today is the only supported day.
		// Tomorrow belongs to the next week, which a tenant materializing on
		// Mondays would not have created yet.
		sunday := time.Date(2026, time.June, 7, 10, 0, 0, 0, time.UTC)

		date, isToday, err := resolvePlanningDate("2026-06-07", sunday)
		require.NoError(t, err)
		assert.True(t, isToday)
		assert.Equal(t, timezone.NewDate(2026, time.June, 7), date)

		_, _, err = resolvePlanningDate("2026-06-08", sunday)
		require.Error(t, err)
	})
}

func TestMaxPlanningDate(t *testing.T) {
	// The horizon always closes with the Sunday of the CURRENT week — the only
	// week the scheduler guarantees is materialized at every moment.
	// Monday: this week's Sunday is 6 days out.
	assert.Equal(t, timezone.NewDate(2026, time.June, 7), maxPlanningDate(timezone.NewDate(2026, time.June, 1)))
	// Sunday: the current week ends today, so the horizon collapses to today.
	assert.Equal(t, timezone.NewDate(2026, time.June, 7), maxPlanningDate(timezone.NewDate(2026, time.June, 7)))
	// Saturday: one day left in the current week.
	assert.Equal(t, timezone.NewDate(2026, time.June, 7), maxPlanningDate(timezone.NewDate(2026, time.June, 6)))
}

func TestResetLiveLocationFields(t *testing.T) {
	since := time.Date(2026, time.June, 1, 8, 0, 0, 0, time.UTC)
	color := "#83CD2D"
	responses := []StudentResponse{
		{ID: 90, Location: "Gruppenraum", LocationSince: &since, RoomColor: &color},
		{ID: 91},
	}

	resetLiveLocationFields(responses)

	for _, response := range responses {
		assert.Empty(t, response.Location)
		assert.Nil(t, response.LocationSince)
		assert.Nil(t, response.RoomColor)
	}
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

func TestExportFilterLabelsForDateWordsAbsenceStatusAsPlanned(t *testing.T) {
	planningDate := timezone.NewDate(2026, time.June, 2)

	labels := exportFilterLabelsForDate(studentExportFilters{Status: "krank"}, planningDate, false)
	assert.Contains(t, labels, "Geplanter Status: Krank")

	todayLabels := exportFilterLabelsForDate(studentExportFilters{Status: "krank"}, planningDate, true)
	assert.Contains(t, todayLabels, "Momentaufnahme: Krank")
}

func TestActiveLiveListFilters(t *testing.T) {
	t.Run("none_when_no_live_filter_is_set", func(t *testing.T) {
		assert.Empty(t, activeLiveListFilters(&studentListParams{}))
		// "Unknown" is the location filter's all-values sentinel.
		assert.Empty(t, activeLiveListFilters(&studentListParams{location: "Unknown"}))
	})

	t.Run("names_every_live_filter_in_use", func(t *testing.T) {
		params := &studentListParams{roomID: 7, locationState: "present", location: "Schulhof"}
		assert.Equal(t, []string{"room_id", "location_state", "location"}, activeLiveListFilters(params))
	})
}

func TestActiveLiveExportFilters(t *testing.T) {
	t.Run("none_when_no_live_filter_is_set", func(t *testing.T) {
		assert.Empty(t, activeLiveExportFilters(studentExportFilters{}))
		assert.Empty(t, activeLiveExportFilters(studentExportFilters{Status: "all"}))
	})

	t.Run("names_every_live_filter_in_use", func(t *testing.T) {
		filters := studentExportFilters{RoomID: "7", Status: "anwesend"}
		assert.Equal(t, []string{"room_id", "status"}, activeLiveExportFilters(filters))
	})

	t.Run("names_every_location_derived_status", func(t *testing.T) {
		for _, status := range []string{"abwesend", "unterwegs", "schulhof", "anwesend"} {
			assert.Equal(t, []string{"status"}, activeLiveExportFilters(studentExportFilters{Status: status}),
				"%s is read off the live location snapshot", status)
		}
	})

	t.Run("absence_statuses_are_not_live", func(t *testing.T) {
		// These come from the status days of the requested date, not from the
		// location snapshot — planning them for another day is the point (#1939).
		for _, status := range []string{"krank", "klassenfahrt", "entschuldigt"} {
			assert.Empty(t, activeLiveExportFilters(studentExportFilters{Status: status}), status)
		}
	})
}

func TestLiveFilterError(t *testing.T) {
	tomorrow := timezone.NewDate(2026, time.June, 2)

	t.Run("today_accepts_live_filters", func(t *testing.T) {
		require.NoError(t, liveFilterError([]string{"room_id"}, timezone.NewDate(2026, time.June, 1), true))
	})

	t.Run("other_day_without_live_filters_is_fine", func(t *testing.T) {
		require.NoError(t, liveFilterError(nil, tomorrow, false))
	})

	t.Run("other_day_rejects_live_filters_naming_them", func(t *testing.T) {
		err := liveFilterError([]string{"room_id", "status"}, tomorrow, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "room_id, status")
		assert.Contains(t, err.Error(), "2026-06-02")
	})
}

func TestResolveExportPlanningDate(t *testing.T) {
	// Monday 2026-06-01.
	now := time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)

	t.Run("today_keeps_live_filters", func(t *testing.T) {
		date, isToday, errResp := resolveExportPlanningDate(studentExportFilters{Status: "anwesend", RoomID: "7"}, now)
		require.Nil(t, errResp)
		assert.True(t, isToday)
		assert.Equal(t, timezone.NewDate(2026, time.June, 1), date)
	})

	t.Run("planning_date_without_live_filters_resolves", func(t *testing.T) {
		date, isToday, errResp := resolveExportPlanningDate(studentExportFilters{Date: "2026-06-02", Status: "all"}, now)
		require.Nil(t, errResp)
		assert.False(t, isToday)
		assert.Equal(t, timezone.NewDate(2026, time.June, 2), date)
	})

	t.Run("planning_date_rejects_live_filters", func(t *testing.T) {
		_, _, errResp := resolveExportPlanningDate(studentExportFilters{Date: "2026-06-02", Status: "anwesend"}, now)
		assert.NotNil(t, errResp, "a live status filter cannot be answered for another day")

		_, _, errResp = resolveExportPlanningDate(studentExportFilters{Date: "2026-06-02", RoomID: "7"}, now)
		assert.NotNil(t, errResp, "a room filter reads today's active visits")
	})

	t.Run("planning_date_keeps_absence_statuses", func(t *testing.T) {
		for _, status := range []string{"krank", "klassenfahrt", "entschuldigt"} {
			date, isToday, errResp := resolveExportPlanningDate(studentExportFilters{Date: "2026-06-02", Status: status}, now)
			require.Nil(t, errResp, "%s is planned per date, not a live snapshot", status)
			assert.False(t, isToday)
			assert.Equal(t, timezone.NewDate(2026, time.June, 2), date)
		}
	})

	t.Run("malformed_date_is_rejected", func(t *testing.T) {
		_, _, errResp := resolveExportPlanningDate(studentExportFilters{Date: "02.06.2026"}, now)
		assert.NotNil(t, errResp)
	})
}
