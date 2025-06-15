package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/uptrace/bun"
)

const (
	SeedActivityCategoriesVersion     = "1.6.4"
	SeedActivityCategoriesDescription = "Seed default activity categories"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[SeedActivityCategoriesVersion] = &Migration{
		Version:     SeedActivityCategoriesVersion,
		Description: SeedActivityCategoriesDescription,
		DependsOn:   []string{"1.3.1"}, // Depends on activities.categories table
	}

	// Migration 1.6.4: Seed default activity categories
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return seedDefaultActivityCategories(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeSeededActivityCategories(ctx, db)
		},
	)
}

// seedDefaultActivityCategories inserts default activity categories
func seedDefaultActivityCategories(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.4: Seeding default activity categories...")

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

	// Define default categories
	categories := []struct {
		name        string
		description string
		color       string
	}{
		{"Sport", "Sportliche Aktivitäten für Kinder", "#7ED321"},
		{"Kunst & Basteln", "Kreative Aktivitäten und Handwerken", "#F5A623"},
		{"Musik", "Musikalische Aktivitäten und Gesang", "#BD10E0"},
		{"Spiele", "Brett-, Karten- und Gruppenspiele", "#50E3C2"},
		{"Lesen", "Leseförderung und Literatur", "#B8E986"},
		{"Hausaufgabenhilfe", "Unterstützung bei den Hausaufgaben", "#4A90E2"},
		{"Natur & Forschen", "Naturerkundung und einfache Experimente", "#7ED321"},
		{"Computer", "Grundlagen im Umgang mit dem Computer", "#9013FE"},
		{"Gruppenraum", "Aktivitäten im Gruppenraum", "#FF6900"},
	}

	// Insert categories with ON CONFLICT DO NOTHING to handle re-runs
	for _, cat := range categories {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO activities.categories (name, description, color, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (name) DO NOTHING
		`, cat.name, cat.description, cat.color, time.Now(), time.Now())
		
		if err != nil {
			return fmt.Errorf("failed to insert category %s: %w", cat.name, err)
		}
	}

	// Commit the transaction
	return tx.Commit()
}

// removeSeededActivityCategories removes the default seeded categories
func removeSeededActivityCategories(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.6.4: Removing seeded activity categories...")

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

	// Remove only the default categories we added
	defaultCategories := []string{
		"Sport",
		"Kunst & Basteln",
		"Musik",
		"Spiele",
		"Lesen",
		"Hausaufgabenhilfe",
		"Natur & Forschen",
		"Computer",
		"Gruppenraum",
	}

	for _, name := range defaultCategories {
		// Check if there are any activity groups using this category
		var count int
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*) 
			FROM activities.groups ag
			JOIN activities.categories ac ON ag.category_id = ac.id
			WHERE ac.name = ?
		`, name).Scan(&count)
		
		if err != nil {
			return fmt.Errorf("failed to check category usage for %s: %w", name, err)
		}
		
		if count > 0 {
			log.Printf("WARNING: Category '%s' is in use by %d activity groups. Skipping removal.", name, count)
			continue
		}
		
		// Delete the category if it's not in use
		_, err = tx.ExecContext(ctx, `
			DELETE FROM activities.categories 
			WHERE name = ?
		`, name)
		
		if err != nil {
			return fmt.Errorf("failed to delete category %s: %w", name, err)
		}
	}

	// Commit the transaction
	return tx.Commit()
}