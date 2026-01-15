package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/uptrace/bun"
)

const (
	AttendanceVersion     = "1.6.5"
	AttendanceDescription = "Create active.attendance table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AttendanceVersion] = &Migration{
		Version:     AttendanceVersion,
		Description: AttendanceDescription,
		DependsOn:   []string{"1.3.5", "1.2.3", "1.3.9"}, // Depends on users.students, users.staff, iot.devices
	}

	// Migration 1.6.5: Create active.attendance table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createAttendanceTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropAttendanceTable(ctx, db)
		},
	)
}

// createAttendanceTable creates the active.attendance table
func createAttendanceTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.5: Creating active.attendance table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logrus.Warnf("Error rolling back transaction: %v", err)
		}
	}()

	// Create the attendance table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS active.attendance (
			id BIGSERIAL PRIMARY KEY,
			student_id BIGINT NOT NULL,                  -- Reference to users.students
			date DATE NOT NULL,                          -- Date of attendance (server timezone)
			check_in_time TIMESTAMPTZ NOT NULL,          -- When student checked in
			check_out_time TIMESTAMPTZ,                  -- When student checked out (NULL if still checked in)
			checked_in_by BIGINT NOT NULL,               -- Reference to users.staff (who checked student in)
			checked_out_by BIGINT,                       -- Reference to users.staff (who checked student out)
			device_id BIGINT NOT NULL,                   -- Reference to iot.devices
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

			-- Foreign key constraints
			CONSTRAINT fk_attendance_student FOREIGN KEY (student_id)
				REFERENCES users.students(id) ON DELETE CASCADE,
			CONSTRAINT fk_attendance_checked_in_by FOREIGN KEY (checked_in_by)
				REFERENCES users.staff(id) ON DELETE RESTRICT,
			CONSTRAINT fk_attendance_checked_out_by FOREIGN KEY (checked_out_by)
				REFERENCES users.staff(id) ON DELETE RESTRICT,
			CONSTRAINT fk_attendance_device FOREIGN KEY (device_id)
				REFERENCES iot.devices(id) ON DELETE RESTRICT,

			-- Business rule: check-in time must be before check-out time (if check-out exists)
			CONSTRAINT chk_checkin_before_checkout CHECK (
				check_out_time IS NULL OR check_in_time <= check_out_time
			)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating active.attendance table: %w", err)
	}

	// Create indexes for attendance table
	_, err = tx.ExecContext(ctx, `
		-- Indexes mentioned in the guide
		CREATE INDEX IF NOT EXISTS idx_attendance_student_date ON active.attendance(student_id, date);
		CREATE INDEX IF NOT EXISTS idx_attendance_date ON active.attendance(date);
		CREATE INDEX IF NOT EXISTS idx_attendance_device ON active.attendance(device_id);

		-- Additional indexes for performance
		CREATE INDEX IF NOT EXISTS idx_attendance_student_id ON active.attendance(student_id);
		CREATE INDEX IF NOT EXISTS idx_attendance_check_in_time ON active.attendance(check_in_time);
		
		-- Index for finding currently checked-in students
		CREATE INDEX IF NOT EXISTS idx_attendance_currently_checked_in ON active.attendance(student_id, date)
		WHERE check_out_time IS NULL;
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for attendance table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for attendance
		DROP TRIGGER IF EXISTS update_attendance_updated_at ON active.attendance;
		CREATE TRIGGER update_attendance_updated_at
		BEFORE UPDATE ON active.attendance
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for attendance: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropAttendanceTable drops the active.attendance table
func dropAttendanceTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.6.5: Removing active.attendance table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logrus.Warnf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_attendance_updated_at ON active.attendance;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for attendance table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS active.attendance CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping active.attendance table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
