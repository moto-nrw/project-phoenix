package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/adapters/staffgroups"
)

// TeachingAssignments and GroupSubstitutions are the consumer-owned reads the
// staff group queries need from School Membership and Workforce; their
// public modules satisfy them.
type (
	TeachingAssignments = staffgroups.TeachingAssignments
	GroupSubstitutions  = staffgroups.GroupSubstitutions
)

type StaffGroupsDependencies struct {
	Groups        schoolstructure.Query
	Assignments   TeachingAssignments
	Substitutions GroupSubstitutions
}

// NewStaffGroups composes the staff-keyed group reads on top of the School
// Structure module returned by New.
func NewStaffGroups(dependencies StaffGroupsDependencies) (schoolstructure.StaffGroupQuery, error) {
	if dependencies.Groups == nil || dependencies.Assignments == nil || dependencies.Substitutions == nil {
		return nil, errors.New("school structure compose: all staff group dependencies are required")
	}
	return staffgroups.New(dependencies.Groups, dependencies.Assignments, dependencies.Substitutions), nil
}
