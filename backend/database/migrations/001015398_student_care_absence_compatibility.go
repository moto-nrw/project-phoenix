package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const studentCareAbsenceCompatibilityVersion = "1.15.398"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentCareAbsenceCompatibilityVersion,
		Description: "Move live student absence compatibility fields to Care Plan (#2759)",
		DependsOn:   []string{studentOwnerCutoverVersion},
	})
	Migrations.MustRegister(studentCareAbsenceCompatibilityUp, studentCareAbsenceCompatibilityDown)
}

func studentCareAbsenceCompatibilityUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `
			SET LOCAL lock_timeout = '5s';
			LOCK TABLE users.student_profiles, users.students_legacy,
				users.student_school_memberships, users.student_care_profiles IN ACCESS EXCLUSIVE MODE;
			ALTER TABLE users.student_care_profiles
				ADD COLUMN sick BOOLEAN NOT NULL DEFAULT false,
				ADD COLUMN sick_since TIMESTAMP,
				ADD COLUMN excused BOOLEAN NOT NULL DEFAULT false,
				ADD COLUMN excused_since TIMESTAMP;
			-- Moving storage must not change the public modification timestamp.
			ALTER TABLE users.student_care_profiles DISABLE TRIGGER update_student_care_profiles_updated_at;
			UPDATE users.student_care_profiles AS c SET
				sick = COALESCE(a.sick, false), sick_since = a.sick_since,
				excused = COALESCE(a.excused, false), excused_since = a.excused_since
			FROM users.student_school_memberships AS m
			JOIN users.students_legacy AS a ON a.tenant_id = m.tenant_id AND a.id = m.student_profile_id
			WHERE c.tenant_id = m.tenant_id AND c.membership_id = m.id;
			ALTER TABLE users.student_care_profiles ENABLE TRIGGER update_student_care_profiles_updated_at;
			DO $$ BEGIN
				IF EXISTS (
					SELECT FROM users.student_care_profiles AS c
					JOIN users.student_school_memberships AS m ON m.tenant_id = c.tenant_id AND m.id = c.membership_id
					JOIN users.students_legacy AS a ON a.tenant_id = m.tenant_id AND a.id = m.student_profile_id
					WHERE (c.sick, c.sick_since, c.excused, c.excused_since)
						IS DISTINCT FROM (COALESCE(a.sick, false), a.sick_since, COALESCE(a.excused, false), a.excused_since)
				) THEN RAISE EXCEPTION 'student care absence compatibility backfill mismatch'; END IF;
			END $$;
		`)
		if err != nil {
			return fmt.Errorf("move student absence compatibility: %w", err)
		}
		return refreshStudentOwnerCompatibility(ctx, tx)
	})
}

func studentCareAbsenceCompatibilityDown(context.Context, *bun.DB) error {
	return errors.New("student care rollback deploys the previous image against the retained compatibility view")
}
