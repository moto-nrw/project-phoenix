package activities

import (
	"errors"
)

// Define attendance status constants
const (
	AttendancePresent = "PRESENT"
	AttendanceAbsent  = "ABSENT"
	AttendanceExcused = "EXCUSED"
	AttendanceUnknown = "UNKNOWN"
)

// Table name constants for BUN ORM schema qualification
const ()

// StudentEnrollment represents a student enrolled in an activity group
type StudentEnrollment struct {
	Model `bun:"schema:activities,table:student_enrollments"`
	TenantModel
	StudentID        int64  `bun:"student_id,notnull" json:"student_id"`
	ActivityGroupID  int64  `bun:"activity_group_id,notnull" json:"activity_group_id"`
	ValidFrom        Date   `bun:"valid_from,notnull" json:"valid_from"`
	ValidUntil       *Date  `bun:"valid_until" json:"valid_until,omitempty"`
	CalendarPeriodID *int64 `bun:"calendar_period_id" json:"calendar_period_id,omitempty"`
	// EnrollmentRequestChildID marks activity rows materialized from an
	// approved enrollment request child so later offering adjustments can
	// replace exactly those rows, even if offering groups or phase dates changed.
	EnrollmentRequestChildID *int64  `bun:"enrollment_request_child_id" json:"enrollment_request_child_id,omitempty"`
	SelectedWeekdays         []int   `bun:"selected_weekdays,type:jsonb,nullzero" json:"selected_weekdays,omitempty"`
	AttendanceStatus         *string `bun:"attendance_status" json:"attendance_status,omitempty"`
	// Weekday scopes this enrollment to a single ISO weekday (1=Mon … 7=Sun)
	// of the recurring template (issue #2129). NULL = applies on every weekday
	// the series runs. Distinct from SelectedWeekdays, which is owned by the
	// enrollment/care-offering decision path and marks a row the template
	// editor must not touch; Weekday is editor-owned and replaceable.
	Weekday *int `bun:"weekday" json:"weekday,omitempty"`
	// StudentAlumnus and the name fields are read projections supplied by the
	// People Directory. They are not persisted with the enrollment row.
	StudentAlumnus   bool   `bun:"-" json:"-"`
	StudentFirstName string `bun:"-" json:"-"`
	StudentLastName  string `bun:"-" json:"-"`

	// Relations - populated when using the ORM's relations
	ActivityGroup *Group `bun:"rel:belongs-to,join:activity_group_id=id" json:"activity_group,omitempty"`
}

// IsValidAttendanceStatus checks if the attendance status is valid
func IsValidAttendanceStatus(status string) bool {
	validStatuses := map[string]bool{
		AttendancePresent: true,
		AttendanceAbsent:  true,
		AttendanceExcused: true,
		AttendanceUnknown: true,
	}
	return validStatuses[status]
}

// Validate ensures student enrollment data is valid
func (se *StudentEnrollment) Validate() error {
	if se.StudentID <= 0 {
		return errors.New("student ID is required")
	}

	if se.ActivityGroupID <= 0 {
		return errors.New("activity group ID is required")
	}

	if se.AttendanceStatus != nil && !IsValidAttendanceStatus(*se.AttendanceStatus) {
		return errors.New("invalid attendance status")
	}

	seenWeekdays := make(map[int]bool, len(se.SelectedWeekdays))
	for _, weekday := range se.SelectedWeekdays {
		if !IsValidWeekday(weekday) {
			return errors.New("selected weekdays must be between 1 and 7")
		}
		if seenWeekdays[weekday] {
			return errors.New("selected weekdays must not contain duplicates")
		}
		seenWeekdays[weekday] = true
	}

	if se.Weekday != nil && !IsValidWeekday(*se.Weekday) {
		return errors.New("weekday must be between 1 and 7")
	}

	return nil
}

// ClearAttendance clears the attendance status
func (se *StudentEnrollment) ClearAttendance() {
	se.AttendanceStatus = nil
}
