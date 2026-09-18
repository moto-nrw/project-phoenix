package repositories

import (
	"context"

	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	staffStore "github.com/moto-nrw/project-phoenix/modules/communication/staffstore"
	"github.com/uptrace/bun"
)

type StaffMessagingTestRepositories struct {
	Thread  usersModels.StaffMessageThreadRepository
	Message usersModels.StaffMessageRepository
	Read    usersModels.StaffMessageReadRepository
}

func NewStaffMessagingTestRepositories(db *bun.DB) (StaffMessagingTestRepositories, error) {
	members, err := NewMembershipTestRepositories(db)
	if err != nil {
		return StaffMessagingTestRepositories{}, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return StaffMessagingTestRepositories{}, err
	}
	return StaffMessagingTestRepositories{
		Thread:  staffStore.NewStaffMessageThreadRepository(db),
		Message: staffStore.NewStaffMessageRepository(db),
		Read: staffStore.NewStaffMessageReadRepository(db, usersRepo.NewMessageableStaffRepository(db, func(ctx context.Context) ([]int64, error) {
			return currentTenantStaffAccounts(ctx, membership, members.Person)
		}, staffMessageIdentity(db, authRepo.NewAccountRepository(db)))),
	}, nil
}
