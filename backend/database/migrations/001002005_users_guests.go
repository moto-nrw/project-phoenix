package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	UsersGuestsVersion     = "1.2.5"
	UsersGuestsDescription = "Users guests table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[UsersGuestsVersion] = &Migration{
		Version:     UsersGuestsVersion,
		Description: UsersGuestsDescription,
		DependsOn:   []string{"1.2.3"}, // Depends on staff table
	}

	// Migration 1.2.4: Users guests table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return usersGuestsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return usersGuestsDown(ctx, db)
		},
	)
}

// usersGuestsUp creates the users.guests table
func usersGuestsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.2.5: Creating users.guests table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the guests table - for guest instructors
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.guests (
			id BIGSERIAL PRIMARY KEY,
			staff_id BIGINT NOT NULL UNIQUE,
			organization TEXT,
			contact_email TEXT,
			contact_phone TEXT,
			activity_expertise TEXT NOT NULL,
			start_date DATE,
			end_date DATE,
			notes TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_guests_staff FOREIGN KEY (staff_id) 
				REFERENCES users.staff(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating guests table: %w", err)
	}

	// Create indexes for guests
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_guests_person_id ON users.guests(person_id);
		CREATE INDEX IF NOT EXISTS idx_guests_organization ON users.guests(organization);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for guests table: %w", err)
	}

	// Create updated_at timestamp trigger
	_, err = tx.ExecContext(ctx, `
		-- Trigger for guests
		DROP TRIGGER IF EXISTS update_guests_updated_at ON users.guests;
		CREATE TRIGGER update_guests_updated_at
		BEFORE UPDATE ON users.guests
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// usersGuestsDown removes the users.guests table
func usersGuestsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.2.5: Removing users.guests table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop the guests table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS users.guests;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users.guests table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
