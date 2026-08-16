package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	pickupChangeRequestsVersion     = "1.15.297"
	pickupChangeRequestsDescription = "Separate weekly and one-day pickup change requests"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     pickupChangeRequestsVersion,
		Description: pickupChangeRequestsDescription,
		DependsOn:   []string{withdrawParentArrivalRequestsVersion},
	})
	Migrations.MustRegister(pickupChangeRequestsUp, pickupChangeRequestsDown)
}

func pickupChangeRequestsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.297: Adding pickup change request type...")
	_, err := db.ExecContext(ctx, `
		ALTER TABLE schedule.care_schedule_change_requests
			ADD COLUMN IF NOT EXISTS request_kind TEXT NOT NULL DEFAULT 'weekly_schedule';
		ALTER TABLE schedule.care_schedule_change_requests
			DROP CONSTRAINT IF EXISTS care_schedule_change_requests_request_kind_check;
		ALTER TABLE schedule.care_schedule_change_requests
			ADD CONSTRAINT care_schedule_change_requests_request_kind_check
			CHECK (request_kind IN ('weekly_schedule', 'pickup_change'));
		DROP INDEX IF EXISTS schedule.uniq_care_schedule_change_requests_pending;
		CREATE UNIQUE INDEX uniq_care_schedule_change_requests_pending
			ON schedule.care_schedule_change_requests (tenant_id, student_id, request_kind)
			WHERE status = 'pending';
		CREATE INDEX IF NOT EXISTS idx_care_schedule_change_requests_kind_status
			ON schedule.care_schedule_change_requests (tenant_id, request_kind, status, created_at DESC);
	`)
	if err != nil {
		return fmt.Errorf("add pickup change request type: %w", err)
	}
	return nil
}

func pickupChangeRequestsDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		UPDATE schedule.care_schedule_change_requests
		SET status = 'withdrawn', decision_reason = 'Rollback: Abholanfrage zurückgezogen.'
		WHERE request_kind = 'pickup_change' AND status = 'pending';
		DROP INDEX IF EXISTS schedule.idx_care_schedule_change_requests_kind_status;
		DROP INDEX IF EXISTS schedule.uniq_care_schedule_change_requests_pending;
		CREATE UNIQUE INDEX uniq_care_schedule_change_requests_pending
			ON schedule.care_schedule_change_requests (tenant_id, student_id)
			WHERE status = 'pending';
		ALTER TABLE schedule.care_schedule_change_requests
			DROP CONSTRAINT IF EXISTS care_schedule_change_requests_request_kind_check;
		ALTER TABLE schedule.care_schedule_change_requests
			DROP COLUMN IF EXISTS request_kind;
	`)
	if err != nil {
		return fmt.Errorf("remove pickup change request type: %w", err)
	}
	return nil
}
