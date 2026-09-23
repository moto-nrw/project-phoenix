package timetableplanning

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInstanceSeriesConversionService_PanicsOnNilDeps(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		_ = NewInstanceSeriesConversionService(InstanceSeriesConversionDependencies{})
	})
}

func TestConvertInstanceToSeries_RejectsInvalidInput(t *testing.T) {
	zeroDate := timezone.Date("")
	t.Parallel()

	svc := &instanceSeriesConversionService{}
	date := timezone.NewDate(2026, time.May, 4)
	periodID := int64(11)

	tests := []struct {
		name string
		ctx  context.Context
		in   ConvertInstanceToSeriesInput
		want string
	}{
		{
			name: "non-positive instance id",
			ctx:  tenant.WithTenantID(context.Background(), 5),
			in: ConvertInstanceToSeriesInput{
				InstanceID: 0,
				Template: timetable.CreateTemplateCommand{
					ScheduleValidFrom: &date,
					CalendarPeriodID:  &periodID,
				},
			},
			want: "instance id must be positive",
		},
		{
			name: "missing start date",
			ctx:  tenant.WithTenantID(context.Background(), 5),
			in: ConvertInstanceToSeriesInput{
				InstanceID: 9,
				Template: timetable.CreateTemplateCommand{
					CalendarPeriodID: &periodID,
				},
			},
			want: "start date is required",
		},
		{
			name: "zero start date",
			ctx:  tenant.WithTenantID(context.Background(), 5),
			in: ConvertInstanceToSeriesInput{
				InstanceID: 9,
				Template: timetable.CreateTemplateCommand{
					ScheduleValidFrom: &zeroDate,
					CalendarPeriodID:  &periodID,
				},
			},
			want: "start date is required",
		},
		{
			name: "missing calendar period",
			ctx:  tenant.WithTenantID(context.Background(), 5),
			in: ConvertInstanceToSeriesInput{
				InstanceID: 9,
				Template: timetable.CreateTemplateCommand{
					ScheduleValidFrom: &date,
				},
			},
			want: "calendar period is required",
		},
		{
			name: "non-positive calendar period",
			ctx:  tenant.WithTenantID(context.Background(), 5),
			in: ConvertInstanceToSeriesInput{
				InstanceID: 9,
				Template: timetable.CreateTemplateCommand{
					ScheduleValidFrom: &date,
					CalendarPeriodID:  new(int64),
				},
			},
			want: "calendar period is required",
		},
		{
			name: "missing tenant",
			ctx:  context.Background(),
			in: ConvertInstanceToSeriesInput{
				InstanceID: 9,
				Template: timetable.CreateTemplateCommand{
					ScheduleValidFrom: &date,
					CalendarPeriodID:  &periodID,
				},
			},
			want: "no tenant in context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.ConvertInstanceToSeries(tt.ctx, tt.in)
			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}
