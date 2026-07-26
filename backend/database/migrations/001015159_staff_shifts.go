package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	staffShiftsVersion     = "1.15.159"
	staffShiftsDescription = "Create schedule.staff_shifts for per-date planned staff shifts (Dienstplan)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffShiftsVersion,
		Description: staffShiftsDescription,
		DependsOn:   []string{parentMessageStaffNameVisibilityVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return staffShiftsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return staffShiftsDown(ctx, db)
		},
	)
}

func staffShiftsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.159: Creating schedule.staff_shifts...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// One row per planned shift. Multiple non-overlapping shifts per staff per
	// day are allowed (split shifts: Frühdienst + Nachmittagsbetreuung); the
	// service layer enforces non-overlap, the unique constraint blocks exact
	// duplicates. Concrete wall-clock times per calendar date — distinct from
	// the contractual weekly Soll in config.staff_work_schedules.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schedule.staff_shifts (
			id            BIGSERIAL PRIMARY KEY,
			tenant_id     BIGINT NOT NULL REFERENCES platform.schools(id),
			staff_id      BIGINT NOT NULL,
			date          DATE NOT NULL,
			start_time    TIME WITHOUT TIME ZONE NOT NULL,
			end_time      TIME WITHOUT TIME ZONE NOT NULL,
			break_minutes INT NOT NULL DEFAULT 0,
			notes         TEXT,
			created_by    BIGINT NOT NULL,
			updated_by    BIGINT,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_staff_shifts_times CHECK (end_time > start_time),
			CONSTRAINT chk_staff_shifts_break CHECK (break_minutes >= 0),
			CONSTRAINT fk_staff_shifts_staff
				FOREIGN KEY (tenant_id, staff_id)
				REFERENCES users.staff(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT fk_staff_shifts_created_by
				FOREIGN KEY (tenant_id, created_by)
				REFERENCES users.staff(tenant_id, id),
			CONSTRAINT fk_staff_shifts_updated_by
				FOREIGN KEY (tenant_id, updated_by)
				REFERENCES users.staff(tenant_id, id),
			CONSTRAINT uniq_staff_shift_start UNIQUE (tenant_id, staff_id, date, start_time)
		);

		CREATE INDEX IF NOT EXISTS idx_staff_shifts_tenant_date
			ON schedule.staff_shifts (tenant_id, date);
		CREATE INDEX IF NOT EXISTS idx_staff_shifts_staff_date
			ON schedule.staff_shifts (tenant_id, staff_id, date);

		DROP TRIGGER IF EXISTS update_staff_shifts_updated_at ON schedule.staff_shifts;
		CREATE TRIGGER update_staff_shifts_updated_at
		BEFORE UPDATE ON schedule.staff_shifts
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE schedule.staff_shifts ENABLE ROW LEVEL SECURITY;
		ALTER TABLE schedule.staff_shifts FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_schedule_staff_shifts ON schedule.staff_shifts;
		CREATE POLICY tenant_isolation_schedule_staff_shifts ON schedule.staff_shifts
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON schedule.staff_shifts TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE schedule.staff_shifts_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating schedule.staff_shifts: %w", err)
	}

	return tx.Commit()
}

func staffShiftsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.159: Dropping schedule.staff_shifts...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_staff_shifts_updated_at ON schedule.staff_shifts;
		DROP TABLE IF EXISTS schedule.staff_shifts CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping schedule.staff_shifts: %w", err)
	}
	return tx.Commit()
}
