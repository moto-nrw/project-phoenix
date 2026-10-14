package api

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

type UserContextTestRepositories struct {
	Timetable     TimetableTestRepositories
	Profile       identityaccess.AccountProfiles
	Substitutions services.GroupSubstitutionRepository
	StaffGroups   schoolstructure.StaffGroupQuery
}

func NewUserContextTestRepositories(db transactionruntime.Database, members MembershipTestRepositories) (UserContextTestRepositories, error) {
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
	staffGroups, err := schoolStructureCompose.NewStaffGroups(schoolStructureCompose.StaffGroupsDependencies{Groups: structure, Assignments: membership, Substitutions: workTime})
	if err != nil {
		return UserContextTestRepositories{}, err
	}
	return UserContextTestRepositories{
		Timetable: timetable, Profile: NewPeopleIdentityAccess(db, nil), Substitutions: members.GroupSubstitution, StaffGroups: staffGroups,
	}, nil
}
