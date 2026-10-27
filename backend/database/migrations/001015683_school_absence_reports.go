package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	schoolAbsenceReportsVersion     = "1.15.683"
	schoolAbsenceReportsDescription = "Schulverwaltung trägt für ein Kind, das ohne Meldung fehlt, eine Meldung nach (#4791)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     schoolAbsenceReportsVersion,
		Description: schoolAbsenceReportsDescription,
		DependsOn: []string{
			classParentLettersVersion,            // the last migration before this one
			studentStatusDaysParentSourceVersion, // the source check this migration widens
			schulverwaltungRoleVersion,           // the role the write permission goes to
		},
	})
	Migrations.MustRegister(schoolAbsenceReportsUp, schoolAbsenceReportsDown)
}

// schoolAbsenceReportsUpSQL lets a status day name the school as its source
// (#4791): the Schulverwaltung entered it in moto schule for a child the
// Lehrkraft marked absent without a report. The row is an ordinary sick or
// excused day of the OGS; only the source tells who may take it back.
const schoolAbsenceReportsUpSQL = `
		ALTER TABLE active.student_status_days
			DROP CONSTRAINT IF EXISTS chk_student_status_days_source;
		ALTER TABLE active.student_status_days
			ADD CONSTRAINT chk_student_status_days_source
			CHECK (source IN ('manual', 'planned', 'next_checkin', 'end_of_day', 'parent', 'school'));
`

// schoolAbsenceReportsDownSQL turns the school's rows into planned ones, the
// source the same write used before, and restores the check of 1.15.115.
const schoolAbsenceReportsDownSQL = `
		UPDATE active.student_status_days
		SET source = 'planned'
		WHERE source = 'school';
		ALTER TABLE active.student_status_days
			DROP CONSTRAINT IF EXISTS chk_student_status_days_source;
		ALTER TABLE active.student_status_days
			ADD CONSTRAINT chk_student_status_days_source
			CHECK (source IN ('manual', 'planned', 'next_checkin', 'end_of_day', 'parent'));
`

// schoolAbsenceReportsUp widens the source check and grants
// class_day:absence_report_write to the system role schulverwaltung alone.
// The Lehrkraft marks the morning; reporting a child sick or excused
// afterwards is the office's task (decision on #4791). The route re-checks
// the class reach; a school's own role of the same name gets nothing.
func schoolAbsenceReportsUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, schoolAbsenceReportsUpSQL); err != nil {
			return fmt.Errorf("allow the school as a status day source: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO auth.permissions (name, description, resource, action)
			VALUES ('class_day:absence_report_write', 'Für ein Kind, das ohne Meldung fehlt, eine Meldung nachtragen (moto schule)', 'class_day', 'absence_report_write')
			ON CONFLICT (name) DO NOTHING`); err != nil {
			return fmt.Errorf("insert class_day:absence_report_write: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO auth.role_permissions (role_id, permission_id)
			SELECT r.id, p.id
			FROM auth.roles r
			JOIN auth.permissions p ON p.name = 'class_day:absence_report_write'
			WHERE r.name = 'schulverwaltung' AND r.tenant_id IS NULL
			ON CONFLICT (role_id, permission_id) DO NOTHING`); err != nil {
			return fmt.Errorf("grant class_day:absence_report_write to the schulverwaltung role: %w", err)
		}
		return nil
	})
}

// schoolAbsenceReportsDown revokes the permission and restores the source
// check; the school's status days stay as planned ones.
func schoolAbsenceReportsDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM auth.role_permissions
			WHERE permission_id IN (SELECT id FROM auth.permissions WHERE name = 'class_day:absence_report_write')`); err != nil {
			return fmt.Errorf("revoke class_day:absence_report_write: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM auth.permissions WHERE name = 'class_day:absence_report_write'`); err != nil {
			return fmt.Errorf("delete class_day:absence_report_write: %w", err)
		}
		if _, err := tx.ExecContext(ctx, schoolAbsenceReportsDownSQL); err != nil {
			return fmt.Errorf("restore the status day source check: %w", err)
		}
		return nil
	})
}
