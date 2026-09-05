package repositories

import (
	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/uptrace/bun"
)

type UserContextTestRepositories struct {
	Timetable     TimetableTestRepositories
	Account       authModels.AccountRepository
	Profile       usersModels.ProfileRepository
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
	substitutions := educationRepo.NewGroupSubstitutionRepository(db)
	substitutions.(substitutionStaffResolverSetter).SetSubstitutionStaffResolver(
		substitutionStaffResolver(lazyStaffLookup{get: func() schoolmembership.Capability { return membership }}))
	return UserContextTestRepositories{
		Timetable: timetable, Profile: usersRepo.NewProfileRepository(db), Substitutions: substitutions,
		Account: schoolAccountRepository{AccountRepository: authRepo.NewAccountRepository(db), schools: organizations},
	}, nil
}
