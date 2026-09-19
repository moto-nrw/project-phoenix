package compose

import (
	"testing"
)

func TestSupervisionEndedMatchesActiveOn(t *testing.T) {
	t.Parallel()
	today := "2026-06-15"
	yesterday := "2026-06-14"
	tomorrow := "2026-06-16"
	invalid := "not-a-date"
	for _, tc := range []struct {
		name    string
		endDate *string
		ended   bool
	}{
		{name: "open"},
		{name: "ends tomorrow", endDate: &tomorrow},
		{name: "ends today", endDate: &today, ended: true},
		{name: "ended yesterday", endDate: &yesterday, ended: true},
		{name: "unparseable", endDate: &invalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := supervisionEnded(tc.endDate, today); got != tc.ended {
				t.Fatalf("supervisionEnded(%v, %q) = %v, want %v", pointerString(tc.endDate), today, got, tc.ended)
			}
		})
	}
}

func pointerString(value *string) string {
	if value == nil {
		return "<nil>"
	}
	return *value
}
