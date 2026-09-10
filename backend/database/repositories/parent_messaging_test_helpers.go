package repositories

import (
	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/uptrace/bun"
)

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
func NewParentMessagingTestRepositories(db *bun.DB) (ParentMessagingTestRepositories, error) {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return ParentMessagingTestRepositories{}, err
	}
	deps := newStaffMembershipDeps(
		NewPersonRepository(db),
		authRepo.NewAccountRepository(db),
		authRepo.NewAccountTenantRepository(db),
		authRepo.NewPermissionRepository(db),
		authRepo.NewRoleRepository(db),
	)
	groupTeachers := newGroupTeacherRepository(membership, educationRepo.NewGroupRepository(db))
	deps.groupTeachers = func() educationModels.GroupTeacherRepository { return groupTeachers }
	return ParentMessagingTestRepositories{
		Thread:  parentStore.NewParentMessageThreadRepository(db, usersRepo.NewMessageableGuardianRepository(db)),
		Message: parentStore.NewParentMessageRepository(db),
		// The three guardian-facing projections resolve the school's staff
		// accounts through School Membership, exactly as the factory binds them.
		Read: parentMessageStaffRepository{
			ParentMessageReads: parentStore.NewParentMessageReadRepository(db),
			membership:         func() schoolmembership.Capability { return membership },
			persons:            func() userModels.PersonRepository { return deps.persons },
		},
	}, nil
}
