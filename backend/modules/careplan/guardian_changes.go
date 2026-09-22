package careplan

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrManualPartialAbsenceConflict refuses a full-day guardian absence on a
	// day that already carries a staff-entered partial absence.
	ErrManualPartialAbsenceConflict = errors.New("care plan: a manual partial absence covers a requested day")
	// ErrPickupExceptionStaffOwned refuses a guardian write to a staff-owned
	// pickup exception; the staff row is neither overwritten nor deleted.
	ErrPickupExceptionStaffOwned = errors.New("care plan: the pickup exception is owned by staff")
	// ErrGuardianPickupExceptionRaced joins the raw database error when a
	// guardian pickup write hits a unique violation from a concurrent write.
	ErrGuardianPickupExceptionRaced = errors.New("guardian pickup exception raced a concurrent write")
)

// GuardianAbsenceReport is a guardian's full-day status for one or more days.
// Dates keep the submitted order; ReportedAt stamps every written row.
type GuardianAbsenceReport struct {
	StudentID         int64
	GuardianAccountID int64
	Dates             []Date
	Status            string
	Note              *string
	ReportedAt        time.Time
}

// GuardianAbsenceReports records guardian absences inside the caller's tenant
// transaction. It takes the student and care-day locks itself; the caller
// owns authorization, care-interval checks, and notifications.
type GuardianAbsenceReports interface {
	// ReportGuardianAbsence returns the active rows with the reported status
	// on exactly the reported dates.
	ReportGuardianAbsence(context.Context, GuardianAbsenceReport) ([]StudentStatusDay, error)
}

// GuardianPickupChange sets (PickupTime non-nil) or clears (PickupTime nil)
// the guardian-owned pickup exception of one day.
type GuardianPickupChange struct {
	TenantID          int64
	StudentID         int64
	GuardianAccountID int64
	Date              Date
	PickupTime        *time.Time
	Reason            *string
}

// GuardianPickupExceptions writes the guardian leg of a day's pickup exception
// and keeps the derived pickup excusal in step. Callers hold the student and
// care-day locks in their tenant transaction.
type GuardianPickupExceptions interface {
	ApplyGuardianPickupException(context.Context, GuardianPickupChange) error
	// WithdrawGuardianPickupException releases the row's derived excusal and
	// deletes the row. The caller has already decided the row may go.
	WithdrawGuardianPickupException(context.Context, PickupException) error
}
