package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	fixSchoolsSubdomainVersion     = "1.13.3"
	fixSchoolsSubdomainDescription = "Shrink schools.subdomain to VARCHAR(63) (DNS label limit) and drop redundant index"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     fixSchoolsSubdomainVersion,
		Description: fixSchoolsSubdomainDescription,
		DependsOn:   []string{"1.13.1"}, // Depends on platform.schools table
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return fixSchoolsSubdomain(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackFixSchoolsSubdomain(ctx, db)
		},
	)
}

func fixSchoolsSubdomain(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.13.3: Fixing schools.subdomain column and dropping redundant index...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Shrink subdomain from VARCHAR(100) to VARCHAR(63) — RFC 1123 DNS label limit
	_, err = tx.ExecContext(ctx, `ALTER TABLE platform.schools ALTER COLUMN subdomain TYPE VARCHAR(63);`)
	if err != nil {
		return fmt.Errorf("error altering schools.subdomain column: %w", err)
	}

	// Drop redundant index — the UNIQUE constraint on subdomain already creates an implicit index
	_, err = tx.ExecContext(ctx, `DROP INDEX IF EXISTS platform.idx_schools_subdomain;`)
	if err != nil {
		return fmt.Errorf("error dropping redundant index: %w", err)
	}

	fmt.Println("Migration 1.13.3: Successfully fixed schools.subdomain and dropped redundant index")
	return tx.Commit()
}

func rollbackFixSchoolsSubdomain(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.13.3: Restoring original subdomain column and index...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Restore subdomain to VARCHAR(100)
	_, err = tx.ExecContext(ctx, `ALTER TABLE platform.schools ALTER COLUMN subdomain TYPE VARCHAR(100);`)
	if err != nil {
		return fmt.Errorf("error restoring schools.subdomain column: %w", err)
	}

	// Recreate the redundant index
	_, err = tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_schools_subdomain ON platform.schools(subdomain);`)
	if err != nil {
		return fmt.Errorf("error recreating index: %w", err)
	}

	fmt.Println("Migration 1.13.3: Successfully rolled back subdomain fix")
	return tx.Commit()
}
