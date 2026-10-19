//go:build testhooks

package api

import (
	"context"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	guardianLinkView "github.com/moto-nrw/project-phoenix/modules/guardianlinkview/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewMessageableGuardianRepository binds the recipient lookup to the native
// Identity capability for focused repository tests.
func NewMessageableGuardianRepository(db transactionruntime.Database) *peopleCompose.RetainedMessageableGuardians {
	return peopleCompose.NewMessageableGuardians(db.Pool(), guardianLinkView.NewReader(), owners.NewPeopleIdentityAccess(db, nil).FindActiveSchoolMemberships)
}

// ParentMessagingTestRepositories is the parent-conversation data layer for
// repository tests.
type ParentMessagingTestRepositories struct {
	Thread  parentStore.ParentMessageThreadRepository
	Message parentStore.ParentMessageRepository
	Read    parentStore.ParentMessageReadRepository
}

// NewParentMessagingTestRepositories composes Communication's conversation
// stores plus the staff-membership decorator the guardian-facing reads need,
// WITHOUT constructing the legacy repository graph.
//
// Tests reach for this instead of the service graph, which only shrinks under
// #2580, so a test that only needs three repositories must not add another
// caller that builds all of them.
func NewParentMessagingTestRepositories(db transactionruntime.Database) (ParentMessagingTestRepositories, error) {
	membership, err := owners.NewSchoolMembership(db)
	if err != nil {
		return ParentMessagingTestRepositories{}, err
	}
	persons := owners.NewPersonRepository(db)
	personAccounts := func(ctx context.Context, ids []int64) (map[int64]*parentStore.StaffPersonAccount, error) {
		rows, err := persons.FindByIDs(ctx, ids)
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
	return ParentMessagingTestRepositories{
		Thread:  parentStore.NewParentMessageThreadRepository(db.Pool(), owners.NewParentGuardianLister(db, owners.NewPeopleIdentityAccess(db, nil).FindActiveSchoolMemberships)),
		Message: parentStore.NewParentMessageRepository(db.Pool()),
		// The three guardian-facing projections resolve the school's staff
		// accounts through School Membership, exactly as the service graph binds them.
		Read: parentStore.NewStaffParentMessageReadRepository(db.Pool(), owners.NewParentMessageInbox(db), owners.ParentStaffMembers{Membership: membership}.List, personAccounts),
	}, nil
}
