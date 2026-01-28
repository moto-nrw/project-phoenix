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

func TestSchedule_TableName(t *testing.T) {
	schedule := &Schedule{}
	if got := schedule.TableName(); got != "activities.schedules" {
		t.Errorf("TableName() = %v, want activities.schedules", got)
	}
}

func TestSchedule_BeforeAppendModel(t *testing.T) {
	t.Run("handles nil query", func(t *testing.T) {
		schedule := &Schedule{Weekday: WeekdayMonday, ActivityGroupID: 1}
		err := schedule.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		schedule := &Schedule{Weekday: WeekdayMonday, ActivityGroupID: 1}
		err := schedule.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestSchedule_GetID(t *testing.T) {
	schedule := &Schedule{
		Model:           base.Model{ID: 42},
		Weekday:         WeekdayMonday,
		ActivityGroupID: 1,
	}

	if got, ok := schedule.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", schedule.GetID())
	}
}

func TestSchedule_GetCreatedAt(t *testing.T) {
	now := time.Now()
	schedule := &Schedule{
		Model:           base.Model{CreatedAt: now},
		Weekday:         WeekdayMonday,
		ActivityGroupID: 1,
	}

	if got := schedule.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestSchedule_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	schedule := &Schedule{
		Model:           base.Model{UpdatedAt: now},
		Weekday:         WeekdayMonday,
		ActivityGroupID: 1,
	}

	if got := schedule.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
