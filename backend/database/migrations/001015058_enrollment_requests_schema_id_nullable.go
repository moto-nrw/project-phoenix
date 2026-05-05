package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	enrollmentRequestsSchemaIDNullableVersion     = "1.15.58"
	enrollmentRequestsSchemaIDNullableDescription = "Make enrollment.requests.schema_id nullable so phases marked 'Basis' (no custom fields) can submit without requiring a tenant-active schema row"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     enrollmentRequestsSchemaIDNullableVersion,
		Description: enrollmentRequestsSchemaIDNullableDescription,
		DependsOn: []string{
			createEnrollmentRequestsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.requests
				ALTER COLUMN schema_id DROP NOT NULL;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping NOT NULL on schema_id: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			// Best-effort rollback: refuse if any rows would lose data.
			// Schema column was nullable in this revision so existing
			// nulls (if any) would block the SET NOT NULL — admin must
			// repoint them before rolling back.
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.requests
				ALTER COLUMN schema_id SET NOT NULL;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed restoring NOT NULL: %w", err)
			}
			return nil
		},
	)
}
