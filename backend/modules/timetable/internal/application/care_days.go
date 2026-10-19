package application

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
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
