package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	addPhotoToStudentsVersion     = "1.15.49"
	addPhotoToStudentsDescription = "Add photo_path and photo consent metadata columns to users.students"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addPhotoToStudentsVersion,
		Description: addPhotoToStudentsDescription,
		DependsOn: []string{
			UsersStudentsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addPhotoToStudentsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addPhotoToStudentsDown(ctx, db)
		},
	)
}

func addPhotoToStudentsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.49: Adding photo_path + consent metadata columns to users.students...")

	_, err := db.NewRaw(`
		ALTER TABLE users.students
		ADD COLUMN IF NOT EXISTS photo_path TEXT,
		ADD COLUMN IF NOT EXISTS photo_consent_given_at TIMESTAMPTZ,
		ADD COLUMN IF NOT EXISTS photo_consent_given_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding photo columns to users.students: %w", err)
	}

	// Partial index over students with consent given. Used by admin overviews
	// and to scope photo-enabled queries; small index because consent rate is
	// expected to be a fraction of all students.
	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_students_photo_consent
		ON users.students(tenant_id)
		WHERE photo_consent_given_at IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating photo consent index: %w", err)
	}

	return nil
}

func addPhotoToStudentsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.49: Removing photo_path + consent metadata columns from users.students...")

	_, err := db.NewRaw(`DROP INDEX IF EXISTS users.idx_students_photo_consent;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping photo consent index: %w", err)
	}

	_, err = db.NewRaw(`
		ALTER TABLE users.students
		DROP COLUMN IF EXISTS photo_consent_given_by,
		DROP COLUMN IF EXISTS photo_consent_given_at,
		DROP COLUMN IF EXISTS photo_path;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping photo columns from users.students: %w", err)
	}

	return nil
}
