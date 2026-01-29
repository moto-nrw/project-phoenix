package activities

import (
	"errors"
	"fmt"
)

var (
	// ErrCategoryNotFound returned when a category doesn't exist
	ErrCategoryNotFound = errors.New("activity category not found")

	// ErrGroupNotFound returned when an activity group doesn't exist
	ErrGroupNotFound = errors.New("activity group not found")

	// ErrScheduleNotFound returned when a schedule doesn't exist
	ErrScheduleNotFound = errors.New("schedule not found")

	// ErrSupervisorNotFound returned when a supervisor doesn't exist
	ErrSupervisorNotFound = errors.New("supervisor not found")

	// ErrEnrollmentNotFound returned when an enrollment doesn't exist
	ErrEnrollmentNotFound = errors.New("enrollment not found")

	// ErrGroupFull returned when an activity group is at maximum capacity
	ErrGroupFull = errors.New("activity group is at maximum capacity")

	// ErrAlreadyEnrolled returned when a student is already enrolled in a group
	ErrAlreadyEnrolled = errors.New("student is already enrolled in this activity group")

	// ErrStudentAlreadyEnrolled alias for ErrAlreadyEnrolled
	ErrStudentAlreadyEnrolled = ErrAlreadyEnrolled

	// ErrNotEnrolled returned when a student is not enrolled in a group
	ErrNotEnrolled = errors.New("student is not enrolled in this activity group")

	// ErrInvalidAttendanceStatus returned for an invalid attendance status
	ErrInvalidAttendanceStatus = errors.New("invalid attendance status")

	// ErrGroupClosed returned when an activity group is not open for enrollment
	ErrGroupClosed = errors.New("activity group is not open for enrollment")

	// ErrCannotDeletePrimary returned when attempting to delete a primary supervisor
	ErrCannotDeletePrimary = errors.New("cannot delete primary supervisor")

	// ErrStaffNotFound returned when a staff member doesn't exist
	ErrStaffNotFound = errors.New("staff not found")

	// ErrNotOwner returned when user is not the owner of the activity
	ErrNotOwner = errors.New("you can only modify activities that you created or supervise")

	// ErrUnauthorized returned when user doesn't have permission to perform the action
	ErrUnauthorized = errors.New("you are not authorized to perform this action")
)

// ActivityError represents an activity-related error
type ActivityError struct {
	Op  string // Operation that failed
	Err error  // Original error
}

// Error returns the error message
func (e *ActivityError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("activity error during %s", e.Op)
	}
	return fmt.Sprintf("activity error during %s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *ActivityError) Unwrap() error {
	return e.Err
}
