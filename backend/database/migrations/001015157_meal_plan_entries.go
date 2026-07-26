package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	// 1.15.157 sits above development's latest (1.15.156,
	// grant_guardian_request_submit_permission). Renumbered off 1.15.155, which
	// development took for parent_messaging_requests — a duplicate version panics
	// the registry at init, and reusing an already-applied file prefix would make
	// bun skip this migration on deployed DBs.
	mealPlanEntriesVersion     = "1.15.157"
	mealPlanEntriesDescription = "Create schedule.meal_plan_entries for the per-day meal plan (Essensplan)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mealPlanEntriesVersion,
		Description: mealPlanEntriesDescription,
		DependsOn:   []string{lateInvitesAndRequestSourcesVersion}, // 1.15.153 — latest development migration before this branch
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return mealPlanEntriesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return mealPlanEntriesDown(ctx, db)
		},
	)
}

func mealPlanEntriesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.157: Creating schedule.meal_plan_entries...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// One row per dish on a calendar day. The plan is school-wide (not per
	// student); a day can carry several dishes (e.g. Menü 1 / Menü 2 /
	// vegetarisch), ordered by position. Natural key is (tenant_id, date,
	// position).
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schedule.meal_plan_entries (
			id          BIGSERIAL PRIMARY KEY,
			tenant_id   BIGINT NOT NULL REFERENCES platform.schools(id),
			date        DATE NOT NULL,
			position    INT NOT NULL DEFAULT 0,
			dish        TEXT NOT NULL,
			note        TEXT,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uniq_meal_plan_entry_date_position UNIQUE (tenant_id, date, position)
		);

		CREATE INDEX IF NOT EXISTS idx_meal_plan_entries_tenant_date
			ON schedule.meal_plan_entries (tenant_id, date, position);

		DROP TRIGGER IF EXISTS update_meal_plan_entries_updated_at ON schedule.meal_plan_entries;
		CREATE TRIGGER update_meal_plan_entries_updated_at
		BEFORE UPDATE ON schedule.meal_plan_entries
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE schedule.meal_plan_entries ENABLE ROW LEVEL SECURITY;
		ALTER TABLE schedule.meal_plan_entries FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_schedule_meal_plan_entries ON schedule.meal_plan_entries;
		CREATE POLICY tenant_isolation_schedule_meal_plan_entries ON schedule.meal_plan_entries
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON schedule.meal_plan_entries TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE schedule.meal_plan_entries_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating schedule.meal_plan_entries: %w", err)
	}

	return tx.Commit()
}

func mealPlanEntriesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.157: Dropping schedule.meal_plan_entries...")

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
		DROP TRIGGER IF EXISTS update_meal_plan_entries_updated_at ON schedule.meal_plan_entries;
		DROP TABLE IF EXISTS schedule.meal_plan_entries CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping schedule.meal_plan_entries: %w", err)
	}
	return tx.Commit()
}
