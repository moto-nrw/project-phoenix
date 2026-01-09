package active

import (
	"testing"
	"time"
)

func TestAttendance_TableName(t *testing.T) {
	a := &Attendance{}
	expected := "active.attendance"

	got := a.TableName()
	if got != expected {
		t.Errorf("Attendance.TableName() = %q, want %q", got, expected)
	}
}

func TestAttendance_IsCheckedIn(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		checkOutTime *time.Time
		expected     bool
	}{
		{
			name:         "checked in (no checkout time)",
			checkOutTime: nil,
			expected:     true,
		},
		{
			name:         "checked out",
			checkOutTime: &now,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Attendance{
				StudentID:    1,
				Date:         now,
				CheckInTime:  now,
				CheckOutTime: tt.checkOutTime,
				CheckedInBy:  1,
				DeviceID:     1,
			}

			if got := a.IsCheckedIn(); got != tt.expected {
				t.Errorf("Attendance.IsCheckedIn() = %v, want %v", got, tt.expected)
			}
		})
	}
}
