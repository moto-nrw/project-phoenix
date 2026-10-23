package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// active.morning_attendance (#4790) is a tenant table younger than the frozen
// school erasure of 1.15.481 (ADR 0092). Its plan, lock set, incoming-FK
// fingerprint and scope checks, deletion order and residual check must name
// the table, or every erasure fails closed; the preview of 1.15.490 (#3837)
// must count it. 1.15.491 installs these five functions frozen anew with the
// table; its down migration reinstalls those of 1.15.481 and 1.15.490.
// Against them the functions add exactly:
//
//   - plan: the entry active.morning_attendance, predicate r.tenant_id = $1;
//   - preflight: the table in the lock set, the fingerprint with its four
//     incoming edges, and scope checks for the single-column edges
//     class_list_entry_id and recorded_by_staff_id (the student edge carries
//     tenant_id);
//   - delete: the table after active.groups, before every parent it names;
//   - residual: the table's rest check;
//   - preview: the table's count after active.groups.
//
// A later tenant table freezes these functions anew from the 1.15.491 ones.
//
// CREATE OR REPLACE keeps each function's owner and grants.

// installMorningAttendanceErasure installs the phases that cover
// active.morning_attendance.
func installMorningAttendanceErasure(ctx context.Context, tx bun.Tx) error {
	for _, install := range []func(context.Context, bun.Tx) error{
		installMorningAttendanceErasurePlan, installMorningAttendanceErasurePreflight,
		installMorningAttendanceErasureDelete, installMorningAttendanceErasureResidual,
		installMorningAttendanceErasurePreview,
	} {
		if err := install(ctx, tx); err != nil {
			return fmt.Errorf("install school erasure phase with active.morning_attendance: %w", err)
		}
	}
	return nil
}

// restoreSchoolErasurePhases reinstalls the 1.15.481 phases and the 1.15.490
// preview.
func restoreSchoolErasurePhases(ctx context.Context, tx bun.Tx) error {
	for _, install := range []func(context.Context, bun.Tx) error{
		installSchoolErasurePlan, installSchoolErasurePreflight, installSchoolErasureDelete, installSchoolErasureResidual,
		restoreSchoolErasurePreview,
	} {
		if err := install(ctx, tx); err != nil {
			return fmt.Errorf("restore school erasure phase: %w", err)
		}
	}
	return nil
}
