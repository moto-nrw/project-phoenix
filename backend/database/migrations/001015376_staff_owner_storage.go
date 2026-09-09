package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     "1.15.376",
		Description: "Expand empty School Membership and Workforce staff storage (#2715)",
		DependsOn:   []string{compositePKIndexesVersion, createTenantRolesVersion, "1.15.109", "1.15.374"},
	})
	Migrations.MustRegister(staffOwnerStorageUp, staffOwnerStorageDown)
}

// Expand deliberately leaves users.staff and every existing writer untouched.
// Lifecycle is represented by created_at, updated_at and deleted_at, as on
// users.staff. Backfill and the owner cutover belong to subsequent migrations.
func staffOwnerStorageUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `
			CREATE TABLE users.staff_school_memberships (
				id BIGSERIAL PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
				person_id BIGINT NOT NULL,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ,
				CONSTRAINT uq_staff_school_memberships_tenant_id UNIQUE (tenant_id, id),
				CONSTRAINT fk_staff_school_memberships_person FOREIGN KEY (tenant_id, person_id)
					REFERENCES users.persons(tenant_id, id) ON DELETE CASCADE
			);
			CREATE UNIQUE INDEX uq_staff_school_memberships_active_person
				ON users.staff_school_memberships (tenant_id, person_id) WHERE deleted_at IS NULL;
			CREATE INDEX idx_staff_school_memberships_person
				ON users.staff_school_memberships (tenant_id, person_id);

			CREATE TABLE users.staff_employment_profiles (
				membership_id BIGINT PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
				staff_notes TEXT,
				employment_type VARCHAR(20),
				work_time_model_id BIGINT REFERENCES config.work_time_models(id) ON DELETE RESTRICT,
				personnel_number TEXT,
				rotation_anchor_date DATE,
				birthday_display_opt_out BOOLEAN NOT NULL DEFAULT FALSE,
				CONSTRAINT fk_staff_employment_profiles_membership FOREIGN KEY (tenant_id, membership_id)
					REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE,
				CONSTRAINT chk_staff_employment_profiles_employment_type
					CHECK (employment_type IN ('full_time', 'part_time', 'minijob'))
			);
			CREATE INDEX idx_staff_employment_profiles_tenant ON users.staff_employment_profiles (tenant_id, membership_id);
			CREATE INDEX idx_staff_employment_profiles_personnel_number
				ON users.staff_employment_profiles (tenant_id, personnel_number) WHERE personnel_number IS NOT NULL;
			CREATE INDEX idx_staff_employment_profiles_work_time_model
				ON users.staff_employment_profiles (work_time_model_id) WHERE work_time_model_id IS NOT NULL;
			GRANT SELECT, INSERT, UPDATE, DELETE ON users.staff_school_memberships, users.staff_employment_profiles TO phoenix_tenant;
			GRANT ALL ON users.staff_school_memberships, users.staff_employment_profiles TO phoenix_admin;
			GRANT USAGE ON SEQUENCE users.staff_school_memberships_id_seq TO phoenix_tenant;
			GRANT ALL ON SEQUENCE users.staff_school_memberships_id_seq TO phoenix_admin;
		`)
		if err != nil {
			return fmt.Errorf("create staff owner storage: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx, "users.staff_school_memberships", "users.staff_employment_profiles"); err != nil {
			return err
		}
		// Work-time models have no composite tenant/id key. Enforce tenant-role
		// writes through their existing RLS without altering that source table.
		_, err = tx.ExecContext(ctx, `
			CREATE POLICY staff_employment_profiles_work_time_model ON users.staff_employment_profiles
				AS RESTRICTIVE FOR ALL TO phoenix_tenant USING (true)
				WITH CHECK (work_time_model_id IS NULL OR EXISTS (
					SELECT 1 FROM config.work_time_models model
					WHERE model.id = work_time_model_id AND model.tenant_id = staff_employment_profiles.tenant_id
				));
		`)
		return err
	})
}

func staffOwnerStorageDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		// Do not silently discard data if somebody has already cut over.
		_, err := tx.ExecContext(ctx, `
			LOCK TABLE users.staff_employment_profiles, users.staff_school_memberships IN ACCESS EXCLUSIVE MODE;
			DO $$ BEGIN
				IF EXISTS (SELECT 1 FROM users.staff_employment_profiles)
					OR EXISTS (SELECT 1 FROM users.staff_school_memberships) THEN
					RAISE EXCEPTION 'staff owner storage is not empty; undo Cutover before rolling back Expand';
				END IF;
			END $$;
			DROP TABLE users.staff_employment_profiles;
			DROP TABLE users.staff_school_memberships;
		`)
		return err
	})
}
