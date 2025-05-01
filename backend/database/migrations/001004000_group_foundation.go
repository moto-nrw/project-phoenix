package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	GroupFoundationVersion     = "1.4.0"
	GroupFoundationDescription = "Group foundation tables"
)

func init() {
	// Migration 1.4.0: Group foundation tables
	Migrations.MustRegister(
		// Up function
		func(ctx context.Context, db *bun.DB) error {
			return groupFoundationUp(ctx, db)
		},
		// Down function
		func(ctx context.Context, db *bun.DB) error {
			return groupFoundationDown(ctx, db)
		},
	)
}

// groupFoundationUp creates the group foundation tables
func groupFoundationUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Creating group foundation tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Create the groups table (WITHOUT representative_id FK initially)
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS groups (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			room_id BIGINT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_groups_room FOREIGN KEY (room_id) REFERENCES rooms(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating groups table: %w", err)
	}

	// Create indexes for groups
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name);
		CREATE INDEX IF NOT EXISTS idx_groups_room_id ON groups(room_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for groups table: %w", err)
	}

	// 2. Create the combined_groups table (WITHOUT specific_group_id FK initially)
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS combined_groups (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			valid_until TIMESTAMPTZ,
			access_policy TEXT NOT NULL DEFAULT 'all',
			modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating combined_groups table: %w", err)
	}

	// Create indexes for combined_groups
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_combined_groups_name ON combined_groups(name);
		CREATE INDEX IF NOT EXISTS idx_combined_groups_is_active ON combined_groups(is_active);
		CREATE INDEX IF NOT EXISTS idx_combined_groups_valid_until ON combined_groups(valid_until);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for combined_groups table: %w", err)
	}

	// 3. Create the group_supervisors junction table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS group_supervisors (
			id BIGSERIAL PRIMARY KEY,
			group_id BIGINT NOT NULL,
			specialist_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_group_supervisors_group FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
			CONSTRAINT group_supervisors_unique UNIQUE (group_id, specialist_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating group_supervisors table: %w", err)
	}

	// Create indexes for group_supervisors
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_group_supervisors_group_id ON group_supervisors(group_id);
		CREATE INDEX IF NOT EXISTS idx_group_supervisors_specialist_id ON group_supervisors(specialist_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for group_supervisors table: %w", err)
	}

	// 4. Create the combined_group_groups junction table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS combined_group_groups (
			id BIGSERIAL PRIMARY KEY,
			combined_group_id BIGINT NOT NULL,
			group_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_combined_group_groups_combined_group FOREIGN KEY (combined_group_id) REFERENCES combined_groups(id) ON DELETE CASCADE,
			CONSTRAINT fk_combined_group_groups_group FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
			CONSTRAINT combined_group_groups_unique UNIQUE (combined_group_id, group_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating combined_group_groups table: %w", err)
	}

	// Create indexes for combined_group_groups
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_combined_group_groups_combined_group_id ON combined_group_groups(combined_group_id);
		CREATE INDEX IF NOT EXISTS idx_combined_group_groups_group_id ON combined_group_groups(group_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for combined_group_groups table: %w", err)
	}

	// 5. Create the combined_group_specialists junction table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS combined_group_specialists (
			id BIGSERIAL PRIMARY KEY,
			combined_group_id BIGINT NOT NULL,
			specialist_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_combined_group_specialists_combined_group FOREIGN KEY (combined_group_id) REFERENCES combined_groups(id) ON DELETE CASCADE,
			CONSTRAINT combined_group_specialists_unique UNIQUE (combined_group_id, specialist_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating combined_group_specialists table: %w", err)
	}

	// Create indexes for combined_group_specialists
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_combined_group_specialists_combined_group_id ON combined_group_specialists(combined_group_id);
		CREATE INDEX IF NOT EXISTS idx_combined_group_specialists_specialist_id ON combined_group_specialists(specialist_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for combined_group_specialists table: %w", err)
	}

	// Create triggers for updated_at columns
	_, err = tx.ExecContext(ctx, `
		-- Function to update updated_at column already created in previous migration
		
		-- Trigger for groups
		DROP TRIGGER IF EXISTS update_groups_modified_at ON groups;
		CREATE TRIGGER update_groups_modified_at
		BEFORE UPDATE ON groups
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
		
		-- Trigger for combined_groups
		DROP TRIGGER IF EXISTS update_combined_groups_modified_at ON combined_groups;
		CREATE TRIGGER update_combined_groups_modified_at
		BEFORE UPDATE ON combined_groups
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// groupFoundationDown removes the group foundation tables
func groupFoundationDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back group foundation tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables in reverse order of dependencies
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS combined_group_specialists;
		DROP TABLE IF EXISTS combined_group_groups;
		DROP TABLE IF EXISTS group_supervisors;
		DROP TABLE IF EXISTS combined_groups;
		DROP TABLE IF EXISTS groups;
	`)
	if err != nil {
		return fmt.Errorf("error dropping group foundation tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
