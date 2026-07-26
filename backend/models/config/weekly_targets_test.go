package config_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/config"
)

// mondayAnchor is a Monday (same anchor week the ResolveWeekIndex tests use).
var mondayAnchor = timezone.NewDate(2026, time.January, 5)

func datePtr(d timezone.Date) *timezone.Date { return &d }

func scheduleEntry(weekIndex, rotation, day, minutes int, validFrom timezone.Date, validUntil *timezone.Date) *config.StaffWorkSchedule {
	return &config.StaffWorkSchedule{
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
		date timezone.Date
		want timezone.Date
	}{
		{"monday maps to itself", mondayAnchor, mondayAnchor},
		{"wednesday maps back to monday", mondayAnchor.AddDays(2), mondayAnchor},
		{"sunday maps back to monday", mondayAnchor.AddDays(6), mondayAnchor},
		{"next monday maps to itself", mondayAnchor.AddDays(7), mondayAnchor.AddDays(7)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := config.MondayOf(tc.date); got != tc.want {
				t.Fatalf("MondayOf(%s) = %s, want %s", tc.date, got, tc.want)
			}
		})
	}
}

func TestISODayIndex(t *testing.T) {
	t.Parallel()

	for offset, want := range []int{
		config.DayMonday, config.DayTuesday, config.DayWednesday,
		config.DayThursday, config.DayFriday, config.DaySaturday, config.DaySunday,
	} {
		if got := config.ISODayIndex(mondayAnchor.AddDays(offset)); got != want {
			t.Fatalf("ISODayIndex(monday+%d) = %d, want %d", offset, got, want)
		}
	}
}

func TestScheduleRotationLength(t *testing.T) {
	t.Parallel()

	if got := config.ScheduleRotationLength(nil); got != 1 {
		t.Fatalf("empty entries: got %d, want 1", got)
	}
	entries := []*config.StaffWorkSchedule{
		nil,
		scheduleEntry(0, 1, config.DayMonday, 240, mondayAnchor, nil),
		scheduleEntry(1, 2, config.DayMonday, 240, mondayAnchor, nil),
	}
	if got := config.ScheduleRotationLength(entries); got != 2 {
		t.Fatalf("mixed entries: got %d, want 2", got)
	}
}

func TestResolveScheduleAnchor(t *testing.T) {
	t.Parallel()

	staffAnchor := mondayAnchor.AddDays(14)
	entries := []*config.StaffWorkSchedule{
		nil,
		scheduleEntry(0, 2, config.DayMonday, 240, mondayAnchor.AddDays(7), nil),
		scheduleEntry(1, 2, config.DayTuesday, 240, mondayAnchor, nil),
	}

	if got := config.ResolveScheduleAnchor(&staffAnchor, entries); got != staffAnchor {
		t.Fatalf("staff anchor should win: got %s, want %s", got, staffAnchor)
	}
	if got := config.ResolveScheduleAnchor(nil, entries); got != mondayAnchor {
		t.Fatalf("earliest valid_from should win: got %s, want %s", got, mondayAnchor)
	}
}

func TestWeeklyTargetFromSchedule(t *testing.T) {
	t.Parallel()

	weekA := mondayAnchor
	weekB := mondayAnchor.AddDays(7)

	cases := []struct {
		name        string
		entries     []*config.StaffWorkSchedule
		staffAnchor *timezone.Date
		weekStart   timezone.Date
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
			entries: []*config.StaffWorkSchedule{
				scheduleEntry(0, 1, config.DayMonday, 240, weekA, nil),
				scheduleEntry(0, 1, config.DayWednesday, 245, weekA, nil),
			},
			weekStart: weekA,
			wantTotal: 485,
			wantFound: true,
		},
		{
			name: "entries valid only after the week do not apply",
			entries: []*config.StaffWorkSchedule{
				scheduleEntry(0, 1, config.DayMonday, 240, weekB, nil),
			},
			weekStart: weekA,
			wantTotal: 0,
			wantFound: false,
		},
		{
			name: "valid_until is exclusive",
			entries: []*config.StaffWorkSchedule{
				// Valid Monday and Tuesday only: valid_until = Wednesday.
				scheduleEntry(0, 1, config.DayMonday, 100, weekA, datePtr(weekA.AddDays(2))),
				scheduleEntry(0, 1, config.DayWednesday, 100, weekA, datePtr(weekA.AddDays(2))),
			},
			weekStart: weekA,
			wantTotal: 100,
			wantFound: true,
		},
		{
			name: "rotation week A picks week_index 0",
			entries: []*config.StaffWorkSchedule{
				scheduleEntry(0, 2, config.DayMonday, 300, weekA, nil),
				scheduleEntry(1, 2, config.DayMonday, 200, weekA, nil),
			},
			staffAnchor: datePtr(weekA),
			weekStart:   weekA,
			wantTotal:   300,
			wantFound:   true,
		},
		{
			name: "rotation week B picks week_index 1",
			entries: []*config.StaffWorkSchedule{
				scheduleEntry(0, 2, config.DayMonday, 300, weekA, nil),
				scheduleEntry(1, 2, config.DayMonday, 200, weekA, nil),
			},
			staffAnchor: datePtr(weekA),
			weekStart:   weekB,
			wantTotal:   200,
			wantFound:   true,
		},
		{
			name: "mid-week validity switch sums both generations",
			entries: []*config.StaffWorkSchedule{
				// Old contract: Mon+Wed 100 each, ends (exclusive) on Wednesday.
				scheduleEntry(0, 1, config.DayMonday, 100, weekA.AddDays(-7), datePtr(weekA.AddDays(2))),
				scheduleEntry(0, 1, config.DayWednesday, 100, weekA.AddDays(-7), datePtr(weekA.AddDays(2))),
				// New contract from Wednesday: Wed+Fri 150 each.
				scheduleEntry(0, 1, config.DayWednesday, 150, weekA.AddDays(2), nil),
				scheduleEntry(0, 1, config.DayFriday, 150, weekA.AddDays(2), nil),
			},
			weekStart: weekA,
			wantTotal: 400,
			wantFound: true,
		},
		{
			name: "zero-minute entry still counts as found",
			entries: []*config.StaffWorkSchedule{
				scheduleEntry(0, 1, config.DayMonday, 0, weekA, nil),
			},
			weekStart: weekA,
			wantTotal: 0,
			wantFound: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			total, found := config.WeeklyTargetFromSchedule(tc.entries, tc.staffAnchor, tc.weekStart)
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

	model := &config.WorkTimeModel{
		RotationLength:     2,
		RotationAnchorDate: weekA,
		Entries: []*config.WorkTimeModelEntry{
			nil,
			{WeekIndex: 0, DayOfWeek: config.DayMonday, TargetMinutes: 615},
			{WeekIndex: 0, DayOfWeek: config.DayTuesday, TargetMinutes: 600},
			{WeekIndex: 1, DayOfWeek: config.DayMonday, TargetMinutes: 480},
		},
	}

	if got := config.WeeklyTargetsFromModel(nil, weekA, []timezone.Date{weekA}); got != nil {
		t.Fatalf("nil model should yield nil, got %v", got)
	}
	if got := config.WeeklyTargetsFromModel(&config.WorkTimeModel{RotationLength: 1}, weekA, []timezone.Date{weekA}); got != nil {
		t.Fatalf("model without entries should yield nil, got %v", got)
	}

	targets := config.WeeklyTargetsFromModel(model, weekA.AddDays(2), []timezone.Date{weekA, weekB, weekA.AddDays(14)})
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
