package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	preserveStaffDeletionAuditVersion     = "1.15.450"
	preserveStaffDeletionAuditDescription = "Preserve data-deletion audit records after permanent staff deletion (#3953)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     preserveStaffDeletionAuditVersion,
		Description: preserveStaffDeletionAuditDescription,
		DependsOn:   []string{deliveryRetentionVersion},
	})

	Migrations.MustRegister(preserveStaffDeletionAuditUp, preserveStaffDeletionAuditDown)
}

// The staff counterpart of 1.15.244: a row in audit.data_deletions is the
// proof that a deletion happened, so it must outlive the staff member it
// names. staff_id stays as a plain identifier, exactly like student_id.
func preserveStaffDeletionAuditUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE audit.data_deletions
			DROP CONSTRAINT IF EXISTS fk_data_deletions_staff;
	`); err != nil {
		return fmt.Errorf("preserve staff deletion audit records: %w", err)
	}
	return nil
}

func preserveStaffDeletionAuditDown(ctx context.Context, db *bun.DB) error {
	var orphanedRows int
	if err := db.NewRaw(`
		SELECT COUNT(*)
		FROM audit.data_deletions AS deletion
		LEFT JOIN users.staff_school_memberships AS membership ON membership.id = deletion.staff_id
		WHERE deletion.staff_id IS NOT NULL AND membership.id IS NULL
	`).Scan(ctx, &orphanedRows); err != nil {
		return fmt.Errorf("check preserved staff deletion audit records: %w", err)
	}
	if orphanedRows > 0 {
		return fmt.Errorf("cannot restore data_deletions staff foreign key while %d preserved deletion audit records exist", orphanedRows)
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE audit.data_deletions
			ADD CONSTRAINT fk_data_deletions_staff
			FOREIGN KEY (staff_id) REFERENCES users.staff_school_memberships(id) ON DELETE CASCADE;
	`); err != nil {
		return fmt.Errorf("restore data_deletions staff foreign key: %w", err)
	}
	return nil
}
