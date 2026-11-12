package contract

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PickupChangeReport is a one-day pickup time of a child that changed, such
// as one a family asked for and the OGS approved, or one the OGS took back
// (#6855). The caller passes it inside the transaction of the change.
type PickupChangeReport struct {
	TenantID  int64
	StudentID int64
	Date      calendar.Date
	// PickupTime is the new wall-clock time ("15:04"); ignored when
	// Withdrawn is set.
	PickupTime string
	// Withdrawn reports that the changed time of the day no longer applies.
	Withdrawn bool
	// ActorAccountID made the change and is not told about it.
	ActorAccountID int64
	// Revision tells repeated changes of the same day apart, so delivery
	// deduplication never swallows the latest one.
	Revision int64
}

// PickupChangeNotifier tells the staff responsible for the child about the
// changed pickup time. The absence notifier Delivery composes implements it.
type PickupChangeNotifier interface {
	NotifyPickupChanged(ctx context.Context, report PickupChangeReport) error
}
