package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	staffShiftSeriesVersion     = "1.15.190"
	staffShiftSeriesDescription = "Create schedule.staff_shift_series + exceptions and link staff_shifts via series_id/detached (#1889)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffShiftSeriesVersion,
		Description: staffShiftSeriesDescription,
		DependsOn:   []string{staffRequiredOverrideVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return staffShiftSeriesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return staffShiftSeriesDown(ctx, db)
		},
	)
}

func staffShiftSeriesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.190: Creating schedule.staff_shift_series...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// One row per recurring shift series (#1889). All selected weekdays share
	// one wall-clock window, break, type, and note, so weekdays live in an
	// array instead of per-weekday rows. The series is bound to a calendar
	// period (validity + week A/B anchor); valid_from/valid_until are split
	// segment bounds within that period (valid_until exclusive, NULL = until
	// period end), mirroring the timetable template split convention.
	// series_root_id links all segments produced by splitting one original
	// series ("Ab jetzt dauerhaft").
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schedule.staff_shift_series (
			id                 BIGSERIAL PRIMARY KEY,
			tenant_id          BIGINT NOT NULL REFERENCES platform.schools(id),
			staff_id           BIGINT NOT NULL,
			weekdays           SMALLINT[] NOT NULL,
			start_time         TIME WITHOUT TIME ZONE NOT NULL,
			end_time           TIME WITHOUT TIME ZONE NOT NULL,
			break_minutes      INT NOT NULL DEFAULT 0,
			shift_type_id      BIGINT,
			notes              TEXT,
			calendar_period_id BIGINT NOT NULL,
			week_pattern       SMALLINT NOT NULL DEFAULT 0,
			valid_from         DATE NOT NULL,
			valid_until        DATE,
			series_root_id     BIGINT,
			created_by         BIGINT NOT NULL,
			updated_by         BIGINT,
			created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_staff_shift_series_times CHECK (end_time > start_time),
			CONSTRAINT chk_staff_shift_series_break CHECK (break_minutes >= 0 AND break_minutes <= 300),
			CONSTRAINT chk_staff_shift_series_weekdays
				CHECK (cardinality(weekdays) > 0 AND weekdays <@ ARRAY[1,2,3,4,5,6,7]::smallint[]),
			CONSTRAINT chk_staff_shift_series_week_pattern CHECK (week_pattern BETWEEN 0 AND 2),
			-- >= not >: valid_until = valid_from is a deliberately emptied
			-- segment (cap at or before the first occurrence, offboarding of
			-- a future-dated series) that materializes nothing.
			CONSTRAINT chk_staff_shift_series_validity
				CHECK (valid_until IS NULL OR valid_until >= valid_from),
			CONSTRAINT fk_staff_shift_series_staff
				FOREIGN KEY (tenant_id, staff_id)
				REFERENCES users.staff(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT fk_staff_shift_series_shift_type
				FOREIGN KEY (tenant_id, shift_type_id)
				REFERENCES schedule.shift_types(tenant_id, id)
				ON DELETE SET NULL (shift_type_id),
			-- Deliberately no ON DELETE action: deleting a calendar period that
			-- still owns series must fail instead of silently killing them.
			CONSTRAINT fk_staff_shift_series_calendar_period
				FOREIGN KEY (tenant_id, calendar_period_id)
				REFERENCES schedule.calendar_periods(tenant_id, id),
			CONSTRAINT fk_staff_shift_series_root
				FOREIGN KEY (tenant_id, series_root_id)
				REFERENCES schedule.staff_shift_series(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT fk_staff_shift_series_created_by
				FOREIGN KEY (tenant_id, created_by)
				REFERENCES users.staff(tenant_id, id),
			CONSTRAINT fk_staff_shift_series_updated_by
				FOREIGN KEY (tenant_id, updated_by)
				REFERENCES users.staff(tenant_id, id),
			-- Backs the composite tenant-safe FK from schedule.staff_shifts.
			CONSTRAINT uniq_staff_shift_series_tenant_id UNIQUE (tenant_id, id)
		);

		CREATE INDEX IF NOT EXISTS idx_staff_shift_series_tenant_staff
			ON schedule.staff_shift_series (tenant_id, staff_id);
		CREATE INDEX IF NOT EXISTS idx_staff_shift_series_root
			ON schedule.staff_shift_series (tenant_id, series_root_id)
			WHERE series_root_id IS NOT NULL;

		DROP TRIGGER IF EXISTS update_staff_shift_series_updated_at ON schedule.staff_shift_series;
		CREATE TRIGGER update_staff_shift_series_updated_at
		BEFORE UPDATE ON schedule.staff_shift_series
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE schedule.staff_shift_series ENABLE ROW LEVEL SECURITY;
		ALTER TABLE schedule.staff_shift_series FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_schedule_staff_shift_series ON schedule.staff_shift_series;
		CREATE POLICY tenant_isolation_schedule_staff_shift_series ON schedule.staff_shift_series
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON schedule.staff_shift_series TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE schedule.staff_shift_series_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating schedule.staff_shift_series: %w", err)
	}

	// One row per deliberately removed single occurrence: deleting one
	// materialized series shift removes the concrete row AND records its date
	// here so a later re-plan or split never regenerates it (the shift-domain
	// counterpart to timetable cancellations, without readers having to
	// filter anything).
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schedule.staff_shift_series_exceptions (
			id         BIGSERIAL PRIMARY KEY,
			tenant_id  BIGINT NOT NULL REFERENCES platform.schools(id),
			series_id  BIGINT NOT NULL,
			date       DATE NOT NULL,
			created_by BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_staff_shift_series_exceptions_series
				FOREIGN KEY (tenant_id, series_id)
				REFERENCES schedule.staff_shift_series(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT fk_staff_shift_series_exceptions_created_by
				FOREIGN KEY (tenant_id, created_by)
				REFERENCES users.staff(tenant_id, id),
			CONSTRAINT uniq_staff_shift_series_exception UNIQUE (tenant_id, series_id, date)
		);

		CREATE INDEX IF NOT EXISTS idx_staff_shift_series_exceptions_series
			ON schedule.staff_shift_series_exceptions (tenant_id, series_id);

		ALTER TABLE schedule.staff_shift_series_exceptions ENABLE ROW LEVEL SECURITY;
		ALTER TABLE schedule.staff_shift_series_exceptions FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_schedule_staff_shift_series_exceptions ON schedule.staff_shift_series_exceptions;
		CREATE POLICY tenant_isolation_schedule_staff_shift_series_exceptions ON schedule.staff_shift_series_exceptions
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON schedule.staff_shift_series_exceptions TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE schedule.staff_shift_series_exceptions_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating schedule.staff_shift_series_exceptions: %w", err)
	}

	// series_id: ON DELETE SET NULL on purpose — a hard-deleted series
	// degrades its concrete shifts to standalone shifts instead of cascading
	// away time-tracking-relevant rows. detached marks a row the user edited
	// via "Nur diese Woche": re-plans delete only non-detached rows.
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE schedule.staff_shifts
			ADD COLUMN IF NOT EXISTS series_id BIGINT,
			ADD COLUMN IF NOT EXISTS detached BOOLEAN NOT NULL DEFAULT FALSE;

		ALTER TABLE schedule.staff_shifts
			DROP CONSTRAINT IF EXISTS fk_staff_shifts_series,
			ADD CONSTRAINT fk_staff_shifts_series
				FOREIGN KEY (tenant_id, series_id)
				REFERENCES schedule.staff_shift_series(tenant_id, id)
				ON DELETE SET NULL (series_id);

		CREATE INDEX IF NOT EXISTS idx_staff_shifts_series
			ON schedule.staff_shifts (tenant_id, series_id)
			WHERE series_id IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error adding series columns to schedule.staff_shifts: %w", err)
	}

	return tx.Commit()
}

func staffShiftSeriesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.190: Dropping schedule.staff_shift_series...")

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
		ALTER TABLE schedule.staff_shifts
			DROP CONSTRAINT IF EXISTS fk_staff_shifts_series;
		DROP INDEX IF EXISTS schedule.idx_staff_shifts_series;
		ALTER TABLE schedule.staff_shifts
			DROP COLUMN IF EXISTS series_id,
			DROP COLUMN IF EXISTS detached;

		DROP TRIGGER IF EXISTS update_staff_shift_series_updated_at ON schedule.staff_shift_series;
		DROP TABLE IF EXISTS schedule.staff_shift_series_exceptions CASCADE;
		DROP TABLE IF EXISTS schedule.staff_shift_series CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping schedule.staff_shift_series: %w", err)
	}
	return tx.Commit()
}
