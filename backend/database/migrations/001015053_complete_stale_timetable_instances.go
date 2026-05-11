package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	completeStaleTimetableInstancesVersion     = "1.15.53"
	completeStaleTimetableInstancesDescription = "Complete timetable instances whose active session already ended"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     completeStaleTimetableInstancesVersion,
		Description: completeStaleTimetableInstancesDescription,
		DependsOn: []string{
			activityInstancesCompositeFKsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return completeStaleTimetableInstancesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return completeStaleTimetableInstancesDown(ctx, db)
		},
	)
}

func completeStaleTimetableInstancesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.53: Completing stale timetable instances linked to ended active sessions...")

	_, err := db.NewRaw(`
		WITH stale_instances AS (
			SELECT ai.id, ag.end_time
			FROM schedule.activity_instances AS ai
			JOIN active.groups AS ag
				ON ag.id = ai.active_group_id
				AND ag.tenant_id = ai.tenant_id
			WHERE ai.status = 'active'
				AND ai.active_group_id IS NOT NULL
				AND ag.end_time IS NOT NULL
		), absent_students AS (
			UPDATE schedule.instance_students AS student
			SET status = 'absent',
				updated_at = NOW()
			FROM stale_instances
			WHERE student.instance_id = stale_instances.id
				AND student.status = 'expected'
			RETURNING student.id
		)
		UPDATE schedule.activity_instances AS ai
		SET status = 'completed',
			completed_at = stale_instances.end_time,
			updated_at = NOW()
		FROM stale_instances
		WHERE ai.id = stale_instances.id;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed completing stale timetable instances: %w", err)
	}

	return nil
}

func completeStaleTimetableInstancesDown(_ context.Context, _ *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.53: no-op for stale timetable completion repair")
	return nil
}
