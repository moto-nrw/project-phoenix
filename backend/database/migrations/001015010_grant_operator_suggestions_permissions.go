package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	grantOperatorSuggestionsPermissionsVersion     = "1.15.10"
	grantOperatorSuggestionsPermissionsDescription = "Grant suggestions schema and operator_audit_log permissions to phoenix_auth for operator flows"
)

func init() {
	MigrationRegistry[grantOperatorSuggestionsPermissionsVersion] = &Migration{
		Version:     grantOperatorSuggestionsPermissionsVersion,
		Description: grantOperatorSuggestionsPermissionsDescription,
		DependsOn:   []string{"1.15.9"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.10: Granting operator suggestions permissions to phoenix_auth...")

			_, err := db.ExecContext(ctx, `
				-- phoenix_auth needs USAGE on suggestions schema for operator suggestion endpoints
				GRANT USAGE ON SCHEMA suggestions TO phoenix_auth;

				-- Read access on posts, comments, votes for listing/viewing suggestions
				GRANT SELECT ON
					suggestions.posts,
					suggestions.comments,
					suggestions.votes
				TO phoenix_auth;

				-- Read/write on tracking tables for marking comments read and posts viewed
				GRANT SELECT, INSERT, UPDATE ON
					suggestions.comment_reads,
					suggestions.post_reads
				TO phoenix_auth;

				-- Sequence usage for inserts into comment_reads and post_reads
				GRANT USAGE ON ALL SEQUENCES IN SCHEMA suggestions TO phoenix_auth;

				-- Write access on comments for operator comment management (add/delete)
				GRANT INSERT, UPDATE, DELETE ON suggestions.comments TO phoenix_auth;

				-- Write access on posts for status updates
				GRANT UPDATE ON suggestions.posts TO phoenix_auth;

				-- Audit log: operators log actions on login, status changes, comments
				GRANT INSERT ON platform.operator_audit_log TO phoenix_auth;
				GRANT USAGE ON ALL SEQUENCES IN SCHEMA platform TO phoenix_auth;
			`)
			if err != nil {
				return fmt.Errorf("error granting operator suggestions permissions to phoenix_auth: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.10: Revoking operator suggestions permissions from phoenix_auth...")

			_, err := db.ExecContext(ctx, `
				REVOKE INSERT ON platform.operator_audit_log FROM phoenix_auth;

				REVOKE UPDATE ON suggestions.posts FROM phoenix_auth;
				REVOKE INSERT, UPDATE, DELETE ON suggestions.comments FROM phoenix_auth;
				REVOKE SELECT, INSERT, UPDATE ON suggestions.comment_reads, suggestions.post_reads FROM phoenix_auth;
				REVOKE SELECT ON suggestions.posts, suggestions.comments, suggestions.votes FROM phoenix_auth;
				REVOKE USAGE ON ALL SEQUENCES IN SCHEMA suggestions FROM phoenix_auth;
				REVOKE USAGE ON SCHEMA suggestions FROM phoenix_auth;
				REVOKE USAGE ON ALL SEQUENCES IN SCHEMA platform FROM phoenix_auth;
			`)
			if err != nil {
				return fmt.Errorf("error revoking operator suggestions permissions from phoenix_auth: %w", err)
			}

			return nil
		},
	)
}
