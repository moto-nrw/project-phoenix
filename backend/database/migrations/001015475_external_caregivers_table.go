package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const externalCaregiversTableVersion = "1.15.475"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     externalCaregiversTableVersion,
		Description: "Rename users.guests to users.external_caregivers (#4573, ADR 0083)",
		DependsOn: []string{
			UsersGuestsVersion,      // creates users.guests
			externSchoolRoleVersion, // the last migration before this one
		},
	})
	Migrations.MustRegister(externalCaregiversTableUp, externalCaregiversTableDown)
}

// externalCaregiversTableUp gives the record that makes a staff member an
// external caregiver the name of what it is (ADR 0083). The former name
// „guests“ was easy to confuse with the system role guest.
//
// A rename keeps the rows, the row-level security, the grants and the foreign
// keys: PostgreSQL ties them to the table, not to its name. The names of the
// sequence, constraints, indexes, policy and trigger follow, so the schema
// says the same as the code. A database that already carries the new name is
// left alone.
func externalCaregiversTableUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		var renamed bool
		if err := tx.NewRaw(`SELECT to_regclass('users.guests') IS NULL
			AND to_regclass('users.external_caregivers') IS NOT NULL`).Scan(ctx, &renamed); err != nil {
			return fmt.Errorf("inspect users.guests: %w", err)
		}
		if renamed {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE users.guests RENAME TO external_caregivers;
			ALTER SEQUENCE users.guests_id_seq RENAME TO external_caregivers_id_seq;
			ALTER TABLE users.external_caregivers RENAME CONSTRAINT guests_pkey TO external_caregivers_pkey;
			ALTER TABLE users.external_caregivers RENAME CONSTRAINT guests_tenant_id_fkey TO external_caregivers_tenant_id_fkey;
			ALTER TABLE users.external_caregivers RENAME CONSTRAINT chk_guests_tenant_id_not_null TO chk_external_caregivers_tenant_id_not_null;
			ALTER TABLE users.external_caregivers RENAME CONSTRAINT fk_guests_staff_tenant TO fk_external_caregivers_staff_tenant;
			ALTER INDEX users.idx_guests_organization RENAME TO idx_external_caregivers_organization;
			ALTER INDEX users.idx_guests_staff_id RENAME TO idx_external_caregivers_staff_id;
			ALTER INDEX users.idx_guests_tenant RENAME TO idx_external_caregivers_tenant;
			ALTER INDEX users.idx_guests_tenant_id RENAME TO idx_external_caregivers_tenant_id;
			ALTER INDEX users.idx_guests_tenant_staff RENAME TO idx_external_caregivers_tenant_staff;
			ALTER POLICY tenant_isolation_users_guests ON users.external_caregivers RENAME TO tenant_isolation_users_external_caregivers;
			ALTER TRIGGER update_guests_updated_at ON users.external_caregivers RENAME TO update_external_caregivers_updated_at;
			COMMENT ON TABLE users.external_caregivers IS
				'Externe Betreuungskraft: macht eine Person des Personals extern, mit Organisation und optionalem Zeitraum (ADR 0083).';
		`); err != nil {
			return fmt.Errorf("rename users.guests to users.external_caregivers: %w", err)
		}
		return nil
	})
}

// externalCaregiversTableDown restores the names the previous image reads.
func externalCaregiversTableDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		var renamed bool
		if err := tx.NewRaw(`SELECT to_regclass('users.external_caregivers') IS NOT NULL`).Scan(ctx, &renamed); err != nil {
			return fmt.Errorf("inspect users.external_caregivers: %w", err)
		}
		if !renamed {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `
			COMMENT ON TABLE users.external_caregivers IS NULL;
			ALTER TRIGGER update_external_caregivers_updated_at ON users.external_caregivers RENAME TO update_guests_updated_at;
			ALTER POLICY tenant_isolation_users_external_caregivers ON users.external_caregivers RENAME TO tenant_isolation_users_guests;
			ALTER INDEX users.idx_external_caregivers_tenant_staff RENAME TO idx_guests_tenant_staff;
			ALTER INDEX users.idx_external_caregivers_tenant_id RENAME TO idx_guests_tenant_id;
			ALTER INDEX users.idx_external_caregivers_tenant RENAME TO idx_guests_tenant;
			ALTER INDEX users.idx_external_caregivers_staff_id RENAME TO idx_guests_staff_id;
			ALTER INDEX users.idx_external_caregivers_organization RENAME TO idx_guests_organization;
			ALTER TABLE users.external_caregivers RENAME CONSTRAINT fk_external_caregivers_staff_tenant TO fk_guests_staff_tenant;
			ALTER TABLE users.external_caregivers RENAME CONSTRAINT chk_external_caregivers_tenant_id_not_null TO chk_guests_tenant_id_not_null;
			ALTER TABLE users.external_caregivers RENAME CONSTRAINT external_caregivers_tenant_id_fkey TO guests_tenant_id_fkey;
			ALTER TABLE users.external_caregivers RENAME CONSTRAINT external_caregivers_pkey TO guests_pkey;
			ALTER SEQUENCE users.external_caregivers_id_seq RENAME TO guests_id_seq;
			ALTER TABLE users.external_caregivers RENAME TO guests;
		`); err != nil {
			return fmt.Errorf("rename users.external_caregivers back to users.guests: %w", err)
		}
		return nil
	})
}
