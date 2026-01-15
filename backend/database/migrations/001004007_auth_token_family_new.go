package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AuthTokenFamilyVersion     = "1.4.7"
	AuthTokenFamilyDescription = "Add token family tracking for detecting token theft"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AuthTokenFamilyVersion] = &Migration{
		Version:     AuthTokenFamilyVersion,
		Description: AuthTokenFamilyDescription,
		DependsOn:   []string{"1.0.2"}, // Depends on auth.tokens
	}

	// Register the migration
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addTokenFamilyTracking(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeTokenFamilyTracking(ctx, db)
		},
	)
}

// addTokenFamilyTracking adds token family tracking columns
func addTokenFamilyTracking(ctx context.Context, db *bun.DB) error {
	LogMigration(AuthTokenFamilyVersion, "Adding token family tracking columns...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			logRollbackError(err)
		}
	}()

	// Add token family tracking columns
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.tokens ADD COLUMN IF NOT EXISTS family_id VARCHAR(100);
		ALTER TABLE auth.tokens ADD COLUMN IF NOT EXISTS generation INTEGER DEFAULT 0;
	`)
	if err != nil {
		return fmt.Errorf("error adding token family columns: %w", err)
	}

	// Create index for efficient family lookups
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_tokens_family_id ON auth.tokens(family_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating index for family_id: %w", err)
	}

	// Add comments
	_, err = tx.ExecContext(ctx, `
		COMMENT ON COLUMN auth.tokens.family_id IS 'Token family identifier for tracking refresh token lineage';
		COMMENT ON COLUMN auth.tokens.generation IS 'Generation counter within the token family to detect token theft';
	`)
	if err != nil {
		return fmt.Errorf("error adding comments to token family columns: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// removeTokenFamilyTracking removes token family tracking columns
func removeTokenFamilyTracking(ctx context.Context, db *bun.DB) error {
	LogMigration(AuthTokenFamilyVersion, "Rolling back: Removing token family tracking columns...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			logRollbackError(err)
		}
	}()

	// Drop the index
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS auth.idx_tokens_family_id;
	`)
	if err != nil {
		return fmt.Errorf("error dropping index: %w", err)
	}

	// Remove the columns
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.tokens DROP COLUMN IF EXISTS family_id;
		ALTER TABLE auth.tokens DROP COLUMN IF EXISTS generation;
	`)
	if err != nil {
		return fmt.Errorf("error dropping token family columns: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
