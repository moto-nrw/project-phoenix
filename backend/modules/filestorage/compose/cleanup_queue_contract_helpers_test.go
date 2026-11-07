//go:build testhooks

package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func cleanupProbeKind(mode string) string {
	if mode == "folder" {
		return "files"
	}
	return "announcement-attachments"
}

func cleanupProbeInsertDocument(ctx context.Context, db bun.IDB, mode string, tenantID, ownerID, accountID int64, name string, softDeleted, bytesGone bool, oldTime time.Time) error {
	var deletedAt, bytesDeletedAt *time.Time
	if softDeleted {
		deletedAt = &oldTime
	}
	if bytesGone {
		bytesDeletedAt = &oldTime
	}
	if mode == "folder" {
		_, err := db.NewRaw(`INSERT INTO documents.files (tenant_id, folder_id, filename_display, filename_stored, size_bytes, content_type, uploaded_by, deleted_at, file_deleted_at)
   VALUES (?, ?, ?, ?, 12, 'application/pdf', ?, ?, ?)`, tenantID, ownerID, name, name, accountID, deletedAt, bytesDeletedAt).Exec(ctx)
		return err
	}
	_, err := db.NewRaw(`INSERT INTO documents.announcement_attachments (tenant_id, announcement_id, filename_display, filename_stored, size_bytes, content_type, uploaded_by, deleted_at, file_deleted_at)
  VALUES (?, ?, ?, ?, 12, 'application/pdf', ?, ?, ?)`, tenantID, ownerID, name, name, accountID, deletedAt, bytesDeletedAt).Exec(ctx)
	return err
}

func cleanupProbeInsertIntent(ctx context.Context, db bun.IDB, mode string, tenantID, ownerID int64, name string, stamp time.Time) error {
	if mode == "folder" {
		_, err := db.NewRaw(`INSERT INTO documents.file_cleanup (tenant_id, object_kind, owner_id, filename_stored, retry_after, cleaned_at, created_at, updated_at)
   VALUES (?, 'files', ?, ?, ?, ?, ?, ?)`, tenantID, ownerID, name, stamp, stamp, stamp, stamp).Exec(ctx)
		return err
	}
	_, err := db.NewRaw(`INSERT INTO documents.announcement_attachment_cleanup (tenant_id, owner_id, filename_stored, retry_after, cleaned_at, created_at, updated_at)
  VALUES (?, ?, ?, ?, ?, ?, ?)`, tenantID, ownerID, name, stamp, stamp, stamp, stamp).Exec(ctx)
	return err
}

func cleanupProbeMetadataRows(ctx context.Context, db bun.IDB, mode string, tenantID, ownerID int64) ([]string, error) {
	rows := []string{}
	if mode == "folder" {
		err := db.NewRaw(`SELECT row_to_json(d)::text FROM documents.files AS d WHERE d.tenant_id = ? AND d.folder_id = ? ORDER BY d.id`, tenantID, ownerID).Scan(ctx, &rows)
		return rows, err
	}
	err := db.NewRaw(`SELECT row_to_json(d)::text FROM documents.announcement_attachments AS d WHERE d.tenant_id = ? AND d.announcement_id = ? ORDER BY d.id`, tenantID, ownerID).Scan(ctx, &rows)
	return rows, err
}

func cleanupProbeIntentRows(ctx context.Context, db bun.IDB, mode string, tenantID int64, names []string) ([]string, error) {
	rows := []string{}
	if mode == "folder" {
		err := db.NewRaw(`SELECT row_to_json(i)::text FROM documents.file_cleanup AS i WHERE i.tenant_id = ? AND i.object_kind = 'files' AND i.filename_stored IN (?) ORDER BY i.id`, tenantID, bun.List(names)).Scan(ctx, &rows)
		return rows, err
	}
	err := db.NewRaw(`SELECT row_to_json(i)::text FROM documents.announcement_attachment_cleanup AS i WHERE i.tenant_id = ? AND i.filename_stored IN (?) ORDER BY i.id`, tenantID, bun.List(names)).Scan(ctx, &rows)
	return rows, err
}

func cleanupProbeObjects(objects *memoryObjects) map[string]string {
	objects.mu.Lock()
	defer objects.mu.Unlock()
	result := map[string]string{}
	for key, value := range objects.objects {
		result[key] = string(value)
	}
	return result
}

func cleanupProtocolTable(mode string) string {
	if mode == "folder" {
		return "documents.file_cleanup"
	}
	return "documents.announcement_attachment_cleanup"
}

func cleanupProtocolSequence(t *testing.T, ctx context.Context, db bun.IDB, mode string) int64 {
	t.Helper()
	var value int64
	require.NoError(t, db.NewRaw("SELECT last_value FROM "+cleanupProtocolTable(mode)+"_id_seq").Scan(ctx, &value))
	return value
}
