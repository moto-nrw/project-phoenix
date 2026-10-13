package services

import (
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	staffStore "github.com/moto-nrw/project-phoenix/modules/communication/staffstore"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	usersModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/uptrace/bun"
)

type StaffMessagingTestRepositories struct {
	Thread  usersModels.StaffMessageThreadRepository
	Message usersModels.StaffMessageRepository
	Read    usersModels.StaffMessageReadRepository
}

func NewStaffMessagingTestRepositories(db *bun.DB, members MembershipTestRepositories) (StaffMessagingTestRepositories, error) {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return StaffMessagingTestRepositories{}, err
	}
	return StaffMessagingTestRepositories{
		Thread:  staffStore.NewStaffMessageThreadRepository(db),
		Message: staffStore.NewStaffMessageRepository(db),
		Read: staffStore.NewStaffMessageReadRepository(db, peopleCompose.NewMessageableStaff(db,
			parentStore.CurrentTenantStaffAccounts(parentStaffMembers{membership: membership}.list, members.Person),
			staffMessageIdentity(newPeopleIdentityAccess(db, nil)))),
	}, nil
}
