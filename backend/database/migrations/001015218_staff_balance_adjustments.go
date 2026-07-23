package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	staffBalanceAdjustmentsVersion     = "1.15.218"
	staffBalanceAdjustmentsDescription = "Create active.staff_balance_adjustments - Stundenkonto payout/comp-time/reset transactions (#1420)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffBalanceAdjustmentsVersion,
		Description: staffBalanceAdjustmentsDescription,
		DependsOn: []string{
			absenceQuestionStatusVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return staffBalanceAdjustmentsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return staffBalanceAdjustmentsDown(ctx, db)
		},
	)
}

// staffBalanceAdjustmentsUp creates the Stundenkonto correction ledger
// (#1420 5a/5c). Every row is a dedicated transaction — payout of plus
// hours, a lump-sum Freizeitausgleich grant, or a school-year reset — with a
// SIGNED minutes_delta (reductions negative). The month carry chain sums
// them by effective_date, so a payout flows into every later Monatskarte.
//
// The partial unique index allows exactly one reset per staff and date: a
// double-clicked reset must fail on the second insert instead of deducting
// the balance twice.
func staffBalanceAdjustmentsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.218: Creating active.staff_balance_adjustments...")

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
		CREATE TABLE IF NOT EXISTS active.staff_balance_adjustments (
			id             BIGSERIAL PRIMARY KEY,
			tenant_id      BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			staff_id       BIGINT NOT NULL REFERENCES users.staff(id) ON DELETE CASCADE,
			type           TEXT NOT NULL,
			minutes_delta  INTEGER NOT NULL,
			effective_date DATE NOT NULL,
			note           TEXT NOT NULL DEFAULT '',
			decided_by     BIGINT NOT NULL REFERENCES users.staff(id) ON DELETE RESTRICT,
			decided_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_sba_type CHECK (type IN ('payout','comp_time','reset'))
		);

		CREATE INDEX IF NOT EXISTS idx_sba_tenant_staff_date
			ON active.staff_balance_adjustments (tenant_id, staff_id, effective_date);

		CREATE UNIQUE INDEX IF NOT EXISTS uq_sba_reset_per_day
			ON active.staff_balance_adjustments (tenant_id, staff_id, effective_date)
			WHERE type = 'reset';

		DROP TRIGGER IF EXISTS update_staff_balance_adjustments_updated_at ON active.staff_balance_adjustments;
		CREATE TRIGGER update_staff_balance_adjustments_updated_at
		BEFORE UPDATE ON active.staff_balance_adjustments
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE active.staff_balance_adjustments ENABLE ROW LEVEL SECURITY;
		ALTER TABLE active.staff_balance_adjustments FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_active_staff_balance_adjustments ON active.staff_balance_adjustments;
		CREATE POLICY tenant_isolation_active_staff_balance_adjustments ON active.staff_balance_adjustments
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON active.staff_balance_adjustments TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE active.staff_balance_adjustments_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating active.staff_balance_adjustments: %w", err)
	}

	return tx.Commit()
}

func staffBalanceAdjustmentsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.218: Dropping active.staff_balance_adjustments...")

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
		DROP TRIGGER IF EXISTS update_staff_balance_adjustments_updated_at ON active.staff_balance_adjustments;
		DROP TABLE IF EXISTS active.staff_balance_adjustments CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping active.staff_balance_adjustments: %w", err)
	}
	return tx.Commit()
}
