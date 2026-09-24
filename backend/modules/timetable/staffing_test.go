package timetable_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

func TestRequiredStaffForChildren(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		children int
		ratio    int
		want     int
	}{
		{"no children needs no staff", 0, 10, 0},
		{"negative children clamps to zero", -5, 10, 0},
		{"exact multiple", 20, 10, 2},
		{"rounds up a partial group", 25, 10, 3},
		{"one child still needs one", 1, 10, 1},
		{"ratio below one is clamped to one", 3, 0, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := timetable.RequiredStaffForChildren(tc.children, tc.ratio); got != tc.want {
				t.Errorf("RequiredStaffForChildren(%d, %d) = %d, want %d",
					tc.children, tc.ratio, got, tc.want)
			}
		})
	}
}

func TestEffectiveRequiredStaff(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		override *int
		children int
		ratio    int
		want     int
	}{
		{"no override derives from ratio", nil, 25, 10, 3}, // ceil(25/10)=3
		{"no override, no children -> 0", nil, 0, 10, 0},   // derived 0
		{"override wins over derived", ptr(5), 25, 10, 5},  // 5, not 3
		{"override 0 means explicitly none", ptr(0), 25, 10, 0},
		{"override wins even below derived", ptr(1), 40, 10, 1}, // 1, not 4
		{"negative override treated as 0", ptr(-3), 25, 10, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := timetable.EffectiveRequiredStaff(tc.override, tc.children, tc.ratio); got != tc.want {
				t.Errorf("EffectiveRequiredStaff(%v, %d, %d) = %d, want %d",
					tc.override, tc.children, tc.ratio, got, tc.want)
			}
		})
	}
}

func TestIsUnderstaffed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		rows []timetable.InstanceStaff
		want bool
	}{
		{"nobody planned or present", nil, true},
		{"one planned person present", []timetable.InstanceStaff{{}}, false},
		{"planned person absent without replacement", []timetable.InstanceStaff{{}, {IsAbsent: true}}, true},
		{"absence covered by a substitute", []timetable.InstanceStaff{{IsAbsent: true}, {IsSubstitute: true}}, false},
		{"only the substitute is present", []timetable.InstanceStaff{{IsSubstitute: true}}, false},
		{"everyone absent", []timetable.InstanceStaff{{IsAbsent: true}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := timetable.IsUnderstaffed(tc.rows); got != tc.want {
				t.Errorf("IsUnderstaffed() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResolveSlotSource(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		hasException bool
		hasSchedule  bool
		weekday      int
		want         string
	}{
		{"exception wins over the schedule", true, true, 1, timetable.SlotSourceException},
		{"exception applies on the weekend", true, false, 6, timetable.SlotSourceException},
		{"weekday schedule", false, true, 5, timetable.SlotSourceSchedule},
		{"no weekly schedule on the weekend", false, true, 7, timetable.SlotSourceNone},
		{"no rule at all", false, false, 3, timetable.SlotSourceNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := timetable.ResolveSlotSource(tc.hasException, tc.hasSchedule, tc.weekday); got != tc.want {
				t.Errorf("ResolveSlotSource() = %q, want %q", got, tc.want)
			}
		})
	}
}
