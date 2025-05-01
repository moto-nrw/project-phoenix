package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	JunctionTablesVersion     = "1.9.0"
	JunctionTablesDescription = "Junction tables for groups, specialists, and combined groups"
)

func init() {
	// Register the migration
	migration := &Migration{
		Version:     JunctionTablesVersion,
		Description: JunctionTablesDescription,
		DependsOn:   []string{"1.4.0", "1.5.0"}, // Depends on group foundation and specialist tables
		Up:          junctionTablesUp,
		Down:        junctionTablesDown,
	}

	registerMigration(migration)
}

// junctionTablesUp creates junction tables for group-specialist, combined group relationships
func junctionTablesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Creating junction tables for groups, specialists, and combined groups...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Create the group_supervisor junction table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS group_supervisor (
			id BIGSERIAL PRIMARY KEY,
			group_id BIGINT NOT NULL,
			specialist_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_group_supervisor_group FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
			CONSTRAINT fk_group_supervisor_specialist FOREIGN KEY (specialist_id) REFERENCES pedagogical_specialist(id) ON DELETE CASCADE,
			CONSTRAINT uq_group_supervisor UNIQUE(group_id, specialist_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating group_supervisor table: %w", err)
	}

	// 2. Create the combined_group_group junction table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS combined_group_group (
			id BIGSERIAL PRIMARY KEY,
			combined_group_id BIGINT NOT NULL,
			group_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_combined_group_group_combined_group FOREIGN KEY (combined_group_id) REFERENCES combined_groups(id) ON DELETE CASCADE,
			CONSTRAINT fk_combined_group_group_group FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
			CONSTRAINT uq_combined_group_group UNIQUE(combined_group_id, group_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating combined_group_group table: %w", err)
	}

	// 3. Create the combined_group_specialist junction table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS combined_group_specialist (
			id BIGSERIAL PRIMARY KEY,
			combined_group_id BIGINT NOT NULL,
			specialist_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_combined_group_specialist_combined_group FOREIGN KEY (combined_group_id) REFERENCES combined_groups(id) ON DELETE CASCADE,
			CONSTRAINT fk_combined_group_specialist_specialist FOREIGN KEY (specialist_id) REFERENCES pedagogical_specialist(id) ON DELETE CASCADE,
			CONSTRAINT uq_combined_group_specialist UNIQUE(combined_group_id, specialist_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating combined_group_specialist table: %w", err)
	}

	// Create indexes for group_supervisor
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_group_supervisor_group_id ON group_supervisor(group_id);
		CREATE INDEX IF NOT EXISTS idx_group_supervisor_specialist_id ON group_supervisor(specialist_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for group_supervisor table: %w", err)
	}

	// Create indexes for combined_group_group
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_combined_group_group_combined_group_id ON combined_group_group(combined_group_id);
		CREATE INDEX IF NOT EXISTS idx_combined_group_group_group_id ON combined_group_group(group_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for combined_group_group table: %w", err)
	}

	// Create indexes for combined_group_specialist
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_combined_group_specialist_combined_group_id ON combined_group_specialist(combined_group_id);
		CREATE INDEX IF NOT EXISTS idx_combined_group_specialist_specialist_id ON combined_group_specialist(specialist_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for combined_group_specialist table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// junctionTablesDown removes the junction tables created in junctionTablesUp
func junctionTablesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back junction tables for groups, specialists, and combined groups...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables in reverse order of dependencies
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS combined_group_specialist;
		DROP TABLE IF EXISTS combined_group_group;
		DROP TABLE IF EXISTS group_supervisor;
	`)
	if err != nil {
		return fmt.Errorf("error dropping junction tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
