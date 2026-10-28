package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffPlannedExitsVersion     = "1.15.701"
	staffPlannedExitsDescription = "Geplanter Austritt: letzter Arbeitstag einer Person, nach dem der Austritt automatisch läuft (#4809)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffPlannedExitsVersion,
		Description: staffPlannedExitsDescription,
		DependsOn: []string{
			manualRosterConfirmedVersion, // the last migration before this one
			"1.15.376",                   // users.staff_school_memberships, the membership the exit belongs to
		},
	})
	Migrations.MustRegister(staffPlannedExitsUp, staffPlannedExitsDown)
}

// staffPlannedExitsUpSQL stores the last working day the Leitung entered on
// the live membership itself (#4809), so the planned exit shares the
// membership's tenant isolation, school erasure and retention. The
// retirement clears the columns, whether the scheduler or an immediate
// deletion retires the member. The planner is kept so the scheduled
// offboarding acts in the name of the person who planned it; no foreign key,
// like the other actor columns, so a later account erasure does not block.
// The notice and attempt columns are the scheduler's progress: each reminder
// is sent once, a blocked offboarding is retried at exit_next_attempt_at, and
// exit_access_locked_at records that sign-in was closed at the end of the day
// although the offboarding itself was still blocked.
const staffPlannedExitsUpSQL = `
	ALTER TABLE users.staff_school_memberships
		ADD COLUMN planned_exit_day DATE,
		ADD COLUMN planned_exit_by_account_id BIGINT,
		ADD COLUMN planned_exit_by_staff_id BIGINT,
		ADD COLUMN planned_exit_by_name TEXT,
		ADD COLUMN planned_exit_at TIMESTAMPTZ,
		ADD COLUMN exit_week_notice_sent_at TIMESTAMPTZ,
		ADD COLUMN exit_day_notice_sent_at TIMESTAMPTZ,
		ADD COLUMN exit_blocked_notice_sent_at TIMESTAMPTZ,
		ADD COLUMN exit_access_locked_at TIMESTAMPTZ,
		ADD COLUMN exit_next_attempt_at TIMESTAMPTZ,
		ADD CONSTRAINT chk_staff_school_memberships_planned_exit CHECK (
			(planned_exit_day IS NULL AND planned_exit_by_account_id IS NULL AND planned_exit_by_staff_id IS NULL
				AND planned_exit_by_name IS NULL AND planned_exit_at IS NULL AND exit_week_notice_sent_at IS NULL
				AND exit_day_notice_sent_at IS NULL AND exit_blocked_notice_sent_at IS NULL
				AND exit_access_locked_at IS NULL AND exit_next_attempt_at IS NULL)
			OR (planned_exit_day IS NOT NULL AND planned_exit_by_account_id IS NOT NULL
				AND planned_exit_by_staff_id IS NOT NULL AND planned_exit_by_name IS NOT NULL
				AND planned_exit_at IS NOT NULL)
		);
	CREATE INDEX idx_staff_school_memberships_planned_exit
		ON users.staff_school_memberships (tenant_id, planned_exit_day)
		WHERE planned_exit_day IS NOT NULL;
	COMMENT ON COLUMN users.staff_school_memberships.planned_exit_day IS
		'Planned last working day; the scheduler offboards the member after the end of that day (Berlin) (#4809).';
`

func staffPlannedExitsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, staffPlannedExitsUpSQL); err != nil {
		return fmt.Errorf("add the planned exit to users.staff_school_memberships: %w", err)
	}
	return nil
}

func staffPlannedExitsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_staff_school_memberships_planned_exit;
		ALTER TABLE users.staff_school_memberships
			DROP CONSTRAINT IF EXISTS chk_staff_school_memberships_planned_exit,
			DROP COLUMN IF EXISTS planned_exit_day,
			DROP COLUMN IF EXISTS planned_exit_by_account_id,
			DROP COLUMN IF EXISTS planned_exit_by_staff_id,
			DROP COLUMN IF EXISTS planned_exit_by_name,
			DROP COLUMN IF EXISTS planned_exit_at,
			DROP COLUMN IF EXISTS exit_week_notice_sent_at,
			DROP COLUMN IF EXISTS exit_day_notice_sent_at,
			DROP COLUMN IF EXISTS exit_blocked_notice_sent_at,
			DROP COLUMN IF EXISTS exit_access_locked_at,
			DROP COLUMN IF EXISTS exit_next_attempt_at;`); err != nil {
		return fmt.Errorf("drop the planned exit from users.staff_school_memberships: %w", err)
	}
	return nil
}
