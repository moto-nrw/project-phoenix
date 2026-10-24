package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	pickupPermissionRequestsVersion     = "1.15.527"
	pickupPermissionRequestsDescription = "Pickup permission weekdays and signed pickup permission requests (#4780)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     pickupPermissionRequestsVersion,
		Description: pickupPermissionRequestsDescription,
		DependsOn:   []string{pickupChangeRequestsVersion, guardianOwnerContractVersion, signaturesVersion},
	})
	Migrations.MustRegister(pickupPermissionRequestsUp, pickupPermissionRequestsDown)
}

// pickupPermissionRequestsUp adds the two halves of #4780.
//
//   - Care Plan's pickup permission of a relationship gets the weekdays it
//     holds on (1 = Monday … 5 = Friday). NULL means every day, the meaning
//     every existing row keeps.
//   - Parents ask for a new or changed pickup permission as a request the OGS
//     confirms (request_kind 'pickup_permission'). One pending request per
//     child and person, like the one-day change is one per child and day.
func pickupPermissionRequestsUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE users.student_guardian_pickup_permissions
			ADD COLUMN IF NOT EXISTS pickup_weekdays SMALLINT[];
		ALTER TABLE users.student_guardian_pickup_permissions
			DROP CONSTRAINT IF EXISTS student_guardian_pickup_permissions_weekdays_check;
		ALTER TABLE users.student_guardian_pickup_permissions
			ADD CONSTRAINT student_guardian_pickup_permissions_weekdays_check
			CHECK (pickup_weekdays IS NULL OR (
				cardinality(pickup_weekdays) BETWEEN 1 AND 5
				AND pickup_weekdays <@ ARRAY[1, 2, 3, 4, 5]::SMALLINT[]
			));
		ALTER TABLE schedule.care_schedule_change_requests
			DROP CONSTRAINT IF EXISTS care_schedule_change_requests_request_kind_check;
		ALTER TABLE schedule.care_schedule_change_requests
			ADD CONSTRAINT care_schedule_change_requests_request_kind_check
			CHECK (request_kind IN ('weekly_schedule', 'pickup_change', 'pickup_permission'));
		CREATE UNIQUE INDEX IF NOT EXISTS uniq_care_schedule_change_requests_pending_pickup_permission
			ON schedule.care_schedule_change_requests (tenant_id, student_id, (payload->>'guardian_profile_id'))
			WHERE status = 'pending' AND request_kind = 'pickup_permission';
	`)
	if err != nil {
		return fmt.Errorf("add pickup permission requests: %w", err)
	}
	return nil
}

// pickupPermissionRequestsDown refuses while a pickup permission request or a
// weekday-limited permission exists: dropping them would silently widen a
// permission the family limited and erase the signed requests.
func pickupPermissionRequestsDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var occupied bool
		if err := tx.NewRaw(`SELECT EXISTS (
				SELECT 1 FROM schedule.care_schedule_change_requests WHERE request_kind = 'pickup_permission'
			) OR EXISTS (
				SELECT 1 FROM users.student_guardian_pickup_permissions WHERE pickup_weekdays IS NOT NULL
			)`).Scan(ctx, &occupied); err != nil {
			return fmt.Errorf("inspect pickup permission requests: %w", err)
		}
		if occupied {
			return errors.New("pickup permission requests: refusing rollback while pickup permission requests or weekday limits exist")
		}
		_, err := tx.ExecContext(ctx, `
			DROP INDEX IF EXISTS schedule.uniq_care_schedule_change_requests_pending_pickup_permission;
			ALTER TABLE schedule.care_schedule_change_requests
				DROP CONSTRAINT IF EXISTS care_schedule_change_requests_request_kind_check;
			ALTER TABLE schedule.care_schedule_change_requests
				ADD CONSTRAINT care_schedule_change_requests_request_kind_check
				CHECK (request_kind IN ('weekly_schedule', 'pickup_change'));
			ALTER TABLE users.student_guardian_pickup_permissions
				DROP CONSTRAINT IF EXISTS student_guardian_pickup_permissions_weekdays_check;
			ALTER TABLE users.student_guardian_pickup_permissions
				DROP COLUMN IF EXISTS pickup_weekdays;
		`)
		if err != nil {
			return fmt.Errorf("remove pickup permission requests: %w", err)
		}
		return nil
	})
}
