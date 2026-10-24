package calendar

import (
	"strings"
	"testing"
)

func TestCanonicalDayToISOWeekday(t *testing.T) {
	t.Parallel()
	for i, day := range []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"} {
		for _, input := range []string{day, strings.ToUpper(day), " \t" + day + "\n"} {
			weekday, ok := CanonicalDayToISOWeekday(input)
			if !ok || weekday != i+1 {
				t.Errorf("CanonicalDayToISOWeekday(%q) = %d, %v; want %d, true", input, weekday, ok, i+1)
			}
		}
	}
	for _, input := range []string{"", "Monday", "mo", "bad", "1"} {
		weekday, ok := CanonicalDayToISOWeekday(input)
		if ok || weekday != 0 {
			t.Errorf("unexpected weekday for %q: %d, %v", input, weekday, ok)
		}
	}
}
