//go:build testhooks

package api

import (
	"context"

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
	personAccounts := func(ctx context.Context, ids []int64) (map[int64]*parentStore.StaffPersonAccount, error) {
		rows, err := members.Person.FindByIDs(ctx, ids)
		if rows == nil {
			return nil, err
		}
		out := make(map[int64]*parentStore.StaffPersonAccount, len(rows))
		for id, row := range rows {
			if row == nil {
				out[id] = nil
				continue
			}
			out[id] = &parentStore.StaffPersonAccount{AccountID: row.AccountID}
		}
		return out, err
	}
	return StaffMessagingTestRepositories{
		Thread:  staffStore.NewStaffMessageThreadRepository(db.Pool()),
		Message: staffStore.NewStaffMessageRepository(db.Pool()),
		Read: staffStore.NewStaffMessageReadRepository(db.Pool(), NewCommunicationStaffColleagues(db,
			parentStore.CurrentTenantStaffAccounts(ParentStaffMembers{Membership: membership}.List, personAccounts),
			StaffMessageIdentity(NewPeopleIdentityAccess(db, nil))), newStaffMessageInbox(db)),
	}, nil
}
