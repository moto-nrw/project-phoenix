package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	closingDaysVersion     = "1.15.211"
	closingDaysDescription = "Create schedule.closing_days - tenant-specific OGS closure periods with Soll=0 semantics"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     closingDaysVersion,
		Description: closingDaysDescription,
		DependsOn: []string{
			deviationEventShiftAnchorVersion, // latest development migration before this branch
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return closingDaysUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return closingDaysDown(ctx, db)
		},
	)
}

// closingDaysUp creates the store for OGS-Schließtage (#1418 3b): closure
// periods a school defines itself (pädagogische Tage, Weihnachtswoche,
// Sommerschließung) on top of the computed public holidays (3a). Rows are
// ranges, not single days — a three-week summer closure is one row, and a
// single closed day is a range with start_date = end_date (hence <= in the
// CHECK, unlike calendar_periods which requires strictly increasing ranges).
//
// There is deliberately no overlap EXCLUDE constraint: overlapping closure
// ranges are harmless because Soll computation collapses them into one date
// set, and avoiding btree_gist keeps the migration dependency-free.
func closingDaysUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.211: Creating schedule.closing_days...")

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
		CREATE TABLE IF NOT EXISTS schedule.closing_days (
			id         BIGSERIAL PRIMARY KEY,
			tenant_id  BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			start_date DATE NOT NULL,
			end_date   DATE NOT NULL,
			reason     TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT closing_days_range_ok CHECK (start_date <= end_date)
		);

		CREATE INDEX IF NOT EXISTS idx_closing_days_tenant_range
			ON schedule.closing_days (tenant_id, start_date, end_date);

		DROP TRIGGER IF EXISTS update_closing_days_updated_at ON schedule.closing_days;
		CREATE TRIGGER update_closing_days_updated_at
		BEFORE UPDATE ON schedule.closing_days
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE schedule.closing_days ENABLE ROW LEVEL SECURITY;
		ALTER TABLE schedule.closing_days FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_schedule_closing_days ON schedule.closing_days;
		CREATE POLICY tenant_isolation_schedule_closing_days ON schedule.closing_days
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON schedule.closing_days TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE schedule.closing_days_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating schedule.closing_days: %w", err)
	}

	return tx.Commit()
}

func closingDaysDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.211: Dropping schedule.closing_days...")

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
		DROP TRIGGER IF EXISTS update_closing_days_updated_at ON schedule.closing_days;
		DROP TABLE IF EXISTS schedule.closing_days CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping schedule.closing_days: %w", err)
	}
	return tx.Commit()
}
