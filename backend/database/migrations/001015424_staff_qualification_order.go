package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const staffQualificationOrderVersion = "1.15.424"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffQualificationOrderVersion,
		Description: "Preserve staff qualification list order (#3399)",
		DependsOn:   []string{staffQualificationsSoftDeleteVersion},
	})
	Migrations.MustRegister(staffQualificationOrderUp, staffQualificationOrderDown)
}

func staffQualificationOrderUp(ctx context.Context, db *bun.DB) error {
	// Existing rows retain their ID order through the read query's tie-breaker.
	// Avoid updating historical rows solely to backfill positions.
	_, err := db.ExecContext(ctx, `
		ALTER TABLE users.staff_qualifications
			ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;
	`)
	if err != nil {
		return fmt.Errorf("add staff qualification order: %w", err)
	}
	return nil
}

func staffQualificationOrderDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `ALTER TABLE users.staff_qualifications DROP COLUMN sort_order;`)
	if err != nil {
		return fmt.Errorf("remove staff qualification order: %w", err)
	}
	return nil
}
