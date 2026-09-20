package carerequests_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

func TestGermanAllowedDepartureModes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		modes []string
		want  string
	}{
		{
			name:  "empty set means the child goes home alone",
			modes: nil,
			want:  "Geht alleine",
		},
		{
			name:  "single mode renders its German label",
			modes: []string{"bus"},
			want:  "Fährt Bus",
		},
		{
			name: "multiple modes are joined with a slash in order",
			modes: []string{
				"bus",
				"pickup",
			},
			want: "Fährt Bus / Wird abgeholt",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := carerequests.WeeklyDiff([]carerequests.WeekdayChange{{Weekday: 1, Mode: "pickup"}}, carerequests.WeeklyPlan{DepartureModes: map[string][]string{"mon": tc.modes}})[0].Old; got != tc.want {
				t.Errorf("germanAllowedDepartureModes(%v) = %q, want %q", tc.modes, got, tc.want)
			}
		})
	}
}

func TestCareDashIfEmpty(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"":            "—",
		"   ":         "—", // whitespace-only collapses to the em dash placeholder
		"07:30":       "07:30",
		" Fährt Bus ": " Fährt Bus ", // non-empty content is returned verbatim
	}
	for in, want := range cases {
		if got := carerequests.WeeklyDiff([]carerequests.WeekdayChange{{Weekday: 1, Arrival: "12:00"}}, carerequests.WeeklyPlan{ArrivalTimes: map[int]string{1: in}})[0].Old; got != want {
			t.Errorf("careDashIfEmpty(%q) = %q, want %q", in, got, want)
		}
	}
}
