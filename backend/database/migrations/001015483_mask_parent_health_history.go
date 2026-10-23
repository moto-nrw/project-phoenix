package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const maskParentHealthHistoryVersion = "1.15.483"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     maskParentHealthHistoryVersion,
		Description: "Remove health free text from parent master-data request history (#4844)",
		DependsOn:   []string{repairStudentDataChangeRequestsVersion},
	})
	Migrations.MustRegister(maskParentHealthHistoryUp, maskParentHealthHistoryDown)
}

// Privileged one-time redaction across schools, without changing RLS or rights.
// A single statement preserves metadata and fails atomically. It is idempotent.
func maskParentHealthHistoryUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
UPDATE users.student_data_change_requests
SET old_value = CASE
      WHEN COALESCE(old_value, 'null'::jsonb) IN ('null'::jsonb, '""'::jsonb)
        THEN '""'::jsonb ELSE '"Angabe vorhanden"'::jsonb END,
    new_value = CASE
      WHEN new_value IN ('null'::jsonb, '""'::jsonb) THEN '""'::jsonb
      WHEN COALESCE(old_value, 'null'::jsonb) IN ('null'::jsonb, '""'::jsonb)
        THEN '"Angabe hinzugefügt"'::jsonb
      ELSE '"Angabe geändert"'::jsonb END
WHERE target = 'student' AND field_key = 'health_info';
 `)
	if err != nil {
		return fmt.Errorf("mask parent health history values: %w", err)
	}
	return nil
}

func maskParentHealthHistoryDown(_ context.Context, _ *bun.DB) error {
	return errors.New("parent health history redaction is irreversible; roll forward")
}
