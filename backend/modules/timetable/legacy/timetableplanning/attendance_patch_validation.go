package timetableplanning

import (
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// ValidateAttendancePatch applies the Timetable owner's attendance patch
// rules (timetable.ValidateAttendancePatch) to a retained row patch and the
// row it applies to.
func ValidateAttendancePatch(patch scheduleModel.AttendanceFieldPatch, current *scheduleModel.InstanceStudent) []timetable.AttendanceFieldError {
	return timetable.ValidateAttendancePatch(timetable.AttendancePatch{
		Status: patch.Status, Substatus: patch.Substatus, SubstatusClear: patch.SubstatusClear,
		Note: patch.Note, NoteClear: patch.NoteClear,
	}, timetable.SlotAttendance{Status: current.Status, Substatus: current.Substatus})
}
