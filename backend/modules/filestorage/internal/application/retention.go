package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/ports"
)

// CleanupRetention deletes a school's finished cleanup intents (#4046) and
// the rows of its removed files (#4045) after their fixed windows.
type CleanupRetention struct {
	store ports.CleanupRetentionStore
}

func NewCleanupRetention(store ports.CleanupRetentionStore) *CleanupRetention {
	if store == nil {
		panic("file storage application: cleanup retention requires its store")
	}
	return &CleanupRetention{store: store}
}

// DeleteFinishedCleanups removes the finished intents of the file storage and
// of the announcement attachments whose bytes went before the window.
func (r *CleanupRetention) DeleteFinishedCleanups(ctx context.Context) (int, int, error) {
	files, err := r.store.DeleteFinishedFileCleanups(ctx, domain.CleanupRetentionWindow)
	if err != nil {
		return 0, 0, err
	}
	attachments, err := r.store.DeleteFinishedAttachmentCleanups(ctx, domain.CleanupRetentionWindow)
	if err != nil {
		return 0, 0, err
	}
	return files + attachments, int(domain.CleanupRetentionWindow.Hours() / 24), nil
}

// DeleteRemovedFiles removes the rows of removed files whose bytes went before
// the window.
func (r *CleanupRetention) DeleteRemovedFiles(ctx context.Context) (int, int, error) {
	deleted, err := r.store.DeleteRemovedFiles(ctx, domain.RemovedFileRetentionWindow)
	if err != nil {
		return 0, 0, err
	}
	return deleted, int(domain.RemovedFileRetentionWindow.Hours() / 24), nil
}
