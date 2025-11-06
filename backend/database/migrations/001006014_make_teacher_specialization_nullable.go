package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	makeTeacherSpecializationNullableVersion     = "1.6.14"
	makeTeacherSpecializationNullableDescription = "Allow teacher specialization to be nullable"
)

var makeTeacherSpecializationNullableDependencies = []string{
	"1.6.13",
}

func init() {
	MigrationRegistry[makeTeacherSpecializationNullableVersion] = &Migration{
		Version:     makeTeacherSpecializationNullableVersion,
		Description: makeTeacherSpecializationNullableDescription,
		DependsOn:   makeTeacherSpecializationNullableDependencies,
	}

	Migrations.MustRegister(
		makeTeacherSpecializationNullableUp,
		makeTeacherSpecializationNullableDown,
	)
}

// makeTeacherSpecializationNullableUp drops the NOT NULL constraint from users.teachers.specialization
func makeTeacherSpecializationNullableUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.14: Making teacher specialization nullable...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		ALTER TABLE users.teachers
		ALTER COLUMN specialization DROP NOT NULL;
	`); err != nil {
		return fmt.Errorf("failed to drop NOT NULL constraint on specialization: %w", err)
	}

	return tx.Commit()
}

// makeTeacherSpecializationNullableDown restores the NOT NULL constraint
func makeTeacherSpecializationNullableDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.6.14: Requiring teacher specialization...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		ALTER TABLE users.teachers
		ALTER COLUMN specialization SET NOT NULL;
	`); err != nil {
		return fmt.Errorf("failed to set NOT NULL constraint on specialization: %w", err)
	}

	return tx.Commit()
}

