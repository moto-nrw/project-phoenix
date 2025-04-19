package models

import (
	"testing"
	"time"
)

func TestTimespan_IsActive(t *testing.T) {
	now := time.Now()
	future := now.Add(1 * time.Hour)
	past := now.Add(-1 * time.Hour)

	tests := []struct {
		name     string
		timespan Timespan
		want     bool
	}{
		{
			name: "No end time",
			timespan: Timespan{
				StartTime: now.Add(-30 * time.Minute),
				EndTime:   nil,
			},
			want: true,
		},
		{
			name: "End time in future",
			timespan: Timespan{
				StartTime: now.Add(-30 * time.Minute),
				EndTime:   &future,
			},
			want: true,
		},
		{
			name: "End time in past",
			timespan: Timespan{
				StartTime: now.Add(-2 * time.Hour),
				EndTime:   &past,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.timespan.IsActive(); got != tt.want {
				t.Errorf("Timespan.IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTimespan_Duration(t *testing.T) {
	now := time.Now()
	startTime := now.Add(-1 * time.Hour)
	endTime := now.Add(-30 * time.Minute)

	tests := []struct {
		name     string
		timespan Timespan
		want     time.Duration
	}{
		{
			name: "With end time",
			timespan: Timespan{
				StartTime: startTime,
				EndTime:   &endTime,
			},
			want: 30 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.timespan.Duration()
			if got != tt.want {
				t.Errorf("Timespan.Duration() = %v, want %v", got, tt.want)
			}
		})
	}

	// Test with no end time
	// This is tested separately because the duration depends on the current time
	t.Run("No end time", func(t *testing.T) {
		ts := Timespan{
			StartTime: startTime,
			EndTime:   nil,
		}

		got := ts.Duration()
		// Should be approximately 1 hour, but might be slightly more due to time elapsed during test
		if got < 59*time.Minute || got > 61*time.Minute {
			t.Errorf("Timespan.Duration() = %v, want approximately %v", got, 1*time.Hour)
		}
	})
}
