package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	withdrawParentArrivalRequestsVersion     = "1.15.296"
	withdrawParentArrivalRequestsDescription = "Withdraw pending parent arrival-time requests"
	withdrawParentArrivalRequestsReason      = "Bringzeiten werden ausschließlich von der OGS gepflegt."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     withdrawParentArrivalRequestsVersion,
		Description: withdrawParentArrivalRequestsDescription,
		DependsOn:   []string{parentCareRequestFieldSettingsVersion},
	})
	Migrations.MustRegister(withdrawParentArrivalRequestsUp, withdrawParentArrivalRequestsDown)
}

func withdrawParentArrivalRequestsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.296: Withdrawing pending parent arrival-time requests...")
	_, err := db.NewRaw(`
		UPDATE schedule.care_schedule_change_requests
		SET status = 'withdrawn', decision_reason = ?
		WHERE status = 'pending'
			AND jsonb_path_exists(
				payload,
				'$.weekdays[*] ? (@.arrival != null && @.arrival != "")'
			)
	`, withdrawParentArrivalRequestsReason).Exec(ctx)
	if err != nil {
		return fmt.Errorf("withdraw pending parent arrival-time requests: %w", err)
	}
	return nil
}

func withdrawParentArrivalRequestsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.296: Restoring eligible parent arrival-time requests...")
	_, err := db.NewRaw(`
		UPDATE schedule.care_schedule_change_requests AS legacy
		SET status = 'pending', decision_reason = NULL
		WHERE legacy.status = 'withdrawn'
			AND legacy.decision_reason = ?
			AND NOT EXISTS (
				SELECT 1
				FROM schedule.care_schedule_change_requests AS current
				WHERE current.tenant_id = legacy.tenant_id
					AND current.student_id = legacy.student_id
					AND current.status = 'pending'
			)
	`, withdrawParentArrivalRequestsReason).Exec(ctx)
	if err != nil {
		return fmt.Errorf("restore parent arrival-time requests: %w", err)
	}
	return nil
}
