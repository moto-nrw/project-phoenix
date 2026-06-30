package mealplan

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// TestWeekRange verifies the Monday-Friday bounds for any day in a week,
// independent of which weekday the input falls on.
func TestWeekRange(t *testing.T) {
	// 2026-06-29 is a Monday; the week runs Mon 06-29 .. Fri 07-03.
	wantMonday := timezone.NewDate(2026, time.June, 29)
	wantFriday := timezone.NewDate(2026, time.July, 3)

	cases := []struct {
		name  string
		input timezone.Date
	}{
		{"monday", timezone.NewDate(2026, time.June, 29)},
		{"tuesday", timezone.NewDate(2026, time.June, 30)},
		{"friday", timezone.NewDate(2026, time.July, 3)},
		{"saturday", timezone.NewDate(2026, time.July, 4)},
		{"sunday", timezone.NewDate(2026, time.July, 5)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			monday, friday := WeekRange(tc.input)
			if monday != wantMonday {
				t.Errorf("monday: got %s, want %s", monday, wantMonday)
			}
			if friday != wantFriday {
				t.Errorf("friday: got %s, want %s", friday, wantFriday)
			}
		})
	}
}
