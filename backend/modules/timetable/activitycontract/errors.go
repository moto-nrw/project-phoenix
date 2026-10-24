package activitycontract

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
	// ErrStudentCareEnded is returned when an enrollment write targets a child
	// whose care at the OGS has ended (#2487). Separate from the alumnus case:
	// a departed child is not a graduate, and the two lead to different places
	// in the UI (Abgänge vs "Beendete Betreuungen").
	ErrStudentCareEnded = errors.New("student care has ended and cannot be enrolled")

	// errStudentRepoMissing marks the internal "cannot check graduation status"
	// condition: the service was built without a student repository. An
	// enrollment write must fail closed on it; a read-only decision that has a
	// safe fallback may catch it instead (see lockEnrollmentStudents).

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

	// ErrSupervisorAlreadyAssigned is returned when the staff member already
	// supervises the activity.
	ErrSupervisorAlreadyAssigned = errors.New("supervisor already assigned to this group")

	// ErrPrimarySupervisorRequired is returned when a primary supervisor is
	// demoted directly; another supervisor has to be made primary instead.
	ErrPrimarySupervisorRequired = errors.New("at least one supervisor must remain primary")

	// ErrSystemActivityProtected is returned when trying to delete or rename a system activity (Schulhof Freispiel, WC).
	ErrSystemActivityProtected = errors.New("systemaktivität kann nicht gelöscht oder umbenannt werden")

	// ErrTimetableTemplateProtected prevents the legacy activities API from
	// bypassing recurring-template locks, lineage checks, and care-offering
	// validation. Timetable templates must be mutated through /timetables.
	ErrTimetableTemplateProtected = errors.New("regeltermine müssen im Betreuungsplan bearbeitet werden")

	// ErrSystemCategoryProtected is returned when a write targets an
	// auto-provisioned system category (Schulhof, WC). Those are infrastructure
	// the kiosk flows resolve by name (#2131).
	ErrSystemCategoryProtected = errors.New("systemkategorie kann nicht bearbeitet oder archiviert werden")

	// ErrSystemCategoryNameReserved prevents school-owned categories from
	// occupying names that the WC and Schulhof infrastructure provisions.
	ErrSystemCategoryNameReserved = errors.New("dieser Name ist für eine Systemkategorie reserviert")

	// ErrCategoryNameExists is returned when a create, rename, or restore would
	// produce two active categories with the same name in one tenant.
	ErrCategoryNameExists = errors.New("eine Kategorie mit diesem Namen existiert bereits")

	// ErrCategoryArchived is returned when an edit targets an archived
	// category. It has to be restored first.
	ErrCategoryArchived = errors.New("archivierte Kategorie muss zuerst wiederhergestellt werden")
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
