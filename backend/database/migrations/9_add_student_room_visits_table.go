package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Check if students table exists before creating the visits table
		var count int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'students'",
		).Scan(&count)

		if err != nil || count == 0 {
			fmt.Print(" [up migration] skipping student room visits table (students table doesn't exist yet)...")
			return nil
		}

		fmt.Print(" [up migration] adding student room visits table...")

		// Create student_room_visits table for tracking RFID room occupancy
		_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS student_room_visits (
			id BIGSERIAL PRIMARY KEY,
			room_id BIGINT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
			student_id BIGINT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
			entry_time TIMESTAMP NOT NULL,
			exit_time TIMESTAMP NULL,
			created_at TIMESTAMP NOT NULL
		);
		
		CREATE INDEX IF NOT EXISTS student_room_visits_room_id_idx ON student_room_visits(room_id);
		CREATE INDEX IF NOT EXISTS student_room_visits_student_id_idx ON student_room_visits(student_id);
		CREATE INDEX IF NOT EXISTS student_room_visits_exit_time_idx ON student_room_visits(exit_time);
		CREATE INDEX IF NOT EXISTS student_room_visits_entry_time_idx ON student_room_visits(entry_time);
		`)

		return err
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] dropping student room visits table...")

		_, err := db.ExecContext(ctx, `
		DROP TABLE IF EXISTS student_room_visits;
		`)

		return err
	})
}
