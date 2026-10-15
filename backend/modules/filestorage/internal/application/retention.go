package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/ports"
)

// CleanupRetention deletes a school's finished cleanup intents after the
// fixed window (#4046).
type CleanupRetention struct {
	store ports.CleanupRetentionStore
	now   func() time.Time
}

func NewCleanupRetention(store ports.CleanupRetentionStore) *CleanupRetention {
	if store == nil {
		panic("file storage application: cleanup retention requires its store")
	}
	return &CleanupRetention{store: store, now: time.Now}
}

// DeleteFinished removes the finished intents of the file storage and of the
// announcement attachments whose bytes went before the window.
func (r *CleanupRetention) DeleteFinished(ctx context.Context) (int, error) {
	before := r.now().Add(-domain.CleanupRetentionWindow)
	files, err := r.store.DeleteFinishedFileCleanups(ctx, before)
	if err != nil {
		return 0, err
	}
	attachments, err := r.store.DeleteFinishedAttachmentCleanups(ctx, before)
	if err != nil {
		return 0, err
	}
	return files + attachments, nil
}
