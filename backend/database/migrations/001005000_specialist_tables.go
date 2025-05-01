package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	SpecialistTablesVersion     = "1.5.0"
	SpecialistTablesDescription = "Pedagogical specialist and device tables"
)

func init() {
	// Migration 1.5.0: Pedagogical specialist and device tables
	Migrations.MustRegister(
		// Up function
		func(ctx context.Context, db *bun.DB) error {
			return specialistTablesUp(ctx, db)
		},
		// Down function
		func(ctx context.Context, db *bun.DB) error {
			return specialistTablesDown(ctx, db)
		},
	)
}

// specialistTablesUp creates the pedagogical specialist and device tables
func specialistTablesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Creating pedagogical specialist and device tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Create the rfid_cards table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS rfid_cards (
			id TEXT PRIMARY KEY,
			active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating rfid_cards table: %w", err)
	}

	// Create indexes for rfid_cards
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_rfid_cards_active ON rfid_cards(active);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for rfid_cards table: %w", err)
	}

	// 2. Create the pedagogical_specialist table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS pedagogical_specialist (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL UNIQUE,
			specialization TEXT NOT NULL,
			qualifications TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_specialist_user FOREIGN KEY (user_id) REFERENCES custom_user(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating pedagogical_specialist table: %w", err)
	}

	// Create indexes for pedagogical_specialist
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_pedagogical_specialist_user_id ON pedagogical_specialist(user_id);
		CREATE INDEX IF NOT EXISTS idx_pedagogical_specialist_specialization ON pedagogical_specialist(specialization);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for pedagogical_specialist table: %w", err)
	}

	// 3. Create the device table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS device (
			id BIGSERIAL PRIMARY KEY,
			device_id TEXT NOT NULL UNIQUE,
			device_type TEXT NOT NULL,
			name TEXT,
			location TEXT,
			status TEXT NOT NULL DEFAULT 'active',
			last_seen TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			registered_by BIGINT,
			CONSTRAINT fk_device_user FOREIGN KEY (registered_by) REFERENCES custom_user(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating device table: %w", err)
	}

	// Create indexes for device
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_device_device_id ON device(device_id);
		CREATE INDEX IF NOT EXISTS idx_device_status ON device(status);
		CREATE INDEX IF NOT EXISTS idx_device_registered_by ON device(registered_by);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for device table: %w", err)
	}

	// Create triggers for updated_at columns
	_, err = tx.ExecContext(ctx, `
		-- Trigger for rfid_cards
		DROP TRIGGER IF EXISTS update_rfid_cards_modified_at ON rfid_cards;
		CREATE TRIGGER update_rfid_cards_modified_at
		BEFORE UPDATE ON rfid_cards
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_at_column();

		-- Trigger for pedagogical_specialist
		DROP TRIGGER IF EXISTS update_pedagogical_specialist_modified_at ON pedagogical_specialist;
		CREATE TRIGGER update_pedagogical_specialist_modified_at
		BEFORE UPDATE ON pedagogical_specialist
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_at_column();
		
		-- Trigger for device
		DROP TRIGGER IF EXISTS update_device_modified_at ON device;
		CREATE TRIGGER update_device_modified_at
		BEFORE UPDATE ON device
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_at_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// specialistTablesDown removes the pedagogical specialist and device tables
func specialistTablesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back pedagogical specialist and device tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables in reverse order of dependencies
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS device;
		DROP TABLE IF EXISTS pedagogical_specialist;
		DROP TABLE IF EXISTS rfid_cards;
	`)
	if err != nil {
		return fmt.Errorf("error dropping specialist tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
