package schedule_test

import (
	"strings"
	"testing"

	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAttendancePatch_CrossFieldRule(t *testing.T) {
	excused := scheduleModel.AttendanceSubstatusExcused
	tests := []struct {
		name         string
		current      *scheduleModel.InstanceStudent
		patch        scheduleModel.AttendanceFieldPatch
		wantErrField string
		wantOK       bool
	}{
		{
			name:    "expected + new substatus without status change rejects",
			current: &scheduleModel.InstanceStudent{Status: scheduleModel.AttendanceStatusExpected},
			patch: scheduleModel.AttendanceFieldPatch{
				Substatus: strPtr(scheduleModel.AttendanceSubstatusLate),
			},
			wantErrField: "substatus",
		},
		{
			name:    "expected to present with substatus is valid",
			current: &scheduleModel.InstanceStudent{Status: scheduleModel.AttendanceStatusExpected},
			patch: scheduleModel.AttendanceFieldPatch{
				Status:    strPtr(scheduleModel.AttendanceStatusPresent),
				Substatus: strPtr(scheduleModel.AttendanceSubstatusLate),
			},
			wantOK: true,
		},
		{
			name:    "expected to absent with substatus is valid",
			current: &scheduleModel.InstanceStudent{Status: scheduleModel.AttendanceStatusExpected},
			patch: scheduleModel.AttendanceFieldPatch{
				Status:    strPtr(scheduleModel.AttendanceStatusAbsent),
				Substatus: strPtr(scheduleModel.AttendanceSubstatusSick),
			},
			wantOK: true,
		},
		{
			name:    "present with substatus is valid",
			current: &scheduleModel.InstanceStudent{Status: scheduleModel.AttendanceStatusPresent},
			patch: scheduleModel.AttendanceFieldPatch{
				Substatus: strPtr(scheduleModel.AttendanceSubstatusLate),
			},
			wantOK: true,
		},
		{
			name:    "present to expected with existing substatus rejects",
			current: &scheduleModel.InstanceStudent{Status: scheduleModel.AttendanceStatusPresent, Substatus: &excused},
			patch: scheduleModel.AttendanceFieldPatch{
				Status: strPtr(scheduleModel.AttendanceStatusExpected),
			},
			wantErrField: "substatus",
		},
		{
			name:    "present to expected clearing substatus is valid",
			current: &scheduleModel.InstanceStudent{Status: scheduleModel.AttendanceStatusPresent, Substatus: &excused},
			patch: scheduleModel.AttendanceFieldPatch{
				Status:         strPtr(scheduleModel.AttendanceStatusExpected),
				SubstatusClear: true,
			},
			wantOK: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := schedule.ValidateAttendancePatch(tc.patch, tc.current)
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
	cur := &scheduleModel.InstanceStudent{Status: scheduleModel.AttendanceStatusPresent}

	t.Run("invalid status", func(t *testing.T) {
		errs := schedule.ValidateAttendancePatch(scheduleModel.AttendanceFieldPatch{Status: strPtr("ghost")}, cur)
		require.Len(t, errs, 1)
		assert.Equal(t, "status", errs[0].Field)
	})

	t.Run("invalid substatus", func(t *testing.T) {
		errs := schedule.ValidateAttendancePatch(scheduleModel.AttendanceFieldPatch{Substatus: strPtr("banana")}, cur)
		require.Len(t, errs, 1)
		assert.Equal(t, "substatus", errs[0].Field)
	})

	t.Run("note too long", func(t *testing.T) {
		tooLong := strings.Repeat("x", scheduleModel.InstanceStudentNoteMaxLength+1)
		errs := schedule.ValidateAttendancePatch(scheduleModel.AttendanceFieldPatch{Note: &tooLong}, cur)
		require.Len(t, errs, 1)
		assert.Equal(t, "note", errs[0].Field)
	})

	t.Run("two per-field errors returned together", func(t *testing.T) {
		tooLong := strings.Repeat("y", scheduleModel.InstanceStudentNoteMaxLength+1)
		errs := schedule.ValidateAttendancePatch(scheduleModel.AttendanceFieldPatch{
			Status: strPtr("ghost"),
			Note:   &tooLong,
		}, cur)
		require.Len(t, errs, 2)
	})
}
