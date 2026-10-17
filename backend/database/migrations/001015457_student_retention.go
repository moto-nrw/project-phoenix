package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const studentRetentionVersion = "1.15.457"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentRetentionVersion,
		Description: "Let the nightly run record the automatic deletion of a child without an actor (#3962)",
		DependsOn: []string{
			studentCareExitsVersion,   // the retention_expired reason of audit.student_deletions
			demoVisitorErasureVersion, // the last migration before this one
		},
	})
	Migrations.MustRegister(studentRetentionUp, studentRetentionDown)
}

// studentRetentionUpSQL lets a student tombstone name no actor (#3962). The
// nightly run deletes a child once the school's retention window after the
// last care day has passed; nobody confirmed that deletion, so the tombstone
// records no account. Only the reason retention_expired may go without one:
// every deletion a person confirms keeps its actor.
const studentRetentionUpSQL = `
		ALTER TABLE audit.student_deletions
			ALTER COLUMN actor_account_id DROP NOT NULL;
		ALTER TABLE audit.student_deletions
			DROP CONSTRAINT IF EXISTS chk_student_deletions_actor;
		ALTER TABLE audit.student_deletions
			ADD CONSTRAINT chk_student_deletions_actor
			CHECK (actor_account_id IS NOT NULL OR reason = 'retention_expired');
`

const studentRetentionDownSQL = `
		ALTER TABLE audit.student_deletions
			DROP CONSTRAINT IF EXISTS chk_student_deletions_actor;
		ALTER TABLE audit.student_deletions
			ALTER COLUMN actor_account_id SET NOT NULL;
`

func studentRetentionUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, studentRetentionUpSQL); err != nil {
		return fmt.Errorf("allow automatic student deletions: %w", err)
	}
	return nil
}

// studentRetentionDown refuses while automatic deletions are recorded: their
// tombstones have no actor to restore.
func studentRetentionDown(ctx context.Context, db *bun.DB) error {
	var automatic int
	if err := db.NewRaw(`SELECT COUNT(*) FROM audit.student_deletions WHERE actor_account_id IS NULL`).Scan(ctx, &automatic); err != nil {
		return fmt.Errorf("check automatic student deletions: %w", err)
	}
	if automatic > 0 {
		return fmt.Errorf("cannot require an actor on audit.student_deletions while %d automatic deletions exist", automatic)
	}
	if _, err := db.ExecContext(ctx, studentRetentionDownSQL); err != nil {
		return fmt.Errorf("require an actor on student deletions: %w", err)
	}
	return nil
}
