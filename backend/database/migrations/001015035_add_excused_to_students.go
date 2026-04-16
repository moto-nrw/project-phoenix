package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	addExcusedToStudentsVersion     = "1.15.35"
	addExcusedToStudentsDescription = "Add excused and excused_since columns to users.students"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addExcusedToStudentsVersion,
		Description: addExcusedToStudentsDescription,
		DependsOn: []string{
			UsersStudentsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addExcusedToStudentsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addExcusedToStudentsDown(ctx, db)
		},
	)
}

func addExcusedToStudentsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.35: Adding excused and excused_since columns to users.students...")

	_, err := db.NewRaw(`
		ALTER TABLE users.students
		ADD COLUMN IF NOT EXISTS excused BOOLEAN DEFAULT FALSE;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding excused column to users.students: %w", err)
	}

	_, err = db.NewRaw(`
		ALTER TABLE users.students
		ADD COLUMN IF NOT EXISTS excused_since TIMESTAMP;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding excused_since column to users.students: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_students_excused ON users.students(excused) WHERE excused = true;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating index on excused: %w", err)
	}

	return nil
}

func addExcusedToStudentsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.35: Removing excused and excused_since columns from users.students...")

	_, err := db.NewRaw(`DROP INDEX IF EXISTS users.idx_students_excused;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping index on excused: %w", err)
	}

	_, err = db.NewRaw(`ALTER TABLE users.students DROP COLUMN IF EXISTS excused_since;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping excused_since column from users.students: %w", err)
	}

	_, err = db.NewRaw(`ALTER TABLE users.students DROP COLUMN IF EXISTS excused;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping excused column from users.students: %w", err)
	}

	return nil
}
