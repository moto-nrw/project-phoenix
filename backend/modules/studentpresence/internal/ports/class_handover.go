package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// ClassHandoverRecord is one stored handover with its row ID, without its
// absent children.
type ClassHandoverRecord struct {
	ID int64
	studentpresence.ClassHandover
}

// ClassHandoverStore persists the class handovers (#4794) in
// active.class_handovers and active.class_handover_absences.
type ClassHandoverStore interface {
	// ListClassHandovers returns the tenant's handovers of date.
	ListClassHandovers(ctx context.Context, date Date) ([]ClassHandoverRecord, Stats, error)
	// ListClassHandoverAbsences maps each handover ID to its absent
	// children.
	ListClassHandoverAbsences(ctx context.Context, handoverIDs []int64) (map[int64][]int64, Stats, error)
	// ReplaceClassHandover deletes a handover of the same class and date,
	// reports whether there was one, and stores the new one with its absent
	// children.
	ReplaceClassHandover(ctx context.Context, in studentpresence.ClassHandoverChange, normalizedClass string, at time.Time) (replaced bool, stats Stats, err error)
	// DeleteClassHandover deletes the handover of the normalized class on
	// date; Rows counts the deleted handovers.
	DeleteClassHandover(ctx context.Context, normalizedClass string, date Date) (Stats, error)
	// DeleteClassHandoversBefore deletes the tenant's handovers dated before
	// cutoff, their absences with them.
	DeleteClassHandoversBefore(ctx context.Context, cutoff Date) (Stats, error)
}
