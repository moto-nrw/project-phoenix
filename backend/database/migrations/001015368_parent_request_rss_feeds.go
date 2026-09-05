package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

const (
	parentRequestRSSFeedsVersion     = "1.15.368"
	parentRequestRSSFeedsDescription = "Add personal RSS subscriptions for new parent requests (#3049)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version: parentRequestRSSFeedsVersion, Description: parentRequestRSSFeedsDescription,
		DependsOn: []string{spontaneousActivityInstanceUniquenessVersion},
	})
	Migrations.MustRegister(parentRequestRSSFeedsUp, parentRequestRSSFeedsDown)
}

func parentRequestRSSFeedsUp(ctx context.Context, db *bun.DB) error {
	slog.Info("migration starting", "migration", parentRequestRSSFeedsVersion)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin parent request RSS migration: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			slog.Warn(
				"migration rollback failed",
				"migration", parentRequestRSSFeedsVersion,
				"error", rollbackErr,
			)
		}
	}()

	_, err = tx.NewRaw(`
		CREATE TABLE users.parent_request_rss_feeds (
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			account_id BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			token_hash TEXT NOT NULL CHECK (length(token_hash) = 64),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (tenant_id, account_id),
			CONSTRAINT uq_parent_request_rss_feeds_token_hash UNIQUE (token_hash)
		);

		CREATE INDEX idx_parent_request_rss_feeds_account
			ON users.parent_request_rss_feeds (account_id);

		CREATE INDEX idx_student_data_change_requests_rss
			ON users.student_data_change_requests (tenant_id, created_at DESC, id DESC)
			WHERE status <> 'auto_applied';
		CREATE INDEX idx_care_schedule_change_requests_rss
			ON schedule.care_schedule_change_requests (tenant_id, created_at DESC, id DESC);
		CREATE INDEX idx_offering_change_requests_rss
			ON enrollment.offering_change_requests (tenant_id, created_at DESC, id DESC);
		CREATE INDEX idx_excused_absence_requests_rss
			ON active.excused_absence_requests (tenant_id, created_at DESC, id DESC);
		CREATE INDEX idx_enrollment_change_requests_parent_rss
			ON enrollment.change_requests (tenant_id, created_at DESC, id DESC)
			WHERE origin = 'parent';

		CREATE TRIGGER update_parent_request_rss_feeds_updated_at
			BEFORE UPDATE ON users.parent_request_rss_feeds
			FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		GRANT SELECT, INSERT, UPDATE ON users.parent_request_rss_feeds TO phoenix_tenant, phoenix_admin;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create parent request RSS feeds: %w", err)
	}
	if err := provisionTenantRLS(ctx, tx, "users.parent_request_rss_feeds"); err != nil {
		return fmt.Errorf("provision parent request RSS tenant isolation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit parent request RSS migration: %w", err)
	}
	return nil
}

func parentRequestRSSFeedsDown(ctx context.Context, db *bun.DB) error {
	slog.Info("migration rollback starting", "migration", parentRequestRSSFeedsVersion)
	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS users.idx_student_data_change_requests_rss;
		DROP INDEX IF EXISTS schedule.idx_care_schedule_change_requests_rss;
		DROP INDEX IF EXISTS enrollment.idx_offering_change_requests_rss;
		DROP INDEX IF EXISTS active.idx_excused_absence_requests_rss;
		DROP INDEX IF EXISTS enrollment.idx_enrollment_change_requests_parent_rss;
		DROP TABLE IF EXISTS users.parent_request_rss_feeds;
	`).Exec(ctx)
	return err
}
