package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

const (
	announcementAttachmentsVersion     = "1.15.357"
	announcementAttachmentsDescription = "Dateien an Elternmitteilungen anhängen und im Elternportal herunterladen (#2890)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     announcementAttachmentsVersion,
		Description: announcementAttachmentsDescription,
		DependsOn:   []string{additionalSupervisionAuditVersion, fileStorageVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return announcementAttachmentsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return announcementAttachmentsDown(ctx, db)
		},
	)
}

// announcementAttachmentsUp creates documents.announcement_attachments and
// documents.announcement_attachment_cleanup (#2890).
//
// Der Anhang hängt an der Mitteilung, nicht an einer vierten Sichtbarkeit der
// Dateiablage: die vier zugesagten Empfängerkreise (Schule, Klasse, Gruppe,
// Angebot, einzelnes Kind) existieren an users.parent_announcements bereits als
// Zieltypen samt Auflösung, Lesestatus und E-Mail-Strecke. Die Dateiablage
// müsste sie neu erfinden — Eltern sind dort weder Rollen noch Konten der
// Schule.
//
// Die Tabellen liegen im Schema documents und nicht bei users, weil sie die
// Form aus models/documents erben (Soft Delete der Metadaten, file_deleted_at
// für die Bytes, eigene Cleanup-Intents) und vom selben generischen Repository
// bedient werden wie die Dateiablage. Der Besitzbezug bleibt trotzdem ein
// echter zusammengesetzter Fremdschlüssel (tenant_id, announcement_id) — kein
// polymorphes entity_type/entity_id, dem die Datenbank nichts glauben könnte.
//
// Der Cleanup-Intent trägt bewusst KEINEN Fremdschlüssel auf die Mitteilung:
// die verunglückte Anfrage kann eine Mitteilung nennen, die inzwischen
// gelöscht wurde, und die verwaisten Bytes müssen trotzdem weg.
func announcementAttachmentsUp(ctx context.Context, db *bun.DB) error {
	slog.Info("migration starting", "migration", announcementAttachmentsVersion)

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			slog.Warn("migration rollback failed",
				"migration", announcementAttachmentsVersion,
				"error", err,
			)
		}
	}()

	// Der zusammengesetzte Fremdschlüssel unten braucht ein passendes UNIQUE
	// auf der Zieltabelle. users.parent_announcements hat bisher nur den
	// Primärschlüssel auf id.
	_, err = tx.NewRaw(`
		ALTER TABLE users.parent_announcements
			DROP CONSTRAINT IF EXISTS uq_parent_announcements_tenant_id;
		ALTER TABLE users.parent_announcements
			ADD CONSTRAINT uq_parent_announcements_tenant_id UNIQUE (tenant_id, id);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("error adding uq_parent_announcements_tenant_id: %w", err)
	}

	_, err = tx.NewRaw(`
		CREATE TABLE IF NOT EXISTS documents.announcement_attachments (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			announcement_id BIGINT NOT NULL,
			category TEXT NOT NULL DEFAULT 'announcement_attachment'
				CHECK (category = 'announcement_attachment'),
			filename_display TEXT NOT NULL,
			filename_stored TEXT NOT NULL,
			size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
			content_type TEXT NOT NULL,
			uploaded_by BIGINT NOT NULL,
			deleted_at TIMESTAMPTZ,
			deleted_by BIGINT,
			file_deleted_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_documents_announcement_attachments_stored UNIQUE (filename_stored),
			CONSTRAINT fk_documents_announcement_attachments_announcement
				FOREIGN KEY (tenant_id, announcement_id)
				REFERENCES users.parent_announcements(tenant_id, id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_documents_announcement_attachments_announcement
			ON documents.announcement_attachments (tenant_id, announcement_id)
			WHERE deleted_at IS NULL;

		CREATE INDEX IF NOT EXISTS idx_documents_announcement_attachments_pending_cleanup
			ON documents.announcement_attachments (tenant_id, announcement_id)
			WHERE deleted_at IS NOT NULL AND file_deleted_at IS NULL;

		CREATE TABLE IF NOT EXISTS documents.announcement_attachment_cleanup (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			owner_id BIGINT NOT NULL,
			filename_stored TEXT NOT NULL,
			retry_after TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '5 minutes',
			cleaned_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_documents_announcement_attachment_cleanup UNIQUE (tenant_id, filename_stored)
		);

		CREATE INDEX IF NOT EXISTS idx_documents_announcement_attachment_cleanup_pending
			ON documents.announcement_attachment_cleanup (tenant_id, retry_after, owner_id)
			WHERE cleaned_at IS NULL;

		DROP TRIGGER IF EXISTS update_documents_announcement_attachments_updated_at
			ON documents.announcement_attachments;
		CREATE TRIGGER update_documents_announcement_attachments_updated_at
		BEFORE UPDATE ON documents.announcement_attachments
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_documents_announcement_attachment_cleanup_updated_at
			ON documents.announcement_attachment_cleanup;
		CREATE TRIGGER update_documents_announcement_attachment_cleanup_updated_at
		BEFORE UPDATE ON documents.announcement_attachment_cleanup
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		-- Soft delete only: einen Anhang zu löschen ist ein UPDATE auf
		-- deleted_at. Zeilen, die über den Mitteilungs-Cascade verschwinden,
		-- brauchen kein Recht.
		REVOKE DELETE ON documents.announcement_attachments FROM phoenix_tenant;
		REVOKE DELETE ON documents.announcement_attachment_cleanup FROM phoenix_tenant;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("error creating documents.announcement_attachments: %w", err)
	}

	// Ein Anhang ist eine Datei; er gehört in dieselbe Spur wie die
	// Dateiablage und nicht in eine zweite Audit-Tabelle. announcement_id ist
	// denormalisiert, weil die Spur den Cascade der Mitteilung überleben muss —
	// dieselbe Begründung, aus der folder_id keinen Fremdschlüssel trägt.
	// Die alte Constraint wurde 1.15.332 inline angelegt und trägt daher einen
	// von PostgreSQL vergebenen Namen. Sie wird über ihre Definition gesucht
	// statt über einen geratenen Namen, und die neue bekommt einen expliziten.
	_, err = tx.NewRaw(`
		ALTER TABLE audit.file_events
			ADD COLUMN IF NOT EXISTS announcement_id BIGINT;

		DO $$
		DECLARE
			old_name TEXT;
		BEGIN
			SELECT con.conname INTO old_name
			FROM pg_constraint con
			JOIN pg_class rel ON rel.oid = con.conrelid
			JOIN pg_namespace nsp ON nsp.oid = rel.relnamespace
			WHERE nsp.nspname = 'audit'
				AND rel.relname = 'file_events'
				AND con.contype = 'c'
				AND pg_get_constraintdef(con.oid) LIKE '%folder_created%';
			IF old_name IS NOT NULL THEN
				EXECUTE format('ALTER TABLE audit.file_events DROP CONSTRAINT %I', old_name);
			END IF;
		END $$;

		ALTER TABLE audit.file_events
			ADD CONSTRAINT chk_file_events_action CHECK (action IN (
				'folder_created', 'folder_updated', 'folder_deleted',
				'file_uploaded', 'file_deleted',
				'announcement_attachment_uploaded', 'announcement_attachment_deleted'
			));
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("error extending audit.file_events: %w", err)
	}

	if err := provisionTenantRLS(ctx, tx,
		"documents.announcement_attachments",
		"documents.announcement_attachment_cleanup",
	); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit announcement attachments migration: %w", err)
	}
	slog.Info("migration finished", "migration", announcementAttachmentsVersion)
	return nil
}

func announcementAttachmentsDown(ctx context.Context, db *bun.DB) error {
	slog.Info("migration rollback starting", "migration", announcementAttachmentsVersion)

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			slog.Warn("migration rollback failed",
				"migration", announcementAttachmentsVersion,
				"error", err,
			)
		}
	}()

	// Erst die Zeilen der neuen Aktionen entfernen, dann die CHECK-Constraint
	// zurückbauen: andernfalls lehnt die alte Constraint die vorhandenen
	// Anhangs-Ereignisse ab und der Rollback bricht ab.
	_, err = tx.NewRaw(`
		DELETE FROM audit.file_events
		WHERE action IN ('announcement_attachment_uploaded', 'announcement_attachment_deleted');

		ALTER TABLE audit.file_events
			DROP CONSTRAINT IF EXISTS chk_file_events_action;
		ALTER TABLE audit.file_events
			ADD CONSTRAINT chk_file_events_action CHECK (action IN (
				'folder_created', 'folder_updated', 'folder_deleted',
				'file_uploaded', 'file_deleted'
			));

		ALTER TABLE audit.file_events DROP COLUMN IF EXISTS announcement_id;

		DROP TABLE IF EXISTS documents.announcement_attachment_cleanup;
		DROP TABLE IF EXISTS documents.announcement_attachments;

		ALTER TABLE users.parent_announcements
			DROP CONSTRAINT IF EXISTS uq_parent_announcements_tenant_id;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("error dropping announcement attachment tables: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit announcement attachments rollback: %w", err)
	}
	slog.Info("migration rollback finished", "migration", announcementAttachmentsVersion)
	return nil
}
