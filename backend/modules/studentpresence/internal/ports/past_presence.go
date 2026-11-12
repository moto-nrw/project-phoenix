package ports

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
)

// PastPresenceStore writes the finished days of a presence history batch
// with one statement per table (#5850). Each method assigns the identifiers
// in the order of its input and fills them into the given rows.
type PastPresenceStore interface {
	RecordAttendances(context.Context, []*Attendance) (Stats, error)
	RecordGroups(context.Context, []studentpresence.LiveGroup) ([]studentpresence.LiveGroup, Stats, error)
	RecordVisits(context.Context, []*studentpresence.Visit) (Stats, error)
}
