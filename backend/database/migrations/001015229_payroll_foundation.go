package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	payrollFoundationVersion     = "1.15.229"
	payrollFoundationDescription = "Add users.staff.personnel_number + audit.personnel_number_changes for the DATEV payroll foundation (#1417)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     payrollFoundationVersion,
		Description: payrollFoundationDescription,
		DependsOn: []string{
			auditAppendOnlyGrantsVersion,
			studentFieldEditsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return payrollFoundationUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return payrollFoundationDown(ctx, db)
		},
	)
}

// payrollFoundationUp lays the ground for the DATEV exports (#1417 2b):
// both LODAS and Lohn und Gehalt match employees via the payroll system's
// Personalnummer, which nothing in Phoenix stored so far.
//
// personnel_number lives on users.staff and is therefore school-scoped. A
// person working at two schools of the same Träger has two staff rows; the
// number belongs to the employer, so both rows should carry the SAME value —
// per-tenant RLS makes that consistency unenforceable here. Documented as a
// known gap (needs the org-wide read layer, see #1986 "Standort-Filter").
// Uniqueness IS enforced within a tenant: one number, one person. The partial
// index skips soft-deleted rows so an offboarded person's number can be
// reassigned.
//
// audit.personnel_number_changes records every change: personnel numbers are
// what payroll bills against, so a silent edit is an audit finding. Same
// append-only shape as audit.time_tracking_deletions (1.15.227); staff_id and
// changed_by carry no FK on purpose so the trail survives offboarding.
func payrollFoundationUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.229: Adding personnel_number + audit.personnel_number_changes...")

	_, err := db.ExecContext(ctx, `
		ALTER TABLE users.staff ADD COLUMN IF NOT EXISTS personnel_number TEXT;

		CREATE UNIQUE INDEX IF NOT EXISTS uq_staff_tenant_personnel_number
			ON users.staff (tenant_id, personnel_number)
			WHERE personnel_number IS NOT NULL AND deleted_at IS NULL;

		CREATE TABLE IF NOT EXISTS audit.personnel_number_changes (
			id          BIGSERIAL PRIMARY KEY,
			tenant_id   BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			staff_id    BIGINT NOT NULL,
			changed_by  BIGINT NOT NULL,
			old_value   TEXT NOT NULL DEFAULT '',
			new_value   TEXT NOT NULL DEFAULT '',
			note        TEXT NOT NULL DEFAULT '',
			occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_personnel_number_changes_tenant_occurred
			ON audit.personnel_number_changes (tenant_id, occurred_at DESC);

		ALTER TABLE audit.personnel_number_changes ENABLE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_personnel_number_changes ON audit.personnel_number_changes
			USING (tenant_id = current_setting('app.current_tenant_id')::BIGINT);
		ALTER TABLE audit.personnel_number_changes FORCE ROW LEVEL SECURITY;

		GRANT SELECT, INSERT ON audit.personnel_number_changes TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE audit.personnel_number_changes_id_seq TO phoenix_tenant;
		REVOKE UPDATE, DELETE ON audit.personnel_number_changes FROM phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating payroll foundation objects: %w", err)
	}
	return nil
}

func payrollFoundationDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.229: Dropping payroll foundation objects...")
	_, err := db.ExecContext(ctx, `
		DROP TABLE IF EXISTS audit.personnel_number_changes;
		DROP INDEX IF EXISTS users.uq_staff_tenant_personnel_number;
		ALTER TABLE users.staff DROP COLUMN IF EXISTS personnel_number;
	`)
	if err != nil {
		return fmt.Errorf("error rolling back payroll foundation: %w", err)
	}
	return nil
}
