package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	studentStatusDaysVersion     = "1.15.46"
	studentStatusDaysDescription = "Create active.student_status_days for sick and excused daily history"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentStatusDaysVersion,
		Description: studentStatusDaysDescription,
		DependsOn: []string{
			addExcusedToStudentsVersion,  // 1.15.38 — excused/excused_since
			addSicknessToStudentsVersion, // 1.7.2 — sick/sick_since
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentStatusDaysUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentStatusDaysDown(ctx, db)
		},
	)
}

func studentStatusDaysUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.46: Creating active.student_status_days...")

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
		CREATE TABLE IF NOT EXISTS active.student_status_days (
			id          BIGSERIAL PRIMARY KEY,
			tenant_id   BIGINT NOT NULL REFERENCES platform.schools(id),
			student_id  BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			date        DATE NOT NULL,
			status      TEXT NOT NULL,
			reported_at TIMESTAMPTZ NOT NULL,
			cleared_at  TIMESTAMPTZ,
			source      TEXT NOT NULL DEFAULT 'manual',
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_student_status_days_status CHECK (status IN ('sick', 'excused')),
			CONSTRAINT chk_student_status_days_source CHECK (source IN ('manual', 'next_checkin', 'end_of_day')),
			CONSTRAINT uniq_student_status_day UNIQUE (tenant_id, student_id, date, status)
		);

		CREATE INDEX IF NOT EXISTS idx_student_status_days_student_date
			ON active.student_status_days (tenant_id, student_id, date DESC);
		CREATE INDEX IF NOT EXISTS idx_student_status_days_status_date
			ON active.student_status_days (tenant_id, status, date DESC);

		DROP TRIGGER IF EXISTS update_student_status_days_updated_at ON active.student_status_days;
		CREATE TRIGGER update_student_status_days_updated_at
		BEFORE UPDATE ON active.student_status_days
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE active.student_status_days ENABLE ROW LEVEL SECURITY;
		ALTER TABLE active.student_status_days FORCE ROW LEVEL SECURITY;

		CREATE POLICY tenant_isolation_active_student_status_days ON active.student_status_days
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE ON active.student_status_days TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE active.student_status_days_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating active.student_status_days: %w", err)
	}

	return tx.Commit()
}

func studentStatusDaysDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.46: Dropping active.student_status_days...")

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
		DROP TRIGGER IF EXISTS update_student_status_days_updated_at ON active.student_status_days;
		DROP TABLE IF EXISTS active.student_status_days CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping active.student_status_days: %w", err)
	}
	return tx.Commit()
}
