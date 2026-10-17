//go:build testhooks

package api

import (
	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	staffStore "github.com/moto-nrw/project-phoenix/modules/communication/staffstore"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

type StaffMessagingTestRepositories struct {
	Thread  communication.StaffThreadRecords
	Message communication.StaffMessageRecords
	Read    communication.StaffReadRecords
}

func NewStaffMessagingTestRepositories(db transactionruntime.Database, members MembershipTestRepositories) (StaffMessagingTestRepositories, error) {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return StaffMessagingTestRepositories{}, err
	}
	return StaffMessagingTestRepositories{
		Thread:  staffStore.NewStaffMessageThreadRepository(db.Pool()),
		Message: staffStore.NewStaffMessageRepository(db.Pool()),
		Read: staffStore.NewStaffMessageReadRepository(db.Pool(), NewCommunicationStaffColleagues(db,
			parentStore.CurrentTenantStaffAccounts(ParentStaffMembers{Membership: membership}.List, members.Person),
			StaffMessageIdentity(NewPeopleIdentityAccess(db, nil)))),
	}, nil
}
