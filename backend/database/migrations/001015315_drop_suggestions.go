package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	dropSuggestionsVersion     = "1.15.315"
	dropSuggestionsDescription = "Drop the suggestions schema, its permissions and the operator audit rows that point into it (#2326)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dropSuggestionsVersion,
		Description: dropSuggestionsDescription,
		DependsOn:   []string{templateSourceSchoolClassesVersion},
	})

	Migrations.MustRegister(dropSuggestionsUp, dropSuggestionsDown)
}

// dropSuggestionsUp removes the product-feedback board retired by #2326: the
// suggestions system shared by the staff, operator and parents portals.
//
// The daily child and canteen feedback captured at the kiosk is a DIFFERENT
// feature that only shares the word. It lives in the feedback schema, keeps
// POST /api/iot/feedback, and is deliberately untouched here.
//
// THIS MIGRATION DESTROYS DATA. Every board post with its votes, comments and
// read markers is deleted and cannot be recovered from within the application.
// The decision not to export or retain that content was taken before the merge;
// the deploy pipeline takes a pg_dump immediately before running migrations,
// which is the only remaining copy.
//
// DROP SCHEMA ... CASCADE is deliberate and sufficient: the tables, sequences,
// indexes, constraints, triggers and RLS policies of the schema, plus the
// composite foreign keys pointing INTO it, all hang off it and go with it. The
// foreign key pointing OUT of it (suggestions.posts → auth.accounts) is owned
// by a dropped table, so no surviving table keeps a dangling reference.
//
// The app.current_actor_type GUC that tenant.WithParentTx used to set existed
// solely for the RLS split between the staff board and the parents board
// (migration 1.15.241). It is session state, not a schema object, so there is
// nothing to drop — the Go side simply stopped setting it.
func dropSuggestionsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.315: Dropping suggestions...")

	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "DROP SCHEMA IF EXISTS suggestions CASCADE"); err != nil {
			return fmt.Errorf("error dropping schema suggestions: %w", err)
		}

		// Permission rows and their role grants. role_permissions has no
		// ON DELETE CASCADE from permissions in every historical shape, so the
		// grants go first.
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM auth.role_permissions
			WHERE permission_id IN (
				SELECT id FROM auth.permissions WHERE resource = 'suggestions'
			)
		`); err != nil {
			return fmt.Errorf("error deleting suggestions role grants: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
			DELETE FROM auth.permissions WHERE resource = 'suggestions'
		`); err != nil {
			return fmt.Errorf("error deleting suggestions permissions: %w", err)
		}

		// The operator audit trail referenced suggestion posts by a resource
		// type that no longer exists. The rows describe operator actions on
		// content that is gone; keeping them would point at nothing.
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM platform.operator_audit_log WHERE resource_type = 'suggestion'
		`); err != nil {
			return fmt.Errorf("error deleting suggestion operator audit rows: %w", err)
		}

		return nil
	})
}

// dropSuggestionsDown is deliberately a no-op.
//
// A down migration must not pretend it can undo this. Recreating the empty
// schema would produce tables the application no longer knows about and would
// NOT bring back a single deleted post, vote or comment — the content only
// exists in the pre-migration database backup. Restoring it is an operational
// task (restore the dump), not something a migration can do.
func dropSuggestionsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.315 down: no-op — dropped suggestions data can only be restored from a backup")
	return nil
}
