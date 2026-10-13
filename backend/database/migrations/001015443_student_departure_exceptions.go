package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentDepartureExceptionsVersion     = "1.15.443"
	studentDepartureExceptionsDescription = "Abholart für einen einzelnen Tag: eigene Ausnahme neben der Gehzeit-Ausnahme"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentDepartureExceptionsVersion,
		Description: studentDepartureExceptionsDescription,
		DependsOn:   []string{requestChildClassSwitchVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentDepartureExceptionsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentDepartureExceptionsDown(ctx, db)
		},
	)
}

// studentDepartureExceptionsUp creates schedule.student_departure_exceptions:
// the Abholart of one child on one day, overriding the weekly plan (#3945).
//
// A table of its own and not a column on schedule.student_pickup_exceptions.
// The Abholart is independent of the Gehzeit (CONTEXT.md "Abholart"), and a
// pickup exception without pickup_time already means "the child does not come"
// to every reader of that table (IsAbsent, the auto partial absence of #2360,
// the slot lists). A mode-only change stored there would either be read as an
// absence or would have to freeze today's Gehzeit into the exception, which
// then silently outlives a later change of the weekly plan.
//
// One mode per day, unlike the weekly plan's set of allowed modes: a day
// change says how the child goes home on that day. The heimweg restriction of
// the enrollment form (#2381) limits the weekly set to one mode, so a single
// value can never violate it.
//
// created_by is the staff membership that set or approved the value; a parent
// request becomes a row only through the OGS's decision, exactly like the
// pickup-time exception it travels with.
func studentDepartureExceptionsUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	_, err = tx.NewRaw(`
		CREATE TABLE IF NOT EXISTS schedule.student_departure_exceptions (
			id             BIGSERIAL PRIMARY KEY,
			tenant_id      BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id     BIGINT NOT NULL,
			exception_date DATE NOT NULL,
			departure_mode VARCHAR(20) NOT NULL,
			reason         VARCHAR(255),
			created_by     BIGINT,
			created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_student_departure_exceptions_mode CHECK (
				departure_mode IN ('alone', 'bus', 'pickup', 'accompanied')
			),
			CONSTRAINT uq_student_departure_exceptions_day UNIQUE (tenant_id, student_id, exception_date),
			CONSTRAINT uq_student_departure_exceptions_tenant_id_id UNIQUE (tenant_id, id),
			CONSTRAINT fk_student_departure_exceptions_student FOREIGN KEY (tenant_id, student_id)
				REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE,
			-- The author may leave the school; the day value stays, as the
			-- decision itself was the school's.
			CONSTRAINT fk_student_departure_exceptions_created_by FOREIGN KEY (tenant_id, created_by)
				REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE SET NULL (created_by)
		);

		COMMENT ON TABLE schedule.student_departure_exceptions IS 'Abholart eines Kindes an einem einzelnen Tag; überschreibt die Abholart aus dem Wochenplan nur an diesem Tag.';

		-- Every day view (Kinderkarte, class day, supervision sheet) reads one
		-- date for many children.
		CREATE INDEX IF NOT EXISTS idx_student_departure_exceptions_date
			ON schedule.student_departure_exceptions (tenant_id, exception_date);

		DROP TRIGGER IF EXISTS update_student_departure_exceptions_updated_at ON schedule.student_departure_exceptions;
		CREATE TRIGGER update_student_departure_exceptions_updated_at
		BEFORE UPDATE ON schedule.student_departure_exceptions
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		GRANT SELECT, INSERT, UPDATE, DELETE ON schedule.student_departure_exceptions TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE schedule.student_departure_exceptions_id_seq TO phoenix_tenant;

		-- A care exit removes future day values like future pickup exceptions
		-- and restores them when the exit is withdrawn.
		ALTER TABLE users.student_care_exit_source_removals
			DROP CONSTRAINT IF EXISTS student_care_exit_source_removals_kind_check;
		ALTER TABLE users.student_care_exit_source_removals
			ADD CONSTRAINT student_care_exit_source_removals_kind_check CHECK (kind IN (
				'source_booking', 'pickup_schedule', 'pickup_exception',
				'arrival_schedule', 'arrival_exception', 'departure_exception'
			));
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("error creating schedule.student_departure_exceptions: %w", err)
	}

	if err := provisionTenantRLS(ctx, tx, "schedule.student_departure_exceptions"); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit student departure exceptions migration: %w", err)
	}
	return nil
}

func studentDepartureExceptionsDown(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	if _, err := tx.NewRaw(`
		DELETE FROM users.student_care_exit_source_removals WHERE kind = 'departure_exception';
		ALTER TABLE users.student_care_exit_source_removals
			DROP CONSTRAINT IF EXISTS student_care_exit_source_removals_kind_check;
		ALTER TABLE users.student_care_exit_source_removals
			ADD CONSTRAINT student_care_exit_source_removals_kind_check CHECK (kind IN (
				'source_booking', 'pickup_schedule', 'pickup_exception',
				'arrival_schedule', 'arrival_exception'
			));
		DROP TABLE IF EXISTS schedule.student_departure_exceptions;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("error dropping schedule.student_departure_exceptions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit student departure exceptions rollback: %w", err)
	}
	return nil
}
