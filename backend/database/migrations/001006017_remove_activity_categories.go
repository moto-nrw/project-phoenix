package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

var Dependencies001006017 = []string{
	"001006006_update_activity_categories",
}

var Rollback001006017 = `
-- Restore removed categories
INSERT INTO activities.categories (name, description, color, created_at, updated_at)
VALUES
	('Gruppenraum', 'Aktivitäten im Gruppenraum', '#FF6900', NOW(), NOW()),
	('Hausaufgaben', 'Unterstützung bei den Hausaufgaben', '#4A90E2', NOW(), NOW())
ON CONFLICT (name) DO NOTHING;

-- Remove Sonstiges category
DELETE FROM activities.categories WHERE name = 'Sonstiges';
`

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Println("Migration 001006017: Removing Gruppenraum and Hausaufgaben categories, adding Sonstiges...")

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

		// Add Sonstiges category first
		_, err = tx.ExecContext(ctx, `
			INSERT INTO activities.categories (name, description, color, created_at, updated_at)
			VALUES (?, ?, ?, NOW(), NOW())
			ON CONFLICT (name) DO NOTHING
		`, "Sonstiges", "Sonstige Aktivitäten", "#808080")
		if err != nil {
			return fmt.Errorf("failed to add Sonstiges category: %w", err)
		}

		// Get the ID of the Sonstiges category
		var sonstigesID int64
		err = tx.QueryRowContext(ctx, `SELECT id FROM activities.categories WHERE name = 'Sonstiges'`).Scan(&sonstigesID)
		if err != nil {
			return fmt.Errorf("failed to get Sonstiges category ID: %w", err)
		}

		// Update activities using Gruppenraum to use Sonstiges
		_, err = tx.ExecContext(ctx, `
			UPDATE activities.groups
			SET category_id = ?
			WHERE category_id IN (
				SELECT id FROM activities.categories WHERE name = 'Gruppenraum'
			)
		`, sonstigesID)
		if err != nil {
			return fmt.Errorf("failed to migrate Gruppenraum activities: %w", err)
		}

		// Update activities using Hausaufgaben to use Sonstiges
		_, err = tx.ExecContext(ctx, `
			UPDATE activities.groups
			SET category_id = ?
			WHERE category_id IN (
				SELECT id FROM activities.categories WHERE name = 'Hausaufgaben'
			)
		`, sonstigesID)
		if err != nil {
			return fmt.Errorf("failed to migrate Hausaufgaben activities: %w", err)
		}

		// Delete the Gruppenraum and Hausaufgaben categories
		_, err = tx.ExecContext(ctx, `
			DELETE FROM activities.categories
			WHERE name IN ('Gruppenraum', 'Hausaufgaben')
		`)
		if err != nil {
			return fmt.Errorf("failed to delete categories: %w", err)
		}

		fmt.Println("Successfully removed Gruppenraum and Hausaufgaben categories, added Sonstiges")

		// Commit the transaction
		return tx.Commit()
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.Exec(Rollback001006017)
		return err
	})
}
