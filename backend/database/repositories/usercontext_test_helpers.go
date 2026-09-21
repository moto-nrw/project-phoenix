package repositories

import (
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/usercontext"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
	"github.com/uptrace/bun"
)

type UserContextTestRepositories struct {
	Timetable     TimetableTestRepositories
	Profile       identityaccess.AccountProfiles
	Substitutions educationModels.GroupSubstitutionRepository
	StaffGroups   usercontext.StaffGroupReads
}

func NewUserContextTestRepositories(db *bun.DB) (UserContextTestRepositories, error) {
	timetable, err := NewTimetableTestRepositories(db)
	if err != nil {
		return UserContextTestRepositories{}, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return UserContextTestRepositories{}, err
	}
	workTime, err := NewWorkforce(db, membership)
	if err != nil {
		return UserContextTestRepositories{}, err
	}
	structure, err := NewSchoolStructure(db)
	if err != nil {
		return UserContextTestRepositories{}, err
	}
	staffGroups, err := NewUserContextStaffGroups(structure, membership, workTime)
	if err != nil {
		return UserContextTestRepositories{}, err
	}
	groups := educationRepo.NewGroupRepository(db)
	substitutions := workforceLegacy.NewGroupSubstitutionRepository(workTime, groups.FindByIDs,
		substitutionStaffResolver(lazyStaffLookup{get: func() schoolmembership.Capability { return membership }}))
	return UserContextTestRepositories{
		Timetable: timetable, Profile: newIdentityAccess(db, nil), Substitutions: substitutions, StaffGroups: staffGroups,
	}, nil
}

// NewUserContextStaffGroupsForTests composes the staff group reads over the
// real School Structure, School Membership and Workforce owners.
func NewUserContextStaffGroupsForTests(db *bun.DB) (usercontext.StaffGroupReads, error) {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return nil, err
	}
	workTime, err := NewWorkforce(db, membership)
	if err != nil {
		return nil, err
	}
	structure, err := NewSchoolStructure(db)
	if err != nil {
		return nil, err
	}
	return NewUserContextStaffGroups(structure, membership, workTime)
}
