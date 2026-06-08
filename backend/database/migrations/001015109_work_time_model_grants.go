package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	workTimeModelGrantsVersion     = "1.15.109"
	workTimeModelGrantsDescription = "Grant phoenix_tenant access to work-time model tables"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     workTimeModelGrantsVersion,
		Description: workTimeModelGrantsDescription,
		DependsOn:   []string{"1.15.102"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return grantWorkTimeModelPermissions(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return revokeWorkTimeModelPermissions(ctx, db)
		},
	)
}

func grantWorkTimeModelPermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.109: Granting phoenix_tenant access to work-time model tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		GRANT SELECT, INSERT, UPDATE, DELETE ON config.work_time_models TO phoenix_tenant;
		GRANT SELECT, INSERT, UPDATE, DELETE ON config.work_time_model_entries TO phoenix_tenant;
		GRANT SELECT, INSERT, UPDATE, DELETE ON config.staff_work_schedules TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE config.work_time_models_id_seq TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE config.work_time_model_entries_id_seq TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE config.staff_work_schedules_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error granting phoenix_tenant work-time model access: %w", err)
	}

	fmt.Println("Migration 1.15.109: Successfully granted work-time model access")
	return tx.Commit()
}

func revokeWorkTimeModelPermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.109: Revoking work-time model grants...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		REVOKE ALL ON config.work_time_models FROM phoenix_tenant;
		REVOKE ALL ON config.work_time_model_entries FROM phoenix_tenant;
		REVOKE ALL ON config.staff_work_schedules FROM phoenix_tenant;
		REVOKE ALL ON SEQUENCE config.work_time_models_id_seq FROM phoenix_tenant;
		REVOKE ALL ON SEQUENCE config.work_time_model_entries_id_seq FROM phoenix_tenant;
		REVOKE ALL ON SEQUENCE config.staff_work_schedules_id_seq FROM phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error revoking phoenix_tenant work-time model access: %w", err)
	}

	fmt.Println("Migration 1.15.109: Successfully rolled back")
	return tx.Commit()
}
