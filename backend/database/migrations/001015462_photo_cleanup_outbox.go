package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const photoCleanupOutboxVersion = "1.15.462"

func init() {
	MigrationRegistry.Register(&Migration{Version: photoCleanupOutboxVersion, Description: "Partition the existing file cleanup outbox for detached managed uploads", DependsOn: []string{"1.15.332"}})
	Migrations.MustRegister(photoCleanupOutboxUp, photoCleanupOutboxDown)
}

func photoCleanupOutboxUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`ALTER TABLE documents.file_cleanup ADD COLUMN object_kind TEXT NOT NULL DEFAULT 'files' CHECK (object_kind IN ('files', 'student-photos', 'enrollment-legal-documents'));
 ALTER TABLE documents.file_cleanup DROP CONSTRAINT uq_documents_file_cleanup;
 ALTER TABLE documents.file_cleanup ADD CONSTRAINT uq_documents_file_cleanup UNIQUE (tenant_id, object_kind, filename_stored);`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("partition file cleanup intents: %w", err)
	}
	return nil
}

func photoCleanupOutboxDown(ctx context.Context, db *bun.DB) error {
	var photos int64
	if err := db.NewRaw(`SELECT count(*) FROM documents.file_cleanup WHERE object_kind <> 'files'`).Scan(ctx, &photos); err != nil {
		return err
	}
	if photos > 0 {
		return fmt.Errorf("photo cleanup rollback requires draining and purging %d managed-upload intents", photos)
	}
	_, err := db.NewRaw(`ALTER TABLE documents.file_cleanup DROP CONSTRAINT uq_documents_file_cleanup;
 ALTER TABLE documents.file_cleanup DROP COLUMN object_kind;
 ALTER TABLE documents.file_cleanup ADD CONSTRAINT uq_documents_file_cleanup UNIQUE (tenant_id, filename_stored);`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("contract photo cleanup intents: %w", err)
	}
	return nil
}
