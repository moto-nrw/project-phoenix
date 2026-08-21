package active

import "testing"

// The "Zuhause" tile exists to complete the dashboard's headcount, so what
// matters is that it does not overlap the other tiles: sick and excused
// children never check in, and counting them here as well would show the same
// child twice.
func TestCalculateStudentsHome(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                          string
		total, present, sick, excused int
		want                          int
	}{
		{
			name:  "remainder after presence and both absence buckets",
			total: 100, present: 60, sick: 5, excused: 3,
			want: 32,
		},
		{
			name:  "absence buckets are not double counted as home",
			total: 10, present: 0, sick: 4, excused: 6,
			want: 0,
		},
		{
			name:  "everyone present leaves nobody at home",
			total: 25, present: 25, sick: 0, excused: 0,
			want: 0,
		},
		{
			name:  "no students at all",
			total: 0, present: 0, sick: 0, excused: 0,
			want: 0,
		},
		{
			// Presence comes from today's attendance rows while the total comes
			// from the student table, so a child checked in and then deactivated
			// mid-day can outrun the total. Clamp instead of showing a negative.
			name:  "clamps when presence outruns the total",
			total: 5, present: 8, sick: 0, excused: 0,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateStudentsHome(tt.total, tt.present, tt.sick, tt.excused)
			if got != tt.want {
				t.Errorf("calculateStudentsHome(%d, %d, %d, %d) = %d, want %d",
					tt.total, tt.present, tt.sick, tt.excused, got, tt.want)
			}
		})
	}
}
