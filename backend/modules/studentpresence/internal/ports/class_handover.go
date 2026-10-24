package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// ClassHandoverStore persists the class handovers (#4794) in
// active.class_handovers and active.class_handover_children.
type ClassHandoverStore interface {
	// ListClassHandovers returns the tenant's handovers of date with their
	// children, all read from one statement snapshot.
	ListClassHandovers(ctx context.Context, date Date) ([]studentpresence.ClassHandover, Stats, error)
	// ReplaceClassHandover deletes a handover of the same class and date,
	// reports whether there was one, and stores the new one with its
	// children.
	ReplaceClassHandover(ctx context.Context, in studentpresence.ClassHandoverChange, normalizedClass string, at time.Time) (replaced bool, stats Stats, err error)
	// DeleteClassHandover deletes the handover of the normalized class on
	// date; Rows counts the deleted handovers.
	DeleteClassHandover(ctx context.Context, normalizedClass string, date Date) (Stats, error)
	// DeleteClassHandoversBefore deletes the tenant's handovers dated before
	// cutoff, their children with them.
	DeleteClassHandoversBefore(ctx context.Context, cutoff Date) (Stats, error)
}
