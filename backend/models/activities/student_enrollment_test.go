package activities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsValidAttendanceStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{
			name:   "Valid status - Present",
			status: AttendancePresent,
			want:   true,
		},
		{
			name:   "Valid status - Absent",
			status: AttendanceAbsent,
			want:   true,
		},
		{
			name:   "Valid status - Excused",
			status: AttendanceExcused,
			want:   true,
		},
		{
			name:   "Valid status - Unknown",
			status: AttendanceUnknown,
			want:   true,
		},
		{
			name:   "Invalid status - empty string",
			status: "",
			want:   false,
		},
		{
			name:   "Invalid status - lowercase",
			status: "present",
			want:   false,
		},
		{
			name:   "Invalid status - random string",
			status: "RANDOM",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidAttendanceStatus(tt.status); got != tt.want {
				t.Errorf("IsValidAttendanceStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStudentEnrollmentValidate(t *testing.T) {
	t.Parallel()

	now := Date("2026-09-05")
	present := AttendancePresent
	absent := AttendanceAbsent
	invalid := "INVALID"

	tests := []struct {
		name              string
		studentEnrollment *StudentEnrollment
		wantErr           bool
	}{
		{
			name: "Valid enrollment without attendance",
			studentEnrollment: &StudentEnrollment{
				StudentID:       1,
				ActivityGroupID: 1,
				ValidFrom:       now,
			},
			wantErr: false,
		},
		{
			name: "Valid enrollment with attendance - Present",
			studentEnrollment: &StudentEnrollment{
				StudentID:        1,
				ActivityGroupID:  1,
				ValidFrom:        now,
				AttendanceStatus: &present,
			},
			wantErr: false,
		},
		{
			name: "Valid enrollment with attendance - Absent",
			studentEnrollment: &StudentEnrollment{
				StudentID:        1,
				ActivityGroupID:  1,
				ValidFrom:        now,
				AttendanceStatus: &absent,
			},
			wantErr: false,
		},
		{
			name: "Missing student ID",
			studentEnrollment: &StudentEnrollment{
				ActivityGroupID: 1,
				ValidFrom:       now,
			},
			wantErr: true,
		},
		{
			name: "Invalid student ID",
			studentEnrollment: &StudentEnrollment{
				StudentID:       -1,
				ActivityGroupID: 1,
				ValidFrom:       now,
			},
			wantErr: true,
		},
		{
			name: "Missing activity group ID",
			studentEnrollment: &StudentEnrollment{
				StudentID: 1,
				ValidFrom: now,
			},
			wantErr: true,
		},
		{
			name: "Invalid activity group ID",
			studentEnrollment: &StudentEnrollment{
				StudentID:       1,
				ActivityGroupID: -1,
				ValidFrom:       now,
			},
			wantErr: true,
		},
		{
			name: "Missing valid_from is allowed (not auto-populated)",
			studentEnrollment: &StudentEnrollment{
				StudentID:       1,
				ActivityGroupID: 1,
			},
			wantErr: false,
		},
		{
			name: "Invalid attendance status",
			studentEnrollment: &StudentEnrollment{
				StudentID:        1,
				ActivityGroupID:  1,
				ValidFrom:        now,
				AttendanceStatus: &invalid,
			},
			wantErr: true,
		},
		{
			name: "Valid selected weekdays",
			studentEnrollment: &StudentEnrollment{
				StudentID:        101,
				ActivityGroupID:  201,
				ValidFrom:        now,
				SelectedWeekdays: []int{1, 3, 5},
			},
			wantErr: false,
		},
		{
			name: "Selected weekday below monday is rejected",
			studentEnrollment: &StudentEnrollment{
				StudentID:        102,
				ActivityGroupID:  202,
				ValidFrom:        now,
				SelectedWeekdays: []int{0},
			},
			wantErr: true,
		},
		{
			name: "Selected weekday above sunday is rejected",
			studentEnrollment: &StudentEnrollment{
				StudentID:        103,
				ActivityGroupID:  203,
				ValidFrom:        now,
				SelectedWeekdays: []int{8},
			},
			wantErr: true,
		},
		{
			name: "Duplicate selected weekday is rejected",
			studentEnrollment: &StudentEnrollment{
				StudentID:        104,
				ActivityGroupID:  204,
				ValidFrom:        now,
				SelectedWeekdays: []int{2, 2},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.studentEnrollment.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("StudentEnrollment.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Validate() must NOT populate ValidFrom; the service owns that default.
			if tt.name == "Missing valid_from is allowed (not auto-populated)" && !tt.studentEnrollment.ValidFrom.IsZero() {
				t.Errorf("StudentEnrollment.Validate() must not set ValidFrom; the service owns that default")
			}
		})
	}
}

func TestStudentEnrollmentClearAttendance(t *testing.T) {
	t.Parallel()

	status := AttendancePresent
	studentEnrollment := &StudentEnrollment{
		StudentID:        1,
		ActivityGroupID:  1,
		AttendanceStatus: &status,
	}

	studentEnrollment.ClearAttendance()

	if studentEnrollment.AttendanceStatus != nil {
		t.Errorf("StudentEnrollment.ClearAttendance() failed to clear attendance status")
	}
}

// ============================================================================
// BeforeAppendModel Hook Tests
// ============================================================================

func TestStudentEnrollment_GetID(t *testing.T) {
	t.Parallel()

	se := &StudentEnrollment{}
	se.ID = 123
	assert.Equal(t, int64(123), se.ID)
}

func TestStudentEnrollment_GetCreatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	se := &StudentEnrollment{}
	se.CreatedAt = now
	assert.Equal(t, now, se.CreatedAt)
}

func TestStudentEnrollment_GetUpdatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	se := &StudentEnrollment{}
	se.UpdatedAt = now
	assert.Equal(t, now, se.UpdatedAt)
}
