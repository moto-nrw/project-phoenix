package timezone

import (
	"testing"
	"time"
)

func TestWallClock_RoundTripPreservesTimeOfDay(t *testing.T) {
	t.Parallel()

	want := WallClockFromTime(time.Date(2026, time.March, 29, 23, 59, 58, 123_456_789, Berlin))
	if got := want.Time(); got != time.Date(1, time.January, 1, 23, 59, 58, 123_456_789, time.UTC) {
		t.Fatalf("Time() = %v, want normalized UTC wall clock", got)
	}

	roundTripped := WallClockFromTime(want.Time())
	if roundTripped != want {
		t.Fatalf("WallClockFromTime(Time()) = %#v, want %#v", roundTripped, want)
	}
}

func TestWallClock_RejectsInvalidComponents(t *testing.T) {
	t.Parallel()

	for _, input := range []struct {
		hour, minute, second, nanosecond int
	}{
		{-1, 0, 0, 0},
		{24, 0, 0, 0},
		{0, 60, 0, 0},
		{0, 0, 60, 0},
		{0, 0, 0, 1_000_000_000},
	} {
		if _, err := NewWallClock(input.hour, input.minute, input.second, input.nanosecond); err == nil {
			t.Errorf("NewWallClock(%d, %d, %d, %d) should fail", input.hour, input.minute, input.second, input.nanosecond)
		}
	}
}

func TestWallClock_MidnightIsNotUnset(t *testing.T) {
	t.Parallel()

	midnight, err := NewWallClock(0, 0, 0, 0)
	if err != nil {
		t.Fatalf("NewWallClock(midnight) error = %v", err)
	}
	if midnight == (WallClock{}) {
		t.Fatal("unset WallClock must not compare equal to midnight")
	}
	if got := midnight.Time(); got != time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("midnight.Time() = %v", got)
	}
}
