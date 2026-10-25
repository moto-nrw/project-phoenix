package postgres

import (
	"context"
	"fmt"
	"time"
)

// CleanupRetentionStore implements ports.CleanupRetentionStore. The direct
// DELETE on the intent tables stays revoked from the tenant role; the
// database functions of 1.15.449 delete only finished intents of the
// transaction's school and refuse a cutoff inside the window; the function
// of 1.15.465 does the same for the rows of removed files.
type CleanupRetentionStore struct{ database Database }

// NewCleanupRetentionStore wires the retention of the cleanup intents.
func NewCleanupRetentionStore(database Database) *CleanupRetentionStore {
	if database == nil {
		panic("file storage postgres: database runtime is required")
	}
	return &CleanupRetentionStore{database: database}
}

func (s *CleanupRetentionStore) DeleteFinishedFileCleanups(ctx context.Context, window time.Duration) (int, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return 0, err
	}
	var deleted int64
	// The guard subtracts calendar days, not elapsed 24-hour durations.
	if err := db.NewRaw(`SELECT documents.delete_finished_file_cleanups(?, statement_timestamp() - (? * INTERVAL '1 day'))`, tenantID, window.Hours()/24).Scan(ctx, &deleted); err != nil {
		return 0, fmt.Errorf("file storage postgres: delete finished file cleanups: %w", err)
	}
	return int(deleted), nil
}

func (s *CleanupRetentionStore) DeleteFinishedAttachmentCleanups(ctx context.Context, window time.Duration) (int, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return 0, err
	}
	var deleted int64
	if err := db.NewRaw(`SELECT documents.delete_finished_announcement_attachment_cleanups(?, statement_timestamp() - (? * INTERVAL '1 day'))`, tenantID, window.Hours()/24).Scan(ctx, &deleted); err != nil {
		return 0, fmt.Errorf("file storage postgres: delete finished attachment cleanups: %w", err)
	}
	return int(deleted), nil
}

func (s *CleanupRetentionStore) DeleteRemovedFiles(ctx context.Context, window time.Duration) (int, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return 0, err
	}
	var deleted int64
	if err := db.NewRaw(`SELECT documents.delete_removed_files(?, statement_timestamp() - (? * INTERVAL '1 day'))`, tenantID, window.Hours()/24).Scan(ctx, &deleted); err != nil {
		return 0, fmt.Errorf("file storage postgres: delete removed files: %w", err)
	}
	return int(deleted), nil
}
