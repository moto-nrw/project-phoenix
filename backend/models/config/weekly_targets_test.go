package config

import (
	"testing"
	"time"
)

// mondayAnchor is a Monday (same anchor week the ResolveWeekIndex tests use).
var mondayAnchor = newTestDate(2026, time.January, 5)

func datePtr(d CalendarDate) *CalendarDate { return &d }

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

func TestWeeklyTargetFromSchedule(t *testing.T) {
	t.Parallel()

	weekA := mondayAnchor
	weekB := mondayAnchor.AddDays(7)

	cases := []struct {
		name        string
		entries     []*StaffWorkSchedule
		staffAnchor *CalendarDate
		weekStart   CalendarDate
		wantTotal   int
		wantFound   bool
	}{
		{
			name:      "no entries yields not found",
			entries:   nil,
			weekStart: weekA,
			wantTotal: 0,
			wantFound: false,
		},
		{
			name: "single-week rotation sums matching days",
			entries: []*StaffWorkSchedule{
				scheduleEntry(0, 1, DayMonday, 240, weekA, nil),
				scheduleEntry(0, 1, DayWednesday, 245, weekA, nil),
			},
			weekStart: weekA,
			wantTotal: 485,
			wantFound: true,
		},
		{
			name: "entries valid only after the week do not apply",
			entries: []*StaffWorkSchedule{
				scheduleEntry(0, 1, DayMonday, 240, weekB, nil),
			},
			weekStart: weekA,
			wantTotal: 0,
			wantFound: false,
		},
		{
			name: "valid_until is exclusive",
			entries: []*StaffWorkSchedule{
				// Valid Monday and Tuesday only: valid_until = Wednesday.
				scheduleEntry(0, 1, DayMonday, 100, weekA, datePtr(weekA.AddDays(2))),
				scheduleEntry(0, 1, DayWednesday, 100, weekA, datePtr(weekA.AddDays(2))),
			},
			weekStart: weekA,
			wantTotal: 100,
			wantFound: true,
		},
		{
			name: "rotation week A picks week_index 0",
			entries: []*StaffWorkSchedule{
				scheduleEntry(0, 2, DayMonday, 300, weekA, nil),
				scheduleEntry(1, 2, DayMonday, 200, weekA, nil),
			},
			staffAnchor: datePtr(weekA),
			weekStart:   weekA,
			wantTotal:   300,
			wantFound:   true,
		},
		{
			name: "rotation week B picks week_index 1",
			entries: []*StaffWorkSchedule{
				scheduleEntry(0, 2, DayMonday, 300, weekA, nil),
				scheduleEntry(1, 2, DayMonday, 200, weekA, nil),
			},
			staffAnchor: datePtr(weekA),
			weekStart:   weekB,
			wantTotal:   200,
			wantFound:   true,
		},
		{
			name: "mid-week validity switch sums both generations",
			entries: []*StaffWorkSchedule{
				// Old contract: Mon+Wed 100 each, ends (exclusive) on Wednesday.
				scheduleEntry(0, 1, DayMonday, 100, weekA.AddDays(-7), datePtr(weekA.AddDays(2))),
				scheduleEntry(0, 1, DayWednesday, 100, weekA.AddDays(-7), datePtr(weekA.AddDays(2))),
				// New contract from Wednesday: Wed+Fri 150 each.
				scheduleEntry(0, 1, DayWednesday, 150, weekA.AddDays(2), nil),
				scheduleEntry(0, 1, DayFriday, 150, weekA.AddDays(2), nil),
			},
			weekStart: weekA,
			wantTotal: 400,
			wantFound: true,
		},
		{
			name: "zero-minute entry still counts as found",
			entries: []*StaffWorkSchedule{
				scheduleEntry(0, 1, DayMonday, 0, weekA, nil),
			},
			weekStart: weekA,
			wantTotal: 0,
			wantFound: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			total, found := WeeklyTargetFromSchedule(tc.entries, tc.staffAnchor, tc.weekStart)
			if total != tc.wantTotal || found != tc.wantFound {
				t.Fatalf("got (%d, %v), want (%d, %v)", total, found, tc.wantTotal, tc.wantFound)
			}
		})
	}
}

func TestWeeklyTargetsFromModel(t *testing.T) {
	t.Parallel()

	weekA := mondayAnchor
	weekB := mondayAnchor.AddDays(7)

	model := &WorkTimeModel{
		RotationLength:     2,
		RotationAnchorDate: weekA,
		Entries: []*WorkTimeModelEntry{
			nil,
			{WeekIndex: 0, DayOfWeek: DayMonday, TargetMinutes: 615},
			{WeekIndex: 0, DayOfWeek: DayTuesday, TargetMinutes: 600},
			{WeekIndex: 1, DayOfWeek: DayMonday, TargetMinutes: 480},
		},
	}

	if got := WeeklyTargetsFromModel(nil, weekA, []CalendarDate{weekA}); got != nil {
		t.Fatalf("nil model should yield nil, got %v", got)
	}
	if got := WeeklyTargetsFromModel(&WorkTimeModel{RotationLength: 1}, weekA, []CalendarDate{weekA}); got != nil {
		t.Fatalf("model without entries should yield nil, got %v", got)
	}

	targets := WeeklyTargetsFromModel(model, weekA.AddDays(2), []CalendarDate{weekA, weekB, weekA.AddDays(14)})
	if len(targets) != 3 {
		t.Fatalf("expected 3 week targets, got %d (%v)", len(targets), targets)
	}
	// The mid-week anchor is Monday-aligned before rotation resolution.
	if targets[weekA] != 1215 {
		t.Fatalf("week A target = %d, want 1215", targets[weekA])
	}
	if targets[weekB] != 480 {
		t.Fatalf("week B target = %d, want 480", targets[weekB])
	}
	if targets[weekA.AddDays(14)] != 1215 {
		t.Fatalf("second week A target = %d, want 1215", targets[weekA.AddDays(14)])
	}
}
