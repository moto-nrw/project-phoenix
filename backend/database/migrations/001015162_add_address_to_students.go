package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	addAddressToStudentsVersion     = "1.15.162"
	addAddressToStudentsDescription = "Add child address columns to users.students"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addAddressToStudentsVersion,
		Description: addAddressToStudentsDescription,
		DependsOn: []string{
			UsersStudentsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addAddressToStudentsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addAddressToStudentsDown(ctx, db)
		},
	)
}

func addAddressToStudentsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.162: Adding child address columns to users.students...")

	_, err := db.NewRaw(`
		ALTER TABLE users.students
		ADD COLUMN IF NOT EXISTS address_street TEXT,
		ADD COLUMN IF NOT EXISTS address_city TEXT,
		ADD COLUMN IF NOT EXISTS address_postal_code TEXT;

		COMMENT ON COLUMN users.students.address_street IS 'Child address street and house number';
		COMMENT ON COLUMN users.students.address_city IS 'Child address city';
		COMMENT ON COLUMN users.students.address_postal_code IS 'Child address postal code';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding child address columns to users.students: %w", err)
	}

	return nil
}

func addAddressToStudentsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.162: Removing child address columns from users.students...")

	_, err := db.NewRaw(`
		ALTER TABLE users.students
		DROP COLUMN IF EXISTS address_postal_code,
		DROP COLUMN IF EXISTS address_city,
		DROP COLUMN IF EXISTS address_street;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping child address columns from users.students: %w", err)
	}

	return nil
}
