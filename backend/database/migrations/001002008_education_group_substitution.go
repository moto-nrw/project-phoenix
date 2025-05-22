package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	EducationGroupSubstitutionVersion     = "1.2.8"
	EducationGroupSubstitutionDescription = "Create education.group_substitution table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[EducationGroupSubstitutionVersion] = &Migration{
		Version:     EducationGroupSubstitutionVersion,
		Description: EducationGroupSubstitutionDescription,
		DependsOn:   []string{"1.2.3", "1.2.7"}, // Depends on education.groups and users.staff
	}

	// Migration 1.2.8: Create education.group_substitution table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createEducationGroupSubstitutionTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropEducationGroupSubstitutionTable(ctx, db)
		},
	)
}

// createEducationGroupSubstitutionTable creates the education.group_substitution table
func createEducationGroupSubstitutionTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.2.8: Creating education.group_substitution table...")

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

	// Create the group_substitution table - tracking when specialists substitute for others
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.group_substitution (
			id BIGSERIAL PRIMARY KEY,
			group_id BIGINT NOT NULL,
			regular_staff_id BIGINT NOT NULL,
			substitute_staff_id BIGINT NOT NULL,
			start_date DATE NOT NULL,
			end_date DATE NOT NULL,
			reason TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_group_substitution_group FOREIGN KEY (group_id) 
				REFERENCES education.groups(id) ON DELETE CASCADE,
			CONSTRAINT fk_group_substitution_regular_staff FOREIGN KEY (regular_staff_id) 
				REFERENCES users.staff(id) ON DELETE CASCADE,
			CONSTRAINT fk_group_substitution_substitute_staff FOREIGN KEY (substitute_staff_id) 
				REFERENCES users.staff(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating group_substitution table: %w", err)
	}

	// Create indexes for group_substitution
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_group_substitution_group_id ON education.group_substitution(group_id);
		CREATE INDEX IF NOT EXISTS idx_group_substitution_regular_staff_id ON education.group_substitution(regular_staff_id);
		CREATE INDEX IF NOT EXISTS idx_group_substitution_substitute_staff_id ON education.group_substitution(substitute_staff_id);
		CREATE INDEX IF NOT EXISTS idx_group_substitution_dates ON education.group_substitution(start_date, end_date);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for group_substitution table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for group_substitution
		CREATE TRIGGER update_group_substitution_updated_at
		BEFORE UPDATE ON education.group_substitution
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating trigger for group_substitution table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropEducationGroupSubstitutionTable drops the education.group_substitution table
func dropEducationGroupSubstitutionTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.2.8: Removing education.group_substitution table...")

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
		DROP TRIGGER IF EXISTS update_group_substitution_updated_at ON education.group_substitution;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for group_substitution table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS education.group_substitution;
	`)
	if err != nil {
		return fmt.Errorf("error dropping education.group_substitution table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
