package presenceprojectionhttp

// Resource types of the GDPR disclosure trail (audit.data_access_log) these
// routes append through Student Presence's RecordDataAccess. They are stored
// values the Audit Platform's readers key on, so they never change.
const (
	dataAccessAttendanceHistory        = "attendance_history"
	dataAccessAttendanceDayLog         = "attendance_day_log"
	dataAccessStudentStatusDayOverview = "student_status_day_overview"
	dataAccessStudentHealthListExport  = "student_health_list_export"
)
