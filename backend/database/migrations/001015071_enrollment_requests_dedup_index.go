package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	enrollmentRequestsDedupIndexVersion     = "1.15.71"
	enrollmentRequestsDedupIndexDescription = "Add composite index supporting the duplicate-enrollment check (phase_id, lower(trim(guardian_email)))"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     enrollmentRequestsDedupIndexVersion,
		Description: enrollmentRequestsDedupIndexDescription,
		DependsOn: []string{
			createEnrollmentRequestsVersion,
			createEnrollmentRequestChildrenVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.71: Adding dedup support index on enrollment.requests...")
			_, err := db.NewRaw(`
				CREATE INDEX IF NOT EXISTS idx_enrollment_requests_phase_email_lc
				ON enrollment.requests (phase_id, LOWER(TRIM(guardian_email)));
			`).Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to create idx_enrollment_requests_phase_email_lc: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.NewRaw(`DROP INDEX IF EXISTS enrollment.idx_enrollment_requests_phase_email_lc;`).Exec(ctx)
			return err
		},
	)
}
