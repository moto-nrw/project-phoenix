package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffStammdatenVersion     = "1.15.251"
	staffStammdatenDescription = "Create staff Stammdaten tables (master data, qualifications, financial data), audit trail and staff:financial permission (#1423)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffStammdatenVersion,
		Description: staffStammdatenDescription,
		DependsOn: []string{
			payrollFoundationVersion, // users.staff conventions + audit append-only shape
			"1.5.3",                  // fix_permission_names — colon-separated permission naming
		},
	})

	Migrations.MustRegister(staffStammdatenUp, staffStammdatenDown)
}

// staffStammdatenUp creates the staff master-data storage for issue #1423.
//
// Three tables instead of columns on users.staff, on purpose:
//   - users.staff_master_data (1:1) carries contact + contract fields. They are
//     optional HR data, not directory data, and keeping them off users.staff
//     keeps every existing staff SELECT from silently widening.
//   - users.staff_qualifications (1:N) — Erste-Hilfe, Schwimmschein and
//     Fortbildungen are rows with their own acquisition/expiry dates.
//   - users.staff_financial_data (1:1) — IBAN, Steuer-ID, SV-Nummer live in
//     their own table so no join or generic repository List can ever pull them
//     into a non-financial code path by accident. Access requires the new
//     staff:financial permission end to end.
//
// audit.staff_master_data_changes is the append-only edit trail (same contract
// as audit.personnel_number_changes: no change without a trace, no FKs on
// staff_id/changed_by so the trail survives offboarding). Read access to the
// financial fields is logged into the existing audit.data_access_log.
//
// staff:financial is catalog-only (no role grant), mirroring 1.15.176: school
// admins match via the admin:* wildcard anyway, and a Träger-Lohnbüro role can
// be granted the permission explicitly without another migration.
func staffStammdatenUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.251: Creating staff Stammdaten tables + staff:financial permission...")

	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS users.staff_master_data (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			staff_id BIGINT NOT NULL REFERENCES users.staff(id) ON DELETE CASCADE,
			gender TEXT CHECK (gender IN ('female', 'male', 'diverse')),
			address_street TEXT,
			address_postal_code TEXT,
			address_city TEXT,
			phone TEXT,
			email TEXT,
			emergency_contact_name TEXT,
			emergency_contact_phone TEXT,
			entry_date DATE,
			contract_end_date DATE,
			probation_end_date DATE,
			weekly_hours NUMERIC(5,2) CHECK (weekly_hours IS NULL OR (weekly_hours >= 0 AND weekly_hours <= 80)),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_staff_master_data_staff UNIQUE (staff_id)
		);

		CREATE INDEX IF NOT EXISTS idx_staff_master_data_tenant
			ON users.staff_master_data (tenant_id);

		CREATE TABLE IF NOT EXISTS users.staff_qualifications (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			staff_id BIGINT NOT NULL REFERENCES users.staff(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			acquired_on DATE,
			expires_on DATE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_staff_qualification_dates
				CHECK (expires_on IS NULL OR acquired_on IS NULL OR expires_on >= acquired_on)
		);

		CREATE INDEX IF NOT EXISTS idx_staff_qualifications_staff
			ON users.staff_qualifications (tenant_id, staff_id);

		CREATE TABLE IF NOT EXISTS users.staff_financial_data (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			staff_id BIGINT NOT NULL REFERENCES users.staff(id) ON DELETE CASCADE,
			iban TEXT,
			tax_id TEXT,
			social_security_number TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_staff_financial_data_staff UNIQUE (staff_id)
		);

		CREATE INDEX IF NOT EXISTS idx_staff_financial_data_tenant
			ON users.staff_financial_data (tenant_id);

		CREATE TABLE IF NOT EXISTS audit.staff_master_data_changes (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			staff_id BIGINT NOT NULL,
			changed_by BIGINT NOT NULL,
			section TEXT NOT NULL,
			field_name TEXT NOT NULL,
			old_value TEXT NOT NULL DEFAULT '',
			new_value TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '',
			occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_staff_master_data_changes_staff
			ON audit.staff_master_data_changes (tenant_id, staff_id, occurred_at DESC);

		ALTER TABLE users.staff_master_data ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.staff_master_data FORCE ROW LEVEL SECURITY;
		ALTER TABLE users.staff_qualifications ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.staff_qualifications FORCE ROW LEVEL SECURITY;
		ALTER TABLE users.staff_financial_data ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.staff_financial_data FORCE ROW LEVEL SECURITY;
		ALTER TABLE audit.staff_master_data_changes ENABLE ROW LEVEL SECURITY;
		ALTER TABLE audit.staff_master_data_changes FORCE ROW LEVEL SECURITY;

		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_policies
				WHERE schemaname = 'users' AND tablename = 'staff_master_data'
					AND policyname = 'tenant_isolation_users_staff_master_data'
			) THEN
				CREATE POLICY tenant_isolation_users_staff_master_data
					ON users.staff_master_data
					FOR ALL
					USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
					WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			END IF;
			IF NOT EXISTS (
				SELECT 1 FROM pg_policies
				WHERE schemaname = 'users' AND tablename = 'staff_qualifications'
					AND policyname = 'tenant_isolation_users_staff_qualifications'
			) THEN
				CREATE POLICY tenant_isolation_users_staff_qualifications
					ON users.staff_qualifications
					FOR ALL
					USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
					WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			END IF;
			IF NOT EXISTS (
				SELECT 1 FROM pg_policies
				WHERE schemaname = 'users' AND tablename = 'staff_financial_data'
					AND policyname = 'tenant_isolation_users_staff_financial_data'
			) THEN
				CREATE POLICY tenant_isolation_users_staff_financial_data
					ON users.staff_financial_data
					FOR ALL
					USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
					WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			END IF;
			IF NOT EXISTS (
				SELECT 1 FROM pg_policies
				WHERE schemaname = 'audit' AND tablename = 'staff_master_data_changes'
					AND policyname = 'tenant_isolation_staff_master_data_changes'
			) THEN
				CREATE POLICY tenant_isolation_staff_master_data_changes
					ON audit.staff_master_data_changes
					USING (tenant_id = current_setting('app.current_tenant_id')::BIGINT);
			END IF;
		END $$;

		GRANT SELECT, INSERT, UPDATE ON users.staff_master_data TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.staff_master_data_id_seq TO phoenix_tenant;
		GRANT SELECT, INSERT, UPDATE, DELETE ON users.staff_qualifications TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.staff_qualifications_id_seq TO phoenix_tenant;
		GRANT SELECT, INSERT, UPDATE ON users.staff_financial_data TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.staff_financial_data_id_seq TO phoenix_tenant;

		GRANT SELECT, INSERT ON audit.staff_master_data_changes TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE audit.staff_master_data_changes_id_seq TO phoenix_tenant;
		REVOKE UPDATE, DELETE ON audit.staff_master_data_changes FROM phoenix_tenant;

		INSERT INTO auth.permissions (name, description, resource, action, is_system)
		VALUES ('staff:financial', 'Read and update staff bank and tax data (IBAN, Steuer-ID, SV-Nummer)', 'staff', 'financial', TRUE)
		ON CONFLICT (name) DO NOTHING;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating staff Stammdaten objects: %w", err)
	}
	return nil
}

func staffStammdatenDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.251: Dropping staff Stammdaten objects...")
	if _, err := db.NewRaw(`
		DELETE FROM auth.permissions WHERE name = 'staff:financial';
		DROP TABLE IF EXISTS audit.staff_master_data_changes CASCADE;
		DROP TABLE IF EXISTS users.staff_financial_data CASCADE;
		DROP TABLE IF EXISTS users.staff_qualifications CASCADE;
		DROP TABLE IF EXISTS users.staff_master_data CASCADE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed rolling back staff Stammdaten objects: %w", err)
	}
	return nil
}
