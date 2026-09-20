// Package compose exposes the retained document cleanup persistence through
// consumer-owned ports. It does not change ownership of documents.file_cleanup.
package compose

import (
	"context"
	"time"

	documentsRepo "github.com/moto-nrw/project-phoenix/database/repositories/documents"
	documentsModel "github.com/moto-nrw/project-phoenix/models/documents"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	"github.com/uptrace/bun"
)

// NewFileCleanupStore binds File Storage's cleanup port without exposing the
// generic document repository or its persistence models to the consumer.
func NewFileCleanupStore(db *bun.DB) filestorageCompose.CleanupStore {
	return fileCleanups{repo: newLegacyFileCleanupRepository(db)}
}

// legacyFileRow satisfies the generic repository's entity constraint; only the
// cleanup half of that repository is used here.
type legacyFileRow struct {
	documentsModel.File
	FolderID int64 `bun:"folder_id,notnull"`
}

func (r *legacyFileRow) GetOwnerID() int64 { return r.FolderID }

func (r *legacyFileRow) Validate() error { return documentsModel.ValidateFile(&r.File) }

type legacyFileCleanupRepository = documentsRepo.Repository[*legacyFileRow, *documentsModel.FileCleanup]

func newLegacyFileCleanupRepository(db *bun.DB) *legacyFileCleanupRepository {
	return documentsRepo.NewRepository[*legacyFileRow, *documentsModel.FileCleanup](db, documentsRepo.Config{
		Table:        "documents.files",
		Alias:        "file",
		OwnerColumn:  "folder_id",
		CleanupTable: "documents.file_cleanup",
		CleanupAlias: "file_cleanup",
	})
}

type fileCleanups struct{ repo *legacyFileCleanupRepository }

func (c fileCleanups) Queue(ctx context.Context, ownerID int64, storedName string, retryAfter time.Time) (filestorageCompose.OperationStats, error) {
	intent := &documentsModel.FileCleanup{OwnerID: ownerID, FilenameStored: storedName, RetryAfter: retryAfter}
	started := time.Now()
	err := c.repo.QueueFileCleanup(ctx, intent)
	return filestorageCompose.OperationStats{Queries: 1, Rows: 1, StatementDuration: time.Since(started)}, err
}

func (c fileCleanups) ListQueued(ctx context.Context) ([]filestorageCompose.CleanupIntent, filestorageCompose.OperationStats, error) {
	started := time.Now()
	rows, err := c.repo.ListQueuedFileCleanups(ctx)
	stats := filestorageCompose.OperationStats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, err
	}
	result := make([]filestorageCompose.CleanupIntent, 0, len(rows))
	for _, row := range rows {
		result = append(result, filestorageCompose.CleanupIntent{
			ID: row.ID, TenantID: row.TenantID, OwnerID: row.OwnerID, FilenameStored: row.FilenameStored,
			RetryAfter: row.RetryAfter, CleanedAt: row.CleanedAt,
		})
	}
	return result, stats, nil
}

func (c fileCleanups) MarkComplete(ctx context.Context, intentID int64) (filestorageCompose.OperationStats, error) {
	started := time.Now()
	err := c.repo.MarkQueuedFileCleanupComplete(ctx, intentID)
	return filestorageCompose.OperationStats{Queries: 1, StatementDuration: time.Since(started)}, err
}

func (c fileCleanups) MarkCompleteByFilename(ctx context.Context, storedName string) (filestorageCompose.OperationStats, error) {
	started := time.Now()
	err := c.repo.MarkQueuedFileCleanupCompleteByFilename(ctx, storedName)
	return filestorageCompose.OperationStats{Queries: 1, StatementDuration: time.Since(started)}, err
}

func (c fileCleanups) ActivateByFilename(ctx context.Context, storedName string) (filestorageCompose.OperationStats, error) {
	started := time.Now()
	err := c.repo.ActivateQueuedFileCleanupByFilename(ctx, storedName)
	return filestorageCompose.OperationStats{Queries: 1, StatementDuration: time.Since(started)}, err
}
