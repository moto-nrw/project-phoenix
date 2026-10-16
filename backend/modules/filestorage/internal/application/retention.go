package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/ports"
)

// CleanupRetention deletes a school's finished cleanup intents after the
// fixed window (#4046).
type CleanupRetention struct {
	store ports.CleanupRetentionStore
}

func NewCleanupRetention(store ports.CleanupRetentionStore) *CleanupRetention {
	if store == nil {
		panic("file storage application: cleanup retention requires its store")
	}
	return &CleanupRetention{store: store}
}

// DeleteFinished removes the finished intents of the file storage and of the
// announcement attachments whose bytes went before the window.
func (r *CleanupRetention) DeleteFinished(ctx context.Context) (int, error) {
	files, err := r.store.DeleteFinishedFileCleanups(ctx, domain.CleanupRetentionWindow)
	if err != nil {
		return 0, err
	}
	attachments, err := r.store.DeleteFinishedAttachmentCleanups(ctx, domain.CleanupRetentionWindow)
	if err != nil {
		return 0, err
	}
	return files + attachments, nil
}
