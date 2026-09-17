package repositories

import (
	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
	"github.com/uptrace/bun"
)

type UserContextTestRepositories struct {
	Timetable     TimetableTestRepositories
	Account       authModels.AccountRepository
	Profile       authModels.ProfileRepository
	Substitutions educationModels.GroupSubstitutionRepository
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
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return UserContextTestRepositories{}, err
	}
	workTime, err := NewWorkforce(db, membership)
	if err != nil {
		return UserContextTestRepositories{}, err
	}
	groups := educationRepo.NewGroupRepository(db)
	substitutions := workforceLegacy.NewGroupSubstitutionRepository(workTime, groups.FindByIDs,
		substitutionStaffResolver(lazyStaffLookup{get: func() schoolmembership.Capability { return membership }}))
	return UserContextTestRepositories{
		Timetable: timetable, Profile: authRepo.NewProfileRepository(db), Substitutions: substitutions,
		Account: schoolAccountRepository{AccountRepository: authRepo.NewAccountRepository(db), schools: organizations},
	}, nil
}
