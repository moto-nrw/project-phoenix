//go:build testhooks

package api

import (
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	staffStore "github.com/moto-nrw/project-phoenix/modules/communication/staffstore"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	usersModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

type StaffMessagingTestRepositories struct {
	Thread  usersModels.StaffMessageThreadRepository
	Message usersModels.StaffMessageRepository
	Read    usersModels.StaffMessageReadRepository
}

func NewStaffMessagingTestRepositories(db transactionruntime.Database, members MembershipTestRepositories) (StaffMessagingTestRepositories, error) {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return StaffMessagingTestRepositories{}, err
	}
	return StaffMessagingTestRepositories{
		Thread:  staffStore.NewStaffMessageThreadRepository(db.Pool()),
		Message: staffStore.NewStaffMessageRepository(db.Pool()),
		Read: staffStore.NewStaffMessageReadRepository(db.Pool(), peopleCompose.NewMessageableStaff(db.Pool(),
			parentStore.CurrentTenantStaffAccounts(ParentStaffMembers{Membership: membership}.List, members.Person),
			StaffMessageIdentity(NewPeopleIdentityAccess(db, nil)))),
	}, nil
}
