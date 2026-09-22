package studentpresence

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// TimeoutResult represents the result of processing a session timeout.
// ActivityID is *int64 because spontaneous sessions (WP-B6) carry no parent
// template; it is serialized as null on the wire rather than omitted.
type TimeoutResult struct {
	SessionID          int64     `json:"session_id"`
	ActivityID         *int64    `json:"activity_id"`
	StudentsCheckedOut int       `json:"students_checked_out"`
	TimeoutAt          time.Time `json:"timeout_at"`
}

// SessionTimeoutInfo provides information about a session's timeout status.
// ActivityID follows the same *int64 contract as TimeoutResult.
type SessionTimeoutInfo struct {
	SessionID          int64         `json:"session_id"`
	ActivityID         *int64        `json:"activity_id"`
	StartTime          time.Time     `json:"start_time"`
	LastActivity       time.Time     `json:"last_activity"`
	TimeoutMinutes     int           `json:"timeout_minutes"`
	InactivityDuration time.Duration `json:"inactivity_duration"`
	TimeUntilTimeout   time.Duration `json:"time_until_timeout"`
	IsTimedOut         bool          `json:"is_timed_out"`
	ActiveStudentCount int           `json:"active_student_count"`
}

// CleanupResult represents the result of a cleanup operation
type CleanupResult struct {
	StartedAt         time.Time
	CompletedAt       time.Time
	StudentsProcessed int
	RecordsDeleted    int64
	Errors            []CleanupError
	Success           bool
}

// CleanupError represents an error during cleanup for a specific student
type CleanupError struct {
	StudentID int64
	Error     string
	Timestamp time.Time
}

// RetentionStats represents statistics about data retention
type RetentionStats struct {
	TotalExpiredVisits   int64
	StudentsAffected     int
	OldestExpiredVisit   *time.Time
	ExpiredVisitsByMonth map[string]int64
}

// CleanupPreview shows what would be deleted
type CleanupPreview struct {
	StudentVisitCounts map[int64]int // Student ID -> number of visits to delete
	TotalVisits        int64
	OldestVisit        *time.Time
}

// DailyAttendanceStatus represents a student's current attendance status for the day
type DailyAttendanceStatus struct {
	StudentID int64 `json:"student_id"`
	// Status is derived from the attendance row's timestamps:
	//   "not_checked_in" — no attendance row today
	//   "checked_in"     — row exists, CheckOutTime nil, YardSince nil (in the building)
	//   "on_yard"        — row exists, CheckOutTime nil, YardSince non-nil (on premises, outside the building)
	//   "checked_out"    — CheckOutTime non-nil (formally left school)
	Status       string        `json:"status"`
	Date         timezone.Date `json:"date"`
	CheckInTime  *time.Time    `json:"check_in_time"`
	CheckOutTime *time.Time    `json:"check_out_time"`
	// YardSince, when non-nil, marks the moment the student moved to the
	// schoolyard without checking out. Only meaningful while Status == "on_yard".
	YardSince    *time.Time `json:"yard_since,omitempty"`
	CheckedInBy  string     `json:"checked_in_by"`  // Formatted as "FirstName LastName"
	CheckedOutBy string     `json:"checked_out_by"` // Formatted as "FirstName LastName"
}

// IsCurrentlyPresent reports whether the attendance row represents a child
// who is still on school premises. A completed attendance keeps its historical
// check-in time but is no longer current presence.
// HasRecordToday reports whether today's attendance holds any check-in,
// including one that has been checked out since.
func (s *DailyAttendanceStatus) HasRecordToday() bool {
	return s != nil && s.CheckInTime != nil
}

func (s *DailyAttendanceStatus) IsCurrentlyPresent() bool {
	return s != nil && s.CheckInTime != nil &&
		(s.Status == "checked_in" || s.Status == "on_yard")
}

// DailyCheckoutResult represents the outcome of confirming a deferred daily
// checkout from an IoT device.
type DailyCheckoutResult struct {
	// Action is "checked_out_daily" when the student went home ("zuhause") or
	// "checked_out" when they stayed in transit ("unterwegs").
	Action string
}

// AttendanceResult represents the result of a student attendance toggle operation
type AttendanceResult struct {
	Action       string    `json:"action"` // "checked_in", "checked_out"
	AttendanceID int64     `json:"attendance_id"`
	StudentID    int64     `json:"student_id"`
	Timestamp    time.Time `json:"timestamp"`
	// Changed reports whether THIS call mutated the attendance row: false when
	// a concurrent caller already established the target state (absorbed
	// in/in race, already-closed row). Not serialized — it informs the web
	// no-op display, while the IoT wire format stays byte-identical.
	Changed bool `json:"-"`
}

// DailySessionCleanupResult represents the result of ending daily sessions
type DailySessionCleanupResult struct {
	SessionsEnded       int       `json:"sessions_ended"`
	VisitsEnded         int       `json:"visits_ended"`
	SupervisorsEnded    int       `json:"supervisors_ended"`
	EndedActiveGroupIDs []int64   `json:"-"`
	ExecutedAt          time.Time `json:"executed_at"`
	Success             bool      `json:"success"`
	Errors              []string  `json:"errors,omitempty"`
}

// AttendanceCleanupResult represents the result of cleaning stale attendance records
type AttendanceCleanupResult struct {
	StartedAt        time.Time      `json:"started_at"`
	CompletedAt      time.Time      `json:"completed_at"`
	RecordsClosed    int            `json:"records_closed"`
	StudentsAffected int            `json:"students_affected"`
	OldestRecordDate *timezone.Date `json:"oldest_record_date,omitempty"`
	Success          bool           `json:"success"`
	Errors           []string       `json:"errors,omitempty"`
}

// AttendanceCleanupPreview shows what attendance records would be cleaned
type AttendanceCleanupPreview struct {
	TotalRecords   int            `json:"total_records"`
	StudentRecords map[int64]int  `json:"student_records"` // studentID -> count
	OldestRecord   *timezone.Date `json:"oldest_record,omitempty"`
	RecordsByDate  map[string]int `json:"records_by_date"` // date -> count
}

// SupervisorCleanupResult represents the result of cleaning stale supervisor records
type SupervisorCleanupResult struct {
	StartedAt        time.Time      `json:"started_at"`
	CompletedAt      time.Time      `json:"completed_at"`
	RecordsClosed    int            `json:"records_closed"`
	StaffAffected    int            `json:"staff_affected"`
	OldestRecordDate *timezone.Date `json:"oldest_record_date,omitempty"`
	Success          bool           `json:"success"`
	Errors           []string       `json:"errors,omitempty"`
}

// SupervisorCleanupPreview shows what supervisor records would be cleaned
type SupervisorCleanupPreview struct {
	TotalRecords  int            `json:"total_records"`
	StaffRecords  map[int64]int  `json:"staff_records"` // staffID -> count
	OldestRecord  *timezone.Date `json:"oldest_record,omitempty"`
	RecordsByDate map[string]int `json:"records_by_date"` // date -> count
}
