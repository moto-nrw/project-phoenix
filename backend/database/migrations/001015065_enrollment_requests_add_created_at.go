package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	enrollmentRequestsAddCreatedAtVersion     = "1.15.65"
	enrollmentRequestsAddCreatedAtDescription = "Add created_at to enrollment.requests + created_at/updated_at to enrollment.request_children to match base.Model embedding (parent-enrollment PR 7)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     enrollmentRequestsAddCreatedAtVersion,
		Description: enrollmentRequestsAddCreatedAtDescription,
		DependsOn: []string{
			createEnrollmentRequestsVersion,
			createEnrollmentRequestChildrenVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.requests
				ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding created_at to enrollment.requests: %w", err)
			}
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.request_children
				ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding timestamps to enrollment.request_children: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.request_children
				DROP COLUMN IF EXISTS created_at,
				DROP COLUMN IF EXISTS updated_at;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping timestamps from enrollment.request_children: %w", err)
			}
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.requests
				DROP COLUMN IF EXISTS created_at;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping created_at from enrollment.requests: %w", err)
			}
			return nil
		},
	)
}
