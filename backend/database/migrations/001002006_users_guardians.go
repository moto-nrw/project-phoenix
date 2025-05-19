package migrations

import (
	"log"
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	UsersPersonsGuardiansVersion     = "1.2.6"
	UsersPersonsGuardiansDescription = "Link users.persons to auth.accounts_parents"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[UsersPersonsGuardiansVersion] = &Migration{
		Version:     UsersPersonsGuardiansVersion,
		Description: UsersPersonsGuardiansDescription,
		DependsOn:   []string{"1.0.9", "1.2.1"}, // Depends on auth.accounts_parents and users.persons tables
	}

	// Migration 1.2.6: Link users.persons to auth.accounts_parents
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return usersPersonsGuardiansUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return usersPersonsGuardiansDown(ctx, db)
		},
	)
}

// usersPersonsGuardiansUp creates a relationship between users.persons and auth.accounts_parents
func usersPersonsGuardiansUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.2.6: Creating users.persons_guardians relationship table...")

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

	// Create the persons_guardians table - links persons to their guardian accounts
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.persons_guardians (
			id BIGSERIAL PRIMARY KEY,
			person_id BIGINT NOT NULL,
			guardian_account_id BIGINT NOT NULL,
			relationship_type TEXT NOT NULL, -- e.g., 'parent', 'guardian', 'relative'
			is_primary BOOLEAN NOT NULL DEFAULT FALSE,
			permissions JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_persons_guardians_person FOREIGN KEY (person_id) REFERENCES users.persons(id) ON DELETE CASCADE,
			CONSTRAINT fk_persons_guardians_guardian FOREIGN KEY (guardian_account_id) REFERENCES auth.accounts_parents(id) ON DELETE CASCADE,
			CONSTRAINT unique_person_guardian_relationship UNIQUE (person_id, guardian_account_id, relationship_type)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating persons_parents table: %w", err)
	}

	// Create indexes for persons_guardians
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_persons_guardians_person_id ON users.persons_guardians(person_id);
		CREATE INDEX IF NOT EXISTS idx_persons_guardians_guardian_account_id ON users.persons_guardians(guardian_account_id);
		CREATE INDEX IF NOT EXISTS idx_persons_guardians_relationship_type ON users.persons_guardians(relationship_type);
		CREATE INDEX IF NOT EXISTS idx_persons_guardians_is_primary ON users.persons_guardians(is_primary);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for persons_parents table: %w", err)
	}

	// Create trigger that enforces only one primary relationship per person-guardian pair
	_, err = tx.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION users.enforce_single_primary_guardian()
		RETURNS TRIGGER AS $$
		BEGIN
			-- If this is being set as primary
			IF NEW.is_primary = TRUE THEN
				-- Set any other relationships between same person and guardian to non-primary
				UPDATE users.persons_guardians
				SET is_primary = FALSE
				WHERE person_id = NEW.person_id
				AND guardian_account_id = NEW.guardian_account_id
				AND id != NEW.id;
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;

		DROP TRIGGER IF EXISTS enforce_single_primary_guardian_trigger ON users.persons_guardians;
		CREATE TRIGGER enforce_single_primary_guardian_trigger
		BEFORE INSERT OR UPDATE OF is_primary ON users.persons_guardians
		FOR EACH ROW
		WHEN (NEW.is_primary = TRUE)
		EXECUTE FUNCTION users.enforce_single_primary_guardian();
	`)
	if err != nil {
		return fmt.Errorf("error creating single primary parent trigger: %w", err)
	}

	// Create updated_at timestamp trigger
	_, err = tx.ExecContext(ctx, `
		-- Trigger for persons_guardians
		DROP TRIGGER IF EXISTS update_persons_guardians_updated_at ON users.persons_guardians;
		CREATE TRIGGER update_persons_guardians_updated_at
		BEFORE UPDATE ON users.persons_guardians
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// usersPersonsGuardiansDown removes the users.persons_guardians relationship table
func usersPersonsGuardiansDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.2.6: Removing users.persons_guardians relationship table...")

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

	// Drop the triggers first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS enforce_single_primary_guardian_trigger ON users.persons_guardians;
		DROP TRIGGER IF EXISTS update_persons_guardians_updated_at ON users.persons_guardians;
		DROP FUNCTION IF EXISTS users.enforce_single_primary_guardian();
	`)
	if err != nil {
		return fmt.Errorf("error dropping triggers and functions: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS users.persons_guardians CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users.persons_parents table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
