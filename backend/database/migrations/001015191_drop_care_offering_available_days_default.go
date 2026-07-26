package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	dropCareOfferingAvailableDaysDefaultVersion     = "1.15.191"
	dropCareOfferingAvailableDaysDefaultDescription = "Drop the Mo-Fr column default on enrollment.care_offerings.available_days"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dropCareOfferingAvailableDaysDefaultVersion,
		Description: dropCareOfferingAvailableDaysDefaultDescription,
		DependsOn:   []string{staffShiftSeriesVersion},
	})
	Migrations.MustRegister(dropCareOfferingAvailableDaysDefaultUp, dropCareOfferingAvailableDaysDefaultDown)
}

// The application always writes available_days explicitly, so this DEFAULT
// never fires from app code — but a hand-written INSERT omitting the column
// would silently create a Mo-Fr offering, the exact misconfiguration behind
// #1885. Without a default, such an INSERT fails loudly (column is NOT NULL).
func dropCareOfferingAvailableDaysDefaultUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.191: Dropping Mo-Fr default on care_offerings.available_days...")
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE enrollment.care_offerings
			ALTER COLUMN available_days DROP DEFAULT;
	`); err != nil {
		return fmt.Errorf("failed dropping available_days default: %w", err)
	}
	fmt.Println("Migration 1.15.191: Completed successfully")
	return nil
}

func dropCareOfferingAvailableDaysDefaultDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE enrollment.care_offerings
			ALTER COLUMN available_days SET DEFAULT '["mon","tue","wed","thu","fri"]'::jsonb;
	`); err != nil {
		return fmt.Errorf("failed restoring available_days default: %w", err)
	}
	return nil
}
