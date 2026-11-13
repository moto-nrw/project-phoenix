package application

import (
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A return from class is no new arrival (#5656); a return after going home stays
// one, as before.
func TestAppendSchoolStatusesKeepsTheArrivalAcrossClass(t *testing.T) {
	t.Parallel()

	const child = int64(7)
	at := func(hour, minute int) time.Time { return time.Date(2026, 9, 29, hour, minute, 0, 0, time.UTC) }
	class := studentpresence.CheckoutReasonClass
	stay := func(staff int64, in, out time.Time, reason *string) *ports.Attendance {
		return &ports.Attendance{StudentID: child, CheckInTime: in, CheckOutTime: &out, CheckedInBy: staff, CheckOutReason: reason}
	}
	open := func(staff int64, in time.Time) *ports.Attendance {
		return &ports.Attendance{StudentID: child, CheckInTime: in, CheckedInBy: staff}
	}
	cases := []struct {
		name     string
		rows     []*ports.Attendance // newest first, as the store delivers them
		arrival  time.Time
		recorder int64
		checkIn  time.Time
	}{
		{
			name:    "single stay",
			rows:    []*ports.Attendance{open(1, at(7, 30))},
			arrival: at(7, 30), recorder: 1, checkIn: at(7, 30),
		},
		{
			name:    "back from class keeps the first arrival",
			rows:    []*ports.Attendance{open(2, at(13, 0)), stay(1, at(7, 30), at(8, 30), &class)},
			arrival: at(7, 30), recorder: 1, checkIn: at(13, 0),
		},
		{
			name:    "still in class keeps its own arrival",
			rows:    []*ports.Attendance{stay(1, at(7, 30), at(8, 30), &class)},
			arrival: at(7, 30), recorder: 1, checkIn: at(7, 30),
		},
		{
			name:    "back after going home is a new arrival",
			rows:    []*ports.Attendance{open(2, at(13, 0)), stay(1, at(7, 30), at(8, 30), nil)},
			arrival: at(13, 0), recorder: 2, checkIn: at(13, 0),
		},
		{
			name: "class after a home return reaches back only to the home return",
			rows: []*ports.Attendance{
				open(3, at(15, 0)), stay(2, at(12, 0), at(13, 0), &class), stay(1, at(7, 30), at(8, 30), nil),
			},
			arrival: at(12, 0), recorder: 2, checkIn: at(15, 0),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			statuses := appendSchoolStatuses(nil, []int64{child}, "2026-09-29", tc.rows)
			require.Len(t, statuses, 1)
			require.NotNil(t, statuses[0].ArrivalTime)
			assert.Equal(t, tc.arrival, *statuses[0].ArrivalTime)
			assert.Equal(t, tc.recorder, statuses[0].CheckedInBy, "the actor line belongs to the arrival")
			require.NotNil(t, statuses[0].CheckInTime)
			assert.Equal(t, tc.checkIn, *statuses[0].CheckInTime, "the stay itself is unchanged")
		})
	}
}
