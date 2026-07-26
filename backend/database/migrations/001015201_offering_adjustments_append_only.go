package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	offeringAdjustmentsAppendOnlyVersion     = "1.15.201"
	offeringAdjustmentsAppendOnlyDescription = "Revoke tenant UPDATE/DELETE on enrollment offering adjustments audit to keep it append-only."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     offeringAdjustmentsAppendOnlyVersion,
		Description: offeringAdjustmentsAppendOnlyDescription,
		DependsOn:   []string{enrollmentOfferingAdjustmentsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.201: Making audit.enrollment_offering_adjustments append-only for phoenix_tenant...")
			// The schema-wide default ACL from 1.14.1 auto-granted phoenix_tenant
			// UPDATE/DELETE on this table when 1.15.140 created it; the explicit
			// GRANT SELECT, INSERT there was additive, not restrictive. Row removal
			// happens exclusively through the ON DELETE CASCADE FKs when the parent
			// enrollment request / request child is deleted.
			if _, err := db.NewRaw(`
				REVOKE UPDATE, DELETE ON audit.enrollment_offering_adjustments FROM phoenix_tenant;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed revoking tenant UPDATE/DELETE on audit.enrollment_offering_adjustments: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.201: Re-granting tenant UPDATE/DELETE on audit.enrollment_offering_adjustments...")
			if _, err := db.NewRaw(`
				GRANT UPDATE, DELETE ON audit.enrollment_offering_adjustments TO phoenix_tenant;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed re-granting tenant UPDATE/DELETE on audit.enrollment_offering_adjustments: %w", err)
			}
			return nil
		},
	)
}
