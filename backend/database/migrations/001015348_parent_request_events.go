package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

const (
	parentRequestEventsVersion     = "1.15.348"
	parentRequestEventsDescription = "Add the append-only parent-request event ledger (#2267)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version: parentRequestEventsVersion, Description: parentRequestEventsDescription,
		DependsOn: []string{parentRequestSharingVersion, AuthAccountsVersion},
	})
	Migrations.MustRegister(parentRequestEventsUp, parentRequestEventsDown)
}

// The ledger is the one place that answers "what happened to this request".
// The four request tables keep only their current state, so a guardian edit
// followed by a decision and a later correction leaves no trace there. Rows
// are append-only for the same reason the sharing events are: a history staff
// can rewrite is not a history.
func parentRequestEventsUp(ctx context.Context, db *bun.DB) error {
	slog.Info("migration starting", slog.String("migration", parentRequestEventsVersion))
	_, err := db.NewRaw(`
		CREATE TABLE users.parent_request_events (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			request_type TEXT NOT NULL,
			request_id BIGINT NOT NULL,
			event_type TEXT NOT NULL,
			actor_account_id BIGINT REFERENCES auth.accounts(id),
			version TEXT NOT NULL DEFAULT '',
			payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
			CONSTRAINT chk_parent_request_event_type CHECK (request_type IN ('master_data', 'care_schedule', 'pickup_change', 'offering', 'excused')),
			CONSTRAINT chk_parent_request_event_kind CHECK (event_type IN ('submitted', 'guardian_edited', 'shared', 'decided', 'corrected', 'marked_done'))
		);
		CREATE INDEX idx_parent_request_events_request
			ON users.parent_request_events (tenant_id, request_type, request_id, id);
		CREATE INDEX idx_parent_request_events_student
			ON users.parent_request_events (tenant_id, student_id, id DESC);

		ALTER TABLE users.parent_request_events ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.parent_request_events FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_parent_request_events
			ON users.parent_request_events
			USING (tenant_id = current_setting('app.current_tenant_id', true)::BIGINT)
			WITH CHECK (tenant_id = current_setting('app.current_tenant_id', true)::BIGINT);

		GRANT SELECT, INSERT ON users.parent_request_events TO phoenix_tenant;
		REVOKE UPDATE, DELETE, TRUNCATE ON users.parent_request_events FROM phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.parent_request_events_id_seq TO phoenix_tenant;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create parent request events: %w", err)
	}
	return nil
}

func parentRequestEventsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`DROP TABLE IF EXISTS users.parent_request_events`).Exec(ctx); err != nil {
		return fmt.Errorf("drop parent request events: %w", err)
	}
	return nil
}
