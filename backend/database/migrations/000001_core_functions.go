package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	// Migration 0: Core database functions
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 0: Setting up core database functions...")

			// Begin a transaction for atomicity
			tx, err := db.BeginTx(ctx, &sql.TxOptions{})
			if err != nil {
				return fmt.Errorf("failed to begin transaction: %w", err)
			}
			defer tx.Rollback()

			// Add any PostgreSQL core functions needed
			// For example, utility functions for auditing, etc.
			_, err = tx.ExecContext(ctx, `
				-- Add any core functions here
				-- For example:
				CREATE OR REPLACE FUNCTION update_modified_column()
				RETURNS TRIGGER AS $$
				BEGIN
					NEW.updated_at = now();
					RETURN NEW;
				END;
				$$ language 'plpgsql';
			`)
			if err != nil {
				return fmt.Errorf("error creating core functions: %w", err)
			}

			// Commit the transaction
			return tx.Commit()
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 0: Removing core database functions...")

			// Begin a transaction for atomicity
			tx, err := db.BeginTx(ctx, &sql.TxOptions{})
			if err != nil {
				return fmt.Errorf("failed to begin transaction: %w", err)
			}
			defer tx.Rollback()

			// Drop any functions created in Up
			_, err = tx.ExecContext(ctx, `
				-- Drop functions created in Up
				DROP FUNCTION IF EXISTS update_modified_column();
			`)
			if err != nil {
				return fmt.Errorf("error dropping core functions: %w", err)
			}

			// Commit the transaction
			return tx.Commit()
		},
	)
}
