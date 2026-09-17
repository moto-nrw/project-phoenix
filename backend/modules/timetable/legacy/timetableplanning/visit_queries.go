package timetableplanning

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type StudentVisitReader interface {
	ListVisits(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error)
}

// ConflictPresence supplies current visits and staff supervision facts.
type ConflictPresence interface {
	StudentVisitReader
	QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
}

// InstancePresence supplies authoritative presence state for lifecycle transitions.
type InstancePresence interface {
	ConflictPresence
	TransferOpenVisits(context.Context, int64, int64) (int64, error)
	// EndGroup releases an absorbed unsupervised group after its visits moved
	// to the started instance's group (#2697).
	EndGroup(context.Context, int64, time.Time) error
	CountOpenVisitsInRoom(context.Context, int64) (int, error)
}
