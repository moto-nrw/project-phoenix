package application

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/require"
)

func TestReviewWeeklyPlanKeepsStoredDaysAndManualTimePrecedence(t *testing.T) {
	t.Parallel()
	clock := func(value string) time.Time {
		parsed, err := time.Parse("15:04", value)
		require.NoError(t, err)
		return parsed
	}
	facts := reviewPlanFacts{
		arrivals:   []careplan.ArrivalSchedule{{Weekday: 1}, {Weekday: 2, ExpectedArrival: clock("11:00")}},
		classTimes: map[string]string{"mon": "12:00", "tue": "13:00", "wed": "14:00"},
		pickups:    []careplan.PickupSchedule{{Weekday: 1, PickupTime: clock("17:00"), Source: careplan.ScheduleSourceCareOffering}, {Weekday: 3, PickupTime: clock("15:00"), Source: careplan.ScheduleSourceStaff}},
	}
	plan, err := facts.weekly("2026-09-09")
	require.NoError(t, err)
	require.Equal(t, map[int]bool{1: true, 2: true}, plan.ArrivalDays)
	require.Equal(t, map[int]string{1: "12:00", 2: "11:00"}, plan.ArrivalTimes)
	require.Equal(t, map[int]string{3: "15:00"}, plan.PickupTimes, "obsolete materialized offering rows must not become a baseline")
}

func TestReviewWeeklyPlanUsesHalfOpenBookingWindowsPerArrivalDay(t *testing.T) {
	t.Parallel()
	facts := reviewPlanFacts{
		authoritative: true,
		classTimes:    map[string]string{"mon": "12:00", "thu": "13:00", "fri": "11:00"},
		bookings: []reviewPlanSelection{{from: "2026-09-10", until: "2026-09-12", offering: careplan.CareOffering{
			IsActive: true, CountsAsCare: true, DaysOfWeekMode: "fixed", AvailableDays: []string{"mon", "thu", "fri"},
			PickupTimes: map[string]string{"mon": "16:00", "thu": "16:00", "fri": "15:00"},
		}}},
	}
	plan, err := facts.weekly("2026-09-09")
	require.NoError(t, err)
	require.Equal(t, map[int]bool{4: true, 5: true}, plan.ArrivalDays)
	require.Empty(t, plan.PickupTimes, "pickup week is the plan effective today, not the plan effective on each weekday")
	plan, err = facts.weekly("2026-09-10")
	require.NoError(t, err)
	require.Equal(t, map[int]string{1: "16:00", 4: "16:00", 5: "15:00"}, plan.PickupTimes)
	end, err := facts.pickupWeek("2026-09-12")
	require.NoError(t, err)
	require.Empty(t, end, "valid_until is exclusive")
}

func TestReviewWeeklyPlanRespectsSelectionsAndLatestOfferingTime(t *testing.T) {
	t.Parallel()
	first := careplan.CareOffering{IsActive: true, CountsAsCare: true, DaysOfWeekMode: "fixed", AvailableDays: []string{"mon", "tue"}, PickupTimes: map[string]string{"mon": "16:00", "tue": "15:00"}}
	second := first
	second.PickupTimes = map[string]string{"mon": "17:00", "tue": "16:00"}
	facts := reviewPlanFacts{authoritative: true, bookings: []reviewPlanSelection{{offering: first, selectedDays: []string{"tue"}}, {offering: second, selectedDays: []string{"tue"}}}}
	plan, err := facts.weekly("2026-09-09")
	require.NoError(t, err)
	require.Equal(t, map[int]bool{2: true}, plan.ArrivalDays)
	require.Empty(t, plan.ArrivalTimes, "a booked day without a class time remains a care day")
	require.Equal(t, map[int]string{2: "16:00"}, plan.PickupTimes)
	manual, err := time.Parse("15:04", "14:00")
	require.NoError(t, err)
	facts.pickups = []careplan.PickupSchedule{{Weekday: 2, PickupTime: manual}, {Weekday: 1, PickupTime: manual}}
	plan, err = facts.weekly("2026-09-09")
	require.NoError(t, err)
	require.Equal(t, map[int]string{2: "14:00"}, plan.PickupTimes, "manual overrides cannot add unbooked care days")
	facts.bookings[0].offering.PickupTimes = map[string]string{"tue": "invalid"}
	_, err = facts.weekly("2026-09-09")
	require.ErrorContains(t, err, "invalid pickup time")
}
