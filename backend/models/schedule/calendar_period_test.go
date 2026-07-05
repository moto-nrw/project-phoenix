package schedule

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalendarPeriod_Validate(t *testing.T) {
	anchor := timezone.NewDate(2025, 9, 1)

	tests := []struct {
		name    string
		period  *CalendarPeriod
		wantErr string
	}{
		{
			name: "valid period without cycle",
			period: &CalendarPeriod{
				Name:            "Schuljahr 2025/2026",
				PeriodType:      PeriodTypeSchoolYear,
				StartDate:       timezone.NewDate(2025, 8, 1),
				EndDate:         timezone.NewDate(2026, 7, 31),
				WeekCycleLength: 1,
			},
			wantErr: "",
		},
		{
			name: "valid period with A/B cycle",
			period: &CalendarPeriod{
				Name:            "Schuljahr 2025/2026",
				PeriodType:      PeriodTypeSchoolYear,
				StartDate:       timezone.NewDate(2025, 8, 1),
				EndDate:         timezone.NewDate(2026, 7, 31),
				WeekCycleLength: 2,
				WeekCycleAnchor: &anchor,
			},
			wantErr: "",
		},
		{
			name: "missing name",
			period: &CalendarPeriod{
				PeriodType:      PeriodTypeSchoolYear,
				StartDate:       timezone.NewDate(2025, 8, 1),
				EndDate:         timezone.NewDate(2026, 7, 31),
				WeekCycleLength: 1,
			},
			wantErr: "name is required",
		},
		{
			name: "invalid period type",
			period: &CalendarPeriod{
				Name:            "Test",
				PeriodType:      "invalid",
				StartDate:       timezone.NewDate(2025, 8, 1),
				EndDate:         timezone.NewDate(2026, 7, 31),
				WeekCycleLength: 1,
			},
			wantErr: "invalid period type",
		},
		{
			name: "end date before start date",
			period: &CalendarPeriod{
				Name:            "Test",
				PeriodType:      PeriodTypeSemester,
				StartDate:       timezone.NewDate(2026, 7, 31),
				EndDate:         timezone.NewDate(2025, 8, 1),
				WeekCycleLength: 1,
			},
			wantErr: "end_date must be after start_date",
		},
		{
			name: "same start and end date",
			period: &CalendarPeriod{
				Name:            "Test",
				PeriodType:      PeriodTypeSemester,
				StartDate:       timezone.NewDate(2025, 8, 1),
				EndDate:         timezone.NewDate(2025, 8, 1),
				WeekCycleLength: 1,
			},
			wantErr: "end_date must be after start_date",
		},
		{
			name: "cycle length > 1 without anchor",
			period: &CalendarPeriod{
				Name:            "Test",
				PeriodType:      PeriodTypeSchoolYear,
				StartDate:       timezone.NewDate(2025, 8, 1),
				EndDate:         timezone.NewDate(2026, 7, 31),
				WeekCycleLength: 2,
			},
			wantErr: "week_cycle_anchor is required when week_cycle_length > 1",
		},
		{
			name: "zero cycle length",
			period: &CalendarPeriod{
				Name:            "Test",
				PeriodType:      PeriodTypeSchoolYear,
				StartDate:       timezone.NewDate(2025, 8, 1),
				EndDate:         timezone.NewDate(2026, 7, 31),
				WeekCycleLength: 0,
			},
			wantErr: "week_cycle_length must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.period.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestIsValidPeriodType(t *testing.T) {
	assert.True(t, IsValidPeriodType(PeriodTypeSchoolYear))
	assert.True(t, IsValidPeriodType(PeriodTypeSemester))
	assert.True(t, IsValidPeriodType(PeriodTypeHoliday))
	assert.True(t, IsValidPeriodType(PeriodTypeCustom))
	assert.False(t, IsValidPeriodType("invalid"))
	assert.False(t, IsValidPeriodType(""))
}

func TestCalendarPeriod_HasWeekCycle(t *testing.T) {
	p := &CalendarPeriod{WeekCycleLength: 1}
	assert.False(t, p.HasWeekCycle())

	p.WeekCycleLength = 2
	assert.True(t, p.HasWeekCycle())
}

func TestCalendarPeriod_ContainsDate(t *testing.T) {
	p := &CalendarPeriod{
		StartDate: timezone.NewDate(2025, 8, 1),
		EndDate:   timezone.NewDate(2026, 7, 31),
	}

	assert.True(t, p.ContainsDate(time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC)))   // start inclusive
	assert.True(t, p.ContainsDate(time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)))  // end inclusive
	assert.True(t, p.ContainsDate(time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC))) // middle
	assert.False(t, p.ContainsDate(time.Date(2025, 7, 31, 0, 0, 0, 0, time.UTC))) // before
	assert.False(t, p.ContainsDate(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)))  // after
}

func TestCalendarPeriod_GetID(t *testing.T) {
	p := &CalendarPeriod{}
	p.ID = 42
	assert.Equal(t, int64(42), p.GetID())
}

func TestCalendarPeriod_GetCreatedAt(t *testing.T) {
	now := time.Now()
	p := &CalendarPeriod{}
	p.CreatedAt = now
	assert.Equal(t, now, p.GetCreatedAt())
}

func TestCalendarPeriod_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	p := &CalendarPeriod{}
	p.UpdatedAt = now
	assert.Equal(t, now, p.GetUpdatedAt())
}
