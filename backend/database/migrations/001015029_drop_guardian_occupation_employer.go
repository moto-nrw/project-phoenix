package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	dropGuardianOccupationEmployerVersion     = "1.15.29"
	dropGuardianOccupationEmployerDescription = "Drop unused occupation and employer columns from guardian_profiles (DSGVO Art. 5 Datenminimierung)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dropGuardianOccupationEmployerVersion,
		Description: dropGuardianOccupationEmployerDescription,
		DependsOn:   []string{"1.15.28", "1.3.5.1"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return dropGuardianOccupationEmployer(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return restoreGuardianOccupationEmployer(ctx, db)
		},
	)
}

func dropGuardianOccupationEmployer(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.29: Dropping occupation and employer from users.guardian_profiles (DSGVO Datenminimierung)...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Safety check: warn if any data exists in these columns
	var count int
	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users.guardian_profiles WHERE occupation IS NOT NULL OR employer IS NOT NULL`,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check existing data: %w", err)
	}
	if count > 0 {
		fmt.Printf("  WARNING: %d rows have non-null occupation/employer values that will be lost\n", count)
	}

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guardian_profiles
			DROP COLUMN IF EXISTS occupation,
			DROP COLUMN IF EXISTS employer;
	`)
	if err != nil {
		return fmt.Errorf("failed to drop columns: %w", err)
	}

	fmt.Println("  Dropped occupation and employer columns successfully")
	return tx.Commit()
}

func restoreGuardianOccupationEmployer(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rollback 1.15.29: Restoring occupation and employer to users.guardian_profiles...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guardian_profiles
			ADD COLUMN IF NOT EXISTS occupation TEXT,
			ADD COLUMN IF NOT EXISTS employer TEXT;
	`)
	if err != nil {
		return fmt.Errorf("failed to restore columns: %w", err)
	}

	fmt.Println("  Restored occupation and employer columns successfully")
	return tx.Commit()
}
