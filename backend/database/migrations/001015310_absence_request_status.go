package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	absenceRequestStatusVersion     = "1.15.310"
	absenceRequestStatusDescription = "Allow sick reports in the parent absence approval queue (#2447, #2449)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     absenceRequestStatusVersion,
		Description: absenceRequestStatusDescription,
		DependsOn:   []string{"1.15.309"},
	})

	Migrations.MustRegister(absenceRequestStatusUp, absenceRequestStatusDown)
}

func absenceRequestStatusUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.310: Adding absence status to parent approval requests...")

	if _, err := db.ExecContext(ctx, `
		ALTER TABLE active.excused_absence_requests
			ADD COLUMN IF NOT EXISTS absence_status TEXT NOT NULL DEFAULT 'excused';

		ALTER TABLE active.excused_absence_requests
			DROP CONSTRAINT IF EXISTS chk_excused_absence_requests_absence_status;
		ALTER TABLE active.excused_absence_requests
			ADD CONSTRAINT chk_excused_absence_requests_absence_status
			CHECK (absence_status IN ('sick', 'excused'));
	`); err != nil {
		return fmt.Errorf("add parent absence request status: %w", err)
	}

	return nil
}

func absenceRequestStatusDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.310: Removing absence status from parent approval requests...")

	if _, err := db.ExecContext(ctx, `
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = 'active'
				  AND table_name = 'excused_absence_requests'
				  AND column_name = 'absence_status'
			) THEN
				IF EXISTS (
					SELECT 1
					FROM active.excused_absence_requests
					WHERE absence_status = 'sick'
				) THEN
					RAISE EXCEPTION 'cannot remove absence_status while sick parent requests exist';
				END IF;
			END IF;
		END $$;

		ALTER TABLE active.excused_absence_requests
			DROP CONSTRAINT IF EXISTS chk_excused_absence_requests_absence_status;
		ALTER TABLE active.excused_absence_requests
			DROP COLUMN IF EXISTS absence_status;
	`); err != nil {
		return fmt.Errorf("remove parent absence request status: %w", err)
	}

	return nil
}
