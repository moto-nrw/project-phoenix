package ports

// CalendarPeriodReferences mirrors the Timetable owner's per-period reference
// counts. The composition root adapts the owner's type; this package must not
// import the Timetable public package (architecture policy).
type CalendarPeriodReferences struct {
	ActivityGroups     int
	Schedules          int
	StudentEnrollments int
	Supervisors        int
	ActivityInstances  int
}

// CalendarPeriodUsage counts, per calendar period, the rows that reference it
// through nullable calendar_period_id FKs: Enrollment's phases and the five
// Timetable planning tables.
type CalendarPeriodUsage struct {
	EnrollmentPhases   int
	ActivityGroups     int
	Schedules          int
	StudentEnrollments int
	Supervisors        int
	ActivityInstances  int
}
