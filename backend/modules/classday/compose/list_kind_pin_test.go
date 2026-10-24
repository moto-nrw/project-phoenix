package compose_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
)

// TestListKindsMatchTimetable pins the list kinds the projection names to the
// values Timetable & Activities stores on templates and instances, so a list
// kind read from a block always selects its list.
func TestListKindsMatchTimetable(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []classday.ListKind{
		timetable.ListKindEdgeHours, timetable.ListKindLearningTime, timetable.ListKindActivity, timetable.ListKindMensa,
	}, classday.AllListKinds)
	for _, kind := range append([]classday.ListKind{classday.ListKindNone, "unknown"}, classday.AllListKinds...) {
		assert.Equal(t, timetable.IsValidListKind(string(kind)), kind.Valid(), "kind %q", kind)
	}
}

// TestSlotAttendanceExpectedMatchesTimetable pins the expected slot status the
// care-day rule compares to the value Timetable & Activities stores.
func TestSlotAttendanceExpectedMatchesTimetable(t *testing.T) {
	t.Parallel()

	assert.Equal(t, timetable.SlotAttendanceExpected, compose.SlotAttendanceExpected)
}
