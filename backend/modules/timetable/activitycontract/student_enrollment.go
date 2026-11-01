package activitycontract

import (
	"errors"
	"time"
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
	ID               int64     `json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	TenantID         int64     `json:"tenant_id"`
	StudentID        int64     `json:"student_id"`
	ActivityGroupID  int64     `json:"activity_group_id"`
	ValidFrom        Date      `json:"valid_from"`
	ValidUntil       *Date     `json:"valid_until,omitempty"`
	CalendarPeriodID *int64    `json:"calendar_period_id,omitempty"`
	// EnrollmentRequestChildID marks activity rows materialized from an
	// approved enrollment request child so later offering adjustments can
	// replace exactly those rows, even if offering groups or phase dates changed.
	EnrollmentRequestChildID *int64  `json:"enrollment_request_child_id,omitempty"`
	SelectedWeekdays         []int   `json:"selected_weekdays,omitempty"`
	AttendanceStatus         *string `json:"attendance_status,omitempty"`
	// Weekday scopes this enrollment to a single ISO weekday (1=Mon … 7=Sun)
	// of the recurring template (issue #2129). NULL = applies on every weekday
	// the series runs. Distinct from SelectedWeekdays, which is owned by the
	// enrollment/care-offering decision path and marks a row the template
	// editor must not touch; Weekday is editor-owned and replaceable.
	Weekday *int `json:"weekday,omitempty"`
	// KeptWithoutBooking marks per ISO weekday the day up to which the school
	// keeps the child on this hand row although ended bookings no longer
	// carry it (#5102).
	KeptWithoutBooking map[int]Date `json:"kept_without_booking,omitempty"`
	// StudentAlumnus and the name fields are read projections supplied by the
	// People Directory. They are not persisted with the enrollment row.
	StudentAlumnus   bool   `json:"-"`
	StudentFirstName string `json:"-"`
	StudentLastName  string `json:"-"`

	// Relations - populated when using the ORM's relations
	ActivityGroup *Group `json:"activity_group,omitempty"`
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

func (v *StudentEnrollment) GetTenantID() int64   { return v.TenantID }
func (v *StudentEnrollment) SetTenantID(id int64) { v.TenantID = id }
