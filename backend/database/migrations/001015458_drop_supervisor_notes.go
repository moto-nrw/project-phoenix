package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const dropSupervisorNotesVersion = "1.15.458"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dropSupervisorNotesVersion,
		Description: "Kindnotizen: frühere Betreuernotizen (users.student_care_profiles.supervisor_notes) entfernen (#4268)",
		DependsOn: []string{
			studentNotesVersion,       // users.student_notes holds the carried-over hints
			demoVisitorErasureVersion, // the last migration before this one
		},
	})
	Migrations.MustRegister(dropSupervisorNotesUp, dropSupervisorNotesDown)
}

// dropSupervisorNotesUp retires the free-text field „Betreuernotizen". Its
// text has lived on as the child's permanent hint since 1.15.438, and every
// write to the field was mirrored there until this release.
//
// The 1.15.438 carry-over runs once more first. It is idempotent and never
// resurrects a hint a school deliberately deleted, so the drop cannot lose a
// text that, for whatever reason, never reached the note card.
func dropSupervisorNotesUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		var columnExists bool
		if err := tx.NewRaw(`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'users' AND table_name = 'student_care_profiles'
			  AND column_name = 'supervisor_notes')`).Scan(ctx, &columnExists); err != nil {
			return fmt.Errorf("inspect users.student_care_profiles.supervisor_notes: %w", err)
		}
		if !columnExists {
			return nil
		}
		if err := studentNotesBackfill(ctx, tx); err != nil {
			return err
		}
		// RESTRICT: an unknown view or function over the column must abort
		// the release, never be dropped along with it.
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE users.student_care_profiles DROP COLUMN IF EXISTS supervisor_notes RESTRICT;
			COMMENT ON COLUMN users.student_notes.origin IS 'staff = von einer Person geschrieben; master_data = aus den früheren Betreuernotizen oder einem Import übernommen, daher ohne Autor.';
		`); err != nil {
			return fmt.Errorf("drop users.student_care_profiles.supervisor_notes: %w", err)
		}
		return nil
	})
}

// dropSupervisorNotesDown restores the column for the previous image and fills
// it from the live carried-over hint of each child's newest membership, which
// is the value that image would have read.
func dropSupervisorNotesDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE users.student_care_profiles ADD COLUMN IF NOT EXISTS supervisor_notes TEXT;
			COMMENT ON COLUMN users.student_notes.origin IS 'staff = von einer Person geschrieben; master_data = beim Umstieg aus users.students.supervisor_notes übernommen, daher ohne Autor.';
			WITH newest_memberships AS (
				SELECT DISTINCT ON (membership.tenant_id, membership.student_profile_id)
					membership.tenant_id, membership.student_profile_id AS student_id, membership.id AS membership_id
				FROM users.student_school_memberships AS membership
				WHERE membership.deleted_at IS NULL
				ORDER BY membership.tenant_id, membership.student_profile_id, membership.id DESC
			)
			UPDATE users.student_care_profiles AS care
			SET supervisor_notes = note.body
			FROM newest_memberships AS membership
			JOIN users.student_notes AS note
				ON note.tenant_id = membership.tenant_id AND note.student_id = membership.student_id
				AND note.origin = 'master_data' AND note.deleted_at IS NULL
			WHERE care.tenant_id = membership.tenant_id AND care.membership_id = membership.membership_id;
		`); err != nil {
			return fmt.Errorf("restore users.student_care_profiles.supervisor_notes: %w", err)
		}
		return nil
	})
}
