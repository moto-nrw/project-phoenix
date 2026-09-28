package timetable

import "fmt"

// Slot attendance of a planned participant (E18): the status, an optional
// substatus and a free-text note. Student Presence records it; the
// timetable's operational views and the planner edit it.
const (
	SlotAttendanceExpected = "expected"
	SlotAttendancePresent  = "present"
	SlotAttendanceAbsent   = "absent"

	SlotSubstatusLate      = "late"
	SlotSubstatusExcused   = "excused"
	SlotSubstatusSick      = "sick"
	SlotSubstatusFieldTrip = "field_trip"
	SlotSubstatusOther     = "other"

	// SlotAttendanceNoteMaxLength bounds the free-text note.
	SlotAttendanceNoteMaxLength = 500
)

// AttendancePatch is a manual decision on one slot: a nil pointer leaves the
// field alone, a Clear flag empties it.
type AttendancePatch struct {
	Status         *string
	Substatus      *string
	SubstatusClear bool
	Note           *string
	NoteClear      bool
}

// HasChanges reports whether the patch carries at least one mutation.
func (p AttendancePatch) HasChanges() bool {
	return p.Status != nil || p.Substatus != nil || p.SubstatusClear || p.Note != nil || p.NoteClear
}

// SlotAttendance is the state a patch applies to.
type SlotAttendance struct {
	Status    string
	Substatus *string
}

// AttendanceFieldError is one field-level rejection of an attendance patch.
type AttendanceFieldError struct {
	Field  string
	Reason string
}

// AttendanceValidationError carries every field a patch got wrong; the
// handlers render it as a 400 with the field list.
type AttendanceValidationError struct {
	Fields []AttendanceFieldError
}

func (e *AttendanceValidationError) Error() string {
	return "timetable attendance validation failed"
}

// ValidateAttendancePatch checks a patch against the slot it applies to: the
// status and substatus vocabulary and the note length first, then the
// cross-field rule on the final state — a substatus may not stand on a slot
// that is still expected.
func ValidateAttendancePatch(patch AttendancePatch, current SlotAttendance) []AttendanceFieldError {
	var errs []AttendanceFieldError
	if patch.Status != nil && !validSlotAttendance(*patch.Status) {
		errs = append(errs, AttendanceFieldError{Field: "status", Reason: "must be one of: expected, present, absent"})
	}
	if patch.Substatus != nil && !validSlotSubstatus(*patch.Substatus) {
		errs = append(errs, AttendanceFieldError{Field: "substatus", Reason: "must be one of: late, excused, sick, field_trip, other"})
	}
	if patch.Note != nil && len(*patch.Note) > SlotAttendanceNoteMaxLength {
		errs = append(errs, AttendanceFieldError{
			Field:  "note",
			Reason: fmt.Sprintf("must be at most %d characters", SlotAttendanceNoteMaxLength),
		})
	}
	if len(errs) > 0 {
		return errs
	}

	finalStatus := current.Status
	if patch.Status != nil {
		finalStatus = *patch.Status
	}
	finalHasSubstatus := current.Substatus != nil
	if patch.SubstatusClear {
		finalHasSubstatus = false
	} else if patch.Substatus != nil {
		finalHasSubstatus = true
	}
	if finalHasSubstatus && finalStatus == SlotAttendanceExpected {
		errs = append(errs, AttendanceFieldError{Field: "substatus", Reason: "cannot be set when status is expected"})
	}
	return errs
}

func validSlotAttendance(status string) bool {
	return status == SlotAttendanceExpected || status == SlotAttendancePresent || status == SlotAttendanceAbsent
}

func validSlotSubstatus(substatus string) bool {
	switch substatus {
	case SlotSubstatusLate, SlotSubstatusExcused, SlotSubstatusSick, SlotSubstatusFieldTrip, SlotSubstatusOther:
		return true
	}
	return false
}
