package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	RoomComplexTablesVersion     = "1.8.0"
	RoomComplexTablesDescription = "Room occupancy, history, and visit tables"
)

func init() {
	// Register the migration
	migration := &Migration{
		Version:     RoomComplexTablesVersion,
		Description: RoomComplexTablesDescription,
		DependsOn:   []string{"1.3.0", "1.7.0"}, // Depends on user foundation and student tables
		Up:          roomComplexTablesUp,
		Down:        roomComplexTablesDown,
	}

	registerMigration(migration)
}

// roomComplexTablesUp creates the room_occupancy, room_history, and visit tables
func roomComplexTablesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Creating room occupancy, history, and visit tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Create the room_occupancy table (without ag_id and group_id FKs initially)
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS room_occupancy (
			id BIGSERIAL PRIMARY KEY,
			device_id TEXT NOT NULL UNIQUE,
			room_id BIGINT NOT NULL,
			timespan_id BIGINT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			max_capacity INTEGER NOT NULL DEFAULT 0,
			current_occupancy INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_room_occupancy_room FOREIGN KEY (room_id) REFERENCES rooms(id) ON DELETE CASCADE,
			CONSTRAINT fk_room_occupancy_timespan FOREIGN KEY (timespan_id) REFERENCES timespan(id) ON DELETE RESTRICT
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating room_occupancy table: %w", err)
	}

	// 2. Create the room_history table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS room_history (
			id BIGSERIAL PRIMARY KEY,
			room_id BIGINT NOT NULL,
			ag_name TEXT NOT NULL,
			day DATE NOT NULL,
			timespan_id BIGINT NOT NULL,
			ag_category_id BIGINT,
			supervisor_id BIGINT NOT NULL,
			max_participant INTEGER NOT NULL DEFAULT 0,
			group_id BIGINT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_room_history_room FOREIGN KEY (room_id) REFERENCES rooms(id) ON DELETE CASCADE,
			CONSTRAINT fk_room_history_timespan FOREIGN KEY (timespan_id) REFERENCES timespan(id) ON DELETE RESTRICT,
			CONSTRAINT fk_room_history_ag_category FOREIGN KEY (ag_category_id) REFERENCES ag_category(id) ON DELETE SET NULL,
			CONSTRAINT fk_room_history_supervisor FOREIGN KEY (supervisor_id) REFERENCES pedagogical_specialist(id) ON DELETE RESTRICT,
			CONSTRAINT fk_room_history_group FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating room_history table: %w", err)
	}

	// 3. Create the visit table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS visit (
			id BIGSERIAL PRIMARY KEY,
			student_id BIGINT NOT NULL,
			room_occupancy_id BIGINT NOT NULL,
			entry_time TIMESTAMPTZ NOT NULL,
			exit_time TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_visit_student FOREIGN KEY (student_id) REFERENCES student(id) ON DELETE CASCADE,
			CONSTRAINT fk_visit_room_occupancy FOREIGN KEY (room_occupancy_id) REFERENCES room_occupancy(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating visit table: %w", err)
	}

	// 4. Create the room_occupancy_supervisors junction table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS room_occupancy_supervisors (
			id BIGSERIAL PRIMARY KEY,
			room_occupancy_id BIGINT NOT NULL,
			supervisor_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_room_occupancy_supervisors_room_occupancy FOREIGN KEY (room_occupancy_id) REFERENCES room_occupancy(id) ON DELETE CASCADE,
			CONSTRAINT fk_room_occupancy_supervisors_supervisor FOREIGN KEY (supervisor_id) REFERENCES pedagogical_specialist(id) ON DELETE CASCADE,
			CONSTRAINT uq_room_occupancy_supervisor UNIQUE(room_occupancy_id, supervisor_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating room_occupancy_supervisors table: %w", err)
	}

	// Create indexes for room_occupancy
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_room_occupancy_room_id ON room_occupancy(room_id);
		CREATE INDEX IF NOT EXISTS idx_room_occupancy_timespan_id ON room_occupancy(timespan_id);
		CREATE INDEX IF NOT EXISTS idx_room_occupancy_device_id ON room_occupancy(device_id);
		CREATE INDEX IF NOT EXISTS idx_room_occupancy_status ON room_occupancy(status);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for room_occupancy table: %w", err)
	}

	// Create indexes for room_history
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_room_history_room_id ON room_history(room_id);
		CREATE INDEX IF NOT EXISTS idx_room_history_day ON room_history(day);
		CREATE INDEX IF NOT EXISTS idx_room_history_timespan_id ON room_history(timespan_id);
		CREATE INDEX IF NOT EXISTS idx_room_history_supervisor_id ON room_history(supervisor_id);
		CREATE INDEX IF NOT EXISTS idx_room_history_group_id ON room_history(group_id);
		CREATE INDEX IF NOT EXISTS idx_room_history_ag_category_id ON room_history(ag_category_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for room_history table: %w", err)
	}

	// Create indexes for visit
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_visit_student_id ON visit(student_id);
		CREATE INDEX IF NOT EXISTS idx_visit_room_occupancy_id ON visit(room_occupancy_id);
		CREATE INDEX IF NOT EXISTS idx_visit_entry_time ON visit(entry_time);
		CREATE INDEX IF NOT EXISTS idx_visit_exit_time ON visit(exit_time);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for visit table: %w", err)
	}

	// Create indexes for room_occupancy_supervisors
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_room_occupancy_supervisors_room_occupancy_id ON room_occupancy_supervisors(room_occupancy_id);
		CREATE INDEX IF NOT EXISTS idx_room_occupancy_supervisors_supervisor_id ON room_occupancy_supervisors(supervisor_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for room_occupancy_supervisors table: %w", err)
	}

	// Create trigger for updated_at column in room_occupancy table
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_room_occupancy_modified_at ON room_occupancy;
		CREATE TRIGGER update_room_occupancy_modified_at
		BEFORE UPDATE ON room_occupancy
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for room_occupancy table: %w", err)
	}

	// Create trigger for updated_at column in visit table
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_visit_modified_at ON visit;
		CREATE TRIGGER update_visit_modified_at
		BEFORE UPDATE ON visit
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for visit table: %w", err)
	}

	// Create a trigger to update room_occupancy current_occupancy when a new visit is inserted or updated
	_, err = tx.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION update_room_occupancy_count() RETURNS TRIGGER AS $$
		BEGIN
			-- Calculate the new count of active visitors (where exit_time is null)
			UPDATE room_occupancy 
			SET 
				current_occupancy = (
					SELECT COUNT(*) 
					FROM visit 
					WHERE room_occupancy_id = NEW.room_occupancy_id AND exit_time IS NULL
				),
				modified_at = CURRENT_TIMESTAMP
			WHERE id = NEW.room_occupancy_id;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;

		DROP TRIGGER IF EXISTS trigger_update_occupancy_on_visit_change ON visit;
		CREATE TRIGGER trigger_update_occupancy_on_visit_change
		AFTER INSERT OR UPDATE OR DELETE ON visit
		FOR EACH ROW
		EXECUTE FUNCTION update_room_occupancy_count();
	`)
	if err != nil {
		return fmt.Errorf("error creating room occupancy count update trigger: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// roomComplexTablesDown removes the room_occupancy, room_history, and visit tables
func roomComplexTablesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back room occupancy, history, and visit tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables in reverse order of dependencies
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS trigger_update_occupancy_on_visit_change ON visit;
		DROP FUNCTION IF EXISTS update_room_occupancy_count();
		DROP TABLE IF EXISTS visit;
		DROP TABLE IF EXISTS room_occupancy_supervisors;
		DROP TABLE IF EXISTS room_history;
		DROP TABLE IF EXISTS room_occupancy;
	`)
	if err != nil {
		return fmt.Errorf("error dropping room complex tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
