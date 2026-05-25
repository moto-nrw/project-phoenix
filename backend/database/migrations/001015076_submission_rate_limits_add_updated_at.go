package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	submissionRateLimitsAddUpdatedAtVersion     = "1.15.76"
	submissionRateLimitsAddUpdatedAtDescription = "Add updated_at column to enrollment.submission_rate_limits for existing databases created before the rate-limit table included base.Model timestamps"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     submissionRateLimitsAddUpdatedAtVersion,
		Description: submissionRateLimitsAddUpdatedAtDescription,
		DependsOn: []string{
			createEnrollmentRateLimitsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.76: Adding updated_at to enrollment.submission_rate_limits...")
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.submission_rate_limits
				ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding updated_at to enrollment.submission_rate_limits: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.76...")
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.submission_rate_limits
				DROP COLUMN IF EXISTS updated_at;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping updated_at from enrollment.submission_rate_limits: %w", err)
			}
			return nil
		},
	)
}
