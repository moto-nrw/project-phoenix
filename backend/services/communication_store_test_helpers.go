package services

import (
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	"github.com/uptrace/bun"
)

// NewParentMessageRepository hands suites that may not import the
// Communication store the parent message repository newFactory builds.
func NewParentMessageRepository(db *bun.DB) userModels.ParentMessageRepository {
	return parentStore.NewParentMessageRepository(db)
}
