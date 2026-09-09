package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const presenceExpandVersion = "1.15.376"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     presenceExpandVersion,
		Description: "Expand empty Presence session and attendance storage without switching Timetable callers (#2718)",
		DependsOn: []string{
			instanceStudentsTenantKeyVersion, activityInstancesCompositeFKsVersion,
			attendancePerCareSlotVersion, partialStudentAbsencesVersion,
			activityCompletionSnapshotVersion, createTenantRolesVersion,
		},
	})
	Migrations.MustRegister(presenceExpandUp, presenceExpandDown)
}

func presenceExpandUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Old rows, planning status, models and callers remain the sole authority.
		// Backfill (#2761) and cutover (#2762) are separate releases. Do not seed,
		// copy rows, add routing triggers, or narrow the old status constraint here.
		_, err := tx.ExecContext(ctx, `
			SET LOCAL lock_timeout = '5s';
			CREATE TABLE active.activity_sessions (
				id BIGSERIAL PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
				schedule_instance_id BIGINT NOT NULL,
				status TEXT NOT NULL DEFAULT 'active',
				active_group_id BIGINT,
				started_by BIGINT,
				started_at TIMESTAMPTZ,
				completed_at TIMESTAMPTZ,
				completed_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
				reopen_until TIMESTAMPTZ,
				completion_snapshot JSONB,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CONSTRAINT uq_activity_sessions_tenant_id UNIQUE (tenant_id, id),
				CONSTRAINT uq_activity_sessions_instance UNIQUE (tenant_id, schedule_instance_id),
				CONSTRAINT check_activity_sessions_status CHECK (status IN ('active', 'completed')),
				CONSTRAINT fk_activity_sessions_instance FOREIGN KEY (tenant_id, schedule_instance_id)
					REFERENCES schedule.activity_instances(tenant_id, id) ON DELETE CASCADE,
				CONSTRAINT fk_activity_sessions_active_group FOREIGN KEY (tenant_id, active_group_id)
					REFERENCES active.groups(tenant_id, id) ON DELETE SET NULL (active_group_id),
				CONSTRAINT fk_activity_sessions_started_by FOREIGN KEY (tenant_id, started_by)
					REFERENCES users.staff(tenant_id, id) ON DELETE SET NULL (started_by)
			);
			COMMENT ON TABLE active.activity_sessions IS
				'Presence-owned target. Empty during Expand #2718; schedule.activity_instances remains authoritative until #2762.';
			COMMENT ON COLUMN active.activity_sessions.completed_by IS
				'Global auth account identity, not a staff ID; preserves the old completion actor contract.';
			CREATE UNIQUE INDEX uq_activity_sessions_active_group ON active.activity_sessions (tenant_id, active_group_id)
				WHERE active_group_id IS NOT NULL;
			CREATE INDEX idx_activity_sessions_status ON active.activity_sessions (tenant_id, status);
			CREATE INDEX idx_activity_sessions_started_by ON active.activity_sessions (tenant_id, started_by)
				WHERE started_by IS NOT NULL;
			CREATE INDEX idx_activity_sessions_completed_by ON active.activity_sessions (completed_by)
				WHERE completed_by IS NOT NULL;
			CREATE INDEX idx_activity_sessions_reopenable ON active.activity_sessions (tenant_id, reopen_until)
				WHERE status = 'completed' AND reopen_until IS NOT NULL;

			CREATE TABLE active.activity_session_attendance (
				id BIGSERIAL PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
				instance_student_id BIGINT NOT NULL,
				status TEXT NOT NULL DEFAULT 'expected',
				substatus TEXT,
				note TEXT,
				checked_in_at TIMESTAMPTZ,
				checked_out_at TIMESTAMPTZ,
				is_unplanned BOOLEAN NOT NULL DEFAULT FALSE,
				not_scheduled BOOLEAN NOT NULL DEFAULT FALSE,
				manual_status_at TIMESTAMPTZ,
				student_status_day_id BIGINT,
				pickup_exception_id BIGINT,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CONSTRAINT uq_activity_session_attendance_tenant_id UNIQUE (tenant_id, id),
				CONSTRAINT uq_activity_session_attendance_participant UNIQUE (tenant_id, instance_student_id),
				CONSTRAINT check_activity_session_attendance_status CHECK (status IN ('expected', 'present', 'absent')),
				CONSTRAINT check_activity_session_attendance_substatus CHECK (
					substatus IS NULL OR substatus IN ('late', 'excused', 'sick', 'field_trip', 'other')),
				CONSTRAINT check_activity_session_attendance_note CHECK (note IS NULL OR char_length(note) <= 500),
				CONSTRAINT fk_activity_session_attendance_participant FOREIGN KEY (tenant_id, instance_student_id)
					REFERENCES schedule.instance_students(tenant_id, id) ON DELETE CASCADE,
				CONSTRAINT fk_activity_session_attendance_status_day FOREIGN KEY (tenant_id, student_status_day_id)
					REFERENCES active.student_status_days(tenant_id, id) ON DELETE SET NULL (student_status_day_id),
				CONSTRAINT fk_activity_session_attendance_pickup_exception FOREIGN KEY (tenant_id, pickup_exception_id)
					REFERENCES schedule.student_pickup_exceptions(tenant_id, id) ON DELETE SET NULL (pickup_exception_id)
			);
			COMMENT ON TABLE active.activity_session_attendance IS
				'Presence-owned target. Empty during Expand #2718; schedule.instance_students remains authoritative until #2762.';
			CREATE INDEX idx_activity_session_attendance_status ON active.activity_session_attendance (tenant_id, status);
			CREATE INDEX idx_activity_session_attendance_status_day ON active.activity_session_attendance (tenant_id, student_status_day_id)
				WHERE student_status_day_id IS NOT NULL;
			CREATE INDEX idx_activity_session_attendance_pickup_exception ON active.activity_session_attendance (tenant_id, pickup_exception_id)
				WHERE pickup_exception_id IS NOT NULL;
			GRANT SELECT, INSERT, UPDATE, DELETE ON active.activity_sessions, active.activity_session_attendance TO phoenix_tenant;
			GRANT ALL ON active.activity_sessions, active.activity_session_attendance TO phoenix_admin;
			GRANT USAGE, SELECT ON SEQUENCE active.activity_sessions_id_seq, active.activity_session_attendance_id_seq TO phoenix_tenant;
			GRANT ALL ON SEQUENCE active.activity_sessions_id_seq, active.activity_session_attendance_id_seq TO phoenix_admin;
		`)
		if err != nil {
			return fmt.Errorf("expand Presence storage: %w", err)
		}
		return provisionTenantRLS(ctx, tx, "active.activity_sessions", "active.activity_session_attendance")
	})
}

func presenceExpandDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Serialize the emptiness check with writers. After backfill starts,
		// rollback belongs to #2761, never to an unconditional DROP here.
		_, err := tx.ExecContext(ctx, `
			SET LOCAL lock_timeout = '5s';
			LOCK TABLE active.activity_sessions, active.activity_session_attendance IN ACCESS EXCLUSIVE MODE;
			DO $$ BEGIN
				IF EXISTS (SELECT FROM active.activity_sessions) OR EXISTS (SELECT FROM active.activity_session_attendance) THEN
					RAISE EXCEPTION 'Presence Expand rollback requires empty target tables';
				END IF;
			END $$;
			DROP TABLE active.activity_session_attendance;
			DROP TABLE active.activity_sessions;
		`)
		if err != nil {
			return fmt.Errorf("rollback Presence Expand: %w", err)
		}
		return nil
	})
}
