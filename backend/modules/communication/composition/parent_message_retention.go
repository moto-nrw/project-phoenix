package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	parentMessages "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"
	"github.com/uptrace/bun"
)

// ParentMessageRetentionSettings resolves the school's retention window.
type ParentMessageRetentionSettings = parentMessages.RetentionSettings

// NewParentMessageRetention composes the nightly deletion of expired parent
// messages (#3959) over the ambient tenant transaction of db.
func NewParentMessageRetention(db *bun.DB, settings ParentMessageRetentionSettings) communication.ParentMessageCleanup {
	return parentMessageRetention{retention: parentMessages.Retention{
		Store:    parentpostgres.NewRetentionStore(parentDatabase(db)),
		Settings: settings,
	}}
}

type parentMessageRetention struct{ retention parentMessages.Retention }

func (r parentMessageRetention) CleanupExpiredParentMessages(ctx context.Context) (communication.ParentMessageCleanupResult, error) {
	result, err := r.retention.CleanupExpiredMessages(ctx)
	return communication.ParentMessageCleanupResult{
		MessagesDeleted: result.MessagesDeleted,
		ThreadsDeleted:  result.ThreadsDeleted,
		RetentionDays:   result.RetentionDays,
	}, err
}
