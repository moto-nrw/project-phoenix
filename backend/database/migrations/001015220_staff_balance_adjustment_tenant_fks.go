package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	staffBalanceAdjustmentTenantFKsVersion     = "1.15.220"
	staffBalanceAdjustmentTenantFKsDescription = "Enforce tenant-safe staff references on balance adjustments (#1420)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffBalanceAdjustmentTenantFKsVersion,
		Description: staffBalanceAdjustmentTenantFKsDescription,
		DependsOn: []string{
			staffBalanceAdjustmentsVersion,
			compositePKIndexesVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return staffBalanceAdjustmentTenantFKsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return staffBalanceAdjustmentTenantFKsDown(ctx, db)
		},
	)
}

func staffBalanceAdjustmentTenantFKsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.220: Enforcing tenant-safe balance-adjustment staff references...")

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
			DROP CONSTRAINT IF EXISTS staff_balance_adjustments_staff_id_fkey,
			DROP CONSTRAINT IF EXISTS staff_balance_adjustments_decided_by_fkey,
			DROP CONSTRAINT IF EXISTS fk_sba_staff_tenant,
			DROP CONSTRAINT IF EXISTS fk_sba_decided_by_tenant;

		ALTER TABLE active.staff_balance_adjustments
			ADD CONSTRAINT fk_sba_staff_tenant
				FOREIGN KEY (tenant_id, staff_id)
				REFERENCES users.staff(tenant_id, id)
				ON DELETE CASCADE,
			ADD CONSTRAINT fk_sba_decided_by_tenant
				FOREIGN KEY (tenant_id, decided_by)
				REFERENCES users.staff(tenant_id, id)
				ON DELETE RESTRICT;
	`)
	if err != nil {
		return fmt.Errorf("failed to enforce tenant-safe balance-adjustment staff references: %w", err)
	}

	return tx.Commit()
}

func staffBalanceAdjustmentTenantFKsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.220: Restoring global balance-adjustment staff references...")

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
			DROP CONSTRAINT IF EXISTS fk_sba_staff_tenant,
			DROP CONSTRAINT IF EXISTS fk_sba_decided_by_tenant;

		ALTER TABLE active.staff_balance_adjustments
			ADD CONSTRAINT staff_balance_adjustments_staff_id_fkey
				FOREIGN KEY (staff_id)
				REFERENCES users.staff(id)
				ON DELETE CASCADE,
			ADD CONSTRAINT staff_balance_adjustments_decided_by_fkey
				FOREIGN KEY (decided_by)
				REFERENCES users.staff(id)
				ON DELETE RESTRICT;
	`)
	if err != nil {
		return fmt.Errorf("failed to restore global balance-adjustment staff references: %w", err)
	}

	return tx.Commit()
}
