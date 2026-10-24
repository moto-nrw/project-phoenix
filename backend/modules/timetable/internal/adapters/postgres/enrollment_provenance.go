package postgres

import (
	"context"

	"github.com/uptrace/bun"
)

// EnrollmentProvenance is Timetable's reader port onto the enrollment
// provenance projection (ADR 0058), which joins Timetable's enrollments to
// Enrollment's approved request children. The root binds it. Both reads run
// in the caller's transaction; EligibleEnrollmentIDs locks the enrollment
// rows it returns.
type EnrollmentProvenance interface {
	EligibleEnrollmentIDs(ctx context.Context, db bun.IDB, tenantID, studentID, requestChildID int64, groupIDs []int64) ([]int64, error)
	ExistingRequestChildIDs(ctx context.Context, db bun.IDB, tenantID int64, ids []int64) ([]int64, error)
}

// ProvenanceStore is the Timetable store with the provenance reader bound:
// the writes that link or restore enrollments by their request source.
type ProvenanceStore struct {
	*Store
	provenance EnrollmentProvenance
}

// NewProvenanceStore binds the provenance reader to store.
func NewProvenanceStore(store *Store, provenance EnrollmentProvenance) *ProvenanceStore {
	if store == nil || provenance == nil {
		panic("timetable postgres: store and enrollment provenance are required")
	}
	return &ProvenanceStore{Store: store, provenance: provenance}
}
