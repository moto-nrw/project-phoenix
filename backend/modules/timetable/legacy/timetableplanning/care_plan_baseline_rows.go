package timetableplanning

import (
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

func careDayAttendance(row *schedule.InstanceStudent) *careplan.CareDayAttendance {
	return &careplan.CareDayAttendance{
		Expected:         row.Status == schedule.AttendanceStatusExpected,
		NotScheduled:     row.NotScheduled,
		ManuallyDecided:  row.ManualStatusAt != nil,
		PlanOwnedAbsence: row.StudentStatusDayID != nil || row.PickupExceptionID != nil,
	}
}
