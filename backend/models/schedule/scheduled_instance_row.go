package schedule

// ScheduledInstanceRow pairs an activity instance with a student's attendance
// row on that instance. The timetable /student/{id}/day and /week endpoints
// assemble their response from rows of this shape: the instance carries the
// when/where/title, the attendance carries the student-specific status.
//
// Both fields are non-nil when returned by
// FindInstancesWithAttendanceByStudentAndDateRange — the INNER JOIN guarantees
// it. Lives in the models package (not the repo) so the method can be declared
// on the InstanceStudentRepository interface without a package cycle.
type ScheduledInstanceRow struct {
	Instance   *ActivityInstance
	Attendance *InstanceStudent
}
