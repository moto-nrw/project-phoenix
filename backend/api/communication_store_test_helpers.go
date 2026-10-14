package api

import (
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewParentMessageRepository hands suites that may not import the
// Communication store the parent message repository NewFactoryWithConfig builds.
func NewParentMessageRepository(db transactionruntime.Database) userModels.ParentMessageRepository {
	return parentStore.NewParentMessageRepository(db.Pool())
}
