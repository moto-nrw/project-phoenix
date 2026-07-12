package schedule

import "testing"

func intPtr(v int) *int { return &v }

func TestRequiredStaffForChildren(t *testing.T) {
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
			if got := RequiredStaffForChildren(tc.children, tc.ratio); got != tc.want {
				t.Errorf("RequiredStaffForChildren(%d, %d) = %d, want %d",
					tc.children, tc.ratio, got, tc.want)
			}
		})
	}
}

func TestEffectiveRequiredStaff(t *testing.T) {
	cases := []struct {
		name     string
		override *int
		children int
		ratio    int
		want     int
	}{
		{"no override derives from ratio", nil, 25, 10, 3},   // ceil(25/10)=3
		{"no override, no children -> 0", nil, 0, 10, 0},     // derived 0
		{"override wins over derived", intPtr(5), 25, 10, 5}, // 5, not 3
		{"override 0 means explicitly none", intPtr(0), 25, 10, 0},
		{"override wins even below derived", intPtr(1), 40, 10, 1}, // 1, not 4
		{"negative override treated as 0", intPtr(-3), 25, 10, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EffectiveRequiredStaff(tc.override, tc.children, tc.ratio); got != tc.want {
				t.Errorf("EffectiveRequiredStaff(%v, %d, %d) = %d, want %d",
					tc.override, tc.children, tc.ratio, got, tc.want)
			}
		})
	}
}
