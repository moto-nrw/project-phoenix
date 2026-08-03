package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	openingBalancesVersion     = "1.15.261"
	openingBalancesDescription = "Opening balances for Stundenkonto and vacation at go-live (#2132)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     openingBalancesVersion,
		Description: openingBalancesDescription,
		DependsOn: []string{
			staffMasterDataContractDatesVersion,
			staffBalanceAdjustmentTenantFKsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return openingBalancesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return openingBalancesDown(ctx, db)
		},
	)
}

// openingBalancesUp enables go-live opening balances (#2132):
//
//  1. The Stundenkonto ledger learns a fourth transaction type 'opening' — a
//     signed rebaseline booking whose resulting balance may be negative
//     (migrated accounts start where the old system left them). The partial
//     unique index allows exactly one opening per staff member: a takeover is
//     a one-time event; corrections go through delete + re-create so the
//     deletion tombstone keeps the history.
//
//  2. active.staff_vacation_openings records vacation days taken BEFORE the
//     moto introduction (per staff and calendar year). The row itself is the
//     audit record — Stichtag, note, decided_by, decided_at — mirroring
//     staff_balance_adjustments. Remaining vacation subtracts taken_before
//     alongside in-app absences, so the Jahresanspruch stays untouched.
//     entered_remaining_days documents what the admin actually typed
//     (Resturlaub zum Stichtag) from which taken_before was derived.
func openingBalancesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.261: Opening balances for Stundenkonto and vacation...")

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
		ALTER TABLE active.staff_balance_adjustments
			DROP CONSTRAINT IF EXISTS chk_sba_type;
		ALTER TABLE active.staff_balance_adjustments
			ADD CONSTRAINT chk_sba_type CHECK (type IN ('payout','comp_time','reset','opening'));

		CREATE UNIQUE INDEX IF NOT EXISTS uq_sba_opening_per_staff
			ON active.staff_balance_adjustments (tenant_id, staff_id)
			WHERE type = 'opening';

		CREATE TABLE IF NOT EXISTS active.staff_vacation_openings (
			id                     BIGSERIAL PRIMARY KEY,
			tenant_id              BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			staff_id               BIGINT NOT NULL,
			year                   INT NOT NULL,
			effective_date         DATE NOT NULL,
			taken_before_days      NUMERIC(5,1) NOT NULL,
			entered_remaining_days NUMERIC(5,1) NOT NULL,
			note                   TEXT NOT NULL DEFAULT '',
			decided_by             BIGINT NOT NULL,
			decided_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_svo_staff_year UNIQUE (tenant_id, staff_id, year),
			CONSTRAINT chk_svo_year CHECK (year BETWEEN 2000 AND 2100),
			CONSTRAINT fk_svo_staff_tenant
				FOREIGN KEY (tenant_id, staff_id)
				REFERENCES users.staff(tenant_id, id)
				ON DELETE CASCADE,
			CONSTRAINT fk_svo_decided_by_tenant
				FOREIGN KEY (tenant_id, decided_by)
				REFERENCES users.staff(tenant_id, id)
				ON DELETE RESTRICT
		);

		ALTER TABLE active.staff_vacation_openings ENABLE ROW LEVEL SECURITY;
		ALTER TABLE active.staff_vacation_openings FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_active_staff_vacation_openings ON active.staff_vacation_openings;
		CREATE POLICY tenant_isolation_active_staff_vacation_openings ON active.staff_vacation_openings
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, DELETE ON active.staff_vacation_openings TO phoenix_tenant;
		-- The row is its own audit record: corrections are delete + re-create
		-- (with a deletion tombstone), never an in-place UPDATE. 1.14.1's
		-- default privileges would grant UPDATE, so revoke it explicitly.
		REVOKE UPDATE ON active.staff_vacation_openings FROM phoenix_tenant;
		GRANT USAGE ON SEQUENCE active.staff_vacation_openings_id_seq TO phoenix_tenant;

		-- Deleted vacation openings leave the same append-only tombstone the
		-- ledger and absences use (#1417).
		ALTER TABLE audit.time_tracking_deletions
			DROP CONSTRAINT IF EXISTS time_tracking_deletions_source_check;
		ALTER TABLE audit.time_tracking_deletions
			ADD CONSTRAINT time_tracking_deletions_source_check
				CHECK (source IN ('balance_adjustment', 'absence', 'vacation_opening'));
	`)
	if err != nil {
		return fmt.Errorf("error creating opening balance structures: %w", err)
	}

	return tx.Commit()
}

func openingBalancesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.261: Removing opening balance structures...")

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
		DELETE FROM audit.time_tracking_deletions WHERE source = 'vacation_opening';
		ALTER TABLE audit.time_tracking_deletions
			DROP CONSTRAINT IF EXISTS time_tracking_deletions_source_check;
		ALTER TABLE audit.time_tracking_deletions
			ADD CONSTRAINT time_tracking_deletions_source_check
				CHECK (source IN ('balance_adjustment', 'absence'));

		DROP TABLE IF EXISTS active.staff_vacation_openings CASCADE;

		DROP INDEX IF EXISTS active.uq_sba_opening_per_staff;
		DELETE FROM active.staff_balance_adjustments WHERE type = 'opening';
		ALTER TABLE active.staff_balance_adjustments
			DROP CONSTRAINT IF EXISTS chk_sba_type;
		ALTER TABLE active.staff_balance_adjustments
			ADD CONSTRAINT chk_sba_type CHECK (type IN ('payout','comp_time','reset'));
	`)
	if err != nil {
		return fmt.Errorf("error removing opening balance structures: %w", err)
	}
	return tx.Commit()
}
