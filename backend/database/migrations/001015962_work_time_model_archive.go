package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const workTimeModelArchiveVersion = "1.15.962"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     workTimeModelArchiveVersion,
		Description: "Arbeitszeitmodell-Vorlagen archivieren (#6891)",
		DependsOn: []string{
			accountTraegerParentAccessFactVersion, // the newest migration on development (1.15.961)
			workTimeModelsVersion,                 // creates config.work_time_models (1.15.102)
		},
	})
	Migrations.MustRegister(workTimeModelArchiveUp, workTimeModelArchiveDown)
}

// workTimeModelArchiveUpSQL lets a school take a template out of the choice
// for new assignments (#6891). Staff already bound to it keep the binding and
// their Soll; only a new assignment is refused. NULL means offered.
const workTimeModelArchiveUpSQL = `
ALTER TABLE config.work_time_models ADD COLUMN archived_at TIMESTAMPTZ;
COMMENT ON COLUMN config.work_time_models.archived_at IS
	'When the school stopped offering the template for new assignments (#6891); NULL while it is offered.';
`

const workTimeModelArchiveDownSQL = `ALTER TABLE config.work_time_models DROP COLUMN archived_at;`

func workTimeModelArchiveUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, workTimeModelArchiveUpSQL); err != nil {
		return fmt.Errorf("add the archive marker of work time models: %w", err)
	}
	return nil
}

func workTimeModelArchiveDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, workTimeModelArchiveDownSQL); err != nil {
		return fmt.Errorf("drop the archive marker of work time models: %w", err)
	}
	return nil
}
