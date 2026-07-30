package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	requestChildOfferingValidityVersion     = "1.15.243"
	requestChildOfferingValidityDescription = "Add effective intervals to child care-offering selections."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     requestChildOfferingValidityVersion,
		Description: requestChildOfferingValidityDescription,
		DependsOn:   []string{offeringChangeRequestsVersion},
	})

	Migrations.MustRegister(requestChildOfferingValidityUp, requestChildOfferingValidityDown)
}

// A future-approved offering change must not replace the selection currently
// in force. The interval is deliberately nullable at either end so existing
// enrollment selections retain their open-ended semantics.
func requestChildOfferingValidityUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.243: Adding validity intervals to request child offerings...")
	if _, err := db.NewRaw(`
		CREATE EXTENSION IF NOT EXISTS btree_gist;

		ALTER TABLE enrollment.request_child_offerings
			ADD COLUMN IF NOT EXISTS valid_from DATE,
			ADD COLUMN IF NOT EXISTS valid_until DATE,
			DROP CONSTRAINT IF EXISTS uq_request_child_offerings_pair;

		ALTER TABLE enrollment.request_child_offerings
			ADD CONSTRAINT request_child_offerings_non_overlapping_validity
			EXCLUDE USING gist (
				request_child_id WITH =,
				care_offering_id WITH =,
				daterange(
					COALESCE(valid_from, '-infinity'::date),
					COALESCE(valid_until, 'infinity'::date),
					'[)'
				) WITH &&
			);

		CREATE INDEX IF NOT EXISTS idx_request_child_offerings_active_validity
			ON enrollment.request_child_offerings (care_offering_id, valid_from, valid_until);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding request child offering validity: %w", err)
	}
	return nil
}

func requestChildOfferingValidityDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.243: Removing validity intervals from request child offerings...")
	if _, err := db.NewRaw(`
		ALTER TABLE enrollment.request_child_offerings
			DROP CONSTRAINT IF EXISTS request_child_offerings_non_overlapping_validity,
			DROP COLUMN IF EXISTS valid_until,
			DROP COLUMN IF EXISTS valid_from,
			ADD CONSTRAINT uq_request_child_offerings_pair UNIQUE (request_child_id, care_offering_id);
		DROP INDEX IF EXISTS enrollment.idx_request_child_offerings_active_validity;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed removing request child offering validity: %w", err)
	}
	return nil
}
