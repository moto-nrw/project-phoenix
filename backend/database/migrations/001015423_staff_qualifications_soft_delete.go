package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffQualificationsSoftDeleteVersion     = "1.15.423"
	staffQualificationsSoftDeleteDescription = "Retire removed staff qualifications instead of deleting them (#3399)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffQualificationsSoftDeleteVersion,
		Description: staffQualificationsSoftDeleteDescription,
		DependsOn:   []string{parentMessageStaffMarkedUnreadVersion},
	})
	Migrations.MustRegister(staffQualificationsSoftDeleteUp, staffQualificationsSoftDeleteDown)
}

// staffQualificationsSoftDeleteUp gives users.staff_qualifications a
// deleted_at column (ADR 0021, decision 4). Removing a qualification retires
// the row so it stays as evidence of an earlier employment; the lists read
// only live rows. The tenant role loses DELETE like users.staff_documents, so
// no caller can go back to hard deletes. The table already carries tenant RLS,
// so the new column inherits it.
func staffQualificationsSoftDeleteUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE users.staff_qualifications
			ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

		REVOKE DELETE ON users.staff_qualifications FROM phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("add staff qualification soft delete: %w", err)
	}
	return nil
}

func staffQualificationsSoftDeleteDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		DELETE FROM users.staff_qualifications WHERE deleted_at IS NOT NULL;

		GRANT DELETE ON users.staff_qualifications TO phoenix_tenant;

		ALTER TABLE users.staff_qualifications
			DROP COLUMN IF EXISTS deleted_at;
	`)
	if err != nil {
		return fmt.Errorf("remove staff qualification soft delete: %w", err)
	}
	return nil
}
