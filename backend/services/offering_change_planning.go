package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// manualPlanningReader serves Care Plan's offering-change planning port: the
// manual planning a switch leaves uncovered and the course groups an offering
// feeds, through Enrollment's manual-planning query and the Timetable
// owner's course-group read.
type manualPlanningReader struct {
	db           *bun.DB
	courseGroups timetable.CourseGroupQuery
}

var _ carePlanCompose.OfferingChangePlanning = manualPlanningReader{}

func (r manualPlanningReader) ListManualPlanningOccurrences(ctx context.Context, studentID int64, from, to string) ([]carePlanCompose.ManualPlanningOccurrence, error) {
	rows, err := repositories.NewManualPlanningQuery(r.db).ListManualPlanningOccurrences(ctx, studentID, from, to)
	if err != nil {
		return nil, err
	}
	result := make([]carePlanCompose.ManualPlanningOccurrence, 0, len(rows))
	for _, row := range rows {
		result = append(result, carePlanCompose.ManualPlanningOccurrence{ActivityGroupID: row.ActivityGroupID, ActivityGroupName: row.ActivityGroupName, InstanceID: row.InstanceID, Date: row.Date})
	}
	return result, nil
}

// CourseGroupsForOfferings resolves both link shapes through the Timetable
// owner: the legacy group on the offering and the templates sourcing it.
func (r manualPlanningReader) CourseGroupsForOfferings(ctx context.Context, offerings []carePlanCompose.CourseOfferingReference, effectiveOn calendar.Date) (map[int64][]carePlanCompose.CourseGroup, error) {
	legacyToOfferings := make(map[int64][]int64, len(offerings))
	wantedOfferings := make(map[int64]bool, len(offerings))
	filter := timetable.CourseGroupFilter{}
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
	groups, err := r.courseGroups.ListCourseGroups(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]carePlanCompose.CourseGroup, len(offerings))
	for _, group := range groups {
		course := timetableCourseGroup(group)
		for _, offeringID := range legacyToOfferings[group.ID] {
			result[offeringID] = append(result[offeringID], course)
		}
		for _, offeringID := range group.SourceCareOfferingIDs {
			if wantedOfferings[offeringID] {
				result[offeringID] = append(result[offeringID], course)
			}
		}
	}
	return result, nil
}

func timetableCourseGroup(group timetable.CourseGroup) carePlanCompose.CourseGroup {
	var participantLimit *int
	if group.MaxParticipants > 0 {
		limit := group.MaxParticipants
		participantLimit = &limit
	}
	return carePlanCompose.CourseGroup{
		ID: group.ID, Active: group.Active, ParticipantLimit: participantLimit,
		ScheduledWeekdays:   append([]int(nil), group.ScheduledWeekdays...),
		SourceGradeLevels:   append([]int(nil), group.SourceGradeLevels...),
		SourceSchoolClasses: append([]string(nil), group.SourceSchoolClasses...),
	}
}

func (r manualPlanningReader) LockCourseGroups(ctx context.Context, groupIDs []int64) ([]carePlanCompose.CourseGroup, error) {
	groups, err := repositories.NewManualPlanningQuery(r.db).LockCourseGroups(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	result := make([]carePlanCompose.CourseGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, carePlanCompose.CourseGroup{
			ID: group.ID, Active: group.Active, ParticipantLimit: group.ParticipantLimit,
			ScheduledWeekdays: group.ScheduledWeekdays, SourceGradeLevels: group.SourceGradeLevels,
			SourceSchoolClasses: group.SourceSchoolClasses,
		})
	}
	return result, nil
}

func (r manualPlanningReader) CountActiveCourseEnrollments(ctx context.Context, groupIDs []int64, from, until calendar.Date, excludeStudentID int64) (map[int64]int, error) {
	return repositories.NewManualPlanningQuery(r.db).CountActiveCourseEnrollments(ctx, groupIDs, from, until, excludeStudentID)
}
