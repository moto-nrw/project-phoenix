package schedule

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func shiftWall(hour, minute int) time.Time {
	return time.Date(1, 1, 1, hour, minute, 0, 0, time.UTC)
}

func testShift(startHour, endHour int) *StaffShift {
	return &StaffShift{
		StaffID:   7,
		Date:      timezone.NewDate(2026, time.July, 6),
		StartTime: shiftWall(startHour, 0),
		EndTime:   shiftWall(endHour, 0),
		CreatedBy: 1,
	}
}

func TestStaffShiftValidate(t *testing.T) {
	t.Parallel()

	require.NoError(t, testShift(8, 16).Validate())

	missingStaff := testShift(8, 16)
	missingStaff.StaffID = 0
	assert.Error(t, missingStaff.Validate())

	missingDate := testShift(8, 16)
	missingDate.Date = timezone.Date{}
	assert.Error(t, missingDate.Validate())

	inverted := testShift(16, 8)
	assert.Error(t, inverted.Validate())

	zeroLength := testShift(8, 8)
	assert.Error(t, zeroLength.Validate())

	negativeBreak := testShift(8, 16)
	negativeBreak.BreakMinutes = -1
	assert.Error(t, negativeBreak.Validate())

	breakEqualsDuration := testShift(8, 9)
	breakEqualsDuration.BreakMinutes = 60
	assert.NoError(t, breakEqualsDuration.Validate())

	breakExceedsDuration := testShift(8, 9)
	breakExceedsDuration.BreakMinutes = 61
	assert.Error(t, breakExceedsDuration.Validate())

	breakExceedsAbsoluteMax := testShift(8, 16)
	breakExceedsAbsoluteMax.BreakMinutes = MaxStaffShiftBreakMinutes + 1
	assert.Error(t, breakExceedsAbsoluteMax.Validate())

	missingCreator := testShift(8, 16)
	missingCreator.CreatedBy = 0
	assert.Error(t, missingCreator.Validate())
}

func TestStaffShiftValidate_IgnoresDriverYearAnchor(t *testing.T) {
	t.Parallel()

	// TIME columns scan back with a driver-arbitrary year anchor; only the
	// wall clock may matter. Year 2000 start vs year 0001 end must still
	// validate on wall-clock order.
	shift := testShift(8, 16)
	shift.StartTime = time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)
	shift.EndTime = time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC)
	assert.NoError(t, shift.Validate())
}

func TestStaffShiftOverlaps(t *testing.T) {
	t.Parallel()

	base := testShift(8, 16)

	assert.True(t, base.Overlaps(testShift(15, 18)), "15-18 overlaps 8-16")
	assert.True(t, base.Overlaps(testShift(9, 10)), "contained window overlaps")
	assert.False(t, base.Overlaps(testShift(16, 18)), "touching boundary does not overlap")
	assert.False(t, base.Overlaps(testShift(6, 8)), "touching boundary does not overlap")
	assert.False(t, base.Overlaps(testShift(17, 18)), "disjoint windows do not overlap")
}

func TestStaffShiftEndInstant(t *testing.T) {
	t.Parallel()

	shift := testShift(8, 16)
	end := shift.EndInstant()

	assert.Equal(t, 2026, end.Year())
	assert.Equal(t, time.July, end.Month())
	assert.Equal(t, 6, end.Day())
	assert.Equal(t, 16, end.Hour())
	assert.Equal(t, 0, end.Minute())
	assert.Equal(t, timezone.Berlin, end.Location())
}

func TestStaffShiftEntityAccessors(t *testing.T) {
	t.Parallel()

	shift := testShift(8, 16)
	shift.ID = 123
	shift.CreatedAt = time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	shift.UpdatedAt = time.Date(2026, time.July, 2, 10, 0, 0, 0, time.UTC)

	assert.Equal(t, int64(123), shift.GetID())
	assert.Equal(t, shift.CreatedAt, shift.GetCreatedAt())
	assert.Equal(t, shift.UpdatedAt, shift.GetUpdatedAt())
}
