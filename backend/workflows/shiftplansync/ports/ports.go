// Package ports declares the consumer-owned seams of the shift-plan-sync
// workflow. Owners satisfy them with their public capabilities; nothing here
// reaches a repository or a model.
package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// Shifts is the Dienstplan surface the sick cascade writes through. Compose
// binds it to Workforce's planning capability (the cancellation, which
// rebuilds the cover set atomically), its row capability (the listings and the
// provenance stamp) and the per-staff shift write lock.
type Shifts interface {
	// ListStaffShifts reads the rows a filter selects, cancelled ones
	// included: the cascade must see an admin cancellation to leave it alone.
	// A non-nil empty StaffIDs, Dates or OriginShiftIDs matches nothing.
	ListStaffShifts(ctx context.Context, filter workforce.StaffShiftFilter) ([]workforce.StaffShift, error)
	// ApplyCancellation cancels or reactivates one shift and replaces its
	// full cover set in one unit of work.
	ApplyCancellation(ctx context.Context, input workforce.CancelStaffShift) (workforce.StaffShiftCancellation, error)
	// SetStaffShiftSickAbsence writes the provenance stamp; nil clears it.
	SetStaffShiftSickAbsence(ctx context.Context, shiftID int64, absenceID *int64) (int64, error)
	// LockStaffShifts serializes the shift writes of one staff member for the
	// rest of the caller's transaction.
	LockStaffShifts(ctx context.Context, staffID int64) error
}
