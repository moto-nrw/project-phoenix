package repositories

import (
	"context"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// caregiverBindingOwners adapts the four binding owners to the People
// Directory port that serializes the caregiver capability re-check.
type caregiverBindingOwners struct {
	membership schoolmembership.Capability
	workforce  workforce.Capability
	presence   studentpresence.Capability
	timetable  timetable.Capability
}

// NewCaregiverBindingOwners binds the People Directory caregiver lock port to
// the School Membership, Workforce, Timetable and Student Presence owners.
func NewCaregiverBindingOwners(deps TimetableDependencies, presence studentpresence.Capability) usersRepo.CaregiverBindingOwners {
	return caregiverBindingOwners{
		membership: deps.Membership,
		workforce:  deps.Workforce,
		presence:   presence,
		timetable:  deps.Capability,
	}
}

func (o caregiverBindingOwners) LockGroupAssignments(ctx context.Context) error {
	return o.membership.LockGroupAssignments(ctx)
}

func (o caregiverBindingOwners) LockGroupSubstitutions(ctx context.Context) error {
	return o.workforce.LockGroupSubstitutions(ctx)
}

func (o caregiverBindingOwners) LockGroupSupervisions(ctx context.Context) error {
	return o.presence.LockGroupSupervisions(ctx)
}

func (o caregiverBindingOwners) LockPlannedSupervisors(ctx context.Context) error {
	return o.timetable.LockPlannedSupervisors(ctx)
}
