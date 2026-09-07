package schedule

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type StudentVisitReader interface {
	ListVisits(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error)
}

// InstancePresence supplies authoritative visit state for lifecycle transitions.
type InstancePresence interface {
	StudentVisitReader
	TransferOpenVisits(context.Context, int64, int64) (int64, error)
	CountOpenVisitsInRoom(context.Context, int64) (int, error)
}
