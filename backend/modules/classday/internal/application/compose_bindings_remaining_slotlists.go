package application

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

const SlotAttendanceExpected = "expected"

// Rules forwards the pure care-plan rules to their owners so every reader of
// the roster shares one derivation (#1565, #2606).
type Rules func(status string, enrolledFrom, enrolledUntil *calendar.Date, date, today calendar.Date) bool

func (Rules) RowCareDay(instanceCompleted bool, row ports.RosterFacts, planVerdict ports.CareDay) ports.CareDay {
	model := &careplan.CareDayAttendance{Expected: row.Status == SlotAttendanceExpected, NotScheduled: row.NotScheduled, ManuallyDecided: row.ManualStatusAt != nil, PlanOwnedAbsence: row.StudentStatusDayID != nil || row.PickupExceptionID != nil}
	return ports.CareDay(careplan.AttendanceRowCareDay(instanceCompleted, model, careplan.CareDayStatus(planVerdict)))
}
func (r Rules) EnrolledOn(student ports.StudentFacts, date, today calendar.Date) bool {
	return r(student.Status, student.EnrolledFrom, student.EnrolledUntil, date, today)
}
