package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	GroupsTablesVersion     = "1.3.0"
	GroupsTablesDescription = "Groups schema tables"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[GroupsTablesVersion] = &Migration{
		Version:     GroupsTablesVersion,
		Description: GroupsTablesDescription,
		DependsOn:   []string{"1.2.0"}, // Depends on users tables
	}

	// Migration 1.3.0: Groups schema tables
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return groupsTablesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return groupsTablesDown(ctx, db)
		},
	)
}

// groupsTablesUp creates the groups schema tables
func groupsTablesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.3.0: Creating groups schema tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Create the groups table - primary OGS groups
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.groups (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			room_id BIGINT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating groups table: %w", err)
	}

	// Create indexes for groups
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_groups_name ON education.groups(name);
		CREATE INDEX IF NOT EXISTS idx_groups_room_id ON education.groups(room_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for groups table: %w", err)
	}

	// 2. Create the combined_groups table - special groupings
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.combined_groups (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			valid_until TIMESTAMPTZ,
			access_policy TEXT NOT NULL DEFAULT 'all',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating combined_groups table: %w", err)
	}

	// Create indexes for combined_groups
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_combined_groups_name ON education.combined_groups(name);
		CREATE INDEX IF NOT EXISTS idx_combined_groups_is_active ON education.combined_groups(is_active);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for combined_groups table: %w", err)
	}

	// 3. Create the group_teacher table - which specialists supervise which groups
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.group_teacher (
			id BIGSERIAL PRIMARY KEY,
			group_id BIGINT NOT NULL,
			teacher_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_group_teacher_group FOREIGN KEY (group_id) 
				REFERENCES education.groups(id) ON DELETE CASCADE,
			CONSTRAINT fk_group_teacher_teacher FOREIGN KEY (teacher_id) 
				REFERENCES users.teachers(id) ON DELETE CASCADE,
			CONSTRAINT uk_group_teacher UNIQUE (group_id, teacher_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating group_teacher table: %w", err)
	}

	// Create indexes for group_teacher
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_group_teacher_group_id ON education.group_teacher(group_id);
		CREATE INDEX IF NOT EXISTS idx_group_teacher_teacher_id ON education.group_teacher(teacher_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for group_teacher table: %w", err)
	}

	// 4. Create the combined_group_members table - which base groups form a combined group
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.combined_group_members (
			id BIGSERIAL PRIMARY KEY,
			combined_group_id BIGINT NOT NULL,
			group_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_combined_group_members_combined_group FOREIGN KEY (combined_group_id) 
				REFERENCES education.combined_groups(id) ON DELETE CASCADE,
			CONSTRAINT fk_combined_group_members_group FOREIGN KEY (group_id) 
				REFERENCES education.groups(id) ON DELETE CASCADE,
			CONSTRAINT uk_combined_group_members UNIQUE (combined_group_id, group_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating combined_group_members table: %w", err)
	}

	// Create indexes for combined_group_members
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_combined_group_members_combined_group_id ON education.combined_group_members(combined_group_id);
		CREATE INDEX IF NOT EXISTS idx_combined_group_members_group_id ON education.combined_group_members(group_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for combined_group_members table: %w", err)
	}

	// 5. Create the combined_group_teacher table - teachers for combined groups
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.combined_group_teacher (
			id BIGSERIAL PRIMARY KEY,
			combined_group_id BIGINT NOT NULL,
			teacher_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_combined_group_teacher_combined_group FOREIGN KEY (combined_group_id) 
				REFERENCES education.combined_groups(id) ON DELETE CASCADE,
			CONSTRAINT fk_combined_group_teacher_teacher FOREIGN KEY (teacher_id) 
				REFERENCES users.teachers(id) ON DELETE CASCADE,
			CONSTRAINT uk_combined_group_teacher UNIQUE (combined_group_id, teacher_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating combined_group_teacher table: %w", err)
	}

	// Create indexes for combined_group_teacher
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_combined_group_teacher_combined_group_id ON education.combined_group_teacher(combined_group_id);
		CREATE INDEX IF NOT EXISTS idx_combined_group_teacher_teacher_id ON education.combined_group_teacher(teacher_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for combined_group_teacher table: %w", err)
	}

	// 6. Create the group_substitution table - tracking when specialists substitute for others
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.group_substitution (
			id BIGSERIAL PRIMARY KEY,
			group_id BIGINT NOT NULL,
			regular_teacher_id BIGINT NOT NULL,
			substitute_teacher_id BIGINT NOT NULL,
			start_date DATE NOT NULL,
			end_date DATE NOT NULL,
			reason TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_group_substitution_group FOREIGN KEY (group_id) 
				REFERENCES education.groups(id) ON DELETE CASCADE,
			CONSTRAINT fk_group_substitution_regular_teacher FOREIGN KEY (regular_teacher_id) 
				REFERENCES users.teachers(id) ON DELETE CASCADE,
			CONSTRAINT fk_group_substitution_substitute_teacher FOREIGN KEY (substitute_teacher_id) 
				REFERENCES users.teachers(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating group_substitution table: %w", err)
	}

	// Create indexes for group_substitution
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_group_substitution_group_id ON education.group_substitution(group_id);
		CREATE INDEX IF NOT EXISTS idx_group_substitution_regular_teacher_id ON education.group_substitution(regular_teacher_id);
		CREATE INDEX IF NOT EXISTS idx_group_substitution_substitute_teacher_id ON education.group_substitution(substitute_teacher_id);
		CREATE INDEX IF NOT EXISTS idx_group_substitution_dates ON education.group_substitution(start_date, end_date);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for group_substitution table: %w", err)
	}

	// Create updated_at timestamp triggers
	_, err = tx.ExecContext(ctx, `
		-- Trigger for groups
		DROP TRIGGER IF EXISTS update_groups_updated_at ON education.groups;
		CREATE TRIGGER update_groups_updated_at
		BEFORE UPDATE ON education.groups
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for combined_groups
		DROP TRIGGER IF EXISTS update_combined_groups_updated_at ON education.combined_groups;
		CREATE TRIGGER update_combined_groups_updated_at
		BEFORE UPDATE ON education.combined_groups
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for group_teacher
		DROP TRIGGER IF EXISTS update_group_teacher_updated_at ON education.group_teacher;
		CREATE TRIGGER update_group_teacher_updated_at
		BEFORE UPDATE ON education.group_teacher
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for group_substitution
		DROP TRIGGER IF EXISTS update_group_substitution_updated_at ON education.group_substitution;
		CREATE TRIGGER update_group_substitution_updated_at
		BEFORE UPDATE ON education.group_substitution
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// groupsTablesDown removes the groups schema tables
func groupsTablesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.3.0: Removing groups schema tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Remove the foreign key from users.students first
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.students DROP CONSTRAINT IF EXISTS fk_students_group;
	`)
	if err != nil {
		return fmt.Errorf("error removing foreign key from students: %w", err)
	}

	// Drop tables in reverse order of dependencies
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS education.group_substitution;
		DROP TABLE IF EXISTS education.combined_group_teacher;
		DROP TABLE IF EXISTS education.combined_group_members;
		DROP TABLE IF EXISTS education.group_teacher;
		DROP TABLE IF EXISTS education.combined_groups;
		DROP TABLE IF EXISTS education.groups;
	`)
	if err != nil {
		return fmt.Errorf("error dropping groups schema tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
