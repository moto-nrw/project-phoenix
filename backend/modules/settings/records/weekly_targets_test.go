package records

import (
	"testing"
	"time"
)

// mondayAnchor is a Monday (same anchor week the ResolveWeekIndex tests use).
var mondayAnchor = newTestDate(2026, time.January, 5)

func scheduleEntry(weekIndex, rotation, day, minutes int, validFrom CalendarDate, validUntil *CalendarDate) *StaffWorkSchedule {
	return &StaffWorkSchedule{
		WeekIndex:      weekIndex,
		RotationLength: rotation,
		DayOfWeek:      day,
		TargetMinutes:  minutes,
		ValidFrom:      validFrom,
		ValidUntil:     validUntil,
	}
}

func TestMondayOf(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		date CalendarDate
		want CalendarDate
	}{
		{"monday maps to itself", mondayAnchor, mondayAnchor},
		{"wednesday maps back to monday", mondayAnchor.AddDays(2), mondayAnchor},
		{"sunday maps back to monday", mondayAnchor.AddDays(6), mondayAnchor},
		{"next monday maps to itself", mondayAnchor.AddDays(7), mondayAnchor.AddDays(7)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := MondayOf(tc.date); got != tc.want {
				t.Fatalf("MondayOf(%s) = %s, want %s", tc.date, got, tc.want)
			}
		})
	}
}

func TestISODayIndex(t *testing.T) {
	t.Parallel()

	for offset, want := range []int{
		DayMonday, DayTuesday, DayWednesday,
		DayThursday, DayFriday, DaySaturday, DaySunday,
	} {
		if got := ISODayIndex(mondayAnchor.AddDays(offset)); got != want {
			t.Fatalf("ISODayIndex(monday+%d) = %d, want %d", offset, got, want)
		}
	}
}

func TestScheduleRotationLength(t *testing.T) {
	t.Parallel()

	if got := ScheduleRotationLength(nil); got != 1 {
		t.Fatalf("empty entries: got %d, want 1", got)
	}
	entries := []*StaffWorkSchedule{
		nil,
		scheduleEntry(0, 1, DayMonday, 240, mondayAnchor, nil),
		scheduleEntry(1, 2, DayMonday, 240, mondayAnchor, nil),
	}
	if got := ScheduleRotationLength(entries); got != 2 {
		t.Fatalf("mixed entries: got %d, want 2", got)
	}
}

func TestResolveScheduleAnchor(t *testing.T) {
	t.Parallel()

	staffAnchor := mondayAnchor.AddDays(14)
	entries := []*StaffWorkSchedule{
		nil,
		scheduleEntry(0, 2, DayMonday, 240, mondayAnchor.AddDays(7), nil),
		scheduleEntry(1, 2, DayTuesday, 240, mondayAnchor, nil),
	}

	if got := ResolveScheduleAnchor(&staffAnchor, entries); got != staffAnchor {
		t.Fatalf("staff anchor should win: got %s, want %s", got, staffAnchor)
	}
	if got := ResolveScheduleAnchor(nil, entries); got != mondayAnchor {
		t.Fatalf("earliest valid_from should win: got %s, want %s", got, mondayAnchor)
	}
}
