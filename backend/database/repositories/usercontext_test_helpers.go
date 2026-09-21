package repositories

import (
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
	"github.com/uptrace/bun"
)

type UserContextTestRepositories struct {
	Timetable     TimetableTestRepositories
	Profile       identityaccess.AccountProfiles
	Substitutions educationModels.GroupSubstitutionRepository
	StaffGroups   schoolstructure.StaffGroupQuery
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
