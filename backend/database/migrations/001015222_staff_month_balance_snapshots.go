package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	staffMonthBalanceSnapshotsVersion     = "1.15.222"
	staffMonthBalanceSnapshotsDescription = "Create active.staff_month_balance_snapshots - frozen month closing balances (#1417)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffMonthBalanceSnapshotsVersion,
		Description: staffMonthBalanceSnapshotsDescription,
		DependsOn: []string{
			staffBalanceAdjustmentTenantFKsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return staffMonthBalanceSnapshotsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return staffMonthBalanceSnapshotsDown(ctx, db)
		},
	)
}

// staffMonthBalanceSnapshotsUp creates the Monatsabschluss table (#1417).
//
// The Stundenkonto is computed live: the carry chain sums every month from the
// account-start anchor up to the requested one. That makes a retroactive
// correction rewrite history — editing an August shift in October moves the
// August balance and every later carry with it. A snapshot freezes ONE value:
// the closing balance of a month. From then on the chain for later months
// starts at that number instead of re-summing from the anchor. Months up to
// and including the frozen one are still recomputed for display, and the
// difference is reported as drift — surfaced, never silently absorbed.
//
// A snapshot is NOT a booking. active.staff_balance_adjustments (#1420) holds
// dated transactions with a signed delta; this table holds no delta at all.
// The two are orthogonal: a 'reset' adjustment does not freeze anything, and a
// snapshot moves no balance.
//
// Reopening supersedes instead of deleting: the row keeps who closed, who
// reopened, and why, so it is its own audit record (the same doctrine as
// StaffBalanceAdjustment). The partial unique index therefore covers only
// non-superseded rows — at most one active snapshot per staff member and
// month, with the reopened history piling up behind it.
//
// Deliberately no DATE column: year/month are integers and closed_at/
// reopened_at are instants, so there is no calendar-date binding to get wrong
// (see .claude/rules/calendar-dates.md). Do not add a month_end_date
// convenience column.
//
// The composite (tenant_id, staff_id) foreign keys are the tenant-safe form
// retrofitted onto the adjustment ledger in 1.15.221 — a plain staff_id FK
// would let a row reference a staff member of another school. closed_by and
// reopened_by use ON DELETE RESTRICT for the same reason the adjustment
// ledger does, and inherit the same wart: offboarding a staff member who once
// closed a month is blocked until those rows are dealt with. Consistency with
// the ledger beats diverging here.
func staffMonthBalanceSnapshotsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.222: Creating active.staff_month_balance_snapshots...")

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
		CREATE TABLE IF NOT EXISTS active.staff_month_balance_snapshots (
			id                      BIGSERIAL PRIMARY KEY,
			tenant_id               BIGINT NOT NULL,
			staff_id                BIGINT NOT NULL,
			year                    SMALLINT NOT NULL,
			month                   SMALLINT NOT NULL,
			closing_balance_minutes INTEGER NOT NULL,
			carry_in_minutes        INTEGER NOT NULL DEFAULT 0,
			target_minutes          INTEGER NOT NULL DEFAULT 0,
			actual_minutes          INTEGER NOT NULL DEFAULT 0,
			credited_minutes        INTEGER NOT NULL DEFAULT 0,
			adjustment_minutes      INTEGER NOT NULL DEFAULT 0,
			closed_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			closed_by               BIGINT NOT NULL,
			close_reason            TEXT NOT NULL DEFAULT '',
			source                  TEXT NOT NULL DEFAULT 'admin',
			reopened_at             TIMESTAMPTZ,
			reopened_by             BIGINT,
			reopen_reason           TEXT NOT NULL DEFAULT '',
			created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_smbs_month CHECK (month BETWEEN 1 AND 12),
			CONSTRAINT chk_smbs_year CHECK (year BETWEEN 2000 AND 2100),
			CONSTRAINT chk_smbs_source CHECK (source IN ('admin','scheduler','migration')),
			CONSTRAINT chk_smbs_reopen CHECK ((reopened_at IS NULL) = (reopened_by IS NULL)),
			CONSTRAINT fk_smbs_tenant
				FOREIGN KEY (tenant_id)
				REFERENCES platform.schools(id)
				ON DELETE CASCADE,
			CONSTRAINT fk_smbs_staff_tenant
				FOREIGN KEY (tenant_id, staff_id)
				REFERENCES users.staff(tenant_id, id)
				ON DELETE CASCADE,
			CONSTRAINT fk_smbs_closed_by_tenant
				FOREIGN KEY (tenant_id, closed_by)
				REFERENCES users.staff(tenant_id, id)
				ON DELETE RESTRICT,
			CONSTRAINT fk_smbs_reopened_by_tenant
				FOREIGN KEY (tenant_id, reopened_by)
				REFERENCES users.staff(tenant_id, id)
				ON DELETE RESTRICT
		);

		-- At most one ACTIVE snapshot per staff member and month; superseded
		-- (reopened) rows stay behind it as the audit trail.
		CREATE UNIQUE INDEX IF NOT EXISTS uq_smbs_active_month
			ON active.staff_month_balance_snapshots (tenant_id, staff_id, year, month)
			WHERE reopened_at IS NULL;

		-- Splice lookup: newest active snapshot at or before a given month.
		CREATE INDEX IF NOT EXISTS idx_smbs_lookup
			ON active.staff_month_balance_snapshots (tenant_id, staff_id, year DESC, month DESC)
			WHERE reopened_at IS NULL;

		-- School-wide close status of one month.
		CREATE INDEX IF NOT EXISTS idx_smbs_tenant_month
			ON active.staff_month_balance_snapshots (tenant_id, year, month)
			WHERE reopened_at IS NULL;

		DROP TRIGGER IF EXISTS update_staff_month_balance_snapshots_updated_at ON active.staff_month_balance_snapshots;
		CREATE TRIGGER update_staff_month_balance_snapshots_updated_at
		BEFORE UPDATE ON active.staff_month_balance_snapshots
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE active.staff_month_balance_snapshots ENABLE ROW LEVEL SECURITY;
		ALTER TABLE active.staff_month_balance_snapshots FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_active_staff_month_balance_snapshots ON active.staff_month_balance_snapshots;
		CREATE POLICY tenant_isolation_active_staff_month_balance_snapshots ON active.staff_month_balance_snapshots
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON active.staff_month_balance_snapshots TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE active.staff_month_balance_snapshots_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating active.staff_month_balance_snapshots: %w", err)
	}

	return tx.Commit()
}

func staffMonthBalanceSnapshotsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.222: Dropping active.staff_month_balance_snapshots...")

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
		DROP TRIGGER IF EXISTS update_staff_month_balance_snapshots_updated_at ON active.staff_month_balance_snapshots;
		DROP TABLE IF EXISTS active.staff_month_balance_snapshots CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping active.staff_month_balance_snapshots: %w", err)
	}
	return tx.Commit()
}
