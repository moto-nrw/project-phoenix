package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/traegerview"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// traegerOccupancyPorts binds the owners' reads behind the Träger's occupancy
// (#4676): the group of each membership at School Membership, the group names
// at School Structure, the running phases and the child of each approved
// enrollment at Enrollment and the care offerings at Care Plan (#5977).
func traegerOccupancyPorts() traegerview.OccupancyPorts {
	enrollment := enrollmentCompose.NewBookedChildren()
	return traegerview.OccupancyPorts{
		Groups:     schoolMembershipCompose.NewSchoolRosters(),
		GroupNames: traegerGroupNames{groups: schoolStructureCompose.NewTraegerGroupNames()},
		Phases:     enrollment,
		Bookings:   traegerCareBookings{bookings: carePlanCompose.NewTraegerCareBookings()},
		Children:   enrollment,
	}
}

type traegerGroupNames struct {
	groups schoolstructure.TraegerGroupNames
}

func (g traegerGroupNames) GroupsByTenant(ctx context.Context, schools []int64) (map[int64][]traegerview.NamedGroup, error) {
	groups, err := g.groups.GroupsByTenant(ctx, schools)
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]traegerview.NamedGroup, len(groups))
	for school, named := range groups {
		for _, group := range named {
			result[school] = append(result[school], traegerview.NamedGroup{ID: group.ID, Name: group.Name})
		}
	}
	return result, nil
}

type traegerCareBookings struct {
	bookings careplan.TraegerCareBookings
}

func (b traegerCareBookings) CareOfferingBookingsByTenant(ctx context.Context, schools []int64, day calendar.Date, runningPhases []int64) (map[int64][]traegerview.CareOfferingBookings, error) {
	offerings, err := b.bookings.CareOfferingBookingsByTenant(ctx, schools, day, runningPhases)
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]traegerview.CareOfferingBookings, len(offerings))
	for school, booked := range offerings {
		for _, offering := range booked {
			result[school] = append(result[school], traegerview.CareOfferingBookings{
				ID: offering.ID, Name: offering.Name, Capacity: offering.Capacity, RequestChildIDs: offering.RequestChildIDs,
			})
		}
	}
	return result, nil
}
