package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	dutyBlockTypeVersion     = "1.15.441"
	dutyBlockTypeDescription = "Timetable block type duty without children and with an optional room (#3822)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dutyBlockTypeVersion,
		Description: dutyBlockTypeDescription,
		DependsOn:   []string{staffShiftSeriesSchoolBreaksVersion}, // preserves ladder order
	})

	Migrations.MustRegister(dutyBlockTypeUp, dutyBlockTypeDown)
}

// dutyBlockTypeUp adds the block type "duty" (UI „Dienst“, #3822): a staff
// task such as Busaufsicht or Essensausgabe that takes part in absences and
// substitutions but has no children, no session and no kiosk entry. A duty
// may have no room, so a planned occurrence may now carry NULL in room_id.
// The rule "only a duty may lack a room" lives in the timetable owner, which
// knows the template type; the column cannot see it.
func dutyBlockTypeUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE activities.groups DROP CONSTRAINT IF EXISTS check_group_type;
			ALTER TABLE activities.groups
				ADD CONSTRAINT check_group_type CHECK (type IN ('activity', 'care', 'external', 'duty'));
		`); err != nil {
			return fmt.Errorf("error widening check_group_type on activities.groups: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE schedule.activity_instances ALTER COLUMN room_id DROP NOT NULL;
		`); err != nil {
			return fmt.Errorf("error making schedule.activity_instances.room_id nullable: %w", err)
		}
		return nil
	})
}

// dutyBlockTypeDown refuses to run while duties or room-less occurrences
// exist: narrowing the constraints would otherwise have to delete or rewrite
// school data.
func dutyBlockTypeDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var blocked bool
		if err := tx.NewRaw(`SELECT EXISTS (SELECT 1 FROM activities.groups WHERE type = 'duty')
			OR EXISTS (SELECT 1 FROM schedule.activity_instances WHERE room_id IS NULL)`).Scan(ctx, &blocked); err != nil {
			return fmt.Errorf("error checking for duty blocks: %w", err)
		}
		if blocked {
			return errors.New("cannot roll back duty block type: duty templates or occurrences without a room exist")
		}
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE schedule.activity_instances ALTER COLUMN room_id SET NOT NULL;
			ALTER TABLE activities.groups DROP CONSTRAINT IF EXISTS check_group_type;
			ALTER TABLE activities.groups
				ADD CONSTRAINT check_group_type CHECK (type IN ('activity', 'care', 'external'));
		`); err != nil {
			return fmt.Errorf("error restoring duty block type constraints: %w", err)
		}
		return nil
	})
}
