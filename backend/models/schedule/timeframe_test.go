package schedule

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestTimeframe_Validate(t *testing.T) {
	now := time.Now()
	future := now.Add(2 * time.Hour)
	past := now.Add(-2 * time.Hour)

	tests := []struct {
		name      string
		timeframe *Timeframe
		wantErr   bool
	}{
		{
			name: "valid timeframe with start only",
			timeframe: &Timeframe{
				StartTime: now,
			},
			wantErr: false,
		},
		{
			name: "valid timeframe with start and end",
			timeframe: &Timeframe{
				StartTime: now,
				EndTime:   &future,
			},
			wantErr: false,
		},
		{
			name: "valid timeframe with all fields",
			timeframe: &Timeframe{
				StartTime:   now,
				EndTime:     &future,
				IsActive:    true,
				Description: "Test timeframe",
			},
			wantErr: false,
		},
		{
			name: "missing start time (zero value)",
			timeframe: &Timeframe{
				StartTime: time.Time{},
			},
			wantErr: true,
		},
		{
			name: "end time before start time",
			timeframe: &Timeframe{
				StartTime: now,
				EndTime:   &past,
			},
			wantErr: true,
		},
		{
			name: "end time equal to start time",
			timeframe: &Timeframe{
				StartTime: now,
				EndTime:   &now,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.timeframe.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Timeframe.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTimeframe_Duration(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		timeframe *Timeframe
		expected  time.Duration
	}{
		{
			name: "with end time - 2 hours",
			timeframe: &Timeframe{
				StartTime: now,
				EndTime:   timePtr(now.Add(2 * time.Hour)),
			},
			expected: 2 * time.Hour,
		},
		{
			name: "with end time - 30 minutes",
			timeframe: &Timeframe{
				StartTime: now,
				EndTime:   timePtr(now.Add(30 * time.Minute)),
			},
			expected: 30 * time.Minute,
		},
		{
			name: "no end time returns 0",
			timeframe: &Timeframe{
				StartTime: now,
				EndTime:   nil,
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.timeframe.Duration()
			if got != tt.expected {
				t.Errorf("Timeframe.Duration() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTimeframe_IsOpen(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	zeroTime := time.Time{}

	tests := []struct {
		name      string
		timeframe *Timeframe
		expected  bool
	}{
		{
			name: "open - nil end time",
			timeframe: &Timeframe{
				StartTime: now,
				EndTime:   nil,
			},
			expected: true,
		},
		{
			name: "open - zero end time",
			timeframe: &Timeframe{
				StartTime: now,
				EndTime:   &zeroTime,
			},
			expected: true,
		},
		{
			name: "closed - has end time",
			timeframe: &Timeframe{
				StartTime: now,
				EndTime:   &future,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.timeframe.IsOpen()
			if got != tt.expected {
				t.Errorf("Timeframe.IsOpen() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTimeframe_Contains(t *testing.T) {
	// Create a timeframe from 10:00 to 12:00
	start := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	closedTimeframe := &Timeframe{
		StartTime: start,
		EndTime:   &end,
	}

	openTimeframe := &Timeframe{
		StartTime: start,
		EndTime:   nil,
	}

	tests := []struct {
		name      string
		timeframe *Timeframe
		checkTime time.Time
		expected  bool
	}{
		{
			name:      "closed - time at start",
			timeframe: closedTimeframe,
			checkTime: start,
			expected:  true,
		},
		{
			name:      "closed - time at end",
			timeframe: closedTimeframe,
			checkTime: end,
			expected:  true,
		},
		{
			name:      "closed - time in middle",
			timeframe: closedTimeframe,
			checkTime: time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
			expected:  true,
		},
		{
			name:      "closed - time before start",
			timeframe: closedTimeframe,
			checkTime: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
			expected:  false,
		},
		{
			name:      "closed - time after end",
			timeframe: closedTimeframe,
			checkTime: time.Date(2024, 1, 15, 13, 0, 0, 0, time.UTC),
			expected:  false,
		},
		{
			name:      "open - time at start",
			timeframe: openTimeframe,
			checkTime: start,
			expected:  true,
		},
		{
			name:      "open - time after start",
			timeframe: openTimeframe,
			checkTime: time.Date(2024, 1, 15, 15, 0, 0, 0, time.UTC),
			expected:  true,
		},
		{
			name:      "open - time before start",
			timeframe: openTimeframe,
			checkTime: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.timeframe.Contains(tt.checkTime)
			if got != tt.expected {
				t.Errorf("Timeframe.Contains() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTimeframe_Overlaps(t *testing.T) {
	// Base timeframe: 10:00 - 12:00
	base := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	baseEnd := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	closedTimeframe := &Timeframe{
		StartTime: base,
		EndTime:   &baseEnd,
	}

	openTimeframe := &Timeframe{
		StartTime: base,
		EndTime:   nil,
	}

	tests := []struct {
		name     string
		tf1      *Timeframe
		tf2      *Timeframe
		expected bool
	}{
		{
			name: "closed - complete overlap",
			tf1:  closedTimeframe,
			tf2: &Timeframe{
				StartTime: base,
				EndTime:   &baseEnd,
			},
			expected: true,
		},
		{
			name: "closed - partial overlap at start",
			tf1:  closedTimeframe,
			tf2: &Timeframe{
				StartTime: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC),
				EndTime:   timePtr(time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)),
			},
			expected: true,
		},
		{
			name: "closed - partial overlap at end",
			tf1:  closedTimeframe,
			tf2: &Timeframe{
				StartTime: time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
				EndTime:   timePtr(time.Date(2024, 1, 15, 13, 0, 0, 0, time.UTC)),
			},
			expected: true,
		},
		{
			name: "closed - contained within",
			tf1:  closedTimeframe,
			tf2: &Timeframe{
				StartTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				EndTime:   timePtr(time.Date(2024, 1, 15, 11, 30, 0, 0, time.UTC)),
			},
			expected: true,
		},
		{
			name: "closed - no overlap before",
			tf1:  closedTimeframe,
			tf2: &Timeframe{
				StartTime: time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC),
				EndTime:   timePtr(time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)),
			},
			expected: false,
		},
		{
			name: "closed - no overlap after",
			tf1:  closedTimeframe,
			tf2: &Timeframe{
				StartTime: time.Date(2024, 1, 15, 13, 0, 0, 0, time.UTC),
				EndTime:   timePtr(time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)),
			},
			expected: false,
		},
		{
			name: "closed - adjacent (end meets start)",
			tf1:  closedTimeframe,
			tf2: &Timeframe{
				StartTime: baseEnd,
				EndTime:   timePtr(time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)),
			},
			expected: false,
		},
		{
			name: "open tf1 - other starts after",
			tf1:  openTimeframe,
			tf2: &Timeframe{
				StartTime: time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
				EndTime:   timePtr(time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)),
			},
			expected: true,
		},
		{
			name: "open tf1 - other starts before",
			tf1:  openTimeframe,
			tf2: &Timeframe{
				StartTime: time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC),
				EndTime:   timePtr(time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)),
			},
			expected: false, // Other ends before tf1 starts
		},
		{
			name: "closed tf1 - open tf2 starts before end",
			tf1:  closedTimeframe,
			tf2: &Timeframe{
				StartTime: time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
				EndTime:   nil,
			},
			expected: true,
		},
		{
			name: "closed tf1 - open tf2 starts after end",
			tf1:  closedTimeframe,
			tf2: &Timeframe{
				StartTime: time.Date(2024, 1, 15, 13, 0, 0, 0, time.UTC),
				EndTime:   nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tf1.Overlaps(tt.tf2)
			if got != tt.expected {
				t.Errorf("Timeframe.Overlaps() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTimeframe_TableName(t *testing.T) {
	tf := &Timeframe{}
	expected := "schedule.timeframes"

	got := tf.TableName()
	if got != expected {
		t.Errorf("Timeframe.TableName() = %q, want %q", got, expected)
	}
}

func TestTimeframe_EntityInterface(t *testing.T) {
	now := time.Now()
	tf := &Timeframe{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Hour),
		},
		StartTime: now,
	}

	t.Run("GetID", func(t *testing.T) {
		got := tf.GetID()
		if got != int64(123) {
			t.Errorf("Timeframe.GetID() = %v, want %v", got, int64(123))
		}
	})

	t.Run("GetCreatedAt", func(t *testing.T) {
		got := tf.GetCreatedAt()
		if !got.Equal(now) {
			t.Errorf("Timeframe.GetCreatedAt() = %v, want %v", got, now)
		}
	})

	t.Run("GetUpdatedAt", func(t *testing.T) {
		expected := now.Add(time.Hour)
		got := tf.GetUpdatedAt()
		if !got.Equal(expected) {
			t.Errorf("Timeframe.GetUpdatedAt() = %v, want %v", got, expected)
		}
	})
}

func TestTimeframe_IsActiveFlag(t *testing.T) {
	t.Run("default is false", func(t *testing.T) {
		tf := &Timeframe{
			StartTime: time.Now(),
		}

		if tf.IsActive != false {
			t.Error("Timeframe.IsActive should default to false")
		}
	})

	t.Run("can be set to true", func(t *testing.T) {
		tf := &Timeframe{
			StartTime: time.Now(),
			IsActive:  true,
		}

		if tf.IsActive != true {
			t.Error("Timeframe.IsActive should be true when set")
		}
	})
}

// Helper function for creating time pointers
func timePtr(t time.Time) *time.Time {
	return &t
}
