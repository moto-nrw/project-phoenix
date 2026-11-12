package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayClosureRetireWaitingRenewalsVersion = "1.15.950"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayClosureRetireWaitingRenewalsVersion,
		Description: "Retire holiday closure commands that wait for renewed approval of long past days (#6806)",
		DependsOn: []string{
			traegerCatalogVersion,       // the newest migration on development
			holidayClosureEventsVersion, // the function this one replaces
		},
	})
	Migrations.MustRegister(holidayClosureRetireWaitingRenewalsUp, holidayClosureRetireWaitingRenewalsDown)
}

// The Träger whose worker has something to do. A command that waits for
// renewed approval shows no family once its days are past, so nobody renews
// it; the worker retires it 30 days after its last day, as an unconfirmed
// proposal (#6806). CREATE OR REPLACE keeps owner, SECURITY DEFINER and
// grants of 1.15.942; down restores its body.
const holidayClosureRetireWaitingRenewalsUpSQL = `CREATE OR REPLACE FUNCTION calendar.holiday_closure_due_traeger_ids()
	RETURNS SETOF BIGINT
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
		SELECT DISTINCT event.organization_id
		FROM calendar.holiday_closure_events AS event
		WHERE (event.state = 'pending' AND event.next_attempt_at <= clock_timestamp())
			OR (event.state = 'leased' AND event.lease_until < clock_timestamp())
			OR event.state IN ('prepared', 'unknown', 'failed')
			OR (event.state IN ('completed', 'no_mail')
				AND COALESCE(event.completed_at, event.processed_at) < clock_timestamp() - interval '30 days')
			OR (event.state = 'needs_confirmation'
				AND event.span_to < (clock_timestamp() AT TIME ZONE 'Europe/Berlin')::date - 30)
		UNION
		SELECT proposal.organization_id FROM calendar.holiday_closure_proposals AS proposal
		WHERE proposal.state = 'open' AND proposal.span_to < (clock_timestamp() AT TIME ZONE 'Europe/Berlin')::date - 30
		ORDER BY 1
	$$;`

const holidayClosureRetireWaitingRenewalsDownSQL = `CREATE OR REPLACE FUNCTION calendar.holiday_closure_due_traeger_ids()
	RETURNS SETOF BIGINT
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
		SELECT DISTINCT event.organization_id
		FROM calendar.holiday_closure_events AS event
		WHERE (event.state = 'pending' AND event.next_attempt_at <= clock_timestamp())
			OR (event.state = 'leased' AND event.lease_until < clock_timestamp())
			OR event.state IN ('prepared', 'unknown', 'failed')
			OR (event.state IN ('completed', 'no_mail')
				AND COALESCE(event.completed_at, event.processed_at) < clock_timestamp() - interval '30 days')
		UNION
		SELECT proposal.organization_id FROM calendar.holiday_closure_proposals AS proposal
		WHERE proposal.state = 'open' AND proposal.span_to < (clock_timestamp() AT TIME ZONE 'Europe/Berlin')::date - 30
		ORDER BY 1
	$$;`

func holidayClosureRetireWaitingRenewalsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayClosureRetireWaitingRenewalsUpSQL); err != nil {
		return fmt.Errorf("retire waiting holiday closure renewals: %w", err)
	}
	return nil
}

func holidayClosureRetireWaitingRenewalsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayClosureRetireWaitingRenewalsDownSQL); err != nil {
		return fmt.Errorf("restore the holiday closure due Träger list: %w", err)
	}
	return nil
}
