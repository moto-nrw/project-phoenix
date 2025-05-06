package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	UsersStudentsVersion     = "1.3.5"
	UsersStudentsDescription = "Users students table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[UsersStudentsVersion] = &Migration{
		Version:     UsersStudentsVersion,
		Description: UsersStudentsDescription,
		DependsOn:   []string{"1.2.1", "1.3.0"}, // Depends on persons table AND groups table
	}

	// Migration 1.3.5: Users students table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return usersStudentsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return usersStudentsDown(ctx, db)
		},
	)
}

// usersStudentsUp creates the users.students table
func usersStudentsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.3.5: Creating users.students table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the students table - for students/children
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.students (
			id BIGSERIAL PRIMARY KEY,
			person_id BIGINT NOT NULL UNIQUE,
			school_class TEXT NOT NULL,
			bus BOOLEAN NOT NULL DEFAULT FALSE,
			guardian_name TEXT NOT NULL,
			guardian_contact TEXT NOT NULL,
			in_house BOOLEAN NOT NULL DEFAULT FALSE,
			wc BOOLEAN NOT NULL DEFAULT FALSE,
			school_yard BOOLEAN NOT NULL DEFAULT FALSE,
			group_id BIGINT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_students_person FOREIGN KEY (person_id) 
				REFERENCES users.persons(id) ON DELETE CASCADE,
			CONSTRAINT fk_students_group FOREIGN KEY (group_id)
				REFERENCES education.groups(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating students table: %w", err)
	}

	// Create indexes for students
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_students_person_id ON users.students(person_id);
		CREATE INDEX IF NOT EXISTS idx_students_school_class ON users.students(school_class);
		CREATE INDEX IF NOT EXISTS idx_students_group_id ON users.students(group_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for students table: %w", err)
	}

	// Create updated_at timestamp trigger
	_, err = tx.ExecContext(ctx, `
		-- Trigger for students
		DROP TRIGGER IF EXISTS update_students_updated_at ON users.students;
		CREATE TRIGGER update_students_updated_at
		BEFORE UPDATE ON users.students
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// usersStudentsDown removes the users.students table
func usersStudentsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.3.5: Removing users.students table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop the students table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS users.students;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users.students table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
