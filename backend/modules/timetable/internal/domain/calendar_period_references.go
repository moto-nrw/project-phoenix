package domain

// CalendarPeriodReferences counts the Timetable-owned rows that point at one
// School Calendar period through their nullable calendar_period_id column.
// The School Calendar deletion preview renders these as usage; they describe
// planning references, not actual room supervision or attendance.
type CalendarPeriodReferences struct {
	ActivityGroups     int
	Schedules          int
	StudentEnrollments int
	Supervisors        int
	ActivityInstances  int
}
