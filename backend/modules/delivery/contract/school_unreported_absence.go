package contract

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// SchoolOffice lists the accounts of the school that hold
// class_day:all_classes, the Schulverwaltung (#4789), for the notice about
// children missing without a report (#4791). The root binds it over the
// People Directory's staff and their accounts.
type SchoolOffice interface {
	SchoolOfficeAccountIDs(ctx context.Context) ([]int64, error)
}

// UnreportedAbsenceReport names the children of one school missing without a
// report on Date, after a morning mark of ActorAccountID (#4791).
type UnreportedAbsenceReport struct {
	TenantID       int64
	Date           calendar.Date
	StudentIDs     []int64
	ActorAccountID int64
}

// UnreportedAbsenceNotifier tells the Schulverwaltung how many children of
// the school miss without a report (#4791). The absence notifier Delivery
// composes with SchoolClassRecipients.Office implements it.
type UnreportedAbsenceNotifier interface {
	NotifyUnreportedAbsences(ctx context.Context, report UnreportedAbsenceReport) error
}
