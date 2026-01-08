package schedule

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestDateframe_Validate(t *testing.T) {
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)
	yesterday := now.AddDate(0, 0, -1)

	tests := []struct {
		name      string
		dateframe *Dateframe
		wantErr   bool
	}{
		{
			name: "valid dateframe",
			dateframe: &Dateframe{
				StartDate: now,
				EndDate:   tomorrow,
			},
			wantErr: false,
		},
		{
			name: "valid single day",
			dateframe: &Dateframe{
				StartDate: now,
				EndDate:   now,
			},
			wantErr: false,
		},
		{
			name: "valid with name and description",
			dateframe: &Dateframe{
				StartDate:   now,
				EndDate:     tomorrow,
				Name:        "School Week",
				Description: "Regular school week schedule",
			},
			wantErr: false,
		},
		{
			name: "zero start date",
			dateframe: &Dateframe{
				StartDate: time.Time{},
				EndDate:   now,
			},
			wantErr: true,
		},
		{
			name: "zero end date",
			dateframe: &Dateframe{
				StartDate: now,
				EndDate:   time.Time{},
			},
			wantErr: true,
		},
		{
			name: "end before start",
			dateframe: &Dateframe{
				StartDate: now,
				EndDate:   yesterday,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dateframe.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Dateframe.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDateframe_Duration(t *testing.T) {
	tests := []struct {
		name      string
		dateframe *Dateframe
		expected  time.Duration
	}{
		{
			name: "one day",
			dateframe: &Dateframe{
				StartDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
			},
			expected: 24 * time.Hour,
		},
		{
			name: "one week",
			dateframe: &Dateframe{
				StartDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 1, 22, 0, 0, 0, 0, time.UTC),
			},
			expected: 7 * 24 * time.Hour,
		},
		{
			name: "same day",
			dateframe: &Dateframe{
				StartDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			},
			expected: 0,
		},
		{
			name: "partial day",
			dateframe: &Dateframe{
				StartDate: time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 1, 15, 16, 0, 0, 0, time.UTC),
			},
			expected: 8 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dateframe.Duration()
			if got != tt.expected {
				t.Errorf("Dateframe.Duration() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDateframe_DaysCount(t *testing.T) {
	tests := []struct {
		name      string
		dateframe *Dateframe
		expected  int
	}{
		{
			name: "one day",
			dateframe: &Dateframe{
				StartDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
			},
			expected: 2, // Includes both start and end dates
		},
		{
			name: "same day",
			dateframe: &Dateframe{
				StartDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			},
			expected: 1, // Same day counts as 1
		},
		{
			name: "one week",
			dateframe: &Dateframe{
				StartDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 1, 21, 0, 0, 0, 0, time.UTC),
			},
			expected: 7, // Mon-Sun = 7 days
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dateframe.DaysCount()
			if got != tt.expected {
				t.Errorf("Dateframe.DaysCount() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDateframe_Contains(t *testing.T) {
	// Create a dateframe: Jan 15-20, 2024
	start := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC)

	dateframe := &Dateframe{
		StartDate: start,
		EndDate:   end,
	}

	tests := []struct {
		name      string
		checkDate time.Time
		expected  bool
	}{
		{
			name:      "date at start",
			checkDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			expected:  true,
		},
		{
			name:      "date at end",
			checkDate: time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
			expected:  true,
		},
		{
			name:      "date in middle",
			checkDate: time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC),
			expected:  true,
		},
		{
			name:      "date in middle with time component",
			checkDate: time.Date(2024, 1, 17, 14, 30, 0, 0, time.UTC),
			expected:  true, // Time is ignored, only date matters
		},
		{
			name:      "date before start",
			checkDate: time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC),
			expected:  false,
		},
		{
			name:      "date after end",
			checkDate: time.Date(2024, 1, 21, 0, 0, 0, 0, time.UTC),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dateframe.Contains(tt.checkDate)
			if got != tt.expected {
				t.Errorf("Dateframe.Contains() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDateframe_Overlaps(t *testing.T) {
	// Base dateframe: Jan 15-20, 2024
	base := &Dateframe{
		StartDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
	}

	tests := []struct {
		name     string
		other    *Dateframe
		expected bool
	}{
		{
			name: "complete overlap (same range)",
			other: &Dateframe{
				StartDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
			},
			expected: true,
		},
		{
			name: "partial overlap at start",
			other: &Dateframe{
				StartDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC),
			},
			expected: true,
		},
		{
			name: "partial overlap at end",
			other: &Dateframe{
				StartDate: time.Date(2024, 1, 18, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 1, 25, 0, 0, 0, 0, time.UTC),
			},
			expected: true,
		},
		{
			name: "contained within base",
			other: &Dateframe{
				StartDate: time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 1, 19, 0, 0, 0, 0, time.UTC),
			},
			expected: true,
		},
		{
			name: "contains base",
			other: &Dateframe{
				StartDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 1, 25, 0, 0, 0, 0, time.UTC),
			},
			expected: true,
		},
		{
			name: "adjacent at end (touching)",
			other: &Dateframe{
				StartDate: time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 1, 25, 0, 0, 0, 0, time.UTC),
			},
			expected: true, // Equal dates count as overlapping
		},
		{
			name: "adjacent at start (touching)",
			other: &Dateframe{
				StartDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			},
			expected: true, // Equal dates count as overlapping
		},
		{
			name: "no overlap - before",
			other: &Dateframe{
				StartDate: time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
			},
			expected: false,
		},
		{
			name: "no overlap - after",
			other: &Dateframe{
				StartDate: time.Date(2024, 1, 25, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 1, 30, 0, 0, 0, 0, time.UTC),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := base.Overlaps(tt.other)
			if got != tt.expected {
				t.Errorf("Dateframe.Overlaps() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDateframe_TableName(t *testing.T) {
	df := &Dateframe{}
	expected := "schedule.dateframes"

	got := df.TableName()
	if got != expected {
		t.Errorf("Dateframe.TableName() = %q, want %q", got, expected)
	}
}

func TestDateframe_EntityInterface(t *testing.T) {
	now := time.Now()
	df := &Dateframe{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Hour),
		},
		StartDate: now,
		EndDate:   now.AddDate(0, 0, 7),
	}

	t.Run("GetID", func(t *testing.T) {
		got := df.GetID()
		if got != int64(123) {
			t.Errorf("Dateframe.GetID() = %v, want %v", got, int64(123))
		}
	})

	t.Run("GetCreatedAt", func(t *testing.T) {
		got := df.GetCreatedAt()
		if !got.Equal(now) {
			t.Errorf("Dateframe.GetCreatedAt() = %v, want %v", got, now)
		}
	})

	t.Run("GetUpdatedAt", func(t *testing.T) {
		expected := now.Add(time.Hour)
		got := df.GetUpdatedAt()
		if !got.Equal(expected) {
			t.Errorf("Dateframe.GetUpdatedAt() = %v, want %v", got, expected)
		}
	})
}
