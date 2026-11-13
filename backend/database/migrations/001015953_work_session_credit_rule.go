package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const workSessionCreditRuleVersion = "1.15.953"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     workSessionCreditRuleVersion,
		Description: "Arbeitszeit: Wegpackzeit und Rundung je Arbeitsblock beim Ausstempeln festhalten (#6851, #6852)",
		DependsOn: []string{
			traegerMemberDisplayNameVersion, // the newest migration on development (1.15.952)
			workSessionsReopenedAtVersion,   // the last column added to active.work_sessions
		},
	})
	Migrations.MustRegister(workSessionCreditRuleUp, workSessionCreditRuleDown)
}

// A completed block keeps the credit rule of the school as it stood when the
// block was closed: the pack-up minutes of a regular checkout and the rounding
// of its duration. The credited time is computed when the block is read, from
// the raw stamps and this rule, so a later settings change never rewrites
// blocks closed before it and a correction of the stamps cannot count twice.
// The defaults describe every existing block: no pack-up time, no rounding.
const workSessionCreditRuleUpSQL = `ALTER TABLE active.work_sessions
	ADD COLUMN IF NOT EXISTS pack_up_minutes INTEGER NOT NULL DEFAULT 0
		CONSTRAINT work_sessions_pack_up_minutes_range CHECK (pack_up_minutes BETWEEN 0 AND 60),
	ADD COLUMN IF NOT EXISTS rounding_interval_minutes INTEGER NOT NULL DEFAULT 0
		CONSTRAINT work_sessions_rounding_interval_range CHECK (rounding_interval_minutes BETWEEN 0 AND 60),
	ADD COLUMN IF NOT EXISTS rounding_mode TEXT NOT NULL DEFAULT ''
		CONSTRAINT work_sessions_rounding_mode_valid CHECK (rounding_mode IN ('', 'nearest', 'up', 'down'));`

const workSessionCreditRuleDownSQL = `ALTER TABLE active.work_sessions
	DROP COLUMN IF EXISTS rounding_mode,
	DROP COLUMN IF EXISTS rounding_interval_minutes,
	DROP COLUMN IF EXISTS pack_up_minutes;`

func workSessionCreditRuleUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, workSessionCreditRuleUpSQL); err != nil {
		return fmt.Errorf("add credit rule to active.work_sessions: %w", err)
	}
	return nil
}

func workSessionCreditRuleDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, workSessionCreditRuleDownSQL); err != nil {
		return fmt.Errorf("drop credit rule from active.work_sessions: %w", err)
	}
	return nil
}
