package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	spontaneousActivityInstanceUniquenessVersion     = "1.15.367"
	spontaneousActivityInstanceUniquenessDescription = "Exclude spontaneous activity-linked sessions from planned timetable uniqueness"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     spontaneousActivityInstanceUniquenessVersion,
		Description: spontaneousActivityInstanceUniquenessDescription,
		DependsOn:   []string{reclassifyPlannedSpontaneousInstancesVersion},
	})
	Migrations.MustRegister(spontaneousActivityInstanceUniquenessUp, spontaneousActivityInstanceUniquenessDown)
}

func spontaneousActivityInstanceUniquenessUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.367: Excluding spontaneous sessions from planned timetable uniqueness...")
	_, err := db.ExecContext(ctx, `
		DROP INDEX IF EXISTS schedule.idx_activity_instances_template_unique;
		CREATE UNIQUE INDEX idx_activity_instances_template_unique
			ON schedule.activity_instances (tenant_id, date, activity_group_id, start_time)
			WHERE activity_group_id IS NOT NULL AND is_spontaneous = FALSE;
	`)
	if err != nil {
		return fmt.Errorf("replace activity instance template uniqueness index: %w", err)
	}
	return nil
}

func spontaneousActivityInstanceUniquenessDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.367: Restoring activity-linked instance uniqueness...")

	// The newer index permits a spontaneous and a planned session to share a
	// template/date/start-time. That state cannot be represented by the old
	// index, so refuse the rollback before changing the active guard.
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var duplicateActivityLinkedInstances bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM schedule.activity_instances
				WHERE activity_group_id IS NOT NULL
				GROUP BY tenant_id, date, activity_group_id, start_time
				HAVING COUNT(*) > 1
			)
		`).Scan(&duplicateActivityLinkedInstances); err != nil {
			return fmt.Errorf("check activity instance uniqueness before rollback: %w", err)
		}
		if duplicateActivityLinkedInstances {
			return fmt.Errorf("cannot restore activity instance template uniqueness: duplicate activity-linked instances exist")
		}

		if _, err := tx.ExecContext(ctx, `
			DROP INDEX IF EXISTS schedule.idx_activity_instances_template_unique;
			CREATE UNIQUE INDEX idx_activity_instances_template_unique
				ON schedule.activity_instances (tenant_id, date, activity_group_id, start_time)
				WHERE activity_group_id IS NOT NULL;
		`); err != nil {
			return fmt.Errorf("restore activity instance template uniqueness index: %w", err)
		}
		return nil
	})
}
