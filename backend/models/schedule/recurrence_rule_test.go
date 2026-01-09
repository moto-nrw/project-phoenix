package schedule

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestRecurrenceRule_Validate(t *testing.T) {
	count5 := 5
	count0 := 0
	negativeCount := -1
	future := time.Now().AddDate(0, 0, 30)

	tests := []struct {
		name    string
		rule    *RecurrenceRule
		wantErr bool
	}{
		{
			name: "valid daily",
			rule: &RecurrenceRule{
				Frequency:     FrequencyDaily,
				IntervalCount: 1,
			},
			wantErr: false,
		},
		{
			name: "valid weekly",
			rule: &RecurrenceRule{
				Frequency:     FrequencyWeekly,
				IntervalCount: 1,
			},
			wantErr: false,
		},
		{
			name: "valid monthly",
			rule: &RecurrenceRule{
				Frequency:     FrequencyMonthly,
				IntervalCount: 1,
			},
			wantErr: false,
		},
		{
			name: "valid yearly",
			rule: &RecurrenceRule{
				Frequency:     FrequencyYearly,
				IntervalCount: 1,
			},
			wantErr: false,
		},
		{
			name: "valid with weekdays",
			rule: &RecurrenceRule{
				Frequency:     FrequencyWeekly,
				IntervalCount: 1,
				Weekdays:      []string{"MON", "WED", "FRI"},
			},
			wantErr: false,
		},
		{
			name: "valid with month days",
			rule: &RecurrenceRule{
				Frequency:     FrequencyMonthly,
				IntervalCount: 1,
				MonthDays:     []int{1, 15, 31},
			},
			wantErr: false,
		},
		{
			name: "valid with end date",
			rule: &RecurrenceRule{
				Frequency:     FrequencyDaily,
				IntervalCount: 1,
				EndDate:       &future,
			},
			wantErr: false,
		},
		{
			name: "valid with count",
			rule: &RecurrenceRule{
				Frequency:     FrequencyDaily,
				IntervalCount: 1,
				Count:         &count5,
			},
			wantErr: false,
		},
		{
			name: "invalid frequency",
			rule: &RecurrenceRule{
				Frequency:     "invalid",
				IntervalCount: 1,
			},
			wantErr: true,
		},
		{
			name: "invalid frequency - empty",
			rule: &RecurrenceRule{
				Frequency:     "",
				IntervalCount: 1,
			},
			wantErr: true,
		},
		{
			name: "invalid interval count - zero",
			rule: &RecurrenceRule{
				Frequency:     FrequencyDaily,
				IntervalCount: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid interval count - negative",
			rule: &RecurrenceRule{
				Frequency:     FrequencyDaily,
				IntervalCount: -1,
			},
			wantErr: true,
		},
		{
			name: "invalid weekday",
			rule: &RecurrenceRule{
				Frequency:     FrequencyWeekly,
				IntervalCount: 1,
				Weekdays:      []string{"MON", "INVALID"},
			},
			wantErr: true,
		},
		{
			name: "invalid month day - zero",
			rule: &RecurrenceRule{
				Frequency:     FrequencyMonthly,
				IntervalCount: 1,
				MonthDays:     []int{0, 15},
			},
			wantErr: true,
		},
		{
			name: "invalid month day - too high",
			rule: &RecurrenceRule{
				Frequency:     FrequencyMonthly,
				IntervalCount: 1,
				MonthDays:     []int{15, 32},
			},
			wantErr: true,
		},
		{
			name: "invalid count - zero",
			rule: &RecurrenceRule{
				Frequency:     FrequencyDaily,
				IntervalCount: 1,
				Count:         &count0,
			},
			wantErr: true,
		},
		{
			name: "invalid count - negative",
			rule: &RecurrenceRule{
				Frequency:     FrequencyDaily,
				IntervalCount: 1,
				Count:         &negativeCount,
			},
			wantErr: true,
		},
		{
			name: "invalid - both end date and count",
			rule: &RecurrenceRule{
				Frequency:     FrequencyDaily,
				IntervalCount: 1,
				EndDate:       &future,
				Count:         &count5,
			},
			wantErr: true,
		},
		{
			name: "normalize frequency to lowercase",
			rule: &RecurrenceRule{
				Frequency:     "DAILY",
				IntervalCount: 1,
			},
			wantErr: false,
		},
		{
			name: "normalize weekdays to uppercase",
			rule: &RecurrenceRule{
				Frequency:     FrequencyWeekly,
				IntervalCount: 1,
				Weekdays:      []string{"mon", "tue"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("RecurrenceRule.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check normalization
			if tt.name == "normalize frequency to lowercase" && tt.rule.Frequency != FrequencyDaily {
				t.Errorf("RecurrenceRule.Validate() failed to normalize frequency, got %v", tt.rule.Frequency)
			}

			if tt.name == "normalize weekdays to uppercase" {
				for _, day := range tt.rule.Weekdays {
					if day != "MON" && day != "TUE" {
						t.Errorf("RecurrenceRule.Validate() failed to normalize weekday, got %v", day)
					}
				}
			}
		})
	}
}

func TestRecurrenceRule_TableName(t *testing.T) {
	rule := &RecurrenceRule{}
	expected := "schedule.recurrence_rules"

	if got := rule.TableName(); got != expected {
		t.Errorf("RecurrenceRule.TableName() = %q, want %q", got, expected)
	}
}

func TestRecurrenceRule_IsFinite(t *testing.T) {
	count5 := 5
	future := time.Now().AddDate(0, 0, 30)

	tests := []struct {
		name     string
		rule     *RecurrenceRule
		expected bool
	}{
		{
			name: "finite - has end date",
			rule: &RecurrenceRule{
				EndDate: &future,
			},
			expected: true,
		},
		{
			name: "finite - has count",
			rule: &RecurrenceRule{
				Count: &count5,
			},
			expected: true,
		},
		{
			name:     "infinite - no end date or count",
			rule:     &RecurrenceRule{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rule.IsFinite(); got != tt.expected {
				t.Errorf("RecurrenceRule.IsFinite() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRecurrenceRule_IsWeekdayBased(t *testing.T) {
	tests := []struct {
		name     string
		rule     *RecurrenceRule
		expected bool
	}{
		{
			name: "weekday based - weekly with weekdays",
			rule: &RecurrenceRule{
				Frequency: FrequencyWeekly,
				Weekdays:  []string{"MON", "WED"},
			},
			expected: true,
		},
		{
			name: "not weekday based - weekly without weekdays",
			rule: &RecurrenceRule{
				Frequency: FrequencyWeekly,
				Weekdays:  []string{},
			},
			expected: false,
		},
		{
			name: "not weekday based - daily with weekdays",
			rule: &RecurrenceRule{
				Frequency: FrequencyDaily,
				Weekdays:  []string{"MON", "WED"},
			},
			expected: false,
		},
		{
			name: "not weekday based - monthly",
			rule: &RecurrenceRule{
				Frequency: FrequencyMonthly,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rule.IsWeekdayBased(); got != tt.expected {
				t.Errorf("RecurrenceRule.IsWeekdayBased() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRecurrenceRule_IsMonthDayBased(t *testing.T) {
	tests := []struct {
		name     string
		rule     *RecurrenceRule
		expected bool
	}{
		{
			name: "month day based - monthly with month days",
			rule: &RecurrenceRule{
				Frequency: FrequencyMonthly,
				MonthDays: []int{1, 15},
			},
			expected: true,
		},
		{
			name: "not month day based - monthly without month days",
			rule: &RecurrenceRule{
				Frequency: FrequencyMonthly,
				MonthDays: []int{},
			},
			expected: false,
		},
		{
			name: "not month day based - weekly with month days",
			rule: &RecurrenceRule{
				Frequency: FrequencyWeekly,
				MonthDays: []int{1, 15},
			},
			expected: false,
		},
		{
			name: "not month day based - daily",
			rule: &RecurrenceRule{
				Frequency: FrequencyDaily,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rule.IsMonthDayBased(); got != tt.expected {
				t.Errorf("RecurrenceRule.IsMonthDayBased() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRecurrenceRule_Clone(t *testing.T) {
	count5 := 5
	future := time.Now().AddDate(0, 0, 30)
	now := time.Now()

	original := &RecurrenceRule{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Hour),
		},
		Frequency:     FrequencyWeekly,
		IntervalCount: 2,
		Weekdays:      []string{"MON", "WED", "FRI"},
		MonthDays:     []int{1, 15},
		EndDate:       &future,
		Count:         &count5,
	}

	clone := original.Clone()

	t.Run("basic fields cloned", func(t *testing.T) {
		if clone.ID != original.ID {
			t.Errorf("Clone().ID = %v, want %v", clone.ID, original.ID)
		}
		if clone.Frequency != original.Frequency {
			t.Errorf("Clone().Frequency = %v, want %v", clone.Frequency, original.Frequency)
		}
		if clone.IntervalCount != original.IntervalCount {
			t.Errorf("Clone().IntervalCount = %v, want %v", clone.IntervalCount, original.IntervalCount)
		}
	})

	t.Run("weekdays cloned - not same slice", func(t *testing.T) {
		if len(clone.Weekdays) != len(original.Weekdays) {
			t.Errorf("Clone().Weekdays length = %v, want %v", len(clone.Weekdays), len(original.Weekdays))
		}
		// Modify clone's weekdays - shouldn't affect original
		clone.Weekdays[0] = "TUE"
		if original.Weekdays[0] == "TUE" {
			t.Error("Clone().Weekdays should be a deep copy")
		}
	})

	t.Run("month days cloned - not same slice", func(t *testing.T) {
		if len(clone.MonthDays) != len(original.MonthDays) {
			t.Errorf("Clone().MonthDays length = %v, want %v", len(clone.MonthDays), len(original.MonthDays))
		}
		// Modify clone's month days - shouldn't affect original
		clone.MonthDays[0] = 99
		if original.MonthDays[0] == 99 {
			t.Error("Clone().MonthDays should be a deep copy")
		}
	})

	t.Run("end date cloned - not same pointer", func(t *testing.T) {
		if clone.EndDate == nil {
			t.Error("Clone().EndDate should not be nil")
		}
		if clone.EndDate == original.EndDate {
			t.Error("Clone().EndDate should be a different pointer")
		}
		if !clone.EndDate.Equal(*original.EndDate) {
			t.Error("Clone().EndDate should have same value")
		}
	})

	t.Run("count cloned - not same pointer", func(t *testing.T) {
		if clone.Count == nil {
			t.Error("Clone().Count should not be nil")
		}
		if clone.Count == original.Count {
			t.Error("Clone().Count should be a different pointer")
		}
		if *clone.Count != *original.Count {
			t.Error("Clone().Count should have same value")
		}
	})
}

func TestRecurrenceRule_Clone_EmptySlices(t *testing.T) {
	original := &RecurrenceRule{
		Frequency:     FrequencyDaily,
		IntervalCount: 1,
	}

	clone := original.Clone()

	if clone.Weekdays != nil {
		t.Error("Clone().Weekdays should be nil when original is empty")
	}
	if clone.MonthDays != nil {
		t.Error("Clone().MonthDays should be nil when original is empty")
	}
	if clone.EndDate != nil {
		t.Error("Clone().EndDate should be nil when original is nil")
	}
	if clone.Count != nil {
		t.Error("Clone().Count should be nil when original is nil")
	}
}
