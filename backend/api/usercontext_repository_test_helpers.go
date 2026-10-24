//go:build testhooks

package api

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
)

// UserContextTestRepositories are the Identity & Access account profiles
// and the group substitutions the caller context reads. The School
// Structure's staff groups come beside them.
type UserContextTestRepositories struct {
	Profile       identityaccess.AccountProfiles
	Substitutions owners.GroupSubstitutions
}

func NewUserContextTestRepositories(db transactionruntime.Database, members owners.MembershipTestRepositories) (UserContextTestRepositories, schoolstructure.StaffGroupQuery, error) {
	membership, err := owners.NewSchoolMembership(db)
	if err != nil {
		return UserContextTestRepositories{}, nil, err
	}
	workTime, err := owners.NewWorkforce(db, membership)
	if err != nil {
		return UserContextTestRepositories{}, nil, err
	}
	structure, err := owners.NewSchoolStructure(db)
	if err != nil {
		return UserContextTestRepositories{}, nil, err
	}
	staffGroups, err := schoolStructureCompose.NewStaffGroups(schoolStructureCompose.StaffGroupsDependencies{Groups: structure, Assignments: membership, Substitutions: workTime})
	if err != nil {
		return UserContextTestRepositories{}, nil, err
	}
	return UserContextTestRepositories{
		Profile: owners.NewPeopleIdentityAccess(db, nil), Substitutions: members.GroupSubstitution,
	}, staffGroups, nil
}
