package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/application"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	parentMessages "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"
	"github.com/uptrace/bun"
)

// ParentMessageRetentionSettings resolves the school's retention window.
type ParentMessageRetentionSettings interface {
	ParentMessageRetentionDays(context.Context) (int, error)
}

// NewParentMessageRetention composes the nightly deletion of expired parent
// messages (#3959) over the ambient tenant transaction of db.
func NewParentMessageRetention(db *bun.DB, settings ParentMessageRetentionSettings) communication.ParentMessageCleanup {
	return application.NewParentMessageRetention(parentMessages.Retention{
		Store:    parentpostgres.NewRetentionStore(parentDatabase(db)),
		Settings: settings,
	})
}
