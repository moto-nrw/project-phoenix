package timetable

// CalendarPeriodReferences counts the Timetable-owned planning rows that
// reference one School Calendar period. The School Calendar deletion preview
// renders them as usage; they are planning references, not actual room
// supervision or attendance.
type CalendarPeriodReferences struct {
	ActivityGroups     int
	Schedules          int
	StudentEnrollments int
	Supervisors        int
	ActivityInstances  int
}
