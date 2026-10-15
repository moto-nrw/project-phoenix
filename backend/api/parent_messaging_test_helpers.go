package api

import (
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// NewMessageableGuardianRepository binds the recipient lookup to the native
// Identity capability for focused repository tests.
func NewMessageableGuardianRepository(db transactionruntime.Database) *peopleCompose.RetainedMessageableGuardians {
	return peopleCompose.NewMessageableGuardians(db.Pool(), NewPeopleIdentityAccess(db, nil).FindActiveSchoolMemberships)
}

// ParentMessagingTestRepositories is the parent-conversation data layer for
// repository tests.
type ParentMessagingTestRepositories struct {
	Thread  userModels.ParentMessageThreadRepository
	Message userModels.ParentMessageRepository
	Read    userModels.ParentMessageReadRepository
}

// NewParentMessagingTestRepositories composes Communication's conversation
// stores plus the staff-membership decorator the guardian-facing reads need,
// WITHOUT constructing the legacy repository graph.
//
// Tests reach for this instead of NewFactory: the factory is shrink-only under
// #2580, so a test that only needs three repositories must not add another
// caller that builds all of them.
func NewParentMessagingTestRepositories(db transactionruntime.Database) (ParentMessagingTestRepositories, error) {
	membership, err := services.NewSchoolMembership(db.Pool())
	if err != nil {
		return ParentMessagingTestRepositories{}, err
	}
	persons := NewPersonRepository(db)
	return ParentMessagingTestRepositories{
		Thread:  parentStore.NewParentMessageThreadRepository(db.Pool(), NewMessageableGuardianRepository(db)),
		Message: parentStore.NewParentMessageRepository(db.Pool()),
		// The three guardian-facing projections resolve the school's staff
		// accounts through School Membership, exactly as the factory binds them.
		Read: parentStore.NewStaffParentMessageReadRepository(db.Pool(), ParentStaffMembers{Membership: membership}.List, persons),
	}, nil
}
