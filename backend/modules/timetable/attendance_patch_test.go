package timetable

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAttendancePatch_CrossFieldRule(t *testing.T) {
	t.Parallel()

	excused := SlotSubstatusExcused
	tests := []struct {
		name         string
		current      SlotAttendance
		patch        AttendancePatch
		wantErrField string
		wantOK       bool
	}{
		{
			name:    "expected + new substatus without status change rejects",
			current: SlotAttendance{Status: SlotAttendanceExpected},
			patch: AttendancePatch{
				Substatus: attendancePtr(SlotSubstatusLate),
			},
			wantErrField: "substatus",
		},
		{
			name:    "expected to present with substatus is valid",
			current: SlotAttendance{Status: SlotAttendanceExpected},
			patch: AttendancePatch{
				Status:    attendancePtr(SlotAttendancePresent),
				Substatus: attendancePtr(SlotSubstatusLate),
			},
			wantOK: true,
		},
		{
			name:    "expected to absent with substatus is valid",
			current: SlotAttendance{Status: SlotAttendanceExpected},
			patch: AttendancePatch{
				Status:    attendancePtr(SlotAttendanceAbsent),
				Substatus: attendancePtr(SlotSubstatusSick),
			},
			wantOK: true,
		},
		{
			name:    "present with substatus is valid",
			current: SlotAttendance{Status: SlotAttendancePresent},
			patch: AttendancePatch{
				Substatus: attendancePtr(SlotSubstatusLate),
			},
			wantOK: true,
		},
		{
			name:    "present to expected with existing substatus rejects",
			current: SlotAttendance{Status: SlotAttendancePresent, Substatus: &excused},
			patch: AttendancePatch{
				Status: attendancePtr(SlotAttendanceExpected),
			},
			wantErrField: "substatus",
		},
		{
			name:    "present to expected clearing substatus is valid",
			current: SlotAttendance{Status: SlotAttendancePresent, Substatus: &excused},
			patch: AttendancePatch{
				Status:         attendancePtr(SlotAttendanceExpected),
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
	t.Parallel()

	cur := SlotAttendance{Status: SlotAttendancePresent}

	t.Run("invalid status", func(t *testing.T) {
		errs := ValidateAttendancePatch(AttendancePatch{Status: attendancePtr("ghost")}, cur)
		require.Len(t, errs, 1)
		assert.Equal(t, "status", errs[0].Field)
	})

	t.Run("invalid substatus", func(t *testing.T) {
		errs := ValidateAttendancePatch(AttendancePatch{Substatus: attendancePtr("banana")}, cur)
		require.Len(t, errs, 1)
		assert.Equal(t, "substatus", errs[0].Field)
	})

	t.Run("note too long", func(t *testing.T) {
		tooLong := strings.Repeat("x", SlotAttendanceNoteMaxLength+1)
		errs := ValidateAttendancePatch(AttendancePatch{Note: &tooLong}, cur)
		require.Len(t, errs, 1)
		assert.Equal(t, "note", errs[0].Field)
	})

	t.Run("two per-field errors returned together", func(t *testing.T) {
		tooLong := strings.Repeat("y", SlotAttendanceNoteMaxLength+1)
		errs := ValidateAttendancePatch(AttendancePatch{
			Status: attendancePtr("ghost"),
			Note:   &tooLong,
		}, cur)
		require.Len(t, errs, 2)
	})
}

func attendancePtr(value string) *string { return &value }
