package schedule

import (
	"fmt"

	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// ValidateAttendancePatch enforces attendance patch business rules against the
// current row so cross-field rules can be evaluated against the final state.
//
// This is a domain decision (e.g. "a substatus may not be set while status is
// still expected"), so it lives in the service layer rather than on the model
// (Rule 12 — models hold data, not decisions). The per-field checks reuse the
// model's exported validators and constants.
func ValidateAttendancePatch(patch scheduleModel.AttendanceFieldPatch, current *scheduleModel.InstanceStudent) []scheduleModel.AttendancePatchFieldError {
	var errs []scheduleModel.AttendancePatchFieldError

	if patch.Status != nil && !scheduleModel.IsValidAttendanceStatus(*patch.Status) {
		errs = append(errs, scheduleModel.AttendancePatchFieldError{Field: "status", Reason: "must be one of: expected, present, absent"})
	}
	if patch.Substatus != nil && !scheduleModel.IsValidAttendanceSubstatus(*patch.Substatus) {
		errs = append(errs, scheduleModel.AttendancePatchFieldError{Field: "substatus", Reason: "must be one of: late, excused, sick, field_trip, other"})
	}
	if patch.Note != nil && len(*patch.Note) > scheduleModel.InstanceStudentNoteMaxLength {
		errs = append(errs, scheduleModel.AttendancePatchFieldError{
			Field:  "note",
			Reason: fmt.Sprintf("must be at most %d characters", scheduleModel.InstanceStudentNoteMaxLength),
		})
	}
	if len(errs) > 0 {
		return errs
	}

	finalStatus := current.Status
	if patch.Status != nil {
		finalStatus = *patch.Status
	}

	finalSubstatusNonNull := current.Substatus != nil
	if patch.SubstatusClear {
		finalSubstatusNonNull = false
	} else if patch.Substatus != nil {
		finalSubstatusNonNull = true
	}

	if finalSubstatusNonNull && finalStatus == scheduleModel.AttendanceStatusExpected {
		errs = append(errs, scheduleModel.AttendancePatchFieldError{
			Field:  "substatus",
			Reason: "cannot be set when status is expected",
		})
	}

	return errs
}
