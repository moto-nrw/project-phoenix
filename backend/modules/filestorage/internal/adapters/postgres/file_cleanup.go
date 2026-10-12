package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
	"github.com/uptrace/bun"
)

type fileCleanupRow struct {
	bun.BaseModel  `bun:"table:documents.file_cleanup,alias:intent"`
	ID             int64      `bun:"id,pk,autoincrement"`
	TenantID       int64      `bun:"tenant_id,notnull"`
	OwnerID        int64      `bun:"owner_id,notnull"`
	FilenameStored string     `bun:"filename_stored,notnull"`
	RetryAfter     time.Time  `bun:"retry_after,notnull"`
	CleanedAt      *time.Time `bun:"cleaned_at"`
	CreatedAt      time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r fileCleanupRow) intent() domain.CleanupIntent {
	return domain.CleanupIntent{
		ID: r.ID, TenantID: r.TenantID, OwnerID: r.OwnerID, FilenameStored: r.FilenameStored,
		RetryAfter: r.RetryAfter, CleanedAt: r.CleanedAt,
	}
}

// FileCleanupStore implements ports.CleanupStore over documents.file_cleanup,
// the durable cleanup intents of the folder files in documents.files. An
// intent's owner is the folder the upload was meant for; like the attachment
// intents it carries no foreign key, so it outlives a deleted folder.
type FileCleanupStore struct{ database Database }

// NewFileCleanupStore wires the folder-file intent store.
func NewFileCleanupStore(database Database) *FileCleanupStore {
	if database == nil {
		panic("file storage postgres: database runtime is required")
	}
	return &FileCleanupStore{database: database}
}

// Queue durably records the intent to remove an object before it is written.
// A settled intent for the same stored name is revived rather than left alone
// (see AttachmentCleanupStore.Queue).
func (s *FileCleanupStore) Queue(ctx context.Context, ownerID int64, storedName string, retryAfter time.Time) (domain.OperationStats, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	row := fileCleanupRow{TenantID: tenantID, OwnerID: ownerID, FilenameStored: storedName, RetryAfter: retryAfter}
	started := time.Now()
	_, err = db.NewInsert().Model(&row).
		ModelTableExpr(`documents.file_cleanup`).
		On("CONFLICT (tenant_id, filename_stored) DO UPDATE").
		Set("cleaned_at = NULL").
		Set("retry_after = EXCLUDED.retry_after").
		Set("owner_id = EXCLUDED.owner_id").
		Exec(ctx)
	stats := measure(started, 1)
	if err != nil {
		return stats, fmt.Errorf("file storage postgres: queue file cleanup: %w", err)
	}
	return stats, nil
}

// ListQueued returns eligible intents and locks them for the caller's
// transaction. CALLER CONTRACT: remove the objects inside the SAME
// transaction; the row locks are released at COMMIT. The lock keeps this pass
// from deleting the bytes of an upload whose metadata transaction has stamped
// cleaned_at but not yet committed; time (retry_after) is the first guard.
func (s *FileCleanupStore) ListQueued(ctx context.Context) ([]domain.CleanupIntent, domain.OperationStats, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []fileCleanupRow
	started := time.Now()
	err = db.NewSelect().Model(&rows).
		ModelTableExpr(`documents.file_cleanup AS "intent"`).
		Where(`"intent".tenant_id = ?`, tenantID).
		Where(`"intent".retry_after <= ?`, time.Now()).
		Where(`"intent".cleaned_at IS NULL`).
		OrderExpr(`"intent".id ASC`).
		Limit(domain.CleanupBatchSize).
		For("UPDATE SKIP LOCKED").
		Scan(ctx)
	stats := measure(started, int64(len(rows)))
	if err != nil {
		return nil, stats, fmt.Errorf("file storage postgres: list queued file cleanups: %w", err)
	}
	result := make([]domain.CleanupIntent, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.intent())
	}
	return result, stats, nil
}

func (s *FileCleanupStore) MarkComplete(ctx context.Context, intentID int64) (domain.OperationStats, error) {
	return s.settle(ctx, "mark file cleanup complete", func(query *bun.UpdateQuery) *bun.UpdateQuery {
		return query.Set(`cleaned_at = ?`, time.Now()).Where(`"intent".id = ?`, intentID)
	})
}

func (s *FileCleanupStore) MarkCompleteByFilename(ctx context.Context, storedName string) (domain.OperationStats, error) {
	return s.settle(ctx, "mark file cleanup complete by filename", func(query *bun.UpdateQuery) *bun.UpdateQuery {
		return query.Set(`cleaned_at = ?`, time.Now()).Where(`"intent".filename_stored = ?`, storedName)
	})
}

func (s *FileCleanupStore) ActivateByFilename(ctx context.Context, storedName string) (domain.OperationStats, error) {
	return s.settle(ctx, "activate file cleanup", func(query *bun.UpdateQuery) *bun.UpdateQuery {
		return query.Set(`retry_after = ?`, time.Now()).Where(`"intent".filename_stored = ?`, storedName)
	})
}

func (s *FileCleanupStore) settle(ctx context.Context, operation string, narrow func(*bun.UpdateQuery) *bun.UpdateQuery) (domain.OperationStats, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := narrow(db.NewUpdate().Model((*fileCleanupRow)(nil)).
		ModelTableExpr(`documents.file_cleanup AS "intent"`)).
		Where(`"intent".tenant_id = ?`, tenantID).
		Where(`"intent".cleaned_at IS NULL`)
	started := time.Now()
	result, err := query.Exec(ctx)
	stats := measure(started, 0)
	if err != nil {
		return stats, fmt.Errorf("file storage postgres: %s: %w", operation, err)
	}
	if affected, err := result.RowsAffected(); err == nil {
		stats.Rows = affected
	}
	return stats, nil
}
