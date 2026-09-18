package migrations

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/uptrace/bun"
)

// StudentCompatibilityOptions never authorizes a write to the three owner
// tables. The repair touches the rollback shape and nothing else.
type StudentCompatibilityOptions struct {
	TenantIDs  []int64
	VerifyOnly bool
}

// StudentCompatibilityTenant is what one school looks like through the
// rollback interface.
type StudentCompatibilityTenant struct {
	TenantID int64 `json:"tenant_id"`
	// Students is how many children the previous image would read.
	Students int64 `json:"students"`
	// Profiles is how many children the owner storage holds.
	Profiles int64 `json:"profiles"`
	// Hidden is the difference: profiles whose enrollment was retired after
	// Cutover. The old shape had no soft deletion, so the previous image
	// would not see them either.
	Hidden int64 `json:"hidden"`
	// Broken counts live enrollments without a care profile. Those children
	// fall out of the compatibility view without having left the school,
	// which is the one shape a rollback would silently lose.
	Broken int64 `json:"broken"`
	// StaleArchivedRows are rollback rows of children the owner storage no
	// longer holds. They are unreachable through the view and only in the way
	// of the per-person unique key.
	StaleArchivedRows int64 `json:"stale_archived_rows"`
	// RepairedRows counts the stale rows this run removed.
	RepairedRows int64 `json:"repaired_rows"`
}

// StudentCompatibilityReport is the runtime evidence of the rollback window.
type StudentCompatibilityReport struct {
	// CompatibilityReads and CompatibilityWrites are the cumulative hit
	// counters of the view. Both must trend to zero before #2760 contracts
	// the old storage.
	CompatibilityReads  int64 `json:"compatibility_reads"`
	CompatibilityWrites int64 `json:"compatibility_writes"`
	// UnvalidatedForeignKeys lists the repointed constraints whose existing
	// rows have not been confirmed yet.
	UnvalidatedForeignKeys []string                     `json:"unvalidated_foreign_keys"`
	Tenants                []StudentCompatibilityTenant `json:"tenants"`
}

// Healthy reports whether the previous image would read every enrolled child
// and no constraint is left unconfirmed.
func (r StudentCompatibilityReport) Healthy() bool {
	if len(r.UnvalidatedForeignKeys) > 0 {
		return false
	}
	for _, tenant := range r.Tenants {
		if tenant.Broken > 0 || tenant.StaleArchivedRows > 0 {
			return false
		}
	}
	return true
}

