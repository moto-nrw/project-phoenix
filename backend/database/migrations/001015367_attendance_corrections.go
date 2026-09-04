package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	attendanceCorrectionsVersion     = "1.15.367"
	attendanceCorrectionsDescription = "Audit trail for corrections to instance attendance (status, substatus, note) (#2898)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version: attendanceCorrectionsVersion, Description: attendanceCorrectionsDescription,
		DependsOn: []string{additionalSupervisionAuditVersion},
	})
	Migrations.MustRegister(attendanceCorrectionsUp, attendanceCorrectionsDown)
}

func attendanceCorrectionsUp(ctx context.Context, db *bun.DB) error {
	// UPDATE/DELETE/TRUNCATE are not granted here on purpose: migration
	// 1.15.225 narrowed ALTER DEFAULT PRIVILEGES on schema audit, so every new
	// audit table starts append-only for phoenix_tenant. Only SELECT and
	// INSERT are added back.
	//
	// instance_id and student_id cascade with their rows: once the instance or
	// the child is gone the correction has nothing left to describe. The actor
	// is ON DELETE SET NULL with a name snapshot so the trail survives an
	// account deletion instead of blocking it — same shape as
	// audit.guardian_changes.
	//
	// reason is NOT NULL and non-empty by constraint. A correction of a closed
	// record without a stated reason is weak evidence; the same rule already
	// governs manual work-session edits ("Notes are required to preserve the
	// audit trail's Verlässlichkeit", services/active/work_session_service.go).
	// Every field row of one correction carries the same reason.
	_, err := db.NewRaw(`
		CREATE TABLE audit.attendance_corrections (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			instance_id BIGINT NOT NULL REFERENCES schedule.activity_instances(id) ON DELETE CASCADE,
			student_id BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			actor_name_snapshot TEXT,
			field_name TEXT NOT NULL CHECK (field_name IN ('status', 'substatus', 'note')),
			old_value TEXT,
			new_value TEXT,
			reason TEXT NOT NULL CHECK (btrim(reason) <> ''),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX idx_attendance_corrections_entry
			ON audit.attendance_corrections (tenant_id, instance_id, student_id, created_at DESC);
		CREATE INDEX idx_attendance_corrections_student
			ON audit.attendance_corrections (tenant_id, student_id, created_at DESC);

		GRANT SELECT, INSERT ON audit.attendance_corrections TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE audit.attendance_corrections_id_seq TO phoenix_tenant;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create attendance correction audit trail: %w", err)
	}
	return provisionTenantRLS(ctx, db, "audit.attendance_corrections")
}

func attendanceCorrectionsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`DROP TABLE IF EXISTS audit.attendance_corrections;`).Exec(ctx); err != nil {
		return fmt.Errorf("drop attendance correction audit trail: %w", err)
	}
	return nil
}
