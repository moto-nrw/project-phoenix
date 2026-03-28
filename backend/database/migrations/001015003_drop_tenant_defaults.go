package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	dropTenantDefaultsVersion     = "1.15.3"
	dropTenantDefaultsDescription = "Drop DEFAULT 1 from tenant_id on all tenant-scoped tables — INSERTs without explicit tenant_id now fail immediately"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dropTenantDefaultsVersion,
		Description: dropTenantDefaultsDescription,
		DependsOn:   []string{"1.15.2"}, // composite FKs in place
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return dropTenantDefaults(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackTenantDefaults(ctx, db)
		},
	)
}

func dropTenantDefaults(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.3: Dropping DEFAULT 1 from tenant_id on all tenant-scoped tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil &&
			err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop DEFAULT 1 from all 58 NOT NULL tenant_id tables.
	// After this, any INSERT that omits tenant_id fails immediately with a NOT NULL violation.
	// This is the final safety-net removal — all code must now set tenant_id explicitly.
	for _, table := range tablesWithNotNullTenantID {
		_, err := tx.ExecContext(ctx, fmt.Sprintf(
			`ALTER TABLE %s ALTER COLUMN tenant_id DROP DEFAULT`, table))
		if err != nil {
			return fmt.Errorf("failed to drop default on %s: %w", table, err)
		}
	}

	// auth.roles has nullable tenant_id with no default — nothing to drop.

	fmt.Printf("Migration 1.15.3: Successfully dropped DEFAULT 1 from tenant_id on %d tables\n",
		len(tablesWithNotNullTenantID))
	return tx.Commit()
}

func rollbackTenantDefaults(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.3: Restoring DEFAULT 1 on tenant_id...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil &&
			err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	for _, table := range tablesWithNotNullTenantID {
		_, err := tx.ExecContext(ctx, fmt.Sprintf(
			`ALTER TABLE %s ALTER COLUMN tenant_id SET DEFAULT 1`, table))
		if err != nil {
			return fmt.Errorf("failed to restore default on %s: %w", table, err)
		}
	}

	fmt.Println("Migration 1.15.3: Successfully restored DEFAULT 1")
	return tx.Commit()
}
