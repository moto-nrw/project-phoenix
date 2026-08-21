package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	dropFeedbackSuggestionsVersion     = "1.15.314"
	dropFeedbackSuggestionsDescription = "Drop the feedback and suggestions schemas, their permissions and settings (#2326)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dropFeedbackSuggestionsVersion,
		Description: dropFeedbackSuggestionsDescription,
		DependsOn:   []string{careRequestSnapshotGermanDateVersion},
	})

	Migrations.MustRegister(dropFeedbackSuggestionsUp, dropFeedbackSuggestionsDown)
}

// dropFeedbackSuggestionsUp removes the two product areas retired by #2326:
// the daily child/canteen feedback captured at the kiosk (schema feedback) and
// the product-feedback board shared by the staff, operator and parents portals
// (schema suggestions).
//
// THIS MIGRATION DESTROYS DATA. Every feedback entry, every board post with its
// votes, comments and read markers is deleted and cannot be recovered from
// within the application. The decision not to export or retain that content was
// taken before the merge; the deploy pipeline takes a pg_dump immediately
// before running migrations, which is the only remaining copy.
//
// DROP SCHEMA ... CASCADE is deliberate and sufficient: the tables, sequences,
// indexes, constraints, triggers and RLS policies of both schemas, plus the
// composite foreign keys pointing INTO them, all hang off those schemas and go
// with them. Foreign keys pointing OUT of them (feedback.entries → users.students,
// suggestions.posts → auth.accounts) are owned by the dropped tables, so no
// surviving table keeps a dangling reference.
//
// The app.current_actor_type GUC that tenant.WithParentTx used to set existed
// solely for the suggestions RLS policies (migration 1.15.241). It is session
// state, not a schema object, so there is nothing to drop — the Go side simply
// stopped setting it.
func dropFeedbackSuggestionsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.314: Dropping feedback and suggestions...")

	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		for _, schema := range []string{"feedback", "suggestions"} {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema)); err != nil {
				return fmt.Errorf("error dropping schema %s: %w", schema, err)
			}
		}

		// Permission rows and their role grants. role_permissions has no
		// ON DELETE CASCADE from permissions in every historical shape, so the
		// grants go first.
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM auth.role_permissions
			WHERE permission_id IN (
				SELECT id FROM auth.permissions WHERE resource IN ('feedback', 'suggestions')
			)
		`); err != nil {
			return fmt.Errorf("error deleting feedback/suggestions role grants: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
			DELETE FROM auth.permissions WHERE resource IN ('feedback', 'suggestions')
		`); err != nil {
			return fmt.Errorf("error deleting feedback/suggestions permissions: %w", err)
		}

		// Tenant overrides and the audit trail of the two retired settings
		// (feedback.enabled, feedback.data_retention_days). Their registry
		// definitions are gone, so a leftover row would fail schema lookups.
		for _, table := range []string{"config.setting_values", "config.setting_audit"} {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf(
				"DELETE FROM %s WHERE setting_key LIKE 'feedback.%%'", table,
			)); err != nil {
				return fmt.Errorf("error deleting feedback settings from %s: %w", table, err)
			}
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

// dropFeedbackSuggestionsDown is deliberately a no-op.
//
// A down migration must not pretend it can undo this. Recreating the empty
// schemas would produce tables the application no longer knows about and would
// NOT bring back a single deleted post, vote, comment or feedback entry — the
// content only exists in the pre-migration database backup. Restoring it is an
// operational task (restore the dump), not something a migration can do.
func dropFeedbackSuggestionsDown(_ context.Context, _ *bun.DB) error {
	fmt.Println("Migration 1.15.314 down: no-op — dropped feedback/suggestions data can only be restored from a backup")
	return nil
}
