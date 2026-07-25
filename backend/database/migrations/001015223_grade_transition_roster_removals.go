package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	// 1.15.222 is this branch's previous migration; 1.15.223 is the next unused
	// version so SafeMigrationMap.Register does not panic at package init on
	// merge.
	gradeTransitionRosterRemovalsVersion     = "1.15.223"
	gradeTransitionRosterRemovalsDescription = "Record which timetable roster rows a grade transition removed so a revert restores exactly those rows"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     gradeTransitionRosterRemovalsVersion,
		Description: gradeTransitionRosterRemovalsDescription,
		DependsOn: []string{
			gradeTransitionHistoryFromStatusVersion,
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
func gradeTransitionRosterRemovalsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.223: Creating schedule.grade_transition_roster_removals...")

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
			transition_id         BIGINT NOT NULL REFERENCES education.grade_transitions(id) ON DELETE CASCADE,
			instance_id           BIGINT NOT NULL REFERENCES schedule.activity_instances(id) ON DELETE CASCADE,
			student_id            BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			room_id               BIGINT,
			status                TEXT NOT NULL,
			substatus             TEXT,
			note                  TEXT,
			is_unplanned          BOOLEAN NOT NULL DEFAULT FALSE,
			not_scheduled         BOOLEAN NOT NULL DEFAULT FALSE,
			manual_status_at      TIMESTAMPTZ,
			student_status_day_id BIGINT,
			created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT unique_roster_removal UNIQUE (transition_id, instance_id, student_id)
		);

		COMMENT ON TABLE schedule.grade_transition_roster_removals IS
			'Snapshot of the schedule.instance_students rows a grade transition deleted on apply, replayed verbatim on revert (#405). room_id and student_status_day_id are deliberately FK-free: they are a snapshot, and the restore re-validates both so a since-deleted room or status day restores as NULL instead of failing the revert.';

		CREATE INDEX IF NOT EXISTS idx_roster_removals_tenant_transition
			ON schedule.grade_transition_roster_removals (tenant_id, transition_id);
		CREATE INDEX IF NOT EXISTS idx_roster_removals_student
			ON schedule.grade_transition_roster_removals (student_id);

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
	fmt.Println("Rolling back migration 1.15.223: Dropping schedule.grade_transition_roster_removals...")

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
