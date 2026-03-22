package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	seedSchulhofVersion     = "1.15.14"
	seedSchulhofDescription = "Seed Schulhof room and category for all tenants that are missing them"
)

func init() {
	MigrationRegistry[seedSchulhofVersion] = &Migration{
		Version:     seedSchulhofVersion,
		Description: seedSchulhofDescription,
		DependsOn:   []string{"1.15.13"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return seedSchulhofInfrastructureUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return seedSchulhofInfrastructureDown(ctx, db)
		},
	)
}

func seedSchulhofInfrastructureUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.14: Seeding Schulhof room and category for tenants missing them...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Step 1: Create Schulhof room for tenants that don't have one
	roomResult, err := tx.ExecContext(ctx, `
		INSERT INTO facilities.rooms (name, capacity, category, color, tenant_id)
		SELECT 'Schulhof', 300, 'Schulhof', '#7ED321', s.id
		FROM platform.schools s
		WHERE s.active = true
		  AND NOT EXISTS (
			SELECT 1 FROM facilities.rooms r
			WHERE r.tenant_id = s.id AND r.name = 'Schulhof'
		  )
	`)
	if err != nil {
		return fmt.Errorf("error creating Schulhof rooms: %w", err)
	}
	roomRows, _ := roomResult.RowsAffected()
	fmt.Printf("  Created Schulhof room for %d tenants\n", roomRows)

	// Step 2: Create Schulhof activity category for tenants that don't have one
	catResult, err := tx.ExecContext(ctx, `
		INSERT INTO activities.categories (name, description, color, tenant_id)
		SELECT 'Schulhof', 'Outdoor playground activities', '#7ED321', s.id
		FROM platform.schools s
		WHERE s.active = true
		  AND NOT EXISTS (
			SELECT 1 FROM activities.categories c
			WHERE c.tenant_id = s.id AND c.name = 'Schulhof'
		  )
	`)
	if err != nil {
		return fmt.Errorf("error creating Schulhof categories: %w", err)
	}
	catRows, _ := catResult.RowsAffected()
	fmt.Printf("  Created Schulhof category for %d tenants\n", catRows)

	fmt.Println("Migration 1.15.14: Done")
	return tx.Commit()
}

func seedSchulhofInfrastructureDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.14: Removing seeded Schulhof infrastructure...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Only delete Schulhof rooms/categories that have no associated activity groups.
	// If an activity group was created on top of the seeded data, leave it intact.
	_, err = tx.ExecContext(ctx, `
		DELETE FROM facilities.rooms r
		WHERE r.name = 'Schulhof'
		  AND NOT EXISTS (
			SELECT 1 FROM activities.groups g WHERE g.planned_room_id = r.id
		  )
	`)
	if err != nil {
		return fmt.Errorf("error removing Schulhof rooms: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DELETE FROM activities.categories c
		WHERE c.name = 'Schulhof'
		  AND NOT EXISTS (
			SELECT 1 FROM activities.groups g WHERE g.category_id = c.id
		  )
	`)
	if err != nil {
		return fmt.Errorf("error removing Schulhof categories: %w", err)
	}

	return tx.Commit()
}
