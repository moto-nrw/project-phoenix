package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	SchemasVersion     = "0.0.0"
	SchemasDescription = "Create database schemas"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[SchemasVersion] = &Migration{
		Version:     SchemasVersion,
		Description: SchemasDescription,
		DependsOn:   []string{},
	}

	// Migration 0.0: Create database schemas
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 0.0.0: Creating database schemas...")

			// Begin a transaction for atomicity
			tx, err := db.BeginTx(ctx, &sql.TxOptions{})
			if err != nil {
				return fmt.Errorf("failed to begin transaction: %w", err)
			}
			defer func() { _ = tx.Rollback() }()

			// Create all the schemas
			_, err = tx.ExecContext(ctx, `
				-- Create schemas for different areas of functionality
				CREATE SCHEMA IF NOT EXISTS auth;
				CREATE SCHEMA IF NOT EXISTS users;
				CREATE SCHEMA IF NOT EXISTS education;
				CREATE SCHEMA IF NOT EXISTS schedule;
				CREATE SCHEMA IF NOT EXISTS activities;
				CREATE SCHEMA IF NOT EXISTS facilities;
				CREATE SCHEMA IF NOT EXISTS iot;
				CREATE SCHEMA IF NOT EXISTS feedback;
				CREATE SCHEMA IF NOT EXISTS active;
				CREATE SCHEMA IF NOT EXISTS config;
				CREATE SCHEMA IF NOT EXISTS meta;
				CREATE SCHEMA IF NOT EXISTS audit;
			`)
			if err != nil {
				return fmt.Errorf("error creating schemas: %w", err)
			}

			// Commit the transaction
			return tx.Commit()
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 0.0.0: Removing database schemas...")

			// Begin a transaction for atomicity
			tx, err := db.BeginTx(ctx, &sql.TxOptions{})
			if err != nil {
				return fmt.Errorf("failed to begin transaction: %w", err)
			}
			defer func() { _ = tx.Rollback() }()

			// Drop the schemas in reverse order of dependencies
			// Only if they're empty - CASCADE would forcibly drop contents
			_, err = tx.ExecContext(ctx, `
				DROP SCHEMA IF EXISTS audit;
				DROP SCHEMA IF EXISTS meta;
				DROP SCHEMA IF EXISTS config;
				DROP SCHEMA IF EXISTS active;
				DROP SCHEMA IF EXISTS feedback;
				DROP SCHEMA IF EXISTS iot;
				DROP SCHEMA IF EXISTS facilities;
				DROP SCHEMA IF EXISTS activities;
				DROP SCHEMA IF EXISTS schedule;
				DROP SCHEMA IF EXISTS education;
				DROP SCHEMA IF EXISTS users;
				DROP SCHEMA IF EXISTS auth;
			`)
			if err != nil {
				return fmt.Errorf("error dropping schemas: %w", err)
			}

			// Commit the transaction
			return tx.Commit()
		},
	)
}
