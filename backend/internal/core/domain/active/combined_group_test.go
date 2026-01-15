package active

import (
	"testing"
	"time"
)

func TestCombinedGroupValidate(t *testing.T) {
	nowTime := time.Now()
	futureTime := nowTime.Add(2 * time.Hour)
	pastTime := nowTime.Add(-2 * time.Hour)

	tests := []struct {
		name          string
		combinedGroup *CombinedGroup
		wantErr       bool
	}{
		{
			name: "Valid combined group",
			combinedGroup: &CombinedGroup{
				StartTime: nowTime,
			},
			wantErr: false,
		},
		{
			name: "Valid combined group with end time",
			combinedGroup: &CombinedGroup{
				StartTime: nowTime,
				EndTime:   &futureTime,
			},
			wantErr: false,
		},
		{
			name:          "Missing start time",
			combinedGroup: &CombinedGroup{},
			wantErr:       true,
		},
		{
			name: "End time before start time",
			combinedGroup: &CombinedGroup{
				StartTime: nowTime,
				EndTime:   &pastTime,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.combinedGroup.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("CombinedGroup.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCombinedGroupIsActive(t *testing.T) {
	nowTime := time.Now()
	futureTime := nowTime.Add(2 * time.Hour)

	tests := []struct {
		name          string
		combinedGroup *CombinedGroup
		want          bool
	}{
		{
			name: "Active combined group (no end time)",
			combinedGroup: &CombinedGroup{
				StartTime: nowTime,
				EndTime:   nil,
			},
			want: true,
		},
		{
			name: "Inactive combined group (has end time)",
			combinedGroup: &CombinedGroup{
				StartTime: nowTime,
				EndTime:   &futureTime,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.combinedGroup.IsActive(); got != tt.want {
				t.Errorf("CombinedGroup.IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCombinedGroupEndCombination(t *testing.T) {
	nowTime := time.Now()

	combinedGroup := &CombinedGroup{
		StartTime: nowTime,
		EndTime:   nil,
	}

	// Test that EndCombination sets the end time
	combinedGroup.EndCombination()

	if combinedGroup.EndTime == nil {
		t.Errorf("CombinedGroup.EndCombination() did not set the end time")
	}
}

func TestCombinedGroupSetEndTime(t *testing.T) {
	nowTime := time.Now()
	futureTime := nowTime.Add(2 * time.Hour)
	pastTime := nowTime.Add(-2 * time.Hour)

	tests := []struct {
		name          string
		combinedGroup *CombinedGroup
		endTime       time.Time
		wantErr       bool
	}{
		{
			name: "Valid end time",
			combinedGroup: &CombinedGroup{
				StartTime: nowTime,
			},
			endTime: futureTime,
			wantErr: false,
		},
		{
			name: "End time before start time",
			combinedGroup: &CombinedGroup{
				StartTime: nowTime,
			},
			endTime: pastTime,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.combinedGroup.SetEndTime(tt.endTime)
			if (err != nil) != tt.wantErr {
				t.Errorf("CombinedGroup.SetEndTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && (tt.combinedGroup.EndTime == nil || !tt.endTime.Equal(*tt.combinedGroup.EndTime)) {
				t.Errorf("CombinedGroup.SetEndTime() did not correctly set the end time")
			}
		})
	}
}

func TestCombinedGroupGetDuration(t *testing.T) {
	nowTime := time.Now()
	futureTime := nowTime.Add(2 * time.Hour)

	tests := []struct {
		name          string
		combinedGroup *CombinedGroup
		wantDuration  time.Duration
	}{
		{
			name: "Active combined group (calculates duration from now)",
			combinedGroup: &CombinedGroup{
				StartTime: nowTime.Add(-1 * time.Hour), // Started 1 hour ago
				EndTime:   nil,
			},
			wantDuration: time.Hour, // Approximately 1 hour, may be slightly more
		},
		{
			name: "Inactive combined group (fixed duration)",
			combinedGroup: &CombinedGroup{
				StartTime: nowTime,
				EndTime:   &futureTime, // 2 hours in the future
			},
			wantDuration: 2 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.combinedGroup.GetDuration()

			if tt.name == "Active combined group (calculates duration from now)" {
				// For active combined groups, we can only check that the duration is approximately correct
				if got < tt.wantDuration || got > tt.wantDuration+10*time.Second {
					t.Errorf("CombinedGroup.GetDuration() = %v, want approximately %v", got, tt.wantDuration)
				}
			} else {
				// For inactive combined groups with fixed end times, we can check exact equality
				if got != tt.wantDuration {
					t.Errorf("CombinedGroup.GetDuration() = %v, want %v", got, tt.wantDuration)
				}
			}
		})
	}
}
