package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	guardianPaymentDataVersion     = "1.15.336"
	guardianPaymentDataDescription = "Store guardian bank details (IBAN) and mark the paying guardian per child (#2608)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianPaymentDataVersion,
		Description: guardianPaymentDataDescription,
		DependsOn: []string{
			staffStammdatenVersion, // users.staff_financial_data — same isolation shape
		},
	})

	Migrations.MustRegister(guardianPaymentDataUp, guardianPaymentDataDown)
}

// guardianPaymentDataUp creates the payment storage for issue #2608.
//
// The IBAN hangs off the guardian, not off the child: a child has no bank
// account, and guardians are already shared across siblings via
// users.students_guardians. Maintaining the IBAN once therefore covers every
// sibling without duplicate data entry, which the issue asks for explicitly.
//
//   - users.guardian_financial_data (1:1 with a guardian profile) is its own
//     table for the same reason users.staff_financial_data is: no join and no
//     generic repository List can pull an IBAN into a code path that is not
//     gated on guardians:financial.
//   - users.students_guardians.is_payer answers "which of this child's
//     guardians pays". Without it the assignment would be implicit ("whoever
//     has an IBAN"), which breaks the moment separated parents each keep their
//     own account. The partial unique index makes at most one payer per child
//     a database invariant rather than a service-layer promise.
//   - audit.guardian_financial_changes is the append-only edit trail. It
//     stores MASKED values only: the trail must not become a second copy of
//     the bank data outside the permission gate. student_id is set on payer
//     changes and NULL on IBAN changes, which are guardian-scoped.
//
// guardians:financial is catalog-only (no role grant), mirroring staff:financial
// in 1.15.251: school admins match via the admin:* wildcard anyway, and the
// school office can be granted the permission explicitly without a migration.
func guardianPaymentDataUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.336: Creating guardian payment data + guardians:financial permission...")

	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS users.guardian_financial_data (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			guardian_profile_id BIGINT NOT NULL,
			iban TEXT,
			account_holder TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_guardian_financial_data_guardian UNIQUE (guardian_profile_id),
			CONSTRAINT fk_guardian_financial_data_guardian_tenant
				FOREIGN KEY (tenant_id, guardian_profile_id)
				REFERENCES users.guardian_profiles(tenant_id, id) ON DELETE CASCADE
		);

		COMMENT ON COLUMN users.guardian_financial_data.account_holder IS
			'Kontoinhaber when the account does not run on the guardian''s own name; NULL = the guardian is the holder';

		CREATE INDEX IF NOT EXISTS idx_guardian_financial_data_tenant
			ON users.guardian_financial_data (tenant_id);

		ALTER TABLE users.students_guardians
			ADD COLUMN IF NOT EXISTS is_payer BOOLEAN NOT NULL DEFAULT FALSE;

		COMMENT ON COLUMN users.students_guardians.is_payer IS
			'This guardian''s bank account is charged for this child (#2608); at most one per child';

		CREATE UNIQUE INDEX IF NOT EXISTS uq_students_guardians_payer
			ON users.students_guardians (tenant_id, student_id)
			WHERE is_payer;

		CREATE TABLE IF NOT EXISTS audit.guardian_financial_changes (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			guardian_profile_id BIGINT NOT NULL,
			student_id BIGINT,
			changed_by BIGINT NOT NULL,
			field_name TEXT NOT NULL,
			old_value TEXT NOT NULL DEFAULT '',
			new_value TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '',
			occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		COMMENT ON TABLE audit.guardian_financial_changes IS
			'Append-only trail of guardian payment changes (#2608). old_value/new_value carry MASKED values only.';

		CREATE INDEX IF NOT EXISTS idx_guardian_financial_changes_guardian
			ON audit.guardian_financial_changes (tenant_id, guardian_profile_id, occurred_at DESC);

		-- The generic repository Update writes every mapped column including the
		-- updated_at it loaded with the row, so without a BEFORE UPDATE trigger
		-- every later edit would re-persist the creation timestamp.
		DROP TRIGGER IF EXISTS update_guardian_financial_data_updated_at ON users.guardian_financial_data;
		CREATE TRIGGER update_guardian_financial_data_updated_at
		BEFORE UPDATE ON users.guardian_financial_data
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		GRANT SELECT, INSERT, UPDATE, DELETE ON users.guardian_financial_data TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.guardian_financial_data_id_seq TO phoenix_tenant;

		GRANT SELECT, INSERT ON audit.guardian_financial_changes TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE audit.guardian_financial_changes_id_seq TO phoenix_tenant;
		REVOKE UPDATE, DELETE ON audit.guardian_financial_changes FROM phoenix_tenant;

		INSERT INTO auth.permissions (name, description, resource, action, is_system)
		VALUES ('guardians:financial', 'Read and update guardian bank details (IBAN) and export them', 'guardians', 'financial', TRUE)
		ON CONFLICT (name) DO NOTHING;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating guardian payment objects: %w", err)
	}
	return provisionTenantRLS(ctx, db,
		"users.guardian_financial_data",
		"audit.guardian_financial_changes",
	)
}

func guardianPaymentDataDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.336: Dropping guardian payment objects...")

	if _, err := db.NewRaw(`
		DELETE FROM auth.role_permissions
		WHERE permission_id IN (SELECT id FROM auth.permissions WHERE name = 'guardians:financial');
		DELETE FROM auth.permissions WHERE name = 'guardians:financial';

		DROP TABLE IF EXISTS audit.guardian_financial_changes;
		DROP TRIGGER IF EXISTS update_guardian_financial_data_updated_at ON users.guardian_financial_data;
		DROP TABLE IF EXISTS users.guardian_financial_data;

		DROP INDEX IF EXISTS users.uq_students_guardians_payer;
		ALTER TABLE users.students_guardians DROP COLUMN IF EXISTS is_payer;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping guardian payment objects: %w", err)
	}
	return nil
}
