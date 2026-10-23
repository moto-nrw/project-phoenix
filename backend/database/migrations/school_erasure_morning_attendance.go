package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

// active.morning_attendance (#4790) is a tenant table younger than the frozen
// school erasure of 1.15.481 (ADR 0092). Its plan, lock set, incoming-FK
// fingerprint and scope checks, deletion order and residual check must name
// the table, or every erasure fails closed. 1.15.484 splices it into each
// phase; its down migration reinstalls the 1.15.481 phases.

// morningAttendanceErasureFingerprint is the preflight's incoming-FK
// fingerprint with the four edges of active.morning_attendance added.
const morningAttendanceErasureFingerprint = "3a88c872eba47b69f82876605b7073ea9d6e2fcad012928c53083f4964937d99"

type erasureSplice struct{ old, new string }

// spliceErasureSQL replaces each anchor, which must occur exactly once.
func spliceErasureSQL(phase, sql string, splices ...erasureSplice) (string, error) {
	for _, s := range splices {
		if n := strings.Count(sql, s.old); n != 1 {
			return "", fmt.Errorf("school erasure %s: anchor %q occurs %d times", phase, s.old, n)
		}
		sql = strings.Replace(sql, s.old, s.new, 1)
	}
	return sql, nil
}

// erasureAfter inserts text behind an anchor.
func erasureAfter(anchor, text string) erasureSplice {
	return erasureSplice{old: anchor, new: anchor + text}
}

func morningAttendanceErasurePhases() ([]string, error) {
	const (
		planAnchor     = "  {\"name\": \"active.groups\",\"predicate\": \"r.tenant_id = $1\"},\n"
		lockAnchor     = "    active.groups,\n"
		scopeAnchor    = "    SELECT 1 FROM active.excused_absence_requests AS c JOIN users.student_profiles AS p ON c.\"student_id\" = p.\"id\" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)\n    UNION ALL\n"
		deleteAnchor   = "   v_counts := v_counts || jsonb_build_object('active.groups', v_count);\n"
		residualAnchor = "    SELECT 1 FROM active.groups AS r WHERE r.tenant_id = p_school_id\n    UNION ALL\n"
		oldFingerprint = "965e62ae91b59194532b9156d076734507fa84e1c90dd8d5b5dc93d16572d510"
	)
	plan, err := spliceErasureSQL("plan", schoolErasurePlanSQL,
		erasureAfter(planAnchor, "  {\"name\": \"active.morning_attendance\",\"predicate\": \"r.tenant_id = $1\"},\n"))
	if err != nil {
		return nil, err
	}
	preflight, err := spliceErasureSQL("preflight", schoolErasurePreflightSQL,
		erasureAfter(lockAnchor, "    active.morning_attendance,\n"),
		erasureSplice{old: oldFingerprint, new: morningAttendanceErasureFingerprint},
		erasureAfter(scopeAnchor, "    SELECT 1 FROM active.morning_attendance AS c JOIN users.class_list_entries AS p ON c.\"class_list_entry_id\" = p.\"id\" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)\n    UNION ALL\n"+
			"    SELECT 1 FROM active.morning_attendance AS c JOIN users.staff_school_memberships AS p ON c.\"recorded_by_staff_id\" = p.\"id\" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)\n    UNION ALL\n"))
	if err != nil {
		return nil, err
	}
	deletion, err := spliceErasureSQL("delete", schoolErasureDeleteSQL,
		erasureAfter(deleteAnchor, "   SELECT count(*) INTO v_count FROM active.morning_attendance AS r WHERE r.tenant_id = p_school_id;\n"+
			"   DELETE FROM active.morning_attendance AS r WHERE r.tenant_id = p_school_id;\n"+
			"   v_counts := v_counts || jsonb_build_object('active.morning_attendance', v_count);\n"))
	if err != nil {
		return nil, err
	}
	residual, err := spliceErasureSQL("residual", schoolErasureResidualSQL,
		erasureAfter(residualAnchor, "    SELECT 1 FROM active.morning_attendance AS r WHERE r.tenant_id = p_school_id\n    UNION ALL\n"))
	if err != nil {
		return nil, err
	}
	return []string{plan, preflight, deletion, residual}, nil
}

// installMorningAttendanceErasure replaces the four erasure phases with the
// versions that cover active.morning_attendance.
func installMorningAttendanceErasure(ctx context.Context, tx bun.Tx) error {
	phases, err := morningAttendanceErasurePhases()
	if err != nil {
		return err
	}
	for _, phase := range phases {
		if _, err := tx.ExecContext(ctx, phase); err != nil {
			return fmt.Errorf("install school erasure phase: %w", err)
		}
	}
	return nil
}

// restoreSchoolErasurePhases reinstalls the 1.15.481 phases.
func restoreSchoolErasurePhases(ctx context.Context, tx bun.Tx) error {
	for _, install := range []func(context.Context, bun.Tx) error{
		installSchoolErasurePlan, installSchoolErasurePreflight, installSchoolErasureDelete, installSchoolErasureResidual,
	} {
		if err := install(ctx, tx); err != nil {
			return fmt.Errorf("restore school erasure phase: %w", err)
		}
	}
	return nil
}
