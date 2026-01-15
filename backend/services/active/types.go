package active

import (
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
)

// Attendance status constants for consistent status checking across the codebase.
// These values are used in AttendanceStatus.Status and AttendanceResult.Action fields.
const (
	StatusNotCheckedIn = "not_checked_in"
	StatusCheckedIn    = "checked_in"
	StatusCheckedOut   = "checked_out"
)

// VisitWithDisplayData represents a visit with student display information
type VisitWithDisplayData struct {
	VisitID       int64      `json:"visit_id"`
	StudentID     int64      `json:"student_id"`
	ActiveGroupID int64      `json:"active_group_id"`
	EntryTime     time.Time  `json:"entry_time"`
	ExitTime      *time.Time `json:"exit_time,omitempty"`
	FirstName     string     `json:"first_name"`
	LastName      string     `json:"last_name"`
	SchoolClass   string     `json:"school_class"`
	OGSGroupName  string     `json:"ogs_group_name"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// DashboardAnalytics represents aggregated analytics for dashboard
type DashboardAnalytics struct {
	// Student Overview
	StudentsPresent      int
	StudentsInTransit    int // Students present but not in any active visit
	StudentsOnPlayground int
	StudentsInRooms      int // Students in indoor rooms (excluding playground)

	// Activities & Rooms
	ActiveActivities    int
	FreeRooms           int
	TotalRooms          int
	CapacityUtilization float64
	ActivityCategories  int

	// OGS Groups
	ActiveOGSGroups      int
	StudentsInGroupRooms int
	SupervisorsToday     int
	StudentsInHomeRoom   int

	// Recent Activity (Privacy-compliant)
	RecentActivity []RecentActivity

	// Current Activities (No personal data)
	CurrentActivities []CurrentActivity

	// Active Groups Summary
	ActiveGroupsSummary []ActiveGroupInfo

	// Timestamp
	LastUpdated time.Time
}

// RecentActivity represents a recent activity without personal data
type RecentActivity struct {
	Type      string
	GroupName string
	RoomName  string
	Count     int
	Timestamp time.Time
}

// CurrentActivity represents current activity status
type CurrentActivity struct {
	Name         string
	Category     string
	Participants int
	MaxCapacity  int
	Status       string
}

// ActiveGroupInfo represents active group summary
type ActiveGroupInfo struct {
	Name         string
	Type         string
	StudentCount int
	Location     string
	Status       string
}

// ActivityConflictInfo represents information about a detected activity conflict
type ActivityConflictInfo struct {
	HasConflict       bool          `json:"has_conflict"`
	ConflictingGroup  *active.Group `json:"conflicting_group,omitempty"`
	ConflictingDevice *string       `json:"conflicting_device,omitempty"`
	ConflictMessage   string        `json:"conflict_message"`
	CanOverride       bool          `json:"can_override"`
}

// TimeoutResult represents the result of processing a session timeout
type TimeoutResult struct {
	SessionID          int64     `json:"session_id"`
	ActivityID         int64     `json:"activity_id"`
	StudentsCheckedOut int       `json:"students_checked_out"`
	TimeoutAt          time.Time `json:"timeout_at"`
}

// SessionTimeoutInfo provides information about a session's timeout status
type SessionTimeoutInfo struct {
	SessionID          int64         `json:"session_id"`
	ActivityID         int64         `json:"activity_id"`
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

// AttendanceStatus represents a student's current attendance status for the day
type AttendanceStatus struct {
	StudentID    int64      `json:"student_id"`
	Status       string     `json:"status"` // "not_checked_in", "checked_in", "checked_out"
	Date         time.Time  `json:"date"`
	CheckInTime  *time.Time `json:"check_in_time"`
	CheckOutTime *time.Time `json:"check_out_time"`
	CheckedInBy  string     `json:"checked_in_by"`  // Formatted as "FirstName LastName"
	CheckedOutBy string     `json:"checked_out_by"` // Formatted as "FirstName LastName"
}

// AttendanceResult represents the result of a student attendance toggle operation
type AttendanceResult struct {
	Action       string    `json:"action"` // "checked_in", "checked_out"
	AttendanceID int64     `json:"attendance_id"`
	StudentID    int64     `json:"student_id"`
	Timestamp    time.Time `json:"timestamp"`
}

// DailySessionCleanupResult represents the result of ending daily sessions
type DailySessionCleanupResult struct {
	SessionsEnded    int       `json:"sessions_ended"`
	VisitsEnded      int       `json:"visits_ended"`
	SupervisorsEnded int       `json:"supervisors_ended"`
	ExecutedAt       time.Time `json:"executed_at"`
	Success          bool      `json:"success"`
	Errors           []string  `json:"errors,omitempty"`
}

// AttendanceCleanupResult represents the result of cleaning stale attendance records
type AttendanceCleanupResult struct {
	StartedAt        time.Time  `json:"started_at"`
	CompletedAt      time.Time  `json:"completed_at"`
	RecordsClosed    int        `json:"records_closed"`
	StudentsAffected int        `json:"students_affected"`
	OldestRecordDate *time.Time `json:"oldest_record_date,omitempty"`
	Success          bool       `json:"success"`
	Errors           []string   `json:"errors,omitempty"`
}

// AttendanceCleanupPreview shows what attendance records would be cleaned
type AttendanceCleanupPreview struct {
	TotalRecords   int            `json:"total_records"`
	StudentRecords map[int64]int  `json:"student_records"` // studentID -> count
	OldestRecord   *time.Time     `json:"oldest_record,omitempty"`
	RecordsByDate  map[string]int `json:"records_by_date"` // date -> count
}
