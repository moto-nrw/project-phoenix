package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const shiftTypesArchiveVersion = "20261113130705"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     shiftTypesArchiveVersion,
		Description: "Dienstarten mit geplanten Diensten archivieren statt löschen (#6845)",
		DependsOn: []string{
			shiftTypesVersion, // creates schedule.shift_types and its name index (1.15.170)
		},
	})
	Migrations.MustRegister(shiftTypesArchiveUp, shiftTypesArchiveDown)
}

// shiftTypesArchiveUpSQL lets the school remove a shift type that planned
// shifts or series still carry without erasing their classification (#6845):
// the row stays as history and leaves the choice. The case-insensitive name
// stays unique among the offered types only, so a new type may reuse the name
// of an archived one. The index keeps its name, which the Workforce store
// classifies as a name conflict.
const shiftTypesArchiveUpSQL = `
ALTER TABLE schedule.shift_types ADD COLUMN archived_at TIMESTAMPTZ;
COMMENT ON COLUMN schedule.shift_types.archived_at IS
	'When the school removed the type while planned shifts or series still carried it (#6845); NULL while it is offered.';
DROP INDEX schedule.uniq_shift_types_tenant_name;
CREATE UNIQUE INDEX uniq_shift_types_tenant_name
	ON schedule.shift_types (tenant_id, LOWER(name))
	WHERE archived_at IS NULL;
`

// shiftTypesArchiveDownSQL restores the unconditional name index. It fails
// while an archived type shares its name with another type of the school;
// rename one of them first.
const shiftTypesArchiveDownSQL = `
DROP INDEX schedule.uniq_shift_types_tenant_name;
CREATE UNIQUE INDEX uniq_shift_types_tenant_name
	ON schedule.shift_types (tenant_id, LOWER(name));
ALTER TABLE schedule.shift_types DROP COLUMN archived_at;
`

func shiftTypesArchiveUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, shiftTypesArchiveUpSQL); err != nil {
		return fmt.Errorf("add the archive marker of shift types: %w", err)
	}
	return nil
}

func shiftTypesArchiveDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, shiftTypesArchiveDownSQL); err != nil {
		return fmt.Errorf("drop the archive marker of shift types: %w", err)
	}
	return nil
}
