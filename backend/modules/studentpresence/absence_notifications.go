package studentpresence

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// AbsenceReport is the recorded absence the student routes announce.
type AbsenceReport struct {
	TenantID       int64
	StudentIDs     []int64
	Status         string
	Dates          []calendar.Date
	FromParent     bool
	ActorAccountID int64
}

// AbsenceNotifier announces a committed absence through the root-bound producer.
type AbsenceNotifier interface {
	NotifyAbsenceReported(context.Context, AbsenceReport) error
}
