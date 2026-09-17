package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const studentOwnerStorageExpandVersion = "1.15.393"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentOwnerStorageExpandVersion,
		Description: "Expand empty People, School Membership and Care Plan student storage without switching users.students callers (#2717)",
		DependsOn: []string{
			UsersStudentsVersion, compositePKIndexesVersion, createTenantRolesVersion,
			addPhotoToStudentsVersion, studentsPerChildConsentsVersion, addAddressToStudentsVersion,
			studentLifecycleVersion, studentsBusDaysVersion, studentsPickupDaysVersion,
			studentsDepartureDaysVersion, studentsAllowedDepartureModesVersion,
			studentsDepartureAccompaniedVersion,
			roomsRetireAtSchoolColorVersion, // 1.15.392 — preserves ladder order
		},
	})
	Migrations.MustRegister(studentOwnerStorageExpandUp, studentOwnerStorageExpandDown)
}

// studentOwnerStorageExpandUp creates the three empty owner tables that will
// replace users.students. Every moved column keeps the SQL type, nullability
// and default of its source column so the later backfill is a plain column
// mapping:
//
//   - users.student_profiles (People Directory) owns the child's identity
//     beyond the person row: person_id, the address fields, extra_info, the
//     photo path and its consent metadata, and the live consent timestamps
//     agb_accepted_at, data_processing_accepted_at and
//     email_contact_accepted_at.
//   - users.student_school_memberships (School Membership) owns the enrollment
//     at one school: school_class, group_id, the lifecycle status and its
//     enrolled_from/enrolled_until window. Its deleted_at is not a moved
//     column — users.students has none — but the owner's own soft deletion of
//     the enrollment; the alumnus status stays the graduation marker it is.
//   - users.student_care_profiles (Care Plan) owns the care arrangement of one
//     membership: supervisor_notes, health_info, pickup_status, departure_days,
//     allowed_departure_modes, departure_companion_note and the persisted
//     pickup_days/bus_days compatibility projections.
//
// Two groups of old columns deliberately have no target here. The legacy
// guardian_name/guardian_contact/guardian_email/guardian_phone values belong to
// users.guardian_profiles and users.guardian_phone_numbers; the backfill
// reconciles them there or reports a mismatch rather than copying them into a
// student-owned table. The legacy sick/sick_since/excused/excused_since columns
// are not copied at all: active.student_status_days is the authority for
// scheduled absence, and the backfill verifies the equivalent current state.
//
// The old table, its constraints and every application caller stay the sole
// authority. No rows are copied, no compatibility view or routing trigger is
// added, and the old table is not altered.
func studentOwnerStorageExpandUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `
			SET LOCAL lock_timeout = '5s';
			CREATE TABLE users.student_profiles (
				id BIGSERIAL PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
				person_id BIGINT NOT NULL,
				address_street TEXT,
				address_city TEXT,
				address_postal_code TEXT,
				extra_info TEXT,
				photo_path TEXT,
				photo_consent_given_at TIMESTAMPTZ,
				photo_consent_given_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
				agb_accepted_at TIMESTAMPTZ,
				data_processing_accepted_at TIMESTAMPTZ,
				email_contact_accepted_at TIMESTAMPTZ,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CONSTRAINT uq_student_profiles_tenant_id UNIQUE (tenant_id, id),
				CONSTRAINT uq_student_profiles_person UNIQUE (tenant_id, person_id),
				CONSTRAINT fk_student_profiles_person FOREIGN KEY (tenant_id, person_id)
					REFERENCES users.persons(tenant_id, id) ON DELETE CASCADE
			);
			COMMENT ON TABLE users.student_profiles IS
				'People-owned target. Empty during Expand #2717; users.students remains authoritative until Cutover.';
			CREATE INDEX idx_student_profiles_photo_consent ON users.student_profiles (tenant_id)
				WHERE photo_consent_given_at IS NOT NULL;

			CREATE TABLE users.student_school_memberships (
				id BIGSERIAL PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
				student_profile_id BIGINT NOT NULL,
				school_class TEXT NOT NULL,
				group_id BIGINT,
				status TEXT NOT NULL DEFAULT 'active',
				enrolled_from DATE,
				enrolled_until DATE,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ,
				CONSTRAINT uq_student_school_memberships_tenant_id UNIQUE (tenant_id, id),
				CONSTRAINT fk_student_school_memberships_profile FOREIGN KEY (tenant_id, student_profile_id)
					REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE,
				-- SET NULL names group_id explicitly: an unqualified composite
				-- SET NULL would also null tenant_id and then fail its NOT NULL,
				-- which is what makes deleting a referenced group on
				-- users.students an error instead of an unassignment.
				CONSTRAINT fk_student_school_memberships_group FOREIGN KEY (tenant_id, group_id)
					REFERENCES education.groups(tenant_id, id) ON DELETE SET NULL (group_id),
				CONSTRAINT chk_student_school_memberships_status
					CHECK (status IN ('pending', 'active', 'inactive', 'alumnus'))
			);
			COMMENT ON TABLE users.student_school_memberships IS
				'School-Membership-owned target. Empty during Expand #2717; users.students remains authoritative until Cutover.';
			COMMENT ON COLUMN users.student_school_memberships.deleted_at IS
				'Soft deletion of the enrollment itself. The alumnus status stays the graduation marker it is on users.students.';
			CREATE UNIQUE INDEX uq_student_school_memberships_active_profile
				ON users.student_school_memberships (tenant_id, student_profile_id) WHERE deleted_at IS NULL;
			CREATE INDEX idx_student_school_memberships_group
				ON users.student_school_memberships (tenant_id, group_id) WHERE group_id IS NOT NULL;
			-- Both class index shapes, as on users.students: the roster reads
			-- filter with plain equality and IN lists, only the class-name
			-- normalization compares lower(btrim(...)).
			CREATE INDEX idx_student_school_memberships_class
				ON users.student_school_memberships (tenant_id, school_class);
			CREATE INDEX idx_student_school_memberships_class_normalized
				ON users.student_school_memberships (tenant_id, lower(btrim(school_class)));
			CREATE INDEX idx_student_school_memberships_status
				ON users.student_school_memberships (tenant_id, status);
			-- The three lifecycle scans users.students carries: the activation
			-- tick, the deactivation tick, and the ended-enrollment sweep.
			CREATE INDEX idx_student_school_memberships_enrolled_from_pending
				ON users.student_school_memberships (tenant_id, enrolled_from) WHERE status = 'pending';
			CREATE INDEX idx_student_school_memberships_enrolled_until_active
				ON users.student_school_memberships (tenant_id, enrolled_until) WHERE status = 'active';
			CREATE INDEX idx_student_school_memberships_enrolled_until_ended
				ON users.student_school_memberships (tenant_id, enrolled_until)
				WHERE enrolled_until IS NOT NULL AND status <> 'alumnus';

			CREATE TABLE users.student_care_profiles (
				membership_id BIGINT PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
				supervisor_notes TEXT,
				health_info TEXT,
				pickup_status TEXT,
				departure_days JSONB NOT NULL DEFAULT '{}',
				allowed_departure_modes JSONB NOT NULL DEFAULT '{}',
				departure_companion_note TEXT,
				pickup_days JSONB NOT NULL DEFAULT '{}',
				bus_days JSONB NOT NULL DEFAULT '{}',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CONSTRAINT uq_student_care_profiles_tenant_id UNIQUE (tenant_id, membership_id),
				CONSTRAINT fk_student_care_profiles_membership FOREIGN KEY (tenant_id, membership_id)
					REFERENCES users.student_school_memberships(tenant_id, id) ON DELETE CASCADE,
				CONSTRAINT check_student_care_profiles_departure_days
					CHECK (users.is_valid_departure_days(departure_days)),
				CONSTRAINT check_student_care_profiles_allowed_departure_modes
					CHECK (users.is_valid_allowed_departure_modes(allowed_departure_modes)),
				CONSTRAINT check_student_care_profiles_pickup_days
					CHECK (users.is_valid_pickup_days(pickup_days)),
				CONSTRAINT check_student_care_profiles_bus_days
					CHECK (users.is_valid_bus_days(bus_days))
			);
			COMMENT ON TABLE users.student_care_profiles IS
				'Care-Plan-owned target. Empty during Expand #2717; users.students remains authoritative until Cutover.';
			CREATE INDEX idx_student_care_profiles_pickup_status
				ON users.student_care_profiles (tenant_id, pickup_status) WHERE pickup_status IS NOT NULL;

			GRANT SELECT, INSERT, UPDATE, DELETE ON users.student_profiles,
				users.student_school_memberships, users.student_care_profiles TO phoenix_tenant;
			GRANT ALL ON users.student_profiles,
				users.student_school_memberships, users.student_care_profiles TO phoenix_admin;
			GRANT USAGE, SELECT ON SEQUENCE users.student_profiles_id_seq,
				users.student_school_memberships_id_seq TO phoenix_tenant;
			GRANT ALL ON SEQUENCE users.student_profiles_id_seq,
				users.student_school_memberships_id_seq TO phoenix_admin;
		`)
		if err != nil {
			return fmt.Errorf("expand student owner storage: %w", err)
		}
		return provisionTenantRLS(ctx, tx,
			"users.student_profiles",
			"users.student_school_memberships",
			"users.student_care_profiles",
		)
	})
}

func studentOwnerStorageExpandDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Serialize the emptiness check with writers. Once a backfill has
		// populated the targets, rollback belongs to that ticket, never to an
		// unconditional DROP here.
		_, err := tx.ExecContext(ctx, `
			SET LOCAL lock_timeout = '5s';
			LOCK TABLE users.student_profiles, users.student_school_memberships,
				users.student_care_profiles IN ACCESS EXCLUSIVE MODE;
			DO $$ BEGIN
				IF EXISTS (SELECT FROM users.student_profiles)
					OR EXISTS (SELECT FROM users.student_school_memberships)
					OR EXISTS (SELECT FROM users.student_care_profiles) THEN
					RAISE EXCEPTION 'Student storage Expand rollback requires empty target tables';
				END IF;
			END $$;
			DROP TABLE users.student_care_profiles;
			DROP TABLE users.student_school_memberships;
			DROP TABLE users.student_profiles;
		`)
		if err != nil {
			return fmt.Errorf("rollback student storage Expand: %w", err)
		}
		return nil
	})
}
