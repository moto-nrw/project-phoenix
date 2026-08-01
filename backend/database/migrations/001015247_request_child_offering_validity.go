package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	requestChildOfferingValidityVersion     = "1.15.247"
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
	fmt.Println("Migration 1.15.247: Adding validity intervals to request child offerings...")
	if _, err := db.NewRaw(`
		CREATE EXTENSION IF NOT EXISTS btree_gist;

		ALTER TABLE enrollment.request_child_offerings
			ADD COLUMN IF NOT EXISTS valid_from DATE,
			ADD COLUMN IF NOT EXISTS valid_until DATE,
			DROP CONSTRAINT IF EXISTS uq_request_child_offerings_pair;

		-- Existing selections applied to the complete care period of their
		-- enrollment. Keep that boundary when converting them to intervals so
		-- completed periods do not consume capacity in later ones.
		UPDATE enrollment.request_child_offerings AS "request_child_offering"
		SET
			valid_from = "phase".service_start_date,
			valid_until = "phase".service_end_date + 1
		FROM enrollment.request_children AS "request_child"
		INNER JOIN enrollment.requests AS "request"
			ON "request".id = "request_child".request_id
			AND "request".tenant_id = "request_child".tenant_id
		INNER JOIN enrollment.phases AS "phase"
			ON "phase".id = "request".phase_id
			AND "phase".tenant_id = "request".tenant_id
		WHERE "request_child_offering".request_child_id = "request_child".id
			AND "request_child_offering".tenant_id = "request_child".tenant_id
			AND "request_child_offering".valid_from IS NULL
			AND "request_child_offering".valid_until IS NULL;

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
	fmt.Println("Rolling back migration 1.15.247: Removing validity intervals from request child offerings...")
	if _, err := db.NewRaw(`
		-- Expired-only intervals must stay absent in the legacy model: retaining
		-- their last row would resurrect a removed booking on rollback.
		DELETE FROM enrollment.request_child_offerings
		WHERE valid_until IS NOT NULL AND valid_until <= CURRENT_DATE;

		-- A pre-1.15.247 schema has one row per child/offering pair. Keep the
		-- interval active today; when none is active, keep the latest scheduled
		-- interval before restoring its UNIQUE key.
		WITH ranked AS (
			SELECT id, ROW_NUMBER() OVER (
				PARTITION BY request_child_id, care_offering_id
				ORDER BY
					CASE WHEN (valid_from IS NULL OR valid_from <= CURRENT_DATE)
						AND (valid_until IS NULL OR valid_until > CURRENT_DATE)
						THEN 0 ELSE 1 END,
					valid_from DESC NULLS LAST,
					id DESC
			) AS row_number
			FROM enrollment.request_child_offerings
		)
		DELETE FROM enrollment.request_child_offerings AS offering
		USING ranked
		WHERE offering.id = ranked.id
			AND ranked.row_number > 1;

		DROP INDEX IF EXISTS enrollment.idx_request_child_offerings_active_validity;

		ALTER TABLE enrollment.request_child_offerings
			DROP CONSTRAINT IF EXISTS request_child_offerings_non_overlapping_validity,
			DROP COLUMN IF EXISTS valid_until,
			DROP COLUMN IF EXISTS valid_from,
			ADD CONSTRAINT uq_request_child_offerings_pair UNIQUE (request_child_id, care_offering_id);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed removing request child offering validity: %w", err)
	}
	return nil
}
