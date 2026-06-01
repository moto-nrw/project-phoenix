package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	careOfferingsIsRequiredVersion     = "1.15.96"
	careOfferingsIsRequiredDescription = "Add is_required flag to enrollment.care_offerings so a school can mark a specific care offering as mandatory for every child in the phase."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careOfferingsIsRequiredVersion,
		Description: careOfferingsIsRequiredDescription,
		DependsOn: []string{
			createCareOfferingsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.96: Adding is_required to enrollment.care_offerings...")
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.care_offerings
				ADD COLUMN IF NOT EXISTS is_required BOOLEAN NOT NULL DEFAULT FALSE;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding is_required column: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.96...")
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.care_offerings
				DROP COLUMN IF EXISTS is_required;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping is_required column: %w", err)
			}
			return nil
		},
	)
}
