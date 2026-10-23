package migrations

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/uptrace/bun"
)

func schoolErasurePrecondition(ctx context.Context, db *bun.DB) error {
	return requireSchoolErasureReferences(ctx, db)
}

// A fresh install has none of these tables. All SQL stays statically visible
// to the architecture guard. Report counts only, never row contents or IDs.
func requireSchoolErasureReferences(ctx context.Context, db bun.IDB) error {
	var existing []string
	if err := db.NewRaw(`SELECT name FROM unnest(ARRAY['active.presence_backfill_batches',
 'audit.room_color_migration_backup','audit.wc_alias_migration_backup',
 'auth.mfa_email_challenges','auth.parent_declaration_permission_grants']) AS name
 WHERE to_regclass(name) IS NOT NULL`).Scan(ctx, &existing); err != nil {
		return fmt.Errorf("check school erasure source tables: %w", err)
	}
	var counts [5]int64
	if slices.Contains(existing, "active.presence_backfill_batches") {
		if err := db.NewRaw(`SELECT count(*) FROM active.presence_backfill_batches r
   WHERE r.tenant_id IS NOT NULL AND NOT EXISTS (SELECT FROM platform.schools s WHERE s.id=r.tenant_id)`).Scan(ctx, &counts[0]); err != nil {
			return fmt.Errorf("count active.presence_backfill_batches orphan references: %w", err)
		}
	}
	if slices.Contains(existing, "audit.room_color_migration_backup") {
		if err := db.NewRaw(`SELECT count(*) FROM audit.room_color_migration_backup r
   WHERE r.tenant_id IS NOT NULL AND NOT EXISTS (SELECT FROM platform.schools s WHERE s.id=r.tenant_id)`).Scan(ctx, &counts[1]); err != nil {
			return fmt.Errorf("count audit.room_color_migration_backup orphan references: %w", err)
		}
	}
	if slices.Contains(existing, "audit.wc_alias_migration_backup") {
		if err := db.NewRaw(`SELECT count(*) FROM audit.wc_alias_migration_backup r
   WHERE r.tenant_id IS NOT NULL AND NOT EXISTS (SELECT FROM platform.schools s WHERE s.id=r.tenant_id)`).Scan(ctx, &counts[2]); err != nil {
			return fmt.Errorf("count audit.wc_alias_migration_backup orphan references: %w", err)
		}
	}
	if slices.Contains(existing, "auth.mfa_email_challenges") {
		if err := db.NewRaw(`SELECT count(*) FROM auth.mfa_email_challenges r
   WHERE r.tenant_id IS NOT NULL AND NOT EXISTS (SELECT FROM platform.schools s WHERE s.id=r.tenant_id)`).Scan(ctx, &counts[3]); err != nil {
			return fmt.Errorf("count auth.mfa_email_challenges orphan references: %w", err)
		}
	}
	if slices.Contains(existing, "auth.parent_declaration_permission_grants") {
		if err := db.NewRaw(`SELECT count(*) FROM auth.parent_declaration_permission_grants r
   WHERE r.tenant_id IS NOT NULL AND NOT EXISTS (SELECT FROM platform.schools s WHERE s.id=r.tenant_id)`).Scan(ctx, &counts[4]); err != nil {
			return fmt.Errorf("count auth.parent_declaration_permission_grants orphan references: %w", err)
		}
	}
	var failures []string
	for i, table := range []string{"active.presence_backfill_batches", "audit.room_color_migration_backup", "audit.wc_alias_migration_backup", "auth.mfa_email_challenges", "auth.parent_declaration_permission_grants"} {
		if counts[i] > 0 {
			failures = append(failures, fmt.Sprintf("%s.tenant_id: %d orphaned rows", table, counts[i]))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("school erasure requires valid school references: %s; run docs/operations/sql/mandantenloeschung-vorpruefung-3837.sql and follow docs/reference/mandantenloeschung-datenbank.md before deployment", strings.Join(failures, "; "))
	}
	return nil
}
