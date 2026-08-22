package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentCareExitRemovalsVersion     = "1.15.316"
	studentCareExitRemovalsDescription = "Record what ending a child's care removed, so changing or cancelling a planned exit puts it back (#2487)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentCareExitRemovalsVersion,
		Description: studentCareExitRemovalsDescription,
		DependsOn: []string{
			studentCareExitsVersion,
			// UNIQUE(tenant_id, id) on users.students, schedule.activity_instances
			// and activities.groups — the parent indexes the composite FKs below
			// reference.
			compositePKIndexesVersion,
			activityInstancesCompositeFKsVersion,
		},
	})

	Migrations.MustRegister(studentCareExitRemovalsUp, studentCareExitRemovalsDown)
}

// studentCareExitRemovalsUp creates the ledger that makes a planned exit
// reversible (#2487).
//
// "Betreuung beenden" reconciles the plan the moment the school decides it:
// future roster rows lose the child and future offering bookings end. That is
// deliberate — the planning screens must stop showing a child on days they will
// not attend. But the acceptance criteria also say a not-yet-effective exit can
// be CHANGED or CANCELLED, and a cancellation that leaves the child active with
// an emptied plan is not a cancellation. Without this table the removals are
// gone for good: the roster rows were deleted, and a booking that started after
// the cutoff was deleted with them.
//
// So every removal is written down first, and Cancel — plus every re-run of
// Confirm, which restores to the "no exit" baseline before applying the new
// last care day — replays it. Resume deliberately does NOT: the criteria
// require that a returning child is planned again by hand, so its ledger rows
// are dropped unreplayed. So are the ones of an exit that has taken effect.
//
// One table, two kinds of removal:
//
//   - kind='roster' snapshots a deleted schedule.instance_students row, column
//     for column, exactly like schedule.grade_transition_roster_removals does
//     for the grade transition (1.15.236). A restore has to be the inverse of
//     the deletion, not a reconstruction from enrollments: an occurrence a
//     supervisor customised by hand would otherwise come back plain.
//   - kind='booking' remembers an activities.student_enrollments row that was
//     capped (previous_valid_until, possibly NULL for an open-ended booking) or
//     deleted outright (was_deleted, plus the columns needed to write it back
//     under its ORIGINAL id, so anything referencing that id stays valid).
//
// Parent references are COMPOSITE (tenant_id, x_id) foreign keys, the
// project-wide pattern: a single-column FK would accept a ledger row carrying
// tenant A's tenant_id next to tenant B's student or instance.
//
// enrollment_id is deliberately FK-free — a deleted booking has no row left to
// point at. room_id, student_status_day_id and calendar_period_id are FK-free
// for the same reason the grade-transition ledger keeps them so: they are a
// snapshot, and the restore re-validates them.
func studentCareExitRemovalsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.316: Creating users.student_care_exit_removals...")

	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS users.student_care_exit_removals (
			id                    BIGSERIAL PRIMARY KEY,
			tenant_id             BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id            BIGINT NOT NULL,
			kind                  TEXT NOT NULL CHECK (kind IN ('roster', 'booking')),

			-- kind='roster': the deleted schedule.instance_students row.
			instance_id           BIGINT,
			room_id               BIGINT,
			status                TEXT,
			substatus             TEXT,
			note                  TEXT,
			is_unplanned          BOOLEAN,
			not_scheduled         BOOLEAN,
			manual_status_at      TIMESTAMPTZ,
			student_status_day_id BIGINT,
			pickup_exception_id   BIGINT,

			-- kind='booking': the capped or deleted activities.student_enrollments row.
			enrollment_id            BIGINT,
			was_deleted              BOOLEAN NOT NULL DEFAULT FALSE,
			previous_valid_until     DATE,
			activity_group_id        BIGINT,
			valid_from               DATE,
			calendar_period_id       BIGINT,
			enrollment_request_child_id BIGINT,
			selected_weekdays        JSONB,
			attendance_status        TEXT,
			weekday                  SMALLINT,

			created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),

			-- A ledger row says what it is. Without these two the restore would
			-- have to guess which half of the row carries the data.
			CONSTRAINT chk_care_exit_removal_roster CHECK (
				kind <> 'roster' OR (instance_id IS NOT NULL AND status IS NOT NULL)
			),
			CONSTRAINT chk_care_exit_removal_booking CHECK (
				kind <> 'booking' OR (enrollment_id IS NOT NULL AND (was_deleted = FALSE OR (activity_group_id IS NOT NULL AND valid_from IS NOT NULL)))
			),

			-- One ledger entry per removed thing. A second "Betreuung beenden"
			-- run over the same child restores first, so a duplicate would mean
			-- the same row was removed twice without being put back.
			CONSTRAINT uq_care_exit_removal_roster UNIQUE (tenant_id, student_id, instance_id),
			CONSTRAINT uq_care_exit_removal_booking UNIQUE (tenant_id, student_id, enrollment_id),

			CONSTRAINT fk_care_exit_removal_student
				FOREIGN KEY (tenant_id, student_id)
				REFERENCES users.students(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT fk_care_exit_removal_instance
				FOREIGN KEY (tenant_id, instance_id)
				REFERENCES schedule.activity_instances(tenant_id, id) ON DELETE CASCADE
		);

		COMMENT ON TABLE users.student_care_exit_removals IS
			'What ending a child''s care removed from the plan (#2487): deleted roster rows and capped/deleted offering bookings. Replayed when a not-yet-effective exit is changed or cancelled; discarded on resume and once the exit takes effect.';

		-- One index per composite FK: the referencing columns of an ON DELETE
		-- CASCADE child must be indexed or every parent delete seq-scans the
		-- ledger.
		CREATE INDEX IF NOT EXISTS idx_care_exit_removals_tenant_student
			ON users.student_care_exit_removals (tenant_id, student_id);
		CREATE INDEX IF NOT EXISTS idx_care_exit_removals_tenant_instance
			ON users.student_care_exit_removals (tenant_id, instance_id);

		GRANT SELECT, INSERT, UPDATE, DELETE ON users.student_care_exit_removals TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.student_care_exit_removals_id_seq TO phoenix_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating users.student_care_exit_removals: %w", err)
	}
	if err := provisionTenantRLS(ctx, db, "users.student_care_exit_removals"); err != nil {
		return err
	}

	return nil
}

func studentCareExitRemovalsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back 1.15.316: Dropping users.student_care_exit_removals...")
	if _, err := db.NewRaw(`DROP TABLE IF EXISTS users.student_care_exit_removals;`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping users.student_care_exit_removals: %w", err)
	}
	return nil
}
