package active

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
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

func TestAttendance_EntityInterface(t *testing.T) {
	now := time.Now()
	a := &Attendance{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Hour),
		},
		StudentID:   1,
		Date:        now,
		CheckInTime: now,
		CheckedInBy: 1,
		DeviceID:    1,
	}

	t.Run("GetID", func(t *testing.T) {
		got := a.GetID()
		if got != int64(123) {
			t.Errorf("Attendance.GetID() = %v, want %v", got, int64(123))
		}
	})

	t.Run("GetCreatedAt", func(t *testing.T) {
		got := a.GetCreatedAt()
		if !got.Equal(now) {
			t.Errorf("Attendance.GetCreatedAt() = %v, want %v", got, now)
		}
	})

	t.Run("GetUpdatedAt", func(t *testing.T) {
		expected := now.Add(time.Hour)
		got := a.GetUpdatedAt()
		if !got.Equal(expected) {
			t.Errorf("Attendance.GetUpdatedAt() = %v, want %v", got, expected)
		}
	})
}
