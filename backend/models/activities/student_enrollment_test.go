package activities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

func TestIsValidAttendanceStatus(t *testing.T) {
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
	now := time.Now()
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
				EnrollmentDate:  now,
			},
			wantErr: false,
		},
		{
			name: "Valid enrollment with attendance - Present",
			studentEnrollment: &StudentEnrollment{
				StudentID:        1,
				ActivityGroupID:  1,
				EnrollmentDate:   now,
				AttendanceStatus: &present,
			},
			wantErr: false,
		},
		{
			name: "Valid enrollment with attendance - Absent",
			studentEnrollment: &StudentEnrollment{
				StudentID:        1,
				ActivityGroupID:  1,
				EnrollmentDate:   now,
				AttendanceStatus: &absent,
			},
			wantErr: false,
		},
		{
			name: "Missing student ID",
			studentEnrollment: &StudentEnrollment{
				ActivityGroupID: 1,
				EnrollmentDate:  now,
			},
			wantErr: true,
		},
		{
			name: "Invalid student ID",
			studentEnrollment: &StudentEnrollment{
				StudentID:       -1,
				ActivityGroupID: 1,
				EnrollmentDate:  now,
			},
			wantErr: true,
		},
		{
			name: "Missing activity group ID",
			studentEnrollment: &StudentEnrollment{
				StudentID:      1,
				EnrollmentDate: now,
			},
			wantErr: true,
		},
		{
			name: "Invalid activity group ID",
			studentEnrollment: &StudentEnrollment{
				StudentID:       1,
				ActivityGroupID: -1,
				EnrollmentDate:  now,
			},
			wantErr: true,
		},
		{
			name: "Missing enrollment date will be set automatically",
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
				EnrollmentDate:   now,
				AttendanceStatus: &invalid,
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

			// Verify enrollment date is set when missing
			if tt.name == "Missing enrollment date will be set automatically" && tt.studentEnrollment.EnrollmentDate.IsZero() {
				t.Errorf("StudentEnrollment.Validate() did not set enrollment date")
			}
		})
	}
}

func TestStudentEnrollmentMarkPresent(t *testing.T) {
	studentEnrollment := &StudentEnrollment{
		StudentID:       1,
		ActivityGroupID: 1,
	}

	studentEnrollment.MarkPresent()

	if studentEnrollment.AttendanceStatus == nil {
		t.Errorf("StudentEnrollment.MarkPresent() failed to set attendance status")
	} else if *studentEnrollment.AttendanceStatus != AttendancePresent {
		t.Errorf("StudentEnrollment.MarkPresent() = %v, want %v", *studentEnrollment.AttendanceStatus, AttendancePresent)
	}
}

func TestStudentEnrollmentMarkAbsent(t *testing.T) {
	studentEnrollment := &StudentEnrollment{
		StudentID:       1,
		ActivityGroupID: 1,
	}

	studentEnrollment.MarkAbsent()

	if studentEnrollment.AttendanceStatus == nil {
		t.Errorf("StudentEnrollment.MarkAbsent() failed to set attendance status")
	} else if *studentEnrollment.AttendanceStatus != AttendanceAbsent {
		t.Errorf("StudentEnrollment.MarkAbsent() = %v, want %v", *studentEnrollment.AttendanceStatus, AttendanceAbsent)
	}
}

func TestStudentEnrollmentMarkExcused(t *testing.T) {
	studentEnrollment := &StudentEnrollment{
		StudentID:       1,
		ActivityGroupID: 1,
	}

	studentEnrollment.MarkExcused()

	if studentEnrollment.AttendanceStatus == nil {
		t.Errorf("StudentEnrollment.MarkExcused() failed to set attendance status")
	} else if *studentEnrollment.AttendanceStatus != AttendanceExcused {
		t.Errorf("StudentEnrollment.MarkExcused() = %v, want %v", *studentEnrollment.AttendanceStatus, AttendanceExcused)
	}
}

func TestStudentEnrollmentClearAttendance(t *testing.T) {
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

func TestStudentEnrollment_BeforeAppendModel(t *testing.T) {
	se := &StudentEnrollment{}

	t.Run("handles SelectQuery", func(t *testing.T) {
		err := se.BeforeAppendModel(&bun.SelectQuery{})
		assert.NoError(t, err)
	})

	t.Run("handles InsertQuery", func(t *testing.T) {
		err := se.BeforeAppendModel(&bun.InsertQuery{})
		assert.NoError(t, err)
	})

	t.Run("handles UpdateQuery", func(t *testing.T) {
		err := se.BeforeAppendModel(&bun.UpdateQuery{})
		assert.NoError(t, err)
	})

	t.Run("handles DeleteQuery", func(t *testing.T) {
		err := se.BeforeAppendModel(&bun.DeleteQuery{})
		assert.NoError(t, err)
	})

	t.Run("handles unknown query type", func(t *testing.T) {
		err := se.BeforeAppendModel("unknown")
		assert.NoError(t, err)
	})
}

func TestStudentEnrollment_TableName(t *testing.T) {
	se := &StudentEnrollment{}
	assert.Equal(t, "activities.student_enrollments", se.TableName())
}

func TestStudentEnrollment_GetID(t *testing.T) {
	se := &StudentEnrollment{}
	se.ID = 123
	assert.Equal(t, int64(123), se.GetID())
}

func TestStudentEnrollment_GetCreatedAt(t *testing.T) {
	now := time.Now()
	se := &StudentEnrollment{}
	se.CreatedAt = now
	assert.Equal(t, now, se.GetCreatedAt())
}

func TestStudentEnrollment_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	se := &StudentEnrollment{}
	se.UpdatedAt = now
	assert.Equal(t, now, se.GetUpdatedAt())
}
