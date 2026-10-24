package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// active.class_handovers and active.class_handover_children (#4794) are
// tenant tables younger than the erasure phases 1.15.491 froze (ADR 0092).
// 1.15.492 installs the five functions frozen anew with both tables; its down
// migration reinstalls those of 1.15.491. Against them the functions add
// exactly:
//
//   - plan: the two entries, predicate r.tenant_id = $1;
//   - preflight: both tables in the lock set, the fingerprint with their five
//     incoming edges, and a scope check for the single-column edge
//     handed_over_by_staff_id (the handover and student edges of the
//     absences carry tenant_id);
//   - delete: the absences, then the handovers, after
//     active.morning_attendance and before every parent they name;
//   - residual: a rest check per table;
//   - preview: a count per table.
//
// A later tenant table freezes these functions anew from the 1.15.492 ones.
//
// CREATE OR REPLACE keeps each function's owner and grants.

// installClassHandoverErasure installs the phases that cover the class
// handover tables.
func installClassHandoverErasure(ctx context.Context, tx bun.Tx) error {
	for _, install := range []func(context.Context, bun.Tx) error{
		installClassHandoverErasurePlan, installClassHandoverErasurePreflight,
		installClassHandoverErasureDelete, installClassHandoverErasureResidual,
		installClassHandoverErasurePreview,
	} {
		if err := install(ctx, tx); err != nil {
			return fmt.Errorf("install school erasure phase with active.class_handovers: %w", err)
		}
	}
	return nil
}
