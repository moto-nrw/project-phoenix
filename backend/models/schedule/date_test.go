package schedule

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDateFromTimeUsesBerlinCalendarDay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Time
		want Date
	}{
		{"summer boundary", time.Date(2026, 6, 9, 22, 0, 0, 0, time.UTC), NewDate(2026, 6, 10)},
		{"winter boundary", time.Date(2026, 1, 9, 23, 0, 0, 0, time.UTC), NewDate(2026, 1, 10)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DateFromTime(tt.in); got != tt.want {
				t.Fatalf("DateFromTime(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestDateCalendarArithmeticAcrossDST(t *testing.T) {
	t.Parallel()

	if got := NewDate(2026, 3, 28).AddDays(2); got != NewDate(2026, 3, 30) {
		t.Fatalf("spring-forward result = %v", got)
	}
	if got := NewDate(2026, 10, 24).AddDays(2); got != NewDate(2026, 10, 26) {
		t.Fatalf("fall-back result = %v", got)
	}
	if got := NewDate(2026, 3, 28).DaysUntil(NewDate(2026, 3, 30)); got != 2 {
		t.Fatalf("DaysUntil = %d, want 2", got)
	}
}

func TestDateJSONIsStrictAndPreservesZero(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(NewDate(2026, 6, 10))
	if err != nil || string(encoded) != `"2026-06-10"` {
		t.Fatalf("Marshal = %s, %v", encoded, err)
	}
	encoded, err = json.Marshal(Date(""))
	if err != nil || string(encoded) != "null" {
		t.Fatalf("Marshal zero = %s, %v", encoded, err)
	}

	for _, input := range []string{`"2026-6-1"`, `"2026-06-10T00:00:00Z"`, `42`} {
		var date Date
		if err := json.Unmarshal([]byte(input), &date); err == nil {
			t.Errorf("Unmarshal(%s) should fail", input)
		}
	}
}

func TestNormalizeWallClockDropsDateAndZone(t *testing.T) {
	t.Parallel()

	input := time.Date(2026, 6, 10, 14, 30, 45, 123, berlin)
	want := time.Date(1, time.January, 1, 14, 30, 45, 123, time.UTC)
	if got := normalizeWallClock(input); !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("NormalizeWallClock(%v) = %v, want %v", input, got, want)
	}
}
