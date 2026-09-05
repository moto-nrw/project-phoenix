package repositories

import (
	"context"

	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/uptrace/bun"
)

type MembershipTestRepositories struct {
	Person         usersModels.PersonRepository
	Account        authModels.AccountRepository
	AccountTenant  authModels.AccountTenantRepository
	Staff          usersModels.StaffRepository
	Teacher        usersModels.TeacherRepository
	Guest          usersModels.GuestRepository
	Group          educationModels.GroupRepository
	GroupTeacher   educationModels.GroupTeacherRepository
	ClassTeacher   educationModels.ClassTeacherRepository
	ClassListEntry usersModels.ClassListEntryRepository
}

// NewMembershipTestRepositories constructs only membership and its identity
// projections. The private compatibility carrier never escapes this builder.
func NewMembershipTestRepositories(db *bun.DB) (MembershipTestRepositories, error) {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return MembershipTestRepositories{}, err
	}
	persons, err := NewPeopleDirectory(db)
	if err != nil {
		return MembershipTestRepositories{}, err
	}
	rooms, err := NewFacilities(db)
	if err != nil {
		return MembershipTestRepositories{}, err
	}
	group := educationRepo.NewGroupRepository(db).(*educationRepo.GroupRepository)
	group.BindRoomDirectory(educationRoomDirectory{rooms})
	group.BindTeachingAssignments(func(ctx context.Context, groupIDs, teacherIDs []int64) ([]educationRepo.TeacherGroupID, error) {
		assignments, err := membership.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{GroupIDs: groupIDs, TeacherIDs: teacherIDs})
		if err != nil {
			return nil, err
		}
		result := make([]educationRepo.TeacherGroupID, 0, len(assignments))
		for _, assignment := range assignments {
			result = append(result, educationRepo.TeacherGroupID{TeacherID: assignment.TeacherID, GroupID: assignment.GroupID})
		}
		return result, nil
	})
	repos := &Factory{
		db: db, Group: group,
		Person: usersRepo.NewPersonRepository(db), Account: authRepo.NewAccountRepository(db),
		AccountTenant: authRepo.NewAccountTenantRepository(db),
	}
	repos.membershipDeps = newStaffMembershipDeps(repos.Person, repos.Account, repos.AccountTenant,
		authRepo.NewPermissionRepository(db), authRepo.NewRoleRepository(db))
	repos.membershipDeps.groupTeachers = func() educationModels.GroupTeacherRepository { return repos.GroupTeacher }
	repos.bindStaffMembershipAdapters(membership)
	repos.bindStaffProjections(lazyStaffLookup{get: func() schoolmembership.Capability { return membership }})
	repos.BindPeopleDirectory(persons)
	return MembershipTestRepositories{
		Person: repos.Person, Account: repos.Account, AccountTenant: repos.AccountTenant,
		Staff: repos.Staff, Teacher: repos.Teacher, Guest: repos.Guest,
		Group: repos.Group, GroupTeacher: repos.GroupTeacher, ClassTeacher: repos.ClassTeacher,
		ClassListEntry: repos.ClassListEntry,
	}, nil
}
