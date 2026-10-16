package application

import (
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// CareDayAttendance reads the attendance provenance of a planned participant
// row for Care Plan's row rule.
func CareDayAttendance(row *scheduleModels.InstanceStudent) timetable.CareDayAttendance {
	return timetable.CareDayAttendance{
		Expected:         row.Status == scheduleModels.AttendanceStatusExpected,
		NotScheduled:     row.NotScheduled,
		ManuallyDecided:  row.ManualStatusAt != nil,
		PlanOwnedAbsence: row.StudentStatusDayID != nil || row.PickupExceptionID != nil,
	}
}
