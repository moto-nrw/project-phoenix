package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const studentStatusDaysOneStandingVersion = "20261113121312"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentStatusDaysOneStandingVersion,
		Description: "Status-Tage: höchstens ein stehender Status je Kind und Tag (#6958)",
		DependsOn: []string{
			studentStatusDaysClassTripVersion, // allows class_trip beside sick and excused (1.15.118)
			schoolAbsenceReportsVersion,       // the current source values (1.15.683)
			presenceExpandVersion,             // activity_sessions, activity_session_attendance.student_status_day_id (1.15.378)
			statusDayProvenanceVersion,        // cleared_source decides whether an archive stands (20261113102344)
		},
	})
	Migrations.MustRegister(studentStatusDaysOneStandingUp, studentStatusDaysOneStandingDown)
}

// studentStatusDaysOneStandingUpSQL gives a child at most one standing status
// (sick, excused, class trip) per day (#6958). A row stands while it is not
// cleared, and an end-of-day archive stands for its day although it carries
// cleared_at, until a clearing records its cleared_source (standingStatusDaySQL
// in the Care Plan store).
//
// Before the index, every day with several standing rows keeps one: a row
// not cleared before an end-of-day archive, then the latest report, as a new
// report replaces the earlier one at runtime. The others are cleared at the
// moment that report came in, with the kept report's source as cleared_source
// and no account; their own source stays (#6842). Session attendance
// of a block not yet completed that pointed at a cleared row points at the
// kept one with its substatus; a completed block keeps its absence as the
// runtime release does (ReleaseStatusDay).
const studentStatusDaysOneStandingUpSQL = `
WITH ranked AS (
	SELECT id, tenant_id, status, reported_at,
		first_value(id) OVER day_order AS kept_id,
		first_value(status) OVER day_order AS kept_status,
		first_value(reported_at) OVER day_order AS kept_reported_at,
		first_value(source) OVER day_order AS kept_source,
		row_number() OVER day_order AS position
	FROM active.student_status_days
	WHERE cleared_at IS NULL OR (source = 'end_of_day' AND cleared_source IS NULL)
	WINDOW day_order AS (
		PARTITION BY tenant_id, student_id, date
		ORDER BY (cleared_at IS NULL) DESC, reported_at DESC, id DESC
	)
),
replaced AS (
	SELECT * FROM ranked WHERE position > 1
),
repointed AS (
	UPDATE active.activity_session_attendance AS attendance SET
		student_status_day_id = replaced.kept_id,
		substatus = CASE replaced.kept_status WHEN 'class_trip' THEN 'field_trip' ELSE replaced.kept_status END,
		updated_at = NOW()
	FROM replaced
	WHERE attendance.tenant_id = replaced.tenant_id AND attendance.student_status_day_id = replaced.id
		AND NOT EXISTS (
			SELECT 1
			FROM schedule.instance_students AS participant
			JOIN active.activity_sessions AS session
				ON session.tenant_id = participant.tenant_id AND session.schedule_instance_id = participant.instance_id
			WHERE participant.tenant_id = attendance.tenant_id AND participant.id = attendance.instance_student_id
				AND session.status = 'completed'
		)
)
UPDATE active.student_status_days AS day SET
	cleared_at = COALESCE(day.cleared_at, GREATEST(replaced.kept_reported_at, day.reported_at)),
	cleared_source = replaced.kept_source,
	cleared_by_account_id = NULL
FROM replaced
WHERE day.id = replaced.id;

CREATE UNIQUE INDEX IF NOT EXISTS uq_student_status_days_one_standing
	ON active.student_status_days (tenant_id, student_id, date)
	WHERE cleared_at IS NULL OR (source = 'end_of_day' AND cleared_source IS NULL);
`

const studentStatusDaysOneStandingDownSQL = `DROP INDEX IF EXISTS active.uq_student_status_days_one_standing;`

func studentStatusDaysOneStandingUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin the one standing status day migration: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			logRollbackFailure(ctx, err)
		}
	}()
	if _, err := tx.ExecContext(ctx, studentStatusDaysOneStandingUpSQL); err != nil {
		return fmt.Errorf("keep one standing status day per child and day: %w", err)
	}
	return tx.Commit()
}

func studentStatusDaysOneStandingDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, studentStatusDaysOneStandingDownSQL); err != nil {
		return fmt.Errorf("drop the one standing status day index: %w", err)
	}
	return nil
}
