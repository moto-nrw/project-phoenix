package parentstore

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"
	"github.com/uptrace/bun"
)

func NewStaffParentMessageReadRepository(db *bun.DB, inbox recordports.ParentInboxReads, members recordports.StaffMembers, persons recordports.StaffPersons) recordports.ParentMessageReadRepository {
	return parentpostgres.NewStaffParentMessageRecords(NewParentMessageReadRepository(db, inbox), members, persons)
}
func WithInboxGroupNames(reads recordports.ParentMessageReadRepository, groups recordports.GroupNames) recordports.ParentMessageReadRepository {
	return parentpostgres.WithInboxGroupNames(reads, groups)
}
func CurrentTenantStaffAccounts(members recordports.StaffMembers, persons recordports.StaffPersons) func(context.Context) ([]int64, error) {
	return parentpostgres.CurrentTenantStaffAccounts(members, persons)
}
