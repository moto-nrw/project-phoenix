package schedule

import (
	"strings"
	"testing"
	"time"

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

func TestValidateAttendancePatch_CrossFieldRule(t *testing.T) {
	excused := AttendanceSubstatusExcused
	tests := []struct {
		name         string
		current      *InstanceStudent
		patch        AttendanceFieldPatch
		wantErrField string
		wantOK       bool
	}{
		{
			name:    "expected + new substatus without status change rejects",
			current: &InstanceStudent{Status: AttendanceStatusExpected},
			patch: AttendanceFieldPatch{
				Substatus: strPtr(AttendanceSubstatusLate),
			},
			wantErrField: "substatus",
		},
		{
			name:    "expected to present with substatus is valid",
			current: &InstanceStudent{Status: AttendanceStatusExpected},
			patch: AttendanceFieldPatch{
				Status:    strPtr(AttendanceStatusPresent),
				Substatus: strPtr(AttendanceSubstatusLate),
			},
			wantOK: true,
		},
		{
			name:    "expected to absent with substatus is valid",
			current: &InstanceStudent{Status: AttendanceStatusExpected},
			patch: AttendanceFieldPatch{
				Status:    strPtr(AttendanceStatusAbsent),
				Substatus: strPtr(AttendanceSubstatusSick),
			},
			wantOK: true,
		},
		{
			name:    "present with substatus is valid",
			current: &InstanceStudent{Status: AttendanceStatusPresent},
			patch: AttendanceFieldPatch{
				Substatus: strPtr(AttendanceSubstatusLate),
			},
			wantOK: true,
		},
		{
			name:    "present to expected with existing substatus rejects",
			current: &InstanceStudent{Status: AttendanceStatusPresent, Substatus: &excused},
			patch: AttendanceFieldPatch{
				Status: strPtr(AttendanceStatusExpected),
			},
			wantErrField: "substatus",
		},
		{
			name:    "present to expected clearing substatus is valid",
			current: &InstanceStudent{Status: AttendanceStatusPresent, Substatus: &excused},
			patch: AttendanceFieldPatch{
				Status:         strPtr(AttendanceStatusExpected),
				SubstatusClear: true,
			},
			wantOK: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateAttendancePatch(tc.patch, tc.current)
			if tc.wantOK {
				assert.Empty(t, errs)
				return
			}
			require.NotEmpty(t, errs)
			assert.Equal(t, tc.wantErrField, errs[0].Field)
		})
	}
}

func TestValidateAttendancePatch_PerFieldErrors(t *testing.T) {
	cur := &InstanceStudent{Status: AttendanceStatusPresent}

	t.Run("invalid status", func(t *testing.T) {
		errs := ValidateAttendancePatch(AttendanceFieldPatch{Status: strPtr("ghost")}, cur)
		require.Len(t, errs, 1)
		assert.Equal(t, "status", errs[0].Field)
	})

	t.Run("invalid substatus", func(t *testing.T) {
		errs := ValidateAttendancePatch(AttendanceFieldPatch{Substatus: strPtr("banana")}, cur)
		require.Len(t, errs, 1)
		assert.Equal(t, "substatus", errs[0].Field)
	})

	t.Run("note too long", func(t *testing.T) {
		tooLong := strings.Repeat("x", InstanceStudentNoteMaxLength+1)
		errs := ValidateAttendancePatch(AttendanceFieldPatch{Note: &tooLong}, cur)
		require.Len(t, errs, 1)
		assert.Equal(t, "note", errs[0].Field)
	})

	t.Run("two per-field errors returned together", func(t *testing.T) {
		tooLong := strings.Repeat("y", InstanceStudentNoteMaxLength+1)
		errs := ValidateAttendancePatch(AttendanceFieldPatch{
			Status: strPtr("ghost"),
			Note:   &tooLong,
		}, cur)
		require.Len(t, errs, 2)
	})
}
