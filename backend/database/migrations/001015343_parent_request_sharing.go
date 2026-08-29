package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

const (
	parentRequestSharingVersion     = "1.15.343"
	parentRequestSharingDescription = "Add append-only named parent-request sharing events (#2267)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version: parentRequestSharingVersion, Description: parentRequestSharingDescription,
		DependsOn: []string{familyProtectionVersion, AuthAccountsVersion},
	})
	Migrations.MustRegister(parentRequestSharingUp, parentRequestSharingDown)
}

func parentRequestSharingUp(ctx context.Context, db *bun.DB) error {
	slog.Info("migration starting", slog.String("migration", parentRequestSharingVersion))
	_, err := db.ExecContext(ctx, `
		CREATE TABLE users.parent_request_share_events (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			request_type TEXT NOT NULL,
			request_id BIGINT NOT NULL,
			author_account_id BIGINT NOT NULL REFERENCES auth.accounts(id),
			recipient_account_ids BIGINT[] NOT NULL DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
			CONSTRAINT chk_parent_request_share_type CHECK (request_type IN ('master_data', 'care_schedule', 'pickup_change', 'offering', 'excused')),
			CONSTRAINT chk_parent_request_share_recipients CHECK (array_position(recipient_account_ids, author_account_id) IS NULL)
		);
		CREATE INDEX idx_parent_request_share_current
			ON users.parent_request_share_events (tenant_id, student_id, request_type, request_id, id DESC);

		ALTER TABLE users.parent_request_share_events ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.parent_request_share_events FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_parent_request_share_events
			ON users.parent_request_share_events
			USING (tenant_id = current_setting('app.current_tenant_id', true)::BIGINT)
			WITH CHECK (tenant_id = current_setting('app.current_tenant_id', true)::BIGINT);

		GRANT SELECT, INSERT ON users.parent_request_share_events TO phoenix_tenant;
		REVOKE UPDATE, DELETE, TRUNCATE ON users.parent_request_share_events FROM phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.parent_request_share_events_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("create parent request share events: %w", err)
	}
	return nil
}

func parentRequestSharingDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS users.parent_request_share_events`); err != nil {
		return fmt.Errorf("drop parent request share events: %w", err)
	}
	return nil
}
