package activities

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestIsValidWeekday(t *testing.T) {
	tests := []struct {
		name    string
		weekday int
		want    bool
	}{
		{
			name:    "Valid weekday - Monday",
			weekday: WeekdayMonday,
			want:    true,
		},
		{
			name:    "Valid weekday - Tuesday",
			weekday: WeekdayTuesday,
			want:    true,
		},
		{
			name:    "Valid weekday - Wednesday",
			weekday: WeekdayWednesday,
			want:    true,
		},
		{
			name:    "Valid weekday - Thursday",
			weekday: WeekdayThursday,
			want:    true,
		},
		{
			name:    "Valid weekday - Friday",
			weekday: WeekdayFriday,
			want:    true,
		},
		{
			name:    "Valid weekday - Saturday",
			weekday: WeekdaySaturday,
			want:    true,
		},
		{
			name:    "Valid weekday - Sunday",
			weekday: WeekdaySunday,
			want:    true,
		},
		{
			name:    "Invalid weekday - zero",
			weekday: 0,
			want:    false,
		},
		{
			name:    "Invalid weekday - negative",
			weekday: -1,
			want:    false,
		},
		{
			name:    "Invalid weekday - too high",
			weekday: 8,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidWeekday(tt.weekday); got != tt.want {
				t.Errorf("IsValidWeekday() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScheduleValidate(t *testing.T) {
	tests := []struct {
		name     string
		schedule *Schedule
		wantErr  bool
	}{
		{
			name: "Valid schedule with timeframe",
			schedule: &Schedule{
				Weekday:         WeekdayMonday,
				TimeframeID:     func() *int64 { id := int64(1); return &id }(),
				ActivityGroupID: 1,
			},
			wantErr: false,
		},
		{
			name: "Valid schedule without timeframe",
			schedule: &Schedule{
				Weekday:         WeekdayFriday,
				ActivityGroupID: 2,
			},
			wantErr: false,
		},
		{
			name: "Invalid weekday",
			schedule: &Schedule{
				Weekday:         99, // Invalid weekday value
				ActivityGroupID: 1,
			},
			wantErr: true,
		},
		{
			name: "Missing weekday",
			schedule: &Schedule{
				ActivityGroupID: 1,
			},
			wantErr: true,
		},
		{
			name: "Missing activity group ID",
			schedule: &Schedule{
				Weekday: WeekdayMonday,
			},
			wantErr: true,
		},
		{
			name: "Invalid activity group ID",
			schedule: &Schedule{
				Weekday:         WeekdayMonday,
				ActivityGroupID: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schedule.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Schedule.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestScheduleHasTimeframe(t *testing.T) {
	tests := []struct {
		name     string
		schedule *Schedule
		want     bool
	}{
		{
			name: "Has timeframe",
			schedule: &Schedule{
				TimeframeID: func() *int64 { id := int64(1); return &id }(),
			},
			want: true,
		},
		{
			name: "Has timeframe with zero value",
			schedule: &Schedule{
				TimeframeID: func() *int64 { id := int64(0); return &id }(),
			},
			want: false,
		},
		{
			name: "No timeframe",
			schedule: &Schedule{
				TimeframeID: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.schedule.HasTimeframe(); got != tt.want {
				t.Errorf("Schedule.HasTimeframe() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScheduleTableName(t *testing.T) {
	schedule := &Schedule{}
	expected := "activities.schedules"

	if got := schedule.TableName(); got != expected {
		t.Errorf("Schedule.TableName() = %v, want %v", got, expected)
	}
}

func TestSchedule_EntityInterface(t *testing.T) {
	now := time.Now()
	schedule := &Schedule{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Hour),
		},
		Weekday:         WeekdayMonday,
		ActivityGroupID: 1,
	}

	t.Run("GetID", func(t *testing.T) {
		got := schedule.GetID()
		if got != int64(123) {
			t.Errorf("Schedule.GetID() = %v, want %v", got, int64(123))
		}
	})

	t.Run("GetCreatedAt", func(t *testing.T) {
		got := schedule.GetCreatedAt()
		if !got.Equal(now) {
			t.Errorf("Schedule.GetCreatedAt() = %v, want %v", got, now)
		}
	})

	t.Run("GetUpdatedAt", func(t *testing.T) {
		expected := now.Add(time.Hour)
		got := schedule.GetUpdatedAt()
		if !got.Equal(expected) {
			t.Errorf("Schedule.GetUpdatedAt() = %v, want %v", got, expected)
		}
	})
}
