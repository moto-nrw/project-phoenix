package timetable

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CourseOfferingLink is a care offering that may feed course groups: through
// its legacy activity group, or through templates that source it.
type CourseOfferingLink struct {
	OfferingID      int64
	ActivityGroupID *int64
}

// CourseGroupsForOfferings resolves both link shapes through the course
// groups valid on effectiveOn and returns each offering's groups. Offerings
// without a positive id link nothing.
func CourseGroupsForOfferings(ctx context.Context, query CourseGroupQuery, offerings []CourseOfferingLink, effectiveOn calendar.Date) (map[int64][]CourseGroup, error) {
	legacyToOfferings := make(map[int64][]int64, len(offerings))
	wantedOfferings := make(map[int64]bool, len(offerings))
	filter := CourseGroupFilter{}
	for _, offering := range offerings {
		if offering.OfferingID <= 0 {
			continue
		}
		wantedOfferings[offering.OfferingID] = true
		filter.SourceOfferingIDs = append(filter.SourceOfferingIDs, offering.OfferingID)
		if offering.ActivityGroupID != nil && *offering.ActivityGroupID > 0 {
			legacyToOfferings[*offering.ActivityGroupID] = append(legacyToOfferings[*offering.ActivityGroupID], offering.OfferingID)
			filter.LegacyGroupIDs = append(filter.LegacyGroupIDs, *offering.ActivityGroupID)
		}
	}
	filter.EffectiveOn = effectiveOn.String()
	groups, err := query.ListCourseGroups(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]CourseGroup, len(offerings))
	for _, group := range groups {
		for _, offeringID := range legacyToOfferings[group.ID] {
			result[offeringID] = append(result[offeringID], group)
		}
		for _, offeringID := range group.SourceCareOfferingIDs {
			if wantedOfferings[offeringID] {
				result[offeringID] = append(result[offeringID], group)
			}
		}
	}
	return result, nil
}

// CourseGroupsByID reads course groups by their own id, without a lock, for
// views that start from the AG rather than from an offering (#4767). An id
// that names no course group of the tenant is absent from the result.
func CourseGroupsByID(ctx context.Context, query CourseGroupQuery, groupIDs []int64, effectiveOn calendar.Date) ([]CourseGroup, error) {
	if len(groupIDs) == 0 {
		return []CourseGroup{}, nil
	}
	return query.ListCourseGroups(ctx, CourseGroupFilter{LegacyGroupIDs: groupIDs, EffectiveOn: effectiveOn.String()})
}

// ParticipantLimit is the group's participant cap; MaxParticipants 0 means
// the group has none.
func (g CourseGroup) ParticipantLimit() *int {
	if g.MaxParticipants <= 0 {
		return nil
	}
	limit := g.MaxParticipants
	return &limit
}
