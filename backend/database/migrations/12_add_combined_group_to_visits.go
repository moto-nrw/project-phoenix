package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] adding combined_group_id to student_room_visits table...")

		// Check if student_room_visits table exists
		var count int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'student_room_visits'",
		).Scan(&count)

		if err != nil || count == 0 {
			fmt.Println(" skipped (table doesn't exist yet)")
			return nil
		}

		// Check if combined_group_id column already exists
		var columnCount int
		err = db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'student_room_visits' AND column_name = 'combined_group_id'",
		).Scan(&columnCount)

		if err != nil {
			return err
		}

		if columnCount > 0 {
			fmt.Println(" skipped (column already exists)")
			return nil
		}

		// Add combined_group_id column
		_, err = db.ExecContext(ctx, `
		ALTER TABLE student_room_visits 
		ADD COLUMN combined_group_id BIGINT NULL REFERENCES combined_groups(id) ON DELETE SET NULL;
		
		CREATE INDEX IF NOT EXISTS student_room_visits_combined_group_id_idx ON student_room_visits(combined_group_id);
		`)

		if err != nil {
			return err
		}

		fmt.Println(" done")
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] removing combined_group_id from student_room_visits table...")

		// Check if column exists before attempting to drop it
		var columnCount int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'student_room_visits' AND column_name = 'combined_group_id'",
		).Scan(&columnCount)

		if err != nil || columnCount == 0 {
			fmt.Println(" skipped (column doesn't exist)")
			return nil
		}

		// Remove the column
		_, err = db.ExecContext(ctx, `
		ALTER TABLE student_room_visits DROP COLUMN IF EXISTS combined_group_id;
		`)

		if err != nil {
			return err
		}

		fmt.Println(" done")
		return nil
	})
}
