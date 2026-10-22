package parentstore

import (
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"
	"github.com/uptrace/bun"
)

func NewParentMessageRepository(db *bun.DB) recordports.ParentMessageRepository {
	return parentpostgres.NewParentMessageRepository(db)
}
func NewParentMessageThreadRepository(db *bun.DB, guardians recordports.GuardianLister, reachable recordports.ReachableStudentLister) recordports.ParentMessageThreadRepository {
	return parentpostgres.NewParentMessageThreadRepository(db, guardians, reachable)
}
