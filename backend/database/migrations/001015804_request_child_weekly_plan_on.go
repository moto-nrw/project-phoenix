package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const requestChildWeeklyPlanOnVersion = "1.15.804"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     requestChildWeeklyPlanOnVersion,
		Description: "Planned weekly plan of an approved future re-enrollment on enrollment.request_children (#6043)",
		DependsOn:   []string{traegerDocumentDeletionVersion}, // the last migration before this one
	})
	Migrations.MustRegister(requestChildWeeklyPlanOnUp, requestChildWeeklyPlanOnDown)
}

// requestChildWeeklyPlanOnUp lets an approved re-enrollment plan its weekly
// plan instead of writing it at once (#6043). A renewal of a child the
// school still cares for, approved before its care starts, keeps the running
// weekly pickup times, care days and Heimweg until the day before
// weekly_plan_on. On that day the rollover tick replaces them with the
// approved submission and clears the plan. weekly_plan_replaces records a
// confirmed replacing edit before the start: the start then applies the
// submission as the complete weekly plan, so a removed field clears it.
func requestChildWeeklyPlanOnUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE enrollment.request_children
				ADD COLUMN IF NOT EXISTS weekly_plan_on DATE,
				ADD COLUMN IF NOT EXISTS weekly_plan_replaces BOOLEAN NOT NULL DEFAULT FALSE;
			CREATE INDEX IF NOT EXISTS idx_request_children_weekly_plan_on
				ON enrollment.request_children (tenant_id, weekly_plan_on)
				WHERE weekly_plan_on IS NOT NULL;
		`); err != nil {
			return fmt.Errorf("error adding weekly plan columns to enrollment.request_children: %w", err)
		}
		return nil
	})
}

// requestChildWeeklyPlanOnDown drops the planned weekly plans. Pending plans
// are lost; the children keep their running weekly plan.
func requestChildWeeklyPlanOnDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			DROP INDEX IF EXISTS enrollment.idx_request_children_weekly_plan_on;
			ALTER TABLE enrollment.request_children
				DROP COLUMN IF EXISTS weekly_plan_replaces,
				DROP COLUMN IF EXISTS weekly_plan_on;
		`); err != nil {
			return fmt.Errorf("error dropping weekly plan columns from enrollment.request_children: %w", err)
		}
		return nil
	})
}
