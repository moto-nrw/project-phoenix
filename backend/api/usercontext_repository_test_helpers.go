package api

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

type UserContextTestRepositories struct {
	Timetable     TimetableTestRepositories
	Profile       identityaccess.AccountProfiles
	Substitutions services.GroupSubstitutionRepository
	StaffGroups   schoolstructure.StaffGroupQuery
}

func NewUserContextTestRepositories(db transactionruntime.Database, members services.MembershipTestRepositories) (UserContextTestRepositories, error) {
	timetable, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return UserContextTestRepositories{}, err
	}
	membership, err := services.NewSchoolMembership(db.Pool())
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
	staffGroups, err := services.NewUserContextStaffGroups(structure, membership, workTime)
	if err != nil {
		return UserContextTestRepositories{}, err
	}
	return UserContextTestRepositories{
		Timetable: timetable, Profile: services.NewPeopleIdentityAccess(db.Pool(), nil), Substitutions: members.GroupSubstitution, StaffGroups: staffGroups,
	}, nil
}
