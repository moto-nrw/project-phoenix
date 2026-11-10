package careplan

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

var (
	ErrPartialAbsenceAlreadyExists   = errors.New("partial absence already exists for this date")
	ErrPartialAbsenceFullDayConflict = errors.New("partial absence conflicts with a full-day status")
	// ErrPartialAbsencePendingRequestConflict refuses a partial-day excusal when
	// a parent full-day excused request is still pending for that date. Approval
	// would later hit ensureNoPartialAbsence and leave the request unapprovable.
	ErrPartialAbsencePendingRequestConflict = errors.New("partial absence conflicts with a pending full-day excused request")
	ErrPartialAbsencePickupConflict         = errors.New("partial absence conflicts with a full-day pickup cancellation")
	ErrPartialAbsenceNotFound               = errors.New("partial absence not found")
	ErrPartialAbsenceWrongStudent           = errors.New("partial absence belongs to another student")
	// ErrPartialAbsenceAutoManaged refuses deleting an auto-derived excusal
	// through the manual endpoints: it would be re-derived on the next pickup
	// write anyway — undoing it means changing or removing the day's pickup
	// time (#2360).
	ErrPartialAbsenceAutoManaged = errors.New("partial absence is derived from the day's pickup time")
)

// PartialAbsenceInput is the one-day command accepted by the partial-absence
// module. FromTime is a wall-clock value; Date is a calendar date.
type PartialAbsenceInput struct {
	StudentID int64
	Date      calendar.Date
	FromTime  time.Time
	Reason    string
	StaffID   int64
}

// PartialAbsenceService coordinates the existing pickup-exception and
// per-block attendance sources. It deliberately stores no third schedule.
type PartialAbsenceService interface {
	ListPartialAbsences(ctx context.Context, studentID int64, from, to calendar.Date) ([]*PickupException, error)
	CreatePartialAbsence(ctx context.Context, input PartialAbsenceInput) (*PickupException, error)
	UpdatePartialAbsence(ctx context.Context, exceptionID int64, input PartialAbsenceInput) (*PickupException, error)
	DeletePartialAbsence(ctx context.Context, exceptionID, studentID int64) error
	// ReplaceManualPartialAbsences removes the staff-entered partial absences
	// on the given days so a full-day status can take their place (#5698). It
	// runs inside the caller's tenant unit of work, takes the student and
	// care-day locks itself and restores blocks and pickup time exactly like
	// DeletePartialAbsence. Automatic excusals stay; a day without a manual
	// partial absence is left untouched.
	ReplaceManualPartialAbsences(ctx context.Context, studentID int64, dates []calendar.Date) error
}
