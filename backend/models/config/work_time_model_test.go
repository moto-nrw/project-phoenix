package config_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/config"
)

func TestResolveWeekIndex(t *testing.T) {
	t.Parallel()

	monday := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC) // Anchor week, A
	cases := []struct {
		name     string
		rotation int
		date     time.Time
		want     int
	}{
		{
			name:     "single-week rotation always returns 0",
			rotation: 1,
			date:     time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
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
			date:     monday.AddDate(0, 0, 7),
			want:     1,
		},
		{
			name:     "two weeks ahead wraps back to 0 (A)",
			rotation: 2,
			date:     monday.AddDate(0, 0, 14),
			want:     0,
		},
		{
			name:     "negative delta wraps forward",
			rotation: 2,
			date:     monday.AddDate(0, 0, -7),
			want:     1,
		},
		{
			name:     "three-week rotation cycles correctly",
			rotation: 3,
			date:     monday.AddDate(0, 0, 14),
			want:     2,
		},
		{
			name:     "rotation 0 treated as 1",
			rotation: 0,
			date:     monday.AddDate(0, 0, 21),
			want:     0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := config.ResolveWeekIndex(tc.rotation, monday, tc.date)
			if got != tc.want {
				t.Fatalf("ResolveWeekIndex(rotation=%d, date=%s) = %d, want %d",
					tc.rotation, tc.date.Format("2006-01-02"), got, tc.want)
			}
		})
	}
}

func TestWorkTimeModelValidate(t *testing.T) {
	t.Parallel()

	anchor := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

	t.Run("valid model passes", func(t *testing.T) {
		m := &config.WorkTimeModel{
			Name:               "Standard 40h",
			RotationLength:     1,
			RotationAnchorDate: anchor,
		}
		if err := m.Validate(); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("rotation > max rejected", func(t *testing.T) {
		m := &config.WorkTimeModel{
			Name:               "Invalid",
			RotationLength:     5,
			RotationAnchorDate: anchor,
		}
		if err := m.Validate(); err == nil {
			t.Fatalf("expected validation error")
		}
	})

	t.Run("missing name rejected", func(t *testing.T) {
		m := &config.WorkTimeModel{
			RotationLength:     1,
			RotationAnchorDate: anchor,
		}
		if err := m.Validate(); err == nil {
			t.Fatalf("expected validation error")
		}
	})

	t.Run("zero anchor rejected", func(t *testing.T) {
		m := &config.WorkTimeModel{
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
		e := &config.WorkTimeModelEntry{
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
		e := &config.WorkTimeModelEntry{
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

	anchor := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

	t.Run("week_index outside rotation rejected", func(t *testing.T) {
		s := &config.StaffWorkSchedule{
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
		s := &config.StaffWorkSchedule{
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

func TestWeeklyTargetForRotationWeek(t *testing.T) {
	t.Parallel()

	entries := []*config.StaffWorkSchedule{
		{WeekIndex: 0, DayOfWeek: config.DayMonday, TargetMinutes: 20 * 60},
		{WeekIndex: 1, DayOfWeek: config.DayMonday, TargetMinutes: 30 * 60},
	}

	weekZero := config.WeeklyTargetForRotationWeek(entries, 0)
	weekOne := config.WeeklyTargetForRotationWeek(entries, 1)
	missing := config.WeeklyTargetForRotationWeek(entries, 2)

	if weekZero == nil || *weekZero != 20*60 {
		t.Fatalf("week 0 target = %v, want 1200", weekZero)
	}
	if weekOne == nil || *weekOne != 30*60 {
		t.Fatalf("week 1 target = %v, want 1800", weekOne)
	}
	if missing != nil {
		t.Fatalf("missing week target = %v, want nil", *missing)
	}
}
