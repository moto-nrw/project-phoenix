package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	requestChildClassSwitchVersion     = "1.15.442"
	requestChildClassSwitchDescription = "Planned class switch of an approved re-enrollment on enrollment.request_children (#3917)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     requestChildClassSwitchVersion,
		Description: requestChildClassSwitchDescription,
		DependsOn:   []string{dutyBlockTypeVersion}, // preserves ladder order
	})

	Migrations.MustRegister(requestChildClassSwitchUp, requestChildClassSwitchDown)
}

// requestChildClassSwitchUp lets an approved re-enrollment plan the child's
// class switch instead of writing it at once (#3917). Approving next school
// year's re-enrollment of a child the school still cares for keeps the
// running class until the new year starts: class_switch_from is the class
// the child had at approval, class_switch_to the class it moves into and
// class_switch_on the first day of the new school year. The rollover tick
// applies the switch on that day, and only while the child still sits in
// class_switch_from; a manual edit or grade transition in between wins.
func requestChildClassSwitchUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE enrollment.request_children
				ADD COLUMN IF NOT EXISTS class_switch_from TEXT,
				ADD COLUMN IF NOT EXISTS class_switch_to TEXT,
				ADD COLUMN IF NOT EXISTS class_switch_on DATE;
			ALTER TABLE enrollment.request_children DROP CONSTRAINT IF EXISTS request_children_class_switch_complete;
			ALTER TABLE enrollment.request_children
				ADD CONSTRAINT request_children_class_switch_complete CHECK (
					(class_switch_from IS NULL AND class_switch_to IS NULL AND class_switch_on IS NULL)
					OR (class_switch_from IS NOT NULL AND class_switch_to IS NOT NULL AND class_switch_on IS NOT NULL)
				);
			CREATE INDEX IF NOT EXISTS idx_request_children_class_switch_on
				ON enrollment.request_children (tenant_id, class_switch_on)
				WHERE class_switch_on IS NOT NULL;
		`); err != nil {
			return fmt.Errorf("error adding class switch columns to enrollment.request_children: %w", err)
		}
		return nil
	})
}

// requestChildClassSwitchDown drops the planned switches. Pending switches are
// lost; the children keep their running class.
func requestChildClassSwitchDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			DROP INDEX IF EXISTS enrollment.idx_request_children_class_switch_on;
			ALTER TABLE enrollment.request_children DROP CONSTRAINT IF EXISTS request_children_class_switch_complete;
			ALTER TABLE enrollment.request_children
				DROP COLUMN IF EXISTS class_switch_on,
				DROP COLUMN IF EXISTS class_switch_to,
				DROP COLUMN IF EXISTS class_switch_from;
		`); err != nil {
			return fmt.Errorf("error dropping class switch columns from enrollment.request_children: %w", err)
		}
		return nil
	})
}
