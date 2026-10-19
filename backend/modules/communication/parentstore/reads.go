package parentstore

import (
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"
	"github.com/uptrace/bun"
)

func NewParentMessageReadRepository(db *bun.DB, inbox recordports.ParentInboxReads) recordports.ParentStaffMessageReads {
	return parentpostgres.NewParentMessageReadRepository(db, inbox)
}
