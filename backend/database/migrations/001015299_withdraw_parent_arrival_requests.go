package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	withdrawParentArrivalRequestsVersion     = "1.15.299"
	withdrawParentArrivalRequestsDescription = "Strip arrival times from pending parent care requests"
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

// Parents can no longer request arrival times, but the apply path merges a
// request per field, so a pending request that also carries pickup or
// departure-mode changes stays reviewable: only its arrival values are
// stripped. A weekday entry left with nothing but its weekday number is
// dropped, and only requests whose payload becomes empty are withdrawn.
func withdrawParentArrivalRequestsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.299: Stripping arrival times from pending parent care requests...")
	_, err := db.NewRaw(`
		UPDATE schedule.care_schedule_change_requests
		SET payload = jsonb_set(
			payload,
			'{weekdays}',
			COALESCE(
				(
					SELECT jsonb_agg(entry - 'arrival' ORDER BY ord)
					FROM jsonb_array_elements(payload->'weekdays') WITH ORDINALITY AS w(entry, ord)
					WHERE (entry - 'arrival' - 'weekday') <> '{}'::jsonb
				),
				'[]'::jsonb
			)
		)
		WHERE status = 'pending'
			AND jsonb_path_exists(
				payload,
				'$.weekdays[*] ? (@.arrival != null && @.arrival != "")'
			)
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("strip arrival times from pending parent care requests: %w", err)
	}

	// Only rows the strip above emptied — an already-malformed pending row
	// (no weekdays key at all) predates this migration and is not its business.
	_, err = db.NewRaw(`
		UPDATE schedule.care_schedule_change_requests
		SET status = 'withdrawn', decision_reason = ?
		WHERE status = 'pending'
			AND payload->'weekdays' = '[]'::jsonb
	`, withdrawParentArrivalRequestsReason).Exec(ctx)
	if err != nil {
		return fmt.Errorf("withdraw arrival-only parent care requests: %w", err)
	}
	return nil
}

// The stripped arrival values are gone; restoring an arrival-only request to
// pending would recreate an empty, invalid payload. Deliberate no-op.
func withdrawParentArrivalRequestsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.299: nothing to restore (arrival values were stripped irreversibly)")
	return nil
}
