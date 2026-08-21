package active

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

func TestAttendance_GetID(t *testing.T) {
	t.Parallel()

	attendance := &Attendance{
		StudentID:   1,
		Date:        timezone.TodayDate(),
		CheckInTime: time.Now(),
	}
	attendance.ID = 123

	assert.Equal(t, int64(123), attendance.GetID())
}

func TestAttendance_GetCreatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	attendance := &Attendance{
		StudentID:   1,
		Date:        timezone.DateFromTime(now),
		CheckInTime: now,
	}
	attendance.CreatedAt = now

	assert.Equal(t, now, attendance.GetCreatedAt())
}

func TestAttendance_GetUpdatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	attendance := &Attendance{
		StudentID:   1,
		Date:        timezone.DateFromTime(now),
		CheckInTime: now,
	}
	attendance.UpdatedAt = now

	assert.Equal(t, now, attendance.GetUpdatedAt())
}

func TestAttendance_IsCheckedIn_WhenCheckedIn(t *testing.T) {
	t.Parallel()

	now := time.Now()
	attendance := &Attendance{
		StudentID:    1,
		Date:         timezone.DateFromTime(now),
		CheckInTime:  now,
		CheckOutTime: nil, // Not checked out
	}

	assert.True(t, attendance.IsCheckedIn(), "Should return true when CheckOutTime is nil")
}

func TestAttendance_IsCheckedIn_WhenCheckedOut(t *testing.T) {
	t.Parallel()

	now := time.Now()
	checkoutTime := now.Add(2 * time.Hour)

	attendance := &Attendance{
		StudentID:    1,
		Date:         timezone.DateFromTime(now),
		CheckInTime:  now,
		CheckOutTime: &checkoutTime, // Checked out
	}

	assert.False(t, attendance.IsCheckedIn(), "Should return false when CheckOutTime is set")
}

func TestAttendance_IsCheckedIn_ZeroValue(t *testing.T) {
	t.Parallel()

	// Test with zero-initialized struct
	attendance := &Attendance{
		StudentID:    1,
		Date:         timezone.TodayDate(),
		CheckInTime:  time.Now(),
		CheckOutTime: nil,
	}

	assert.True(t, attendance.IsCheckedIn())
}

func TestAttendance_CompleteLifecycle(t *testing.T) {
	t.Parallel()

	now := time.Now()

	// Create attendance record (check-in)
	attendance := &Attendance{
		StudentID:    42,
		Date:         timezone.DateFromTime(now),
		CheckInTime:  now,
		CheckedInBy:  1,
		DeviceID:     100,
		CheckOutTime: nil,
		CheckedOutBy: nil,
	}
	attendance.ID = 1
	attendance.CreatedAt = now
	attendance.UpdatedAt = now

	// Initially checked in
	assert.True(t, attendance.IsCheckedIn())
	assert.Nil(t, attendance.CheckOutTime)
	assert.Nil(t, attendance.CheckedOutBy)

	// Simulate checkout
	checkoutTime := now.Add(3 * time.Hour)
	checkedOutBy := int64(2)
	attendance.CheckOutTime = &checkoutTime
	attendance.CheckedOutBy = &checkedOutBy
	attendance.UpdatedAt = checkoutTime

	// Now checked out
	assert.False(t, attendance.IsCheckedIn())
	assert.NotNil(t, attendance.CheckOutTime)
	assert.Equal(t, checkoutTime, *attendance.CheckOutTime)
	assert.NotNil(t, attendance.CheckedOutBy)
	assert.Equal(t, int64(2), *attendance.CheckedOutBy)
}

func TestAttendance_MultipleRecords(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name          string
		checkOutTime  *time.Time
		wantCheckedIn bool
	}{
		{
			name:          "record 1 - checked in",
			checkOutTime:  nil,
			wantCheckedIn: true,
		},
		{
			name:          "record 2 - checked out",
			checkOutTime:  timePtr(now.Add(1 * time.Hour)),
			wantCheckedIn: false,
		},
		{
			name:          "record 3 - checked in",
			checkOutTime:  nil,
			wantCheckedIn: true,
		},
		{
			name:          "record 4 - checked out",
			checkOutTime:  timePtr(now.Add(2 * time.Hour)),
			wantCheckedIn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attendance := &Attendance{
				StudentID:    1,
				Date:         timezone.DateFromTime(now),
				CheckInTime:  now,
				CheckOutTime: tt.checkOutTime,
			}

			assert.Equal(t, tt.wantCheckedIn, attendance.IsCheckedIn())
		})
	}
}

func TestAttendance_Fields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	checkoutTime := now.Add(2 * time.Hour)
	checkedOutBy := int64(99)

	attendance := &Attendance{
		StudentID:    42,
		Date:         timezone.DateFromTime(now),
		CheckInTime:  now,
		CheckOutTime: &checkoutTime,
		CheckedInBy:  10,
		CheckedOutBy: &checkedOutBy,
		DeviceID:     200,
	}
	attendance.ID = 1
	attendance.CreatedAt = now
	attendance.UpdatedAt = checkoutTime

	// Verify all fields
	assert.Equal(t, int64(1), attendance.ID)
	assert.Equal(t, int64(42), attendance.StudentID)
	assert.Equal(t, timezone.DateFromTime(now), attendance.Date)
	assert.Equal(t, now, attendance.CheckInTime)
	assert.NotNil(t, attendance.CheckOutTime)
	assert.Equal(t, checkoutTime, *attendance.CheckOutTime)
	assert.Equal(t, int64(10), attendance.CheckedInBy)
	assert.NotNil(t, attendance.CheckedOutBy)
	assert.Equal(t, int64(99), *attendance.CheckedOutBy)
	assert.Equal(t, int64(200), attendance.DeviceID)
	assert.Equal(t, now, attendance.CreatedAt)
	assert.Equal(t, checkoutTime, attendance.UpdatedAt)
}

// Helper function to create time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}

func TestAttendance_IsOnYard(t *testing.T) {
	t.Parallel()

	now := time.Now()
	yard := now.Add(-10 * time.Minute)
	checkout := now

	tests := []struct {
		name string
		att  Attendance
		want bool
	}{
		{
			name: "in building only (no yard, no checkout)",
			att:  Attendance{YardSince: nil, CheckOutTime: nil},
			want: false,
		},
		{
			name: "on yard (yard set, no checkout)",
			att:  Attendance{YardSince: &yard, CheckOutTime: nil},
			want: true,
		},
		{
			name: "checked out wins over yard — student has left premises",
			att:  Attendance{YardSince: &yard, CheckOutTime: &checkout},
			want: false,
		},
		{
			name: "checked out with no yard history",
			att:  Attendance{YardSince: nil, CheckOutTime: &checkout},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.att.IsOnYard())
		})
	}
}
