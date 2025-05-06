package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	FacilitiesVisitsVersion     = "1.4.3"
	FacilitiesVisitsDescription = "Create facilities.visits table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[FacilitiesVisitsVersion] = &Migration{
		Version:     FacilitiesVisitsVersion,
		Description: FacilitiesVisitsDescription,
		DependsOn:   []string{"1.4.1", "1.2.1"}, // Depends on room_occupancy and persons
	}

	// Migration 1.4.3: Create facilities.visits table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createFacilitiesVisitsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropFacilitiesVisitsTable(ctx, db)
		},
	)
}

// createFacilitiesVisitsTable creates the facilities.visits table
func createFacilitiesVisitsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.3: Creating facilities.visits table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the facilities.visits table - tracks all persons (students, teachers, guests)
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS facilities.visits (
			id BIGSERIAL PRIMARY KEY,
			person_id BIGINT NOT NULL REFERENCES users.persons(id) ON DELETE CASCADE,
			room_occupancy_id BIGINT NOT NULL REFERENCES facilities.room_occupancy(id) ON DELETE CASCADE,
			entry_time TIMESTAMPTZ NOT NULL,
			exit_time TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating facilities.visits table: %w", err)
	}

	// Create indexes for visits
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX idx_visits_person_id ON facilities.visits(person_id);
		CREATE INDEX idx_visits_room_occupancy_id ON facilities.visits(room_occupancy_id);
		CREATE INDEX idx_visits_entry_time ON facilities.visits(entry_time);
		CREATE INDEX idx_visits_exit_time ON facilities.visits(exit_time) WHERE exit_time IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for visits table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for visits
		CREATE TRIGGER update_facilities_visits_updated_at
		BEFORE UPDATE ON facilities.visits
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating trigger for visits table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropFacilitiesVisitsTable drops the facilities.visits table
func dropFacilitiesVisitsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.4.3: Removing facilities.visits table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_facilities_visits_updated_at ON facilities.visits;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for visits table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS facilities.visits;
	`)
	if err != nil {
		return fmt.Errorf("error dropping facilities.visits table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
