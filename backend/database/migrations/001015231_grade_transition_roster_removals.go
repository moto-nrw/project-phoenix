package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	// 1.15.230 is this branch's previous migration; see the version note there
	// for why this branch sits above development's 1.15.229.
	gradeTransitionRosterRemovalsVersion     = "1.15.231"
	gradeTransitionRosterRemovalsDescription = "Record which timetable roster rows a grade transition removed so a revert restores exactly those rows"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     gradeTransitionRosterRemovalsVersion,
		Description: gradeTransitionRosterRemovalsDescription,
		DependsOn: []string{
			gradeTransitionHistoryFromStatusVersion,
			// UNIQUE(tenant_id, id) on education.grade_transitions and
			// users.students, and on schedule.activity_instances respectively —
			// the parent indexes the composite FKs below reference.
			compositePKIndexesVersion,
			activityInstancesCompositeFKsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return gradeTransitionRosterRemovalsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return gradeTransitionRosterRemovalsDown(ctx, db)
		},
	)
}

// gradeTransitionRosterRemovalsUp creates the ledger of instance_students rows a
// grade transition deleted when it graduated a child (#405 review).
//
// Without it, a revert can only RECONSTRUCT future rosters from enrollments,
// which is not the inverse of the deletion: an occurrence a supervisor had
// deliberately customized loses that customization. A child removed from a
// single instance by hand comes back (the enrollment still covers the date),
// and a child added to a single instance by hand without a matching enrollment
// is deleted on apply and can never be recreated. Snapshotting the deleted rows
// makes the revert an exact inverse — every column that carried a decision
// (status, substatus, note, room, the unplanned / non-booking / manual-status
// markers, the owning status day) is replayed as it was.
//
// The table lives in `schedule` because every column except transition_id is a
// schedule row; transition_id references education.grade_transitions and
// cascades, so deleting a transition drops its ledger with it.
//
// Every parent reference is a COMPOSITE (tenant_id, x_id) foreign key, matching
// the project-wide pattern established by 1.14.4 / 1.15.2 / 1.15.51. A
// single-column FK would accept a ledger row carrying tenant A's tenant_id
// together with tenant B's transition, instance, or student — those ids are
// globally valid and the RLS policy only checks the ledger's own tenant_id — so
// a delete in one school could cascade into another school's data. The
// composite form makes the tenant boundary a database invariant instead of a
// convention (#405 review).
func gradeTransitionRosterRemovalsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.231: Creating schedule.grade_transition_roster_removals...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schedule.grade_transition_roster_removals (
			id                    BIGSERIAL PRIMARY KEY,
			tenant_id             BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			transition_id         BIGINT NOT NULL,
			instance_id           BIGINT NOT NULL,
			student_id            BIGINT NOT NULL,
			room_id               BIGINT,
			status                TEXT NOT NULL,
			substatus             TEXT,
			note                  TEXT,
			is_unplanned          BOOLEAN NOT NULL DEFAULT FALSE,
			not_scheduled         BOOLEAN NOT NULL DEFAULT FALSE,
			manual_status_at      TIMESTAMPTZ,
			student_status_day_id BIGINT,
			created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT unique_roster_removal UNIQUE (transition_id, instance_id, student_id),
			CONSTRAINT fk_roster_removals_transition
				FOREIGN KEY (tenant_id, transition_id)
				REFERENCES education.grade_transitions(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT fk_roster_removals_instance
				FOREIGN KEY (tenant_id, instance_id)
				REFERENCES schedule.activity_instances(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT fk_roster_removals_student
				FOREIGN KEY (tenant_id, student_id)
				REFERENCES users.students(tenant_id, id) ON DELETE CASCADE
		);

		COMMENT ON TABLE schedule.grade_transition_roster_removals IS
			'Snapshot of the schedule.instance_students rows a grade transition deleted on apply, replayed verbatim on revert (#405). room_id and student_status_day_id are deliberately FK-free: they are a snapshot, and the restore re-validates both so a since-deleted room or status day restores as NULL instead of failing the revert.';

		-- One index per composite FK: the referencing columns of an ON DELETE
		-- CASCADE child must be indexed or every parent delete seq-scans the
		-- ledger.
		CREATE INDEX IF NOT EXISTS idx_roster_removals_tenant_transition
			ON schedule.grade_transition_roster_removals (tenant_id, transition_id);
		CREATE INDEX IF NOT EXISTS idx_roster_removals_tenant_instance
			ON schedule.grade_transition_roster_removals (tenant_id, instance_id);
		CREATE INDEX IF NOT EXISTS idx_roster_removals_tenant_student
			ON schedule.grade_transition_roster_removals (tenant_id, student_id);

		ALTER TABLE schedule.grade_transition_roster_removals ENABLE ROW LEVEL SECURITY;
		ALTER TABLE schedule.grade_transition_roster_removals FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_schedule_roster_removals ON schedule.grade_transition_roster_removals;
		CREATE POLICY tenant_isolation_schedule_roster_removals ON schedule.grade_transition_roster_removals
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON schedule.grade_transition_roster_removals TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE schedule.grade_transition_roster_removals_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating schedule.grade_transition_roster_removals: %w", err)
	}

	return tx.Commit()
}

func gradeTransitionRosterRemovalsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.231: Dropping schedule.grade_transition_roster_removals...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS schedule.grade_transition_roster_removals CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping schedule.grade_transition_roster_removals: %w", err)
	}
	return tx.Commit()
}
