package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	timetableConflictAcksVersion     = "1.15.263"
	timetableConflictAcksDescription = "Per-user acknowledgements for Betreuungsplan conflict warnings (#2139)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     timetableConflictAcksVersion,
		Description: timetableConflictAcksDescription,
		DependsOn:   []string{openingBalancesVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return timetableConflictAcksUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return timetableConflictAcksDown(ctx, db)
		},
	)
}

// timetableConflictAcksUp creates schedule.timetable_conflict_acks (#2139):
// one row per (tenant, account, conflict fingerprint) records that this user
// has reviewed and dismissed one concrete planning conflict. The fingerprint
// encodes kind, person, involved instances, date, overlap window, and
// relevant rooms — when the conflict changes, its fingerprint changes and the
// stored ack simply stops matching (no cleanup needed; stale rows are
// harmless and tiny).
func timetableConflictAcksUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.263: Per-user Betreuungsplan conflict acknowledgements...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schedule.timetable_conflict_acks (
			id          BIGSERIAL PRIMARY KEY,
			tenant_id   BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			account_id  BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			fingerprint TEXT NOT NULL,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_conflict_ack_fingerprint CHECK (fingerprint ~ '^[a-f0-9]{16,64}$'),
			CONSTRAINT uniq_conflict_ack UNIQUE (tenant_id, account_id, fingerprint)
		);

		CREATE INDEX IF NOT EXISTS idx_conflict_acks_tenant_account
			ON schedule.timetable_conflict_acks (tenant_id, account_id);

		DROP TRIGGER IF EXISTS update_timetable_conflict_acks_updated_at ON schedule.timetable_conflict_acks;
		CREATE TRIGGER update_timetable_conflict_acks_updated_at
		BEFORE UPDATE ON schedule.timetable_conflict_acks
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE schedule.timetable_conflict_acks ENABLE ROW LEVEL SECURITY;
		ALTER TABLE schedule.timetable_conflict_acks FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_schedule_timetable_conflict_acks ON schedule.timetable_conflict_acks;
		CREATE POLICY tenant_isolation_schedule_timetable_conflict_acks ON schedule.timetable_conflict_acks
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON schedule.timetable_conflict_acks TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE schedule.timetable_conflict_acks_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating schedule.timetable_conflict_acks: %w", err)
	}

	return tx.Commit()
}

func timetableConflictAcksDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.263: Dropping schedule.timetable_conflict_acks...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS schedule.timetable_conflict_acks;
	`)
	if err != nil {
		return fmt.Errorf("error dropping schedule.timetable_conflict_acks: %w", err)
	}

	return tx.Commit()
}
