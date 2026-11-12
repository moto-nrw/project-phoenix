package careplan

import (
	"context"

	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PickupChangeNotice is a one-day pickup time a family asked for that became
// current on approval, or that a corrected decision took back (#6855).
type PickupChangeNotice struct {
	TenantID   int64
	StudentID  int64
	Date       calendar.Date
	PickupTime string // "15:04"
	// Withdrawn reports that the approved time no longer applies.
	Withdrawn bool
	// ReviewerAccountID decided it and needs no notice.
	ReviewerAccountID int64
	// RequestID tells repeated changes of the same day apart.
	RequestID int64
}

// PickupChangeNotifier tells the staff responsible for the child, inside the
// decision's transaction, that its pickup time of one day changed.
type PickupChangeNotifier interface {
	NotifyPickupChangeApproved(ctx context.Context, notice PickupChangeNotice) error
}
