package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AddPerformanceIndexesVersion     = "1.6.13"
	AddPerformanceIndexesDescription = "Add missing performance indexes for common query patterns"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AddPerformanceIndexesVersion] = &Migration{
		Version:     AddPerformanceIndexesVersion,
		Description: AddPerformanceIndexesDescription,
		DependsOn:   []string{"1.6.12"}, // Depends on guardian contacts optional
	}

	// Migration 1.6.13: Add performance indexes
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addPerformanceIndexesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addPerformanceIndexesDown(ctx, db)
		},
	)
}

// addPerformanceIndexesUp adds missing indexes for better query performance
func addPerformanceIndexesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.13: Adding performance indexes...")

	// Indexes for users.students table
	indexes := []string{
		// Foreign key index for person_id (if not exists)
		`CREATE INDEX IF NOT EXISTS idx_students_person_id ON users.students(person_id)`,

		// Foreign key index for group_id (if not exists)
		`CREATE INDEX IF NOT EXISTS idx_students_group_id ON users.students(group_id)`,

		// Composite index for common query pattern: school_class + group_id
		`CREATE INDEX IF NOT EXISTS idx_students_school_class_group ON users.students(school_class, group_id)`,

		// Indexes for users.guardians table
		// Email lookup (case-insensitive, partial index for non-NULL values)
		`CREATE INDEX IF NOT EXISTS idx_guardians_email_lower ON users.guardians(LOWER(email)) WHERE email IS NOT NULL`,

		// Phone lookup (partial index for non-NULL values)
		`CREATE INDEX IF NOT EXISTS idx_guardians_phone ON users.guardians(phone) WHERE phone IS NOT NULL`,

		// Indexes for users.students_guardians join table
		// Foreign key index for student_id
		`CREATE INDEX IF NOT EXISTS idx_students_guardians_student_id ON users.students_guardians(student_id)`,

		// Foreign key index for guardian_id
		`CREATE INDEX IF NOT EXISTS idx_students_guardians_guardian_id ON users.students_guardians(guardian_id)`,

		// Composite index for finding primary guardians
		`CREATE INDEX IF NOT EXISTS idx_students_guardians_primary ON users.students_guardians(student_id, is_primary) WHERE is_primary = true`,

		// Indexes for active.visits table
		// Foreign key index for student_id
		`CREATE INDEX IF NOT EXISTS idx_visits_student_id ON active.visits(student_id)`,

		// Foreign key index for active_group_id
		`CREATE INDEX IF NOT EXISTS idx_visits_active_group_id ON active.visits(active_group_id)`,

		// Index for finding active (not ended) visits
		`CREATE INDEX IF NOT EXISTS idx_visits_active ON active.visits(student_id, exit_time) WHERE exit_time IS NULL`,

		// Composite index for entry time queries
		`CREATE INDEX IF NOT EXISTS idx_visits_entry_time ON active.visits(entry_time DESC)`,

		// Indexes for active.attendance table
		// Foreign key index for student_id
		`CREATE INDEX IF NOT EXISTS idx_attendance_student_id ON active.attendance(student_id)`,

		// Index for date-based queries
		`CREATE INDEX IF NOT EXISTS idx_attendance_date ON active.attendance(date)`,

		// Composite index for finding student attendance by date
		`CREATE INDEX IF NOT EXISTS idx_attendance_student_date ON active.attendance(student_id, date)`,

		// Indexes for active.groups table
		// Foreign key index for group_id
		`CREATE INDEX IF NOT EXISTS idx_active_groups_group_id ON active.groups(group_id)`,

		// Foreign key index for room_id
		`CREATE INDEX IF NOT EXISTS idx_active_groups_room_id ON active.groups(room_id)`,

		// Index for finding active sessions
		`CREATE INDEX IF NOT EXISTS idx_active_groups_active ON active.groups(end_time) WHERE end_time IS NULL`,

		// Indexes for education.group_teacher join table
		// Foreign key index for teacher_id
		`CREATE INDEX IF NOT EXISTS idx_group_teacher_teacher_id ON education.group_teacher(teacher_id)`,

		// Foreign key index for group_id (likely already exists but ensure it)
		`CREATE INDEX IF NOT EXISTS idx_group_teacher_group_id ON education.group_teacher(group_id)`,

		// Indexes for users.persons table
		// Foreign key index for account_id
		`CREATE INDEX IF NOT EXISTS idx_persons_account_id ON users.persons(account_id)`,

		// RFID tag lookup
		`CREATE INDEX IF NOT EXISTS idx_persons_tag_id ON users.persons(tag_id) WHERE tag_id IS NOT NULL`,
	}

	for _, indexSQL := range indexes {
		fmt.Printf("Creating index: %s\n", indexSQL)
		_, err := db.ExecContext(ctx, indexSQL)
		if err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	fmt.Println("Successfully created performance indexes")
	return nil
}

// addPerformanceIndexesDown removes the indexes
func addPerformanceIndexesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back performance indexes...")

	indexes := []string{
		`DROP INDEX IF EXISTS users.idx_students_person_id`,
		`DROP INDEX IF EXISTS users.idx_students_group_id`,
		`DROP INDEX IF EXISTS users.idx_students_school_class_group`,
		`DROP INDEX IF EXISTS users.idx_guardians_email_lower`,
		`DROP INDEX IF EXISTS users.idx_guardians_phone`,
		`DROP INDEX IF EXISTS users.idx_students_guardians_student_id`,
		`DROP INDEX IF EXISTS users.idx_students_guardians_guardian_id`,
		`DROP INDEX IF EXISTS users.idx_students_guardians_primary`,
		`DROP INDEX IF EXISTS active.idx_visits_student_id`,
		`DROP INDEX IF EXISTS active.idx_visits_active_group_id`,
		`DROP INDEX IF EXISTS active.idx_visits_active`,
		`DROP INDEX IF EXISTS active.idx_visits_entry_time`,
		`DROP INDEX IF EXISTS active.idx_attendance_student_id`,
		`DROP INDEX IF EXISTS active.idx_attendance_date`,
		`DROP INDEX IF EXISTS active.idx_attendance_student_date`,
		`DROP INDEX IF EXISTS active.idx_active_groups_group_id`,
		`DROP INDEX IF EXISTS active.idx_active_groups_room_id`,
		`DROP INDEX IF EXISTS active.idx_active_groups_active`,
		`DROP INDEX IF EXISTS education.idx_group_teacher_teacher_id`,
		`DROP INDEX IF EXISTS education.idx_group_teacher_group_id`,
		`DROP INDEX IF EXISTS users.idx_persons_account_id`,
		`DROP INDEX IF EXISTS users.idx_persons_tag_id`,
	}

	for _, indexSQL := range indexes {
		_, err := db.ExecContext(ctx, indexSQL)
		if err != nil {
			// Log but don't fail - index might not exist
			fmt.Printf("Warning: failed to drop index: %v\n", err)
		}
	}

	fmt.Println("Rolled back performance indexes")
	return nil
}
