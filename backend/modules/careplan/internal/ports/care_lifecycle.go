package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The care lifecycle's consumer-owned ports (#3427). Care Plan decides; the
// owners of the child row, the timetable, the applications, the presence
// records, the bracelets and the change history carry out their half inside
// the same tenant transaction. Composition binds each port to its owner.

// CareStudentDirectory is the People Directory half: the child rows and the
// reversible membership commands the care-exit transaction runs.
type CareStudentDirectory interface {
	// FindCareStudents reads the tenant's rows of the given children, FOR
	// UPDATE when lock is set. A missing child is absent from the map.
	FindCareStudents(ctx context.Context, studentIDs []int64, lock bool) (map[int64]domain.CareStudent, error)
	// FindCarePersons reads the person rows by person id.
	FindCarePersons(ctx context.Context, personIDs []int64) (map[int64]domain.CarePerson, error)
	// ListDueForDeactivation lists the still-active children whose enrolment
	// ended on or before lastCareDay.
	ListDueForDeactivation(ctx context.Context, lastCareDay calendar.Date) ([]int64, error)
	// ListStudentIDs lists every child id of the tenant.
	ListStudentIDs(ctx context.Context) ([]int64, error)
	// EndCare sets the last care day of every given child; nil clears it.
	// It fails with careplan.ErrCareExitPreviewChanged when a row moved.
	EndCare(ctx context.Context, studentIDs []int64, lastCareDay *calendar.Date) error
	// ResumeCare reopens one child's interval from start with status, frozen
	// on the given day. It fails with careplan.ErrCareResumeNotEnded when the
	// care did not end.
	ResumeCare(ctx context.Context, studentID int64, start calendar.Date, status string, on calendar.Date) error
	// ListWithdrawalStudents reads the display rows a withdrawal list joins.
	ListWithdrawalStudents(ctx context.Context, studentIDs []int64) ([]careplan.WithdrawalStudent, error)
}

// CareExitRoster is the Timetable half of the roster: the planned rows a care
// exit removes and restores, and the open check-ins it closes. Removed rows
// come back as roster-kind ledger entries.
type CareExitRoster interface {
	LockPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after calendar.Date) error
	RemovePlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after calendar.Date) ([]careplan.CareExitRemoval, error)
	RestoreRosterForCareExit(ctx context.Context, studentIDs []int64, removals []careplan.CareExitRemoval) (int, error)
	CountPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after calendar.Date, restorable []careplan.CareExitRemoval) (map[int64]int, error)
	ListOpenStudentAssignments(ctx context.Context, studentIDs []int64) ([]int64, error)
	LatestStudentAssignmentAttendanceDate(ctx context.Context, studentID int64) (*calendar.Date, error)
	LockOpenStudentAssignments(ctx context.Context, studentIDs []int64) error
	CloseOpenStudentAssignments(ctx context.Context, studentIDs []int64, at time.Time) (int64, error)
	ReconnectCareExitAssignmentPickupExceptions(ctx context.Context, studentIDs, pickupExceptionIDs []int64, removals []careplan.CareExitRemoval) error
}

// CareExitBookings is the Timetable half of the activity bookings.
type CareExitBookings interface {
	LockStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil calendar.Date) error
	EndStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil calendar.Date) (domain.CareExitBookingChanges, error)
	RestoreStudentEnrollmentsForCareExit(ctx context.Context, studentIDs, calendarPeriodIDs []int64, removals []domain.CareExitBookingRestore) (int, error)
	CountRunningEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil calendar.Date, restorable []domain.CareExitBookingRestore) (map[int64]int, error)
}

// CareExitEnrollment is the Enrollment half: the applications that created
// the children and the source bookings a care exit ends and restores. The
// booking commands join the caller's transaction.
type CareExitEnrollment interface {
	CreatedStudentRequestChildIDs(ctx context.Context, studentIDs []int64) ([]int64, error)
	CareExitApplicationLinks(ctx context.Context, studentIDs []int64) ([]domain.CareExitApplication, error)
	CareExitOfferingLinks(ctx context.Context, studentIDs []int64) ([]domain.CareExitOfferingLink, error)
	LockCareExitOfferingLinks(ctx context.Context, requestChildIDs []int64, from calendar.Date) error
	CareExitOfferingSnapshots(ctx context.Context, studentIDs []int64, validUntil calendar.Date, sourceRequestChildID *int64) ([]domain.CareExitOfferingSnapshot, error)
	EndCareExitOfferingLinks(ctx context.Context, requestChildIDs []int64, sourceRequestChildID *int64, validUntil calendar.Date) (int64, error)
	RestoreCareExitOfferingLinks(ctx context.Context, restores []domain.CareExitOfferingRestore) (int64, error)
}

// CareExitPresence is the Student Presence half: school attendance and room
// visits stay owned there.
type CareExitPresence interface {
	ListOpenPresence(ctx context.Context, studentIDs []int64) ([]int64, error)
	LatestPresenceDate(ctx context.Context, studentID int64) (*calendar.Date, error)
	LockOpenPresence(ctx context.Context, studentIDs []int64) error
	CloseOpenPresence(ctx context.Context, studentIDs []int64, at time.Time) (int64, error)
}

// CareCalendarPeriods is the School Calendar query the booking restore
// re-validates period references through.
type CareCalendarPeriods interface {
	ListCalendarPeriodIDs(ctx context.Context) ([]int64, error)
}

// CareExitTagReleaser frees the physical bracelets of children whose care
// ended and reports how many it released.
type CareExitTagReleaser interface {
	ReleaseStudentTags(ctx context.Context, studentIDs []int64) (int, error)
}

// CareEndRecorder appends one change-history row for a moved last care day,
// attributed to the acting account. The reason never goes there: the history
// is readable with users:read, the reason only with users:delete.
type CareEndRecorder interface {
	RecordCareEndChange(ctx context.Context, studentID int64, before, after *calendar.Date, actorAccountID int64) error
}

// CareExitDirectory holds the directory reads the lifecycle takes through
// the named tenant-safe student directory projection.
type CareExitDirectory interface {
	ListEndedCare(ctx context.Context, asOf domain.Date, filter careplan.EndedCareFilter) ([]domain.EndedCareStudent, int, error)
	LockCareExitPeople(ctx context.Context, studentIDs []int64) error
	ListCareBookingStudents(ctx context.Context, on domain.Date, studentIDs []int64) ([]domain.CareBookingStudent, error)
}

// UnitOfWork runs a command in the caller's tenant transaction, or in a new
// one when none is open.
type UnitOfWork func(ctx context.Context, command func(context.Context) error) error
