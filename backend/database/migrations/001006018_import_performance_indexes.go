package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	ImportPerformanceIndexesVersion     = "1.6.18.1"
	ImportPerformanceIndexesDescription = "Add performance indexes for CSV import operations"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ImportPerformanceIndexesVersion] = &Migration{
		Version:     ImportPerformanceIndexesVersion,
		Description: ImportPerformanceIndexesDescription,
		DependsOn:   []string{"1.0.1", "1.1.1", "1.2.1"}, // Depends on users, education tables
	}

	// Register the migration
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createImportPerformanceIndexes(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropImportPerformanceIndexes(ctx, db)
		},
	)
}

// createImportPerformanceIndexes creates indexes for import performance
func createImportPerformanceIndexes(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.18: Creating performance indexes for imports...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in up migration: %v", err)
		}
	}()

	// 1. Guardian email index (case-insensitive for deduplication)
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_guardian_profiles_email_lower
		ON users.guardian_profiles(LOWER(email))
		WHERE email IS NOT NULL
	`)
	if err != nil {
		return fmt.Errorf("error creating guardian email index: %w", err)
	}

	// 2. Group name index (case-insensitive for fuzzy matching)
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_groups_name_lower
		ON education.groups(LOWER(name))
	`)
	if err != nil {
		return fmt.Errorf("error creating group name index: %w", err)
	}

	// 3. Room name index (case-insensitive for fuzzy matching)
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_rooms_name_lower
		ON facilities.rooms(LOWER(name))
	`)
	if err != nil {
		return fmt.Errorf("error creating room name index: %w", err)
	}

	// 4. Person name index for duplicate detection
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_persons_name_lower
		ON users.persons(LOWER(first_name), LOWER(last_name))
	`)
	if err != nil {
		return fmt.Errorf("error creating person name index: %w", err)
	}

	// 5. Student school class index
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_students_school_class_lower
		ON users.students(LOWER(school_class))
	`)
	if err != nil {
		return fmt.Errorf("error creating school class index: %w", err)
	}

	// Add comments for documentation (indexes are in the same schema as their tables)
	_, err = tx.ExecContext(ctx, `
		COMMENT ON INDEX users.idx_guardian_profiles_email_lower IS 'Case-insensitive index for guardian email deduplication during import';
		COMMENT ON INDEX education.idx_groups_name_lower IS 'Case-insensitive index for group name resolution during import';
		COMMENT ON INDEX facilities.idx_rooms_name_lower IS 'Case-insensitive index for room name resolution during import';
		COMMENT ON INDEX users.idx_persons_name_lower IS 'Case-insensitive index for person name matching during duplicate detection';
		COMMENT ON INDEX users.idx_students_school_class_lower IS 'Case-insensitive index for school class filtering';
	`)
	if err != nil {
		return fmt.Errorf("error adding index comments: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Println("✓ Migration 1.6.18: Performance indexes created successfully")
	return nil
}

// dropImportPerformanceIndexes drops the import performance indexes (rollback)
func dropImportPerformanceIndexes(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.18: Rolling back performance indexes...")

	// IMPORTANT: Indexes are in the same schema as their tables, so we need schema prefixes
	_, err := db.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_guardian_profiles_email_lower;
		DROP INDEX IF EXISTS education.idx_groups_name_lower;
		DROP INDEX IF EXISTS facilities.idx_rooms_name_lower;
		DROP INDEX IF EXISTS users.idx_persons_name_lower;
		DROP INDEX IF EXISTS users.idx_students_school_class_lower;
	`)
	if err != nil {
		return fmt.Errorf("error dropping performance indexes: %w", err)
	}

	log.Println("✓ Migration 1.6.18: Performance indexes dropped successfully")
	return nil
}
