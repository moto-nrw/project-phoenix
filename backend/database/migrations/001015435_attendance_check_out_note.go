package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	attendanceCheckOutNoteVersion     = "1.15.435"
	attendanceCheckOutNoteDescription = "Add active.attendance.check_out_note - optional reason when a child goes home before the Gehzeit (#3324)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     attendanceCheckOutNoteVersion,
		Description: attendanceCheckOutNoteDescription,
		DependsOn: []string{
			AttendanceVersion,              // active.attendance
			parentMessageCountScopeVersion, // latest migration at authoring time
		},
	})
	Migrations.MustRegister(attendanceCheckOutNoteUp, attendanceCheckOutNoteDown)
}

// attendanceCheckOutNoteUp stores why a child was checked out of the OGS
// earlier than planned (#3324). The note belongs to one stay: a child that
// comes back the same day gets a new attendance row, so the note never has to
// be cleared. NULL means nobody wrote one; existing rows stay NULL.
func attendanceCheckOutNoteUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE active.attendance
			ADD COLUMN IF NOT EXISTS check_out_note TEXT;
		ALTER TABLE active.attendance
			DROP CONSTRAINT IF EXISTS chk_attendance_check_out_note_length;
		ALTER TABLE active.attendance
			ADD CONSTRAINT chk_attendance_check_out_note_length
			CHECK (check_out_note IS NULL OR char_length(check_out_note) BETWEEN 1 AND 500);
	`)
	if err != nil {
		return fmt.Errorf("error adding active.attendance.check_out_note: %w", err)
	}
	return nil
}

func attendanceCheckOutNoteDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE active.attendance DROP CONSTRAINT IF EXISTS chk_attendance_check_out_note_length;
		ALTER TABLE active.attendance DROP COLUMN IF EXISTS check_out_note;
	`)
	if err != nil {
		return fmt.Errorf("error dropping active.attendance.check_out_note: %w", err)
	}
	return nil
}