// RepairStudentOwnerCompatibility restores the rollback shape from the
// authoritative owner storage and reports what the previous image would see.
// It rewrites view and routing definitions, resumes the foreign-key
// validation, and removes archive rows whose child no longer exists; it never
// writes users.student_profiles, users.student_school_memberships or
// users.student_care_profiles, and it never resets the hit counters.
func RepairStudentOwnerCompatibility(ctx context.Context, db *bun.DB, options StudentCompatibilityOptions) (StudentCompatibilityReport, error) {
	report := StudentCompatibilityReport{UnvalidatedForeignKeys: []string{}, Tenants: []StudentCompatibilityTenant{}}
	if db == nil {
		return report, errors.New("student compatibility: database is required")
	}
	for _, id := range options.TenantIDs {
		if id <= 0 {
			return report, errors.New("student compatibility: school IDs must be positive")
		}
	}
	release, err := lockStorageBackfill(ctx, db, StudentOwnerBackfillName)
	if err != nil {
		return report, err
	}
	defer release()
	var kind string
	if err := db.NewRaw(`SELECT relkind::text FROM pg_class WHERE oid = 'users.students'::regclass`).Scan(ctx, &kind); err != nil {
		return report, fmt.Errorf("student compatibility: inspect users.students: %w", err)
	}
	if kind != "v" {
		return report, fmt.Errorf("student compatibility: users.students is not the committed compatibility view (relkind %q)", kind)
	}
	if !options.VerifyOnly {
		if err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '5s'`); err != nil {
				return err
			}
			return refreshStudentOwnerCompatibility(ctx, tx)
		}); err != nil {
			return report, err
		}
		if err := ValidateStudentOwnerForeignKeys(ctx, db); err != nil {
			return report, err
		}
	}
	tenantIDs := options.TenantIDs
	if len(tenantIDs) == 0 {
		if err := db.NewRaw(`SELECT id FROM platform.schools ORDER BY id`).Scan(ctx, &tenantIDs); err != nil {
			return report, fmt.Errorf("student compatibility: list schools: %w", err)
		}
	} else {
		tenantIDs = slices.Clone(tenantIDs)
		slices.Sort(tenantIDs)
		tenantIDs = slices.Compact(tenantIDs)
	}
	for _, tenantID := range tenantIDs {
		tenant, err := inspectStudentCompatibilityTenant(ctx, db, tenantID, options.VerifyOnly)
		if err != nil {
			return report, err
		}
		report.Tenants = append(report.Tenants, tenant)
	}
	if err := db.NewRaw(`SELECT
		coalesce(pg_sequence_last_value('users.student_compatibility_reads'), 0),
		coalesce(pg_sequence_last_value('users.student_compatibility_writes'), 0)`).
		Scan(ctx, &report.CompatibilityReads, &report.CompatibilityWrites); err != nil {
		return report, fmt.Errorf("student compatibility: read hit counters: %w", err)
	}
	if err := db.NewRaw(`SELECT con.conrelid::regclass::text || '.' || con.conname
		FROM pg_constraint AS con
		WHERE con.confrelid = 'users.student_profiles'::regclass
		  AND con.contype = 'f' AND NOT con.convalidated
		ORDER BY 1`).Scan(ctx, &report.UnvalidatedForeignKeys); err != nil {
		return report, fmt.Errorf("student compatibility: list unvalidated foreign keys: %w", err)
	}
	if !report.Healthy() {
		return report, errors.New("student compatibility: the rollback shape is incomplete; see the report")
	}
	return report, nil
}

// inspectStudentCompatibilityTenant measures what the previous image would
// read without reading the view itself: a query against users.students would
// raise the very hit counter this report exists to watch, and #2760 gates on
// that counter reaching zero. Students is therefore counted from the owner
// storage under the view's own predicate — a profile with a live enrollment
// and a care profile.
func inspectStudentCompatibilityTenant(ctx context.Context, db *bun.DB, tenantID int64, verifyOnly bool) (StudentCompatibilityTenant, error) {
	tenant := StudentCompatibilityTenant{TenantID: tenantID}
	if err := db.NewRaw(`SELECT
		(SELECT count(*) FROM users.student_profiles p
			JOIN users.student_school_memberships m
				ON m.tenant_id = p.tenant_id AND m.student_profile_id = p.id AND m.deleted_at IS NULL
			JOIN users.student_care_profiles c ON c.tenant_id = m.tenant_id AND c.membership_id = m.id
			WHERE p.tenant_id = ?0),
		(SELECT count(*) FROM users.student_profiles WHERE tenant_id = ?0),
		(SELECT count(*) FROM users.student_profiles p
			WHERE p.tenant_id = ?0 AND NOT EXISTS (
				SELECT 1 FROM users.student_school_memberships m
				WHERE m.tenant_id = p.tenant_id AND m.student_profile_id = p.id AND m.deleted_at IS NULL)),
		(SELECT count(*) FROM users.student_school_memberships m
			WHERE m.tenant_id = ?0 AND m.deleted_at IS NULL AND NOT EXISTS (
				SELECT 1 FROM users.student_care_profiles c
				WHERE c.tenant_id = m.tenant_id AND c.membership_id = m.id)),
		(SELECT count(*) FROM users.students_legacy l
			WHERE l.tenant_id = ?0 AND NOT EXISTS (
				SELECT 1 FROM users.student_profiles p WHERE p.tenant_id = l.tenant_id AND p.id = l.id))`,
		tenantID).Scan(ctx, &tenant.Students, &tenant.Profiles, &tenant.Hidden, &tenant.Broken, &tenant.StaleArchivedRows); err != nil {
		return tenant, fmt.Errorf("student compatibility: inspect tenant %d: %w", tenantID, err)
	}
	if verifyOnly || tenant.StaleArchivedRows == 0 {
		return tenant, nil
	}
	removed, err := rowsAffected(db.NewRaw(`DELETE FROM users.students_legacy AS l
		WHERE l.tenant_id = ? AND NOT EXISTS (
			SELECT 1 FROM users.student_profiles p WHERE p.tenant_id = l.tenant_id AND p.id = l.id)`,
		tenantID).Exec(ctx))
	if err != nil {
		return tenant, fmt.Errorf("student compatibility: drop unreachable archive rows of tenant %d: %w", tenantID, err)
	}
	tenant.RepairedRows = removed
	tenant.StaleArchivedRows -= removed
	return tenant, nil
}
