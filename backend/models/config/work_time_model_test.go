package config

import (
	"testing"
	"time"
)

func newTestDate(year int, month time.Month, day int) CalendarDate {
	return CalendarDate{Year: year, Month: month, Day: day}
}

func TestResolveWeekIndex(t *testing.T) {
	t.Parallel()

	monday := newTestDate(2026, time.January, 5) // Anchor week, A
	cases := []struct {
		name     string
		rotation int
		date     CalendarDate
		want     int
	}{
		{
			name:     "single-week rotation always returns 0",
			rotation: 1,
			date:     newTestDate(2026, time.June, 1),
			want:     0,
		},
		{
			name:     "anchor itself maps to 0",
			rotation: 2,
			date:     monday,
			want:     0,
		},
		{
			name:     "next week wraps to 1 (B)",
			rotation: 2,
			date:     monday.AddDays(7),
			want:     1,
		},
		{
			name:     "two weeks ahead wraps back to 0 (A)",
			rotation: 2,
			date:     monday.AddDays(14),
			want:     0,
		},
		{
			name:     "negative delta wraps forward",
			rotation: 2,
			date:     monday.AddDays(-7),
			want:     1,
		},
		{
			name:     "three-week rotation cycles correctly",
			rotation: 3,
			date:     monday.AddDays(14),
			want:     2,
		},
		{
			name:     "rotation 0 treated as 1",
			rotation: 0,
			date:     monday.AddDays(21),
			want:     0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveWeekIndex(tc.rotation, monday, tc.date)
			if got != tc.want {
				t.Fatalf("ResolveWeekIndex(rotation=%d, date=%s) = %d, want %d",
					tc.rotation, tc.date.String(), got, tc.want)
			}
		})
	}
}

func TestWorkTimeModelValidate(t *testing.T) {
	t.Parallel()

	anchor := newTestDate(2026, time.January, 5)

	t.Run("valid model passes", func(t *testing.T) {
		m := &WorkTimeModel{
			Name:               "Standard 40h",
			RotationLength:     1,
			RotationAnchorDate: anchor,
		}
		if err := m.Validate(); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("rotation > max rejected", func(t *testing.T) {
		m := &WorkTimeModel{
			Name:               "Invalid",
			RotationLength:     5,
			RotationAnchorDate: anchor,
		}
		if err := m.Validate(); err == nil {
			t.Fatalf("expected validation error")
		}
	})

	t.Run("missing name rejected", func(t *testing.T) {
		m := &WorkTimeModel{
			RotationLength:     1,
			RotationAnchorDate: anchor,
		}
		if err := m.Validate(); err == nil {
			t.Fatalf("expected validation error")
		}
	})

	t.Run("zero anchor rejected", func(t *testing.T) {
		m := &WorkTimeModel{
			Name:           "x",
			RotationLength: 1,
		}
		if err := m.Validate(); err == nil {
			t.Fatalf("expected validation error")
		}
	})
}

func TestWorkTimeModelEntryValidate(t *testing.T) {
	t.Parallel()

	t.Run("week_index out of bounds rejected", func(t *testing.T) {
		e := &WorkTimeModelEntry{
			ModelID:       1,
			WeekIndex:     4,
			DayOfWeek:     0,
			TargetMinutes: 480,
		}
		if err := e.Validate(); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("target above 12h rejected", func(t *testing.T) {
		e := &WorkTimeModelEntry{
			ModelID:       1,
			WeekIndex:     0,
			DayOfWeek:     0,
			TargetMinutes: 800,
		}
		if err := e.Validate(); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestStaffWorkScheduleValidate(t *testing.T) {
	t.Parallel()

	anchor := newTestDate(2026, time.January, 5)

	t.Run("week_index outside rotation rejected", func(t *testing.T) {
		s := &StaffWorkSchedule{
			StaffID:        42,
			WeekIndex:      2,
			RotationLength: 2,
			DayOfWeek:      0,
			TargetMinutes:  480,
			ValidFrom:      anchor,
		}
		if err := s.Validate(); err == nil {
			t.Fatalf("expected error: week_index 2 cannot exist with rotation_length 2")
		}
	})

	t.Run("rotation 1 is the default and accepts week_index 0", func(t *testing.T) {
		s := &StaffWorkSchedule{
			StaffID:        42,
			WeekIndex:      0,
			RotationLength: 1,
			DayOfWeek:      0,
			TargetMinutes:  480,
			ValidFrom:      anchor,
		}
		if err := s.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
