package migrations

import (
	"log"
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AuthAccountsParentsVersion     = "1.0.9"
	AuthAccountsParentsDescription = "Create auth.accounts_parents table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AuthAccountsParentsVersion] = &Migration{
		Version:     AuthAccountsParentsVersion,
		Description: AuthAccountsParentsDescription,
		DependsOn:   []string{"1.0.1"}, // Depends on schemas, core functions and infrastructure
	}

	// Migration 1.0.9: Create auth.accounts_parents table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createAuthAccountsParentsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropAuthAccountsParentsTable(ctx, db)
		},
	)
}

// createAuthAccountsParentsTable creates the auth.accounts_parents table
func createAuthAccountsParentsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.0.9: Creating auth.accounts_parents table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Create the accounts_parents table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.accounts_parents (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_login TIMESTAMPTZ,
			email TEXT NOT NULL,
			username TEXT UNIQUE,
			active BOOLEAN NOT NULL DEFAULT TRUE,
			password_hash TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating accounts_parents table: %w", err)
	}

	// Create indexes for accounts_parents
	_, err = tx.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_parents_email ON auth.accounts_parents(email);
		CREATE INDEX IF NOT EXISTS idx_accounts_parents_username ON auth.accounts_parents(username);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for accounts_parents table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for accounts_parents
		DROP TRIGGER IF EXISTS update_accounts_parents_updated_at ON auth.accounts_parents;
		CREATE TRIGGER update_accounts_parents_updated_at
		BEFORE UPDATE ON auth.accounts_parents
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for accounts_parents: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropAuthAccountsParentsTable drops the auth.accounts_parents table
func dropAuthAccountsParentsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.0.9: Removing auth.accounts_parents table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_accounts_parents_updated_at ON auth.accounts_parents;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for accounts_parents table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS auth.accounts_parents CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping auth.accounts_parents table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
