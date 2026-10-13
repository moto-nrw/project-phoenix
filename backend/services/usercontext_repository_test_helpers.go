package services

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/uptrace/bun"
)

type UserContextTestRepositories struct {
	Timetable     TimetableTestRepositories
	Profile       identityaccess.AccountProfiles
	Substitutions GroupSubstitutionRepository
	StaffGroups   schoolstructure.StaffGroupQuery
}

func NewUserContextTestRepositories(db *bun.DB, members MembershipTestRepositories) (UserContextTestRepositories, error) {
	timetable, err := NewTimetableTestRepositoriesWithMembers(db, members)
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
	return UserContextTestRepositories{
		Timetable: timetable, Profile: newIdentityAccess(db, nil), Substitutions: members.GroupSubstitution, StaffGroups: staffGroups,
	}, nil
}
