package repositories

import (
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	authModels "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
	authRepo "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authpostgres"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
	"github.com/uptrace/bun"
)

type UserContextTestRepositories struct {
	Timetable     TimetableTestRepositories
	Account       authModels.AccountRepository
	Profile       identityaccess.AccountProfiles
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
	workTime, err := NewWorkforce(db, membership)
	if err != nil {
		return UserContextTestRepositories{}, err
	}
	groups := educationRepo.NewGroupRepository(db)
	substitutions := workforceLegacy.NewGroupSubstitutionRepository(workTime, groups.FindByIDs,
		substitutionStaffResolver(lazyStaffLookup{get: func() schoolmembership.Capability { return membership }}))
	return UserContextTestRepositories{
		Timetable: timetable, Profile: newIdentityAccess(db, nil), Substitutions: substitutions,
		Account: authRepo.NewAccountRepository(db),
	}, nil
}
