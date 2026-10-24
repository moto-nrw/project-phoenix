package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const classHandoverVersion = "1.15.492"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     classHandoverVersion,
		Description: "Übergabe nach dem Unterricht: die Lehrkraft gibt die Klasse an die OGS ab (#4794)",
		DependsOn: []string{
			morningAttendanceVersion, // the erasure phases frozen anew from its ones
			lehrkraftRoleVersion,     // the role the write permission goes to
			schoolErasureVersion,     // the erasure the new tables join
		},
	})
	Migrations.MustRegister(classHandoverUp, classHandoverDown)
}

// classHandoverUpSQL creates the handover of a class after lessons (#4794):
//
//   - active.class_handovers holds one row per class and school day: when
//     and by whom the Lehrkraft handed the class over to the OGS. A
//     withdrawal on the same day deletes the row. The class is free text
//     like users.student_profiles.school_class and matches trimmed and
//     lowercased.
//   - active.class_handover_absences names the children with an OGS record
//     the Lehrkraft marked "nicht da" at the handover. Both keys carry the
//     tenant, so a row can neither name another school's handover nor
//     another school's child; it goes with either through ON DELETE CASCADE.
//   - handed_over_by_staff_id keeps who handed over; a removed staff member
//     leaves the handover without a name.
//
// The handover is a moment of its own: it never changes the morning
// attendance (active.morning_attendance) or the OGS attendance. The nightly
// retention deletes it with the morning attendance after the school's window
// gdpr.morning_attendance_retention_days.
const classHandoverUpSQL = `
		CREATE TABLE IF NOT EXISTS active.class_handovers (
			id                      BIGSERIAL PRIMARY KEY,
			tenant_id               BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			school_class            TEXT NOT NULL,
			date                    DATE NOT NULL,
			handed_over_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			handed_over_by_staff_id BIGINT REFERENCES users.staff_school_memberships(id) ON DELETE SET NULL,
			created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uniq_class_handovers_tenant_id UNIQUE (tenant_id, id),
			CONSTRAINT chk_class_handovers_school_class CHECK (BTRIM(school_class) <> '')
		);

		COMMENT ON TABLE active.class_handovers IS
			'A Lehrkraft handed her class over to the OGS after lessons on one school day (#4794)';

		CREATE UNIQUE INDEX IF NOT EXISTS uniq_class_handovers_class_date
			ON active.class_handovers (tenant_id, LOWER(BTRIM(school_class)), date);
		CREATE INDEX IF NOT EXISTS idx_class_handovers_tenant_date
			ON active.class_handovers (tenant_id, date);
		CREATE INDEX IF NOT EXISTS idx_class_handovers_handed_over_by
			ON active.class_handovers (handed_over_by_staff_id) WHERE handed_over_by_staff_id IS NOT NULL;

		DROP TRIGGER IF EXISTS update_class_handovers_updated_at ON active.class_handovers;
		CREATE TRIGGER update_class_handovers_updated_at
		BEFORE UPDATE ON active.class_handovers
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		CREATE TABLE IF NOT EXISTS active.class_handover_absences (
			id          BIGSERIAL PRIMARY KEY,
			tenant_id   BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			handover_id BIGINT NOT NULL,
			student_id  BIGINT NOT NULL,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_class_handover_absences_handover FOREIGN KEY (tenant_id, handover_id)
				REFERENCES active.class_handovers(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT fk_class_handover_absences_student FOREIGN KEY (tenant_id, student_id)
				REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT uniq_class_handover_absences_student UNIQUE (handover_id, student_id)
		);

		COMMENT ON TABLE active.class_handover_absences IS
			'Children the Lehrkraft marked "nicht da" when she handed her class over to the OGS (#4794)';

		CREATE INDEX IF NOT EXISTS idx_class_handover_absences_tenant_student
			ON active.class_handover_absences (tenant_id, student_id);

		GRANT SELECT, INSERT, UPDATE, DELETE ON active.class_handovers TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE active.class_handovers_id_seq TO phoenix_tenant;
		GRANT SELECT, INSERT, UPDATE, DELETE ON active.class_handover_absences TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE active.class_handover_absences_id_seq TO phoenix_tenant;
`

// classHandoverWriteScopeUpSQL turns operations.school_portal_write_scope
// into a multi-select (#4794): a stored single value becomes a list that
// keeps it; "none" becomes the empty list.
const classHandoverWriteScopeUpSQL = `
		UPDATE config.setting_values
		SET value = CASE WHEN value = '"none"'::jsonb THEN '[]'::jsonb ELSE jsonb_build_array(value) END
		WHERE setting_key = 'operations.school_portal_write_scope' AND jsonb_typeof(value) = 'string';
`

// classHandoverWriteScopeDownSQL restores the single value: the class
// arrival exception when the list ticks it, else "none". A ticked class
// handover alone has no single-value form and falls back to "none".
const classHandoverWriteScopeDownSQL = `
		UPDATE config.setting_values
		SET value = CASE WHEN value @> '["class_arrival_exceptions"]'::jsonb
			THEN '"class_arrival_exceptions"'::jsonb ELSE '"none"'::jsonb END
		WHERE setting_key = 'operations.school_portal_write_scope' AND jsonb_typeof(value) = 'array';
`

const classHandoverDownSQL = `
		DROP TABLE IF EXISTS active.class_handover_absences;
		DROP TABLE IF EXISTS active.class_handovers;
`

// classHandoverUp creates the tables with tenant RLS, migrates the write
// scope to its list form, freezes the erasure phases anew with the tables and
// grants class_day:handover_write to the lehrkraft role. Like the other
// school-portal writes it opens a door without widening a scope: the route
// re-checks the class assignment, and the ticked write scope
// "class_handover" decides whether the door is unlocked.
func classHandoverUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackClassHandover(ctx, tx)
	if _, err := tx.ExecContext(ctx, classHandoverUpSQL); err != nil {
		return fmt.Errorf("create active.class_handovers: %w", err)
	}
	if _, err := tx.ExecContext(ctx, classHandoverWriteScopeUpSQL); err != nil {
		return fmt.Errorf("migrate operations.school_portal_write_scope to a list: %w", err)
	}
	if err := installClassHandoverErasure(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if err := provisionTenantRLS(ctx, db, "active.class_handovers", "active.class_handover_absences"); err != nil {
		return err
	}
	return grantPermissionToRoles(ctx, db, permissionSpec{
		Name:        "class_day:handover_write",
		Description: "Eine zugewiesene Klasse nach dem Unterricht an die OGS übergeben (moto schule)",
		Resource:    "class_day",
		Action:      "handover_write",
	}, "lehrkraft")
}

// classHandoverDown reinstalls the 1.15.491 erasure phases, restores the
// single-value write scope and drops the tables in one transaction.
func classHandoverDown(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackClassHandover(ctx, tx)
	if err := installMorningAttendanceErasure(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, classHandoverWriteScopeDownSQL); err != nil {
		return fmt.Errorf("restore operations.school_portal_write_scope: %w", err)
	}
	if _, err := tx.ExecContext(ctx, classHandoverDownSQL); err != nil {
		return fmt.Errorf("drop active.class_handovers: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return dropPermission(ctx, db, "class_day:handover_write")
}

func rollbackClassHandover(ctx context.Context, tx bun.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		logRollbackFailure(ctx, err)
	}
}
