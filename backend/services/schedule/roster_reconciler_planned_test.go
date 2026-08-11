package schedule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/models/schedule"
)

func TestInstanceRowIsStillPlanned(t *testing.T) {
	t.Parallel()

	now := time.Now()
	exceptionID := int64(9)
	statusDayID := int64(3)

	tests := []struct {
		name string
		row  *schedule.InstanceStudent
		want bool
	}{
		{
			name: "expected",
			row:  &schedule.InstanceStudent{Status: schedule.AttendanceStatusExpected},
			want: true,
		},
		{
			name: "full-day status provenance",
			row: &schedule.InstanceStudent{
				Status:             schedule.AttendanceStatusAbsent,
				StudentStatusDayID: &statusDayID,
			},
			want: true,
		},
		{
			name: "partial absence provenance",
			row: &schedule.InstanceStudent{
				Status:            schedule.AttendanceStatusAbsent,
				PickupExceptionID: &exceptionID,
			},
			want: true,
		},
		{
			name: "manual absence is not a pure plan",
			row: &schedule.InstanceStudent{
				Status:         schedule.AttendanceStatusAbsent,
				ManualStatusAt: &now,
			},
			want: false,
		},
		{
			name: "observed presence is not a pure plan",
			row: &schedule.InstanceStudent{
				Status:      schedule.AttendanceStatusPresent,
				CheckedInAt: &now,
			},
			want: false,
		},
		{
			name: "walk-in is not a pure plan",
			row: &schedule.InstanceStudent{
				Status:      schedule.AttendanceStatusExpected,
				IsUnplanned: true,
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, instanceRowIsStillPlanned(tc.row))
		})
	}
}
