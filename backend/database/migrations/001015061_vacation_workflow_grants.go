package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	vacationWorkflowGrantsVersion     = "1.15.61"
	vacationWorkflowGrantsDescription = "Grant phoenix_tenant access to staff_vacation_quota + staff_absence_audit tables (missing from 1.15.59)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     vacationWorkflowGrantsVersion,
		Description: vacationWorkflowGrantsDescription,
		DependsOn:   []string{"1.15.59"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addVacationWorkflowGrants(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeVacationWorkflowGrants(ctx, db)
		},
	)
}

func addVacationWorkflowGrants(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.60: Granting phoenix_tenant access to vacation tables...")

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
		GRANT SELECT, INSERT, UPDATE, DELETE ON active.staff_vacation_quota TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE active.staff_vacation_quota_id_seq TO phoenix_tenant;
		GRANT SELECT, INSERT ON active.staff_absence_audit TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE active.staff_absence_audit_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error granting phoenix_tenant access: %w", err)
	}

	fmt.Println("Migration 1.15.60: vacation table grants ready")
	return tx.Commit()
}

func removeVacationWorkflowGrants(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.60: Revoking vacation table grants...")

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
		REVOKE ALL ON active.staff_vacation_quota FROM phoenix_tenant;
		REVOKE ALL ON SEQUENCE active.staff_vacation_quota_id_seq FROM phoenix_tenant;
		REVOKE ALL ON active.staff_absence_audit FROM phoenix_tenant;
		REVOKE ALL ON SEQUENCE active.staff_absence_audit_id_seq FROM phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error revoking phoenix_tenant access: %w", err)
	}

	return tx.Commit()
}
