package schedule

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
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

func TestInstanceStudent_TableName(t *testing.T) {
	assert.Equal(t, "schedule.instance_students", (&InstanceStudent{}).TableName())
}

func TestInstanceStudent_BeforeAppendModel(t *testing.T) {
	s := &InstanceStudent{}
	for _, q := range []any{&bun.SelectQuery{}, &bun.UpdateQuery{}, &bun.DeleteQuery{}, "unknown"} {
		require.NoError(t, s.BeforeAppendModel(q))
	}
}
