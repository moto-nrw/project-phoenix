//go:build testhooks

package api

import (
	"context"

	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

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

func NewStaffMessagingTestRepositories(db transactionruntime.Database, members owners.MembershipTestRepositories) (StaffMessagingTestRepositories, error) {
	membership, err := owners.NewSchoolMembership(db)
	if err != nil {

		return StaffMessagingTestRepositories{}, err
	}
	personAccounts := func(ctx context.Context, ids []int64) (map[int64]*recordports.StaffPersonAccount, error) {
		rows, err := members.Person.FindByIDs(ctx, ids)
		if rows == nil {
			return nil, err
		}
		out := make(map[int64]*recordports.StaffPersonAccount, len(rows))
		for id, row := range rows {
			if row == nil {
				out[id] = nil
				continue
			}
			out[id] = &recordports.StaffPersonAccount{AccountID: row.AccountID}
		}
		return out, err
	}
	return StaffMessagingTestRepositories{
		Thread:  staffStore.NewStaffMessageThreadRepository(db.Pool()),
		Message: staffStore.NewStaffMessageRepository(db.Pool()),
		Read: staffStore.NewStaffMessageReadRepository(db.Pool(), owners.NewCommunicationStaffColleagues(db,
			parentStore.CurrentTenantStaffAccounts(owners.ParentStaffMembers{Membership: membership}.List, personAccounts),
			owners.StaffMessageIdentity(owners.NewPeopleIdentityAccess(db, nil))), owners.NewStaffMessageInbox(db)),
	}, nil
}
