package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const guardianStorageExpandVersion = "1.15.387"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianStorageExpandVersion,
		Description: "Expand empty People, Care Plan and Identity guardian storage without switching users.students_guardians callers (#2716)",
		DependsOn: []string{
			UsersStudentsGuardiansVersion, guardianPaymentDataVersion, studentGuardianRolesVersion,
			guardianFKRestrictVersion, guardianProfileAccountFKVersion, createTenantRolesVersion,
		},
	})
	Migrations.MustRegister(guardianStorageExpandUp, guardianStorageExpandDown)
}

// guardianStorageExpandUp creates the three empty owner tables that will
// replace users.students_guardians. Every moved column keeps the SQL type,
// nullability and default of its source column so the later backfill is a
// plain column mapping:
//
//   - users.student_guardian_relationships (People Directory) owns the
//     relationship itself: student, guardian, relationship_type, guardian_role,
//     is_primary, is_emergency_contact, emergency_priority and is_payer.
//   - users.student_guardian_pickup_permissions (Care Plan) owns can_pickup and
//     pickup_notes, one row per relationship.
//   - auth.guardian_student_access (Identity & Access) owns the guardian account
//     binding and the parent-portal permissions JSON, one row per relationship.
//
// The old table, its triggers and every application caller stay the sole
// authority. No rows are copied, no compatibility view or routing trigger is
// added, and the old table is not altered. Single-primary and single-payer per
// child become partial unique indexes on the relationship table instead of the
// old demotion trigger; the old trigger already guarantees both invariants, so
// the backfill never has to resolve conflicts.
func guardianStorageExpandUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `
			SET LOCAL lock_timeout = '5s';
			CREATE TABLE users.student_guardian_relationships (
				id BIGSERIAL PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
				student_id BIGINT NOT NULL,
				guardian_profile_id BIGINT NOT NULL,
				relationship_type TEXT NOT NULL,
				guardian_role TEXT NOT NULL DEFAULT 'custom',
				is_primary BOOLEAN NOT NULL DEFAULT FALSE,
				is_emergency_contact BOOLEAN NOT NULL DEFAULT FALSE,
				emergency_priority INT DEFAULT 1,
				is_payer BOOLEAN NOT NULL DEFAULT FALSE,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CONSTRAINT uq_student_guardian_relationships_tenant_id UNIQUE (tenant_id, id),
				CONSTRAINT uq_student_guardian_relationships_pair UNIQUE (tenant_id, student_id, guardian_profile_id),
				CONSTRAINT fk_student_guardian_relationships_student FOREIGN KEY (tenant_id, student_id)
					REFERENCES users.students(tenant_id, id) ON DELETE CASCADE,
				CONSTRAINT fk_student_guardian_relationships_guardian FOREIGN KEY (tenant_id, guardian_profile_id)
					REFERENCES users.guardian_profiles(tenant_id, id) ON DELETE RESTRICT
			);
			COMMENT ON TABLE users.student_guardian_relationships IS
				'People-owned target. Empty during Expand #2716; users.students_guardians remains authoritative until Cutover.';
			CREATE INDEX idx_student_guardian_relationships_guardian
				ON users.student_guardian_relationships (tenant_id, guardian_profile_id);
			CREATE UNIQUE INDEX uq_student_guardian_relationships_primary
				ON users.student_guardian_relationships (tenant_id, student_id) WHERE is_primary;
			CREATE UNIQUE INDEX uq_student_guardian_relationships_payer
				ON users.student_guardian_relationships (tenant_id, student_id) WHERE is_payer;
			CREATE INDEX idx_student_guardian_relationships_emergency
				ON users.student_guardian_relationships (tenant_id, student_id, emergency_priority) WHERE is_emergency_contact;

			CREATE TABLE users.student_guardian_pickup_permissions (
				id BIGSERIAL PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
				relationship_id BIGINT NOT NULL,
				can_pickup BOOLEAN NOT NULL DEFAULT FALSE,
				pickup_notes TEXT,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CONSTRAINT uq_student_guardian_pickup_permissions_tenant_id UNIQUE (tenant_id, id),
				CONSTRAINT uq_student_guardian_pickup_permissions_relationship UNIQUE (tenant_id, relationship_id),
				CONSTRAINT fk_student_guardian_pickup_permissions_relationship FOREIGN KEY (tenant_id, relationship_id)
					REFERENCES users.student_guardian_relationships(tenant_id, id) ON DELETE CASCADE
			);
			COMMENT ON TABLE users.student_guardian_pickup_permissions IS
				'Care-Plan-owned target. Empty during Expand #2716; users.students_guardians remains authoritative until Cutover.';

			CREATE TABLE auth.guardian_student_access (
				id BIGSERIAL PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
				relationship_id BIGINT NOT NULL,
				account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
				permissions JSONB NOT NULL DEFAULT '{}',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CONSTRAINT uq_guardian_student_access_tenant_id UNIQUE (tenant_id, id),
				CONSTRAINT uq_guardian_student_access_relationship UNIQUE (tenant_id, relationship_id),
				CONSTRAINT check_guardian_student_access_permissions CHECK (jsonb_typeof(permissions) = 'object'),
				CONSTRAINT fk_guardian_student_access_relationship FOREIGN KEY (tenant_id, relationship_id)
					REFERENCES users.student_guardian_relationships(tenant_id, id) ON DELETE CASCADE
			);
			COMMENT ON TABLE auth.guardian_student_access IS
				'Identity-owned target. Empty during Expand #2716; users.students_guardians remains authoritative until Cutover.';
			COMMENT ON COLUMN auth.guardian_student_access.account_id IS
				'Global auth account of the guardian, mirroring users.guardian_profiles.account_id; NULL until the guardian has a portal account.';
			CREATE INDEX idx_guardian_student_access_account ON auth.guardian_student_access (tenant_id, account_id)
				WHERE account_id IS NOT NULL;
			CREATE INDEX idx_guardian_student_access_account_global ON auth.guardian_student_access (account_id)
				WHERE account_id IS NOT NULL;

			GRANT SELECT, INSERT, UPDATE, DELETE ON users.student_guardian_relationships,
				users.student_guardian_pickup_permissions, auth.guardian_student_access TO phoenix_tenant;
			GRANT ALL ON users.student_guardian_relationships,
				users.student_guardian_pickup_permissions, auth.guardian_student_access TO phoenix_admin;
			GRANT USAGE, SELECT ON SEQUENCE users.student_guardian_relationships_id_seq,
				users.student_guardian_pickup_permissions_id_seq, auth.guardian_student_access_id_seq TO phoenix_tenant;
			GRANT ALL ON SEQUENCE users.student_guardian_relationships_id_seq,
				users.student_guardian_pickup_permissions_id_seq, auth.guardian_student_access_id_seq TO phoenix_admin;
		`)
		if err != nil {
			return fmt.Errorf("expand guardian storage: %w", err)
		}
		return provisionTenantRLS(ctx, tx,
			"users.student_guardian_relationships",
			"users.student_guardian_pickup_permissions",
			"auth.guardian_student_access",
		)
	})
}

func guardianStorageExpandDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Serialize the emptiness check with writers. Once a backfill has
		// populated the targets, rollback belongs to that ticket, never to an
		// unconditional DROP here.
		_, err := tx.ExecContext(ctx, `
			SET LOCAL lock_timeout = '5s';
			LOCK TABLE users.student_guardian_relationships, users.student_guardian_pickup_permissions,
				auth.guardian_student_access IN ACCESS EXCLUSIVE MODE;
			DO $$ BEGIN
				IF EXISTS (SELECT FROM users.student_guardian_relationships)
					OR EXISTS (SELECT FROM users.student_guardian_pickup_permissions)
					OR EXISTS (SELECT FROM auth.guardian_student_access) THEN
					RAISE EXCEPTION 'Guardian storage Expand rollback requires empty target tables';
				END IF;
			END $$;
			DROP TABLE auth.guardian_student_access;
			DROP TABLE users.student_guardian_pickup_permissions;
			DROP TABLE users.student_guardian_relationships;
		`)
		if err != nil {
			return fmt.Errorf("rollback guardian storage Expand: %w", err)
		}
		return nil
	})
}
