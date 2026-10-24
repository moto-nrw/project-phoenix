package workforce_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The weekly Soll sums over schedule rows and templates (#3259). The cases
// moved here with the arithmetic from models/config.

// Schedule day indexes (Monday=0 … Sunday=6), see workforce.ISODayIndex.
const (
	dayMonday    = 0
	dayTuesday   = 1
	dayWednesday = 2
	dayFriday    = 4
)

// mondayAnchor is a Monday.
var mondayAnchor = calendar.NewDate(2026, time.January, 5)

func datePtr(d calendar.Date) *calendar.Date { return &d }

func scheduleEntry(weekIndex, rotation, day, minutes int, validFrom calendar.Date, validUntil *calendar.Date) *workforce.ScheduleRow {
	return &workforce.ScheduleRow{
		WeekIndex:      weekIndex,
		RotationLength: rotation,
		DayOfWeek:      day,
		TargetMinutes:  minutes,
		ValidFrom:      validFrom,
		ValidUntil:     validUntil,
	}
}

func TestWeeklyTargetFromSchedule(t *testing.T) {
	t.Parallel()

	weekA := mondayAnchor
	weekB := mondayAnchor.AddDays(7)

	cases := []struct {
		name        string
		entries     []*workforce.ScheduleRow
		staffAnchor *calendar.Date
		weekStart   calendar.Date
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
			entries: []*workforce.ScheduleRow{
				scheduleEntry(0, 1, dayMonday, 240, weekA, nil),
				scheduleEntry(0, 1, dayWednesday, 245, weekA, nil),
			},
			weekStart: weekA,
			wantTotal: 485,
			wantFound: true,
		},
		{
			name: "entries valid only after the week do not apply",
			entries: []*workforce.ScheduleRow{
				scheduleEntry(0, 1, dayMonday, 240, weekB, nil),
			},
			weekStart: weekA,
			wantTotal: 0,
			wantFound: false,
		},
		{
			name: "valid_until is exclusive",
			entries: []*workforce.ScheduleRow{
				// Valid Monday and Tuesday only: valid_until = Wednesday.
				scheduleEntry(0, 1, dayMonday, 100, weekA, datePtr(weekA.AddDays(2))),
				scheduleEntry(0, 1, dayWednesday, 100, weekA, datePtr(weekA.AddDays(2))),
			},
			weekStart: weekA,
			wantTotal: 100,
			wantFound: true,
		},
		{
			name: "rotation week A picks week_index 0",
			entries: []*workforce.ScheduleRow{
				scheduleEntry(0, 2, dayMonday, 300, weekA, nil),
				scheduleEntry(1, 2, dayMonday, 200, weekA, nil),
			},
			staffAnchor: datePtr(weekA),
			weekStart:   weekA,
			wantTotal:   300,
			wantFound:   true,
		},
		{
			name: "rotation week B picks week_index 1",
			entries: []*workforce.ScheduleRow{
				scheduleEntry(0, 2, dayMonday, 300, weekA, nil),
				scheduleEntry(1, 2, dayMonday, 200, weekA, nil),
			},
			staffAnchor: datePtr(weekA),
			weekStart:   weekB,
			wantTotal:   200,
			wantFound:   true,
		},
		{
			name: "mid-week validity switch sums both generations",
			entries: []*workforce.ScheduleRow{
				// Old contract: Mon+Wed 100 each, ends (exclusive) on Wednesday.
				scheduleEntry(0, 1, dayMonday, 100, weekA.AddDays(-7), datePtr(weekA.AddDays(2))),
				scheduleEntry(0, 1, dayWednesday, 100, weekA.AddDays(-7), datePtr(weekA.AddDays(2))),
				// New contract from Wednesday: Wed+Fri 150 each.
				scheduleEntry(0, 1, dayWednesday, 150, weekA.AddDays(2), nil),
				scheduleEntry(0, 1, dayFriday, 150, weekA.AddDays(2), nil),
			},
			weekStart: weekA,
			wantTotal: 400,
			wantFound: true,
		},
		{
			name: "zero-minute entry still counts as found",
			entries: []*workforce.ScheduleRow{
				scheduleEntry(0, 1, dayMonday, 0, weekA, nil),
			},
			weekStart: weekA,
			wantTotal: 0,
			wantFound: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			total, found := workforce.WeeklyTargetFromSchedule(tc.entries, tc.staffAnchor, tc.weekStart)
			if total != tc.wantTotal || found != tc.wantFound {
				t.Fatalf("got (%d, %v), want (%d, %v)", total, found, tc.wantTotal, tc.wantFound)
			}
		})
	}
}

func TestWeeklyTargetsFromTemplate(t *testing.T) {
	t.Parallel()

	weekA := mondayAnchor
	weekB := mondayAnchor.AddDays(7)

	model := &workforce.ScheduleTemplate{
		RotationLength:     2,
		RotationAnchorDate: weekA,
		Entries: []*workforce.ScheduleTemplateEntry{
			nil,
			{WeekIndex: 0, DayOfWeek: dayMonday, TargetMinutes: 615},
			{WeekIndex: 0, DayOfWeek: dayTuesday, TargetMinutes: 600},
			{WeekIndex: 1, DayOfWeek: dayMonday, TargetMinutes: 480},
		},
	}

	if got := workforce.WeeklyTargetsFromTemplate(nil, weekA, []calendar.Date{weekA}); got != nil {
		t.Fatalf("nil template should yield nil, got %v", got)
	}
	if got := workforce.WeeklyTargetsFromTemplate(&workforce.ScheduleTemplate{RotationLength: 1}, weekA, []calendar.Date{weekA}); got != nil {
		t.Fatalf("template without entries should yield nil, got %v", got)
	}

	targets := workforce.WeeklyTargetsFromTemplate(model, weekA.AddDays(2), []calendar.Date{weekA, weekB, weekA.AddDays(14)})
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
