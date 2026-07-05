package schedule

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceStudent_Validate(t *testing.T) {
	base := func() *InstanceStudent {
		return &InstanceStudent{
			InstanceID: 1,
			StudentID:  2,
			Status:     AttendanceStatusExpected,
		}
	}

	t.Run("valid minimal", func(t *testing.T) {
		require.NoError(t, base().Validate())
	})

	t.Run("missing instance_id", func(t *testing.T) {
		s := base()
		s.InstanceID = 0
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "instance_id is required")
	})

	t.Run("missing student_id", func(t *testing.T) {
		s := base()
		s.StudentID = 0
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "student_id is required")
	})

	t.Run("invalid status", func(t *testing.T) {
		s := base()
		s.Status = "unknown"
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid attendance status")
	})

	t.Run("valid substatus", func(t *testing.T) {
		s := base()
		late := AttendanceSubstatusLate
		s.Substatus = &late
		require.NoError(t, s.Validate())
	})

	t.Run("invalid substatus", func(t *testing.T) {
		s := base()
		bad := "unknown"
		s.Substatus = &bad
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid attendance substatus")
	})

	t.Run("note exactly 500 chars is allowed", func(t *testing.T) {
		s := base()
		n := strings.Repeat("a", InstanceStudentNoteMaxLength)
		s.Note = &n
		require.NoError(t, s.Validate())
	})

	t.Run("note over 500 chars is rejected", func(t *testing.T) {
		s := base()
		n := strings.Repeat("a", InstanceStudentNoteMaxLength+1)
		s.Note = &n
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "note cannot exceed 500 characters")
	})

	t.Run("negative room_id rejected", func(t *testing.T) {
		s := base()
		bad := int64(-1)
		s.RoomID = &bad
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "room_id must be positive when set")
	})
}

func TestIsValidAttendanceStatus(t *testing.T) {
	assert.True(t, IsValidAttendanceStatus(AttendanceStatusExpected))
	assert.True(t, IsValidAttendanceStatus(AttendanceStatusPresent))
	assert.True(t, IsValidAttendanceStatus(AttendanceStatusAbsent))
	assert.False(t, IsValidAttendanceStatus(""))
	assert.False(t, IsValidAttendanceStatus("unknown"))
}

func TestIsValidAttendanceSubstatus(t *testing.T) {
	for _, s := range []string{
		AttendanceSubstatusLate,
		AttendanceSubstatusExcused,
		AttendanceSubstatusSick,
		AttendanceSubstatusFieldTrip,
		AttendanceSubstatusOther,
	} {
		assert.True(t, IsValidAttendanceSubstatus(s), s)
	}
	assert.False(t, IsValidAttendanceSubstatus(""))
	assert.False(t, IsValidAttendanceSubstatus("unknown"))
}

func TestInstanceStudent_EntityInterface(t *testing.T) {
	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	s := &InstanceStudent{}
	s.ID = 42
	s.CreatedAt = now
	s.UpdatedAt = now.Add(time.Minute)

	assert.Equal(t, int64(42), s.GetID())
	assert.Equal(t, now, s.GetCreatedAt())
	assert.Equal(t, now.Add(time.Minute), s.GetUpdatedAt())
}

func TestAttendanceFieldPatch_HasChanges(t *testing.T) {
	sentinel := "x"
	tests := []struct {
		name  string
		patch AttendanceFieldPatch
		want  bool
	}{
		{"empty patch", AttendanceFieldPatch{}, false},
		{"status set", AttendanceFieldPatch{Status: &sentinel}, true},
		{"substatus set", AttendanceFieldPatch{Substatus: &sentinel}, true},
		{"substatus cleared", AttendanceFieldPatch{SubstatusClear: true}, true},
		{"note set", AttendanceFieldPatch{Note: &sentinel}, true},
		{"note cleared", AttendanceFieldPatch{NoteClear: true}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.patch.HasChanges())
		})
	}
}

// TestValidateAttendancePatch_* moved to services/schedule/attendance_patch_validation_test.go
// when the cross-field attendance invariant moved out of the model (#586, Rule 12).
