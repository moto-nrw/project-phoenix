package timetable

// CareDayStatus is Care Plan's care-day verdict for one child on a block's
// date as the timetable renders and counts it (#1747). Care Plan decides it;
// the values and their JSON form are Care Plan's careplan.CareDayStatus.
type CareDayStatus string

const (
	// CareDayScheduled — the care plan puts the child in the OGS that day.
	CareDayScheduled CareDayStatus = "scheduled"
	// CareDayNotScheduled — the child is not booked into care that day; not
	// expected and never an absence.
	CareDayNotScheduled CareDayStatus = "not_scheduled"
	// CareDayCancelled — somebody cancelled the day ("Kommt heute nicht");
	// not expected, but a reported absence.
	CareDayCancelled CareDayStatus = "cancelled"
	// CareDayUnknown — the plan says nothing about the day; the child stays
	// expected.
	CareDayUnknown CareDayStatus = "unknown"
)

// CareDayAttendance is the attendance provenance of one planned participant
// that Care Plan's row rule reads: whether the row is still expected, carries
// the frozen non-booking marker, was decided by hand, or holds an absence a
// status day or partial excusal wrote.
type CareDayAttendance struct {
	Expected         bool
	NotScheduled     bool
	ManuallyDecided  bool
	PlanOwnedAbsence bool
}
