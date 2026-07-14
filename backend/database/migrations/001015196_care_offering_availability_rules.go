package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	careOfferingAvailabilityRulesVersion     = "1.15.196"
	careOfferingAvailabilityRulesDescription = "Add extensible availability rules to enrollment care offerings (#1906)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careOfferingAvailabilityRulesVersion,
		Description: careOfferingAvailabilityRulesDescription,
		DependsOn:   []string{categoryShiftTypeVersion},
	})

	Migrations.MustRegister(careOfferingAvailabilityRulesUp, careOfferingAvailabilityRulesDown)
}

func careOfferingAvailabilityRulesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.196: Adding care-offering availability rules...")
	_, err := db.NewRaw(`
		ALTER TABLE enrollment.care_offerings
			ADD COLUMN IF NOT EXISTS availability_rule JSONB;

		COMMENT ON COLUMN enrollment.care_offerings.availability_rule IS
			'Optional typed rule controlling per-child availability; NULL means universally available.';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding care-offering availability rules: %w", err)
	}
	return nil
}

func careOfferingAvailabilityRulesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.196: Removing care-offering availability rules...")
	_, err := db.NewRaw(`
		ALTER TABLE enrollment.care_offerings
			DROP COLUMN IF EXISTS availability_rule;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed removing care-offering availability rules: %w", err)
	}
	return nil
}
