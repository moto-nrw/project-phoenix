package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	withdrawParentArrivalRequestsVersion     = "1.15.301"
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
// Strip and withdraw are ONE statement joined on id, not two passes: the
// withdrawal then applies only to rows this migration actually emptied. A
// pending row that already carried an empty weekdays array predates this
// migration and stays untouched.
func withdrawParentArrivalRequestsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.301: Stripping arrival times from pending parent care requests...")
	_, err := db.NewRaw(`
		UPDATE schedule.care_schedule_change_requests AS req
		SET payload = jsonb_set(req.payload, '{weekdays}', stripped.weekdays),
			status = CASE WHEN stripped.weekdays = '[]'::jsonb THEN 'withdrawn' ELSE req.status END,
			decision_reason = CASE WHEN stripped.weekdays = '[]'::jsonb THEN ? ELSE req.decision_reason END
		FROM (
			SELECT
				candidate.id,
				COALESCE(
					(
						SELECT jsonb_agg(entry - 'arrival' ORDER BY ord)
						FROM jsonb_array_elements(candidate.payload->'weekdays') WITH ORDINALITY AS w(entry, ord)
						WHERE (entry - 'arrival' - 'weekday') <> '{}'::jsonb
					),
					'[]'::jsonb
				) AS weekdays
			FROM schedule.care_schedule_change_requests AS candidate
			WHERE candidate.status = 'pending'
				AND jsonb_path_exists(
					candidate.payload,
					'$.weekdays[*] ? (@.arrival != null && @.arrival != "")'
				)
		) AS stripped
		WHERE req.id = stripped.id
	`, withdrawParentArrivalRequestsReason).Exec(ctx)
	if err != nil {
		return fmt.Errorf("strip arrival times from pending parent care requests: %w", err)
	}
	return nil
}

// The stripped arrival values are gone; restoring an arrival-only request to
// pending would recreate an empty, invalid payload. Deliberate no-op.
func withdrawParentArrivalRequestsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.301: nothing to restore (arrival values were stripped irreversibly)")
	return nil
}
