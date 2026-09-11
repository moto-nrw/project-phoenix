package repositories

import (
	"context"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
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
		Thread:  usersRepo.NewStaffMessageThreadRepository(db),
		Message: usersRepo.NewStaffMessageRepository(db),
		Read: usersRepo.NewStaffMessageReadRepository(db, func(ctx context.Context) ([]int64, error) {
			return currentTenantStaffAccounts(ctx, membership, members.Person)
		}),
	}, nil
}
