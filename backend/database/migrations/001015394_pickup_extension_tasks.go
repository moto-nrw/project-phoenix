package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	pickupExtensionTasksVersion     = "1.15.394"
	pickupExtensionTasksDescription = "Open tasks for later pickup times that still need a Betreuungsblock (#3261)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     pickupExtensionTasksVersion,
		Description: pickupExtensionTasksDescription,
		// 1.15.299 is the latest change to schedule.student_pickup_exceptions,
		// which the day tasks reference.
		DependsOn: []string{compositePKIndexesVersion, roomsRetireAtSchoolColorVersion, absenceAllowanceCarryoverVersion, autoPartialAbsenceVersion},
	})

	Migrations.MustRegister(pickupExtensionTasksUp, pickupExtensionTasksDown)
}

// pickupExtensionTasksUp creates schedule.pickup_extension_tasks (#3261).
// A row records that a child now stays longer than before and may not be on
// any Betreuungsblock for the extra time. It is the mirror of the automatic
// partial absence for earlier pickups (#2360), but it never changes a roster
// by itself: the Leitung picks the block, or none.
//
// Two shapes share the table:
//   - day task: one date, tied to the day pickup exception. Deleting the
//     exception deletes the task.
//   - weekday task: a lasting weekday change, valid from effective_from on.
//
// Whether a task is still open is decided when it is read, against the
// current blocks, so no row has to be rewritten when a plan changes.
func pickupExtensionTasksUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			logRollbackFailure(ctx, rollbackErr)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schedule.pickup_extension_tasks (
			id                   BIGSERIAL PRIMARY KEY,
			tenant_id            BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id           BIGINT NOT NULL,
			pickup_exception_id  BIGINT,
			task_date            DATE,
			weekday              SMALLINT,
			effective_from       DATE,
			previous_pickup_time TIME NOT NULL,
			pickup_time          TIME NOT NULL,
			created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_pickup_extension_task_shape CHECK (
				(pickup_exception_id IS NOT NULL AND task_date IS NOT NULL
					AND weekday IS NULL AND effective_from IS NULL)
				OR (pickup_exception_id IS NULL AND task_date IS NULL
					AND weekday BETWEEN 1 AND 5 AND effective_from IS NOT NULL)
			),
			CONSTRAINT fk_pickup_extension_task_student
				FOREIGN KEY (tenant_id, student_id)
				REFERENCES users.students(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT fk_pickup_extension_task_exception
				FOREIGN KEY (tenant_id, pickup_exception_id)
				REFERENCES schedule.student_pickup_exceptions(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT chk_pickup_extension_task_later CHECK (pickup_time > previous_pickup_time)
		);

		CREATE UNIQUE INDEX IF NOT EXISTS uniq_pickup_extension_tasks_day
			ON schedule.pickup_extension_tasks (tenant_id, student_id, task_date)
			WHERE task_date IS NOT NULL;
		CREATE UNIQUE INDEX IF NOT EXISTS uniq_pickup_extension_tasks_weekday
			ON schedule.pickup_extension_tasks (tenant_id, student_id, weekday)
			WHERE weekday IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_pickup_extension_tasks_exception
			ON schedule.pickup_extension_tasks (pickup_exception_id)
			WHERE pickup_exception_id IS NOT NULL;

		DROP TRIGGER IF EXISTS update_pickup_extension_tasks_updated_at ON schedule.pickup_extension_tasks;
		CREATE TRIGGER update_pickup_extension_tasks_updated_at
		BEFORE UPDATE ON schedule.pickup_extension_tasks
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE schedule.pickup_extension_tasks ENABLE ROW LEVEL SECURITY;
		ALTER TABLE schedule.pickup_extension_tasks FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_schedule_pickup_extension_tasks ON schedule.pickup_extension_tasks;
		CREATE POLICY tenant_isolation_schedule_pickup_extension_tasks ON schedule.pickup_extension_tasks
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON schedule.pickup_extension_tasks TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE schedule.pickup_extension_tasks_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating schedule.pickup_extension_tasks: %w", err)
	}

	return tx.Commit()
}

func pickupExtensionTasksDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS schedule.pickup_extension_tasks;`)
	if err != nil {
		return fmt.Errorf("error dropping schedule.pickup_extension_tasks: %w", err)
	}
	return nil
}
