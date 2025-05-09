package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	UsersTeachersVersion     = "1.2.3"
	UsersTeachersDescription = "Users teachers table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[UsersTeachersVersion] = &Migration{
		Version:     UsersTeachersVersion,
		Description: UsersTeachersDescription,
		DependsOn:   []string{"1.2.1"}, // Depends on persons table
	}

	// Migration 1.2.3: Users teachers table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return usersTeachersUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return usersTeachersDown(ctx, db)
		},
	)
}

// usersTeachersUp creates the users.teachers table
func usersTeachersUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.2.3: Creating users.teachers table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the teachers table - for pedagogical specialists
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.teachers (
			id BIGSERIAL PRIMARY KEY,
			person_id BIGINT NOT NULL UNIQUE,
			specialization TEXT NOT NULL,
			role TEXT,
			qualifications TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_teachers_person FOREIGN KEY (person_id) 
				REFERENCES users.persons(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating teachers table: %w", err)
	}

	// Create indexes for teachers
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_teachers_person_id ON users.teachers(person_id);
		CREATE INDEX IF NOT EXISTS idx_teachers_specialization ON users.teachers(specialization);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for teachers table: %w", err)
	}

	// Create updated_at timestamp trigger
	_, err = tx.ExecContext(ctx, `
		-- Trigger for teachers
		DROP TRIGGER IF EXISTS update_teachers_updated_at ON users.teachers;
		CREATE TRIGGER update_teachers_updated_at
		BEFORE UPDATE ON users.teachers
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// usersTeachersDown removes the users.teachers table
func usersTeachersDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.2.3: Removing users.teachers table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop the teachers table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS users.teachers;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users.teachers table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
