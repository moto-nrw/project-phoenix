package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/traegerview"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func occupancyPortsBound(ports traegerview.OccupancyPorts) bool {
	return ports.Groups != nil && ports.GroupNames != nil && ports.Bookings != nil && ports.Children != nil
}

// Occupancy returns the children of every school of the Träger by OGS group
// and care form, today (#4676). Every owner is read once for the whole school
// set, so the read costs the same statements for 3 and for 100 schools. The
// answer holds counts and the names of groups and offers, no child, so it is
// not written to the access log (ADR 0084 section 7).
func (o *Overview) Occupancy(ctx context.Context, organizationID int64) (traegerview.Occupancy, error) {
	if organizationID <= 0 {
		return traegerview.Occupancy{}, traegerview.ErrTraegerRequired
	}
	now := o.now()
	result := traegerview.Occupancy{Day: calendar.DateFromTime(now), AsOf: now}
	err := o.read(ctx, organizationID, func(adminCtx context.Context, ids []int64) error {
		schools, err := o.rows.Schools(adminCtx, ids)
		if err != nil {
			return err
		}
		reads, err := o.occupancyReads(adminCtx, ids, now)
		if err != nil {
			return err
		}
		result.Schools = make([]traegerview.SchoolOccupancy, 0, len(schools))
		for _, school := range schools {
			result.Schools = append(result.Schools, reads.school(school))
		}
		return nil
	})
	if err != nil {
		return traegerview.Occupancy{}, err
	}
	return result, nil
}

// occupancyReads are the owners' answers for a school set, keyed by school.
type occupancyReads struct {
	roster   map[int64]map[int64]int64
	groups   map[int64]map[int64]int64
	names    map[int64][]traegerview.NamedGroup
	bookings map[int64][]traegerview.CareOfferingBookings
	children map[int64]map[int64]int64
}

func (o *Overview) occupancyReads(ctx context.Context, scope []int64, now time.Time) (occupancyReads, error) {
	var reads occupancyReads
	if len(scope) == 0 {
		return reads, nil
	}
	var err error
	if reads.roster, err = o.rosters.ActiveStudentMembershipsByTenant(ctx, scope, now); err != nil {
		return reads, err
	}
	if reads.groups, err = o.occupancy.Groups.MembershipGroupIDsByTenant(ctx, scope); err != nil {
		return reads, err
	}
	if reads.names, err = o.occupancy.GroupNames.GroupsByTenant(ctx, scope); err != nil {
		return reads, err
	}
	day := calendar.DateFromTime(now)
	var phases []int64
	if reads.children, phases, err = o.occupancy.Children.BookedChildrenAndRunningPhases(ctx, scope, day); err != nil {
		return reads, err
	}
	reads.bookings, err = o.occupancy.Bookings.CareOfferingBookingsByTenant(ctx, scope, day, phases)
	return reads, err
}

// school applies the definitions of traegerview.Occupancy to one school: only
// the children of the roster count, each child once per group and per care
// form.
func (r occupancyReads) school(school traegerview.School) traegerview.SchoolOccupancy {
	roster := r.roster[school.ID]
	row := traegerview.SchoolOccupancy{School: school, Children: len(roster)}
	row.Groups, row.WithoutGroup = groupOccupancy(roster, r.groups[school.ID], r.names[school.ID])
	row.CareForms, row.WithoutCareForm = careFormOccupancy(roster, r.bookings[school.ID], r.children[school.ID])
	return row
}

// groupOccupancy counts the children of the roster (student to membership) by
// the group of their membership. Every named group is listed in the order of
// GroupNames, also an empty one; a group ID without a name is a group of another school and counts as
// none.
func groupOccupancy(roster, membershipGroups map[int64]int64, names []traegerview.NamedGroup) ([]traegerview.OccupancyGroup, int) {
	counts := make(map[int64]int, len(names))
	for _, group := range names {
		counts[group.ID] = 0
	}
	without := 0
	for _, membership := range roster {
		group, ok := membershipGroups[membership]
		if _, named := counts[group]; !ok || !named {
			without++
			continue
		}
		counts[group]++
	}
	groups := make([]traegerview.OccupancyGroup, 0, len(names))
	for _, group := range names {
		groups = append(groups, traegerview.OccupancyGroup{ID: group.ID, Name: group.Name, Children: counts[group.ID]})
	}
	return groups, without
}

// careFormOccupancy counts the children of the roster booked on each care
// offering, through the request child each booking belongs to. A booking of a
// child that is not on the roster (inactive, left, not yet started) does not
// count. The offerings keep the order of CareBookings.
func careFormOccupancy(roster map[int64]int64, offerings []traegerview.CareOfferingBookings, children map[int64]int64) ([]traegerview.OccupancyCareForm, int) {
	booked := make(map[int64]struct{}, len(roster))
	forms := make([]traegerview.OccupancyCareForm, 0, len(offerings))
	for _, offering := range offerings {
		students := make(map[int64]struct{}, len(offering.RequestChildIDs))
		for _, requestChild := range offering.RequestChildIDs {
			student, ok := children[requestChild]
			if !ok {
				continue
			}
			if _, onRoster := roster[student]; onRoster {
				students[student] = struct{}{}
				booked[student] = struct{}{}
			}
		}
		forms = append(forms, traegerview.OccupancyCareForm{ID: offering.ID, Name: offering.Name, Children: len(students), Capacity: offering.Capacity})
	}
	return forms, len(roster) - len(booked)
}
