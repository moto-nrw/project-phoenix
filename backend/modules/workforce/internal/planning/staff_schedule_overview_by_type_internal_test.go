package planning

// Unit tests for the per-Schichtart split of the weekly planned minutes
// (#3819). int64 literals are fake in-memory IDs, not DB rows.

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func typedShift(t *testing.T, staffID int64, date timezone.Date, start, end string, typeID int64) *StaffShift {
	t.Helper()
	shift := testShift(t, staffID, date, start, end)
	shift.ShiftTypeID = &typeID
	return shift
}

// The split adds up to the weekly total: cancelled shifts drop out of both,
// breaks are deducted per type, a weekend shift lands in its own week, and
// shifts without a Schichtart collect in one nil entry.
func TestPlannedShiftMinutesByType_SplitsTheWeeklyTotal(t *testing.T) {
	t.Parallel()

	monday := timezone.NewDate(2026, 9, 28)
	ganztag := typedShift(t, 7, monday, "08:00", "14:00", 2) // 360
	ganztag.BreakMinutes = 30                                // → 330
	vertretung := typedShift(t, 7, monday.AddDays(1), "10:00", "12:00", 9)
	verfuegung := typedShift(t, 7, monday.AddDays(5), "09:00", "10:30", 4) // Saturday, same week
	untyped := testShift(t, 7, monday.AddDays(2), "13:00", "14:00")
	cancelled := typedShift(t, 7, monday.AddDays(3), "08:00", "16:00", 2)
	cancelled.Cancelled = true
	otherStaff := typedShift(t, 8, monday, "08:00", "09:00", 2)

	shifts := []*StaffShift{ganztag, vertretung, verfuegung, untyped, cancelled, otherStaff}
	byType := plannedShiftMinutesByType(shifts)
	key := staffDateKey{StaffID: 7, Date: monday}

	breakdown := shiftTypeBreakdown(byType[key])
	require.Len(t, breakdown, 4)
	got := make([][2]int64, 0, len(breakdown))
	total := 0
	for _, entry := range breakdown {
		id := int64(0)
		if entry.ShiftTypeID != nil {
			id = *entry.ShiftTypeID
		}
		got = append(got, [2]int64{id, int64(entry.Minutes)})
		total += entry.Minutes
	}
	// Ordered by Schichtart id, the shifts without one last.
	assert.Equal(t, [][2]int64{{2, 330}, {4, 90}, {9, 120}, {0, 60}}, got)
	assert.Nil(t, breakdown[3].ShiftTypeID)
	assert.Equal(t, plannedShiftMinutes(shifts)[key], total,
		"the per-type entries must add up to the weekly planned minutes")

	assert.Equal(t, map[int64]int{2: 60}, byType[staffDateKey{StaffID: 8, Date: monday}])
}

func TestShiftTypeBreakdown_EmptyWeekHasNoEntries(t *testing.T) {
	t.Parallel()

	assert.Nil(t, shiftTypeBreakdown(nil))
}
