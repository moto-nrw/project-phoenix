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

	// ErrStudentNotFound returned when a student is unavailable to activity
	// enrollment reads. Graduated students are soft-deleted and must be
	// indistinguishable from absent students on staff-facing routes.
	ErrStudentNotFound = errors.New("student not found")

	// ErrGroupFull returned when an activity group is at maximum capacity
	ErrGroupFull = errors.New("activity group is at maximum capacity")

	// ErrAlreadyEnrolled returned when a student is already enrolled in a group
	ErrAlreadyEnrolled = errors.New("student is already enrolled in this activity group")

	// ErrStudentAlreadyEnrolled alias for ErrAlreadyEnrolled
	ErrStudentAlreadyEnrolled = ErrAlreadyEnrolled

	// ErrNotEnrolled returned when a student is not enrolled in a group
	ErrNotEnrolled = errors.New("student is not enrolled in this activity group")

	// ErrStudentIsAlumnus returned when an enrollment write targets a graduated
	// student. Their enrollments are hidden from every staff-facing read, so a
	// new one could neither be seen nor removed (#405 review).
	ErrStudentIsAlumnus = errors.New("student has graduated and cannot be enrolled")

	// errStudentRepoMissing marks the internal "cannot check graduation status"
	// condition: the service was built without a student repository. An
	// enrollment write must fail closed on it; a read-only decision that has a
	// safe fallback may catch it instead (see lockEnrollmentStudents).
	errStudentRepoMissing = errors.New("student repository not configured")

	// ErrInvalidAttendanceStatus returned for an invalid attendance status
	ErrInvalidAttendanceStatus = errors.New("invalid attendance status")

	// ErrGroupClosed returned when an activity group is not open for enrollment
	ErrGroupClosed = errors.New("activity group is not open for enrollment")

	// ErrStaffNotFound returned when a staff member doesn't exist
	ErrStaffNotFound = errors.New("staff not found")

	// ErrNotOwner returned when user is not the owner of the activity
	ErrNotOwner = errors.New("you can only modify activities that you created or supervise")

	// ErrOnlySupervisorRequiresReplacement is returned when removing the last
	// remaining supervisor would leave an activity without any supervisor.
	ErrOnlySupervisorRequiresReplacement = errors.New("cannot remove the only supervisor from an activity without a replacement")

	// ErrSystemActivityProtected is returned when trying to delete or rename a system activity (Schulhof Freispiel, WC).
	ErrSystemActivityProtected = errors.New("Systemaktivität kann nicht gelöscht oder umbenannt werden") //nolint:staticcheck // ST1005: user-facing German message

	// ErrTimetableTemplateProtected prevents the legacy activities API from
	// bypassing recurring-template locks, lineage checks, and care-offering
	// validation. Timetable templates must be mutated through /timetables.
	ErrTimetableTemplateProtected = errors.New("Regeltermine müssen im Betreuungsplan bearbeitet werden") //nolint:staticcheck // ST1005: user-facing German message

	// ErrSystemCategoryProtected is returned when a write targets an
	// auto-provisioned system category (Schulhof, WC). Those are infrastructure
	// the kiosk flows resolve by name (#2131).
	ErrSystemCategoryProtected = errors.New("Systemkategorie kann nicht bearbeitet oder archiviert werden") //nolint:staticcheck // ST1005: user-facing German message

	// ErrCategoryNameExists is returned when a create, rename, or restore would
	// produce two active categories with the same name in one tenant.
	ErrCategoryNameExists = errors.New("Eine Kategorie mit diesem Namen existiert bereits") //nolint:staticcheck // ST1005: user-facing German message

	// ErrCategoryArchived is returned when an edit targets an archived
	// category. It has to be restored first.
	ErrCategoryArchived = errors.New("Archivierte Kategorie muss zuerst wiederhergestellt werden") //nolint:staticcheck // ST1005: user-facing German message
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
