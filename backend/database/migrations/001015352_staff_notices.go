package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

const (
	staffNoticesVersion     = "1.15.352"
	staffNoticesDescription = "Tagesinformationen: interne Hinweise der Leitung an das Team, wiederkehrend nach Wochentag (#2180)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffNoticesVersion,
		Description: staffNoticesDescription,
		DependsOn:   []string{parentRequestDoneVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return staffNoticesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return staffNoticesDown(ctx, db)
		},
	)
}

// staffNoticesUp creates users.staff_notices und users.staff_notice_acks
// (#2180).
//
// Ein Hinweis ist KEINE Zeile pro Tag: er trägt einen Gültigkeitszeitraum,
// optional Wochentage und ein week_pattern und wird beim Lesen gegen das Datum
// geprüft. Das Vokabular ist absichtlich dasselbe wie bei Stundenplan und
// Dienstplan (weekdays als ISO 1..7, week_pattern 0/1/2, valid_from/
// valid_until), aber es entsteht KEINE zweite Recurrence-Engine: niemand
// bearbeitet einen Hinweis tageweise, also gibt es auch nichts zu
// materialisieren.
//
// Die Struktur ist der von users.parent_announcements nachgebaut (Titel, Text,
// Wichtigkeit, optionale Kenntnisnahme in einer eigenen Tabelle) — nur ohne
// Zielgruppen: die Reichweite ist zunächst schulweit.
func staffNoticesUp(ctx context.Context, db *bun.DB) error {
	slog.Info("migration starting", "migration", staffNoticesVersion)

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			slog.Warn("migration rollback failed", "migration", staffNoticesVersion, "error", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.staff_notices (
			id                       BIGSERIAL PRIMARY KEY,
			tenant_id                BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			title                    VARCHAR(200) NOT NULL,
			body                     TEXT NOT NULL DEFAULT '',
			priority                 VARCHAR(20) NOT NULL DEFAULT 'info',
			valid_from               DATE NOT NULL,
			valid_until              DATE,
			weekdays                 SMALLINT[] NOT NULL DEFAULT '{}',
			week_pattern             SMALLINT NOT NULL DEFAULT 0,
			requires_acknowledgement BOOLEAN NOT NULL DEFAULT FALSE,
			active                   BOOLEAN NOT NULL DEFAULT TRUE,
			created_by               BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE RESTRICT,
			created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_staff_notices_title CHECK (length(btrim(title)) > 0),
			CONSTRAINT chk_staff_notices_priority CHECK (priority IN ('info', 'important')),
			CONSTRAINT chk_staff_notices_week_pattern CHECK (week_pattern BETWEEN 0 AND 2),
			CONSTRAINT chk_staff_notices_range CHECK (valid_until IS NULL OR valid_until >= valid_from),
			CONSTRAINT chk_staff_notices_weekdays CHECK (
				weekdays <@ ARRAY[1,2,3,4,5,6,7]::SMALLINT[]
			)
		);

		-- Die Leseabfrage der Startseite filtert immer auf Mandant, aktiv und
		-- Gültigkeitszeitraum.
		CREATE INDEX IF NOT EXISTS idx_staff_notices_tenant_window
			ON users.staff_notices (tenant_id, active, valid_from, valid_until);

		DROP TRIGGER IF EXISTS update_staff_notices_updated_at ON users.staff_notices;
		CREATE TRIGGER update_staff_notices_updated_at
		BEFORE UPDATE ON users.staff_notices
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE users.staff_notices ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.staff_notices FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_users_staff_notices ON users.staff_notices;
		CREATE POLICY tenant_isolation_users_staff_notices ON users.staff_notices
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON users.staff_notices TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.staff_notices_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating users.staff_notices: %w", err)
	}

	// Kenntnisnahme: eine Zeile je Person und Hinweis. Die Bestätigung gilt für
	// den Hinweis, nicht für den einzelnen Tag — ein wiederkehrender Hinweis
	// wird einmal zur Kenntnis genommen, nicht jeden Dienstag erneut.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.staff_notice_acks (
			notice_id       BIGINT NOT NULL REFERENCES users.staff_notices(id) ON DELETE CASCADE,
			account_id      BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			acknowledged_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (notice_id, account_id)
		);

		CREATE INDEX IF NOT EXISTS idx_staff_notice_acks_tenant_account
			ON users.staff_notice_acks (tenant_id, account_id);

		ALTER TABLE users.staff_notice_acks ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.staff_notice_acks FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_users_staff_notice_acks ON users.staff_notice_acks;
		CREATE POLICY tenant_isolation_users_staff_notice_acks ON users.staff_notice_acks
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON users.staff_notice_acks TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating users.staff_notice_acks: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit staff notices migration: %w", err)
	}
	slog.Info("migration finished", "migration", staffNoticesVersion)
	return nil
}

func staffNoticesDown(ctx context.Context, db *bun.DB) error {
	slog.Info("migration rollback starting", "migration", staffNoticesVersion)

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			slog.Warn("migration rollback failed", "migration", staffNoticesVersion, "error", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS users.staff_notice_acks;
		DROP TABLE IF EXISTS users.staff_notices;
	`)
	if err != nil {
		return fmt.Errorf("error dropping staff notice tables: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit staff notices rollback: %w", err)
	}
	slog.Info("migration rollback finished", "migration", staffNoticesVersion)
	return nil
}
