package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

const (
	familyProtectionVersion     = "1.15.341"
	familyProtectionDescription = "Add append-only per-child family protection events (#2267)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version: familyProtectionVersion, Description: familyProtectionDescription,
		DependsOn: []string{AuthAccountsVersion, UsersStudentsVersion, createOrgsAndSchoolsVersion},
	})
	Migrations.MustRegister(familyProtectionUp, familyProtectionDown)
}

func familyProtectionUp(ctx context.Context, db *bun.DB) error {
	slog.Info("migration starting", slog.String("migration", familyProtectionVersion))
	_, err := db.ExecContext(ctx, `
		CREATE TABLE users.student_family_protection_events (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			enabled BOOLEAN NOT NULL,
			reason TEXT NOT NULL,
			actor_account_id BIGINT NOT NULL REFERENCES auth.accounts(id),
			created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
			CONSTRAINT chk_family_protection_reason_length CHECK (char_length(reason) <= 500)
		);
		CREATE INDEX idx_family_protection_current
			ON users.student_family_protection_events (tenant_id, student_id, id DESC);

		ALTER TABLE users.student_family_protection_events ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.student_family_protection_events FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_student_family_protection_events
			ON users.student_family_protection_events
			USING (tenant_id = current_setting('app.current_tenant_id', true)::BIGINT)
			WITH CHECK (tenant_id = current_setting('app.current_tenant_id', true)::BIGINT);

		GRANT SELECT, INSERT ON users.student_family_protection_events TO phoenix_tenant;
		REVOKE UPDATE, DELETE, TRUNCATE ON users.student_family_protection_events FROM phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.student_family_protection_events_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("create family protection events: %w", err)
	}
	return nil
}

func familyProtectionDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS users.student_family_protection_events`); err != nil {
		return fmt.Errorf("drop family protection events: %w", err)
	}
	return nil
}
