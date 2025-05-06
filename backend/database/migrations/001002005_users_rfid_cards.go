package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	UsersRFIDCardsVersion     = "1.2.5"
	UsersRFIDCardsDescription = "Users RFID cards table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[UsersRFIDCardsVersion] = &Migration{
		Version:     UsersRFIDCardsVersion,
		Description: UsersRFIDCardsDescription,
		DependsOn:   []string{"1.1.0"}, // Only depends on infrastructure
	}

	// Migration 1.2.5: Users RFID cards table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return usersRFIDCardsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return usersRFIDCardsDown(ctx, db)
		},
	)
}

// usersRFIDCardsUp creates the users.rfid_cards table
func usersRFIDCardsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.2.5: Creating users.rfid_cards table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the rfid_cards table - for physical tracking
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.rfid_cards (
			id TEXT PRIMARY KEY,
			active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating rfid_cards table: %w", err)
	}

	// Create updated_at timestamp trigger
	_, err = tx.ExecContext(ctx, `
		-- Trigger for rfid_cards
		DROP TRIGGER IF EXISTS update_rfid_cards_updated_at ON users.rfid_cards;
		CREATE TRIGGER update_rfid_cards_updated_at
		BEFORE UPDATE ON users.rfid_cards
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// usersRFIDCardsDown removes the users.rfid_cards table
func usersRFIDCardsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.2.5: Removing users.rfid_cards table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop the rfid_cards table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS users.rfid_cards;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users.rfid_cards table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
