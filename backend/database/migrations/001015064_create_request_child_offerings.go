package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	createRequestChildOfferingsVersion     = "1.15.64"
	createRequestChildOfferingsDescription = "Create enrollment.request_child_offerings join table - per-child × care_offering selections (PR 6, populated in PR 7)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     createRequestChildOfferingsVersion,
		Description: createRequestChildOfferingsDescription,
		DependsOn: []string{
			createCareOfferingsVersion,
			createEnrollmentRequestChildrenVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createRequestChildOfferingsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return createRequestChildOfferingsDown(ctx, db)
		},
	)
}

func createRequestChildOfferingsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.64: Creating enrollment.request_child_offerings table...")

	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS enrollment.request_child_offerings (
			id                BIGSERIAL PRIMARY KEY,
			tenant_id         BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			request_child_id  BIGINT NOT NULL REFERENCES enrollment.request_children(id) ON DELETE CASCADE,
			care_offering_id  BIGINT NOT NULL REFERENCES enrollment.care_offerings(id) ON DELETE RESTRICT,
			selected_days     JSONB,
			notes             TEXT,
			created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_request_child_offerings_pair
				UNIQUE (request_child_id, care_offering_id)
		);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating enrollment.request_child_offerings: %w", err)
	}

	if _, err := db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_request_child_offerings_offering
			ON enrollment.request_child_offerings (care_offering_id);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating index on care_offering_id: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE enrollment.request_child_offerings ENABLE ROW LEVEL SECURITY;
		ALTER TABLE enrollment.request_child_offerings FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_enrollment_request_child_offerings
			ON enrollment.request_child_offerings
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed enabling RLS on request_child_offerings: %w", err)
	}

	return nil
}

func createRequestChildOfferingsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.64: Dropping enrollment.request_child_offerings...")
	if _, err := db.NewRaw(`DROP TABLE IF EXISTS enrollment.request_child_offerings CASCADE;`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping enrollment.request_child_offerings: %w", err)
	}
	return nil
}
