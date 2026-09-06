package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/uptrace/bun"
)

type manualPlanningReader struct {
	db           *bun.DB
	courseGroups timetable.CourseGroupQuery
}

func (r manualPlanningReader) ListManualPlanningOccurrences(ctx context.Context, studentID int64, from, to string) ([]enrollment.ManualPlanningOccurrence, error) {
	rows, err := repositories.NewManualPlanningQuery(r.db).ListManualPlanningOccurrences(ctx, studentID, from, to)
	if err != nil {
		return nil, err
	}
	result := make([]enrollment.ManualPlanningOccurrence, 0, len(rows))
	for _, row := range rows {
		result = append(result, enrollment.ManualPlanningOccurrence{ActivityGroupID: row.ActivityGroupID, ActivityGroupName: row.ActivityGroupName, InstanceID: row.InstanceID, Date: row.Date})
	}
	return result, nil
}

func (r manualPlanningReader) CourseGroupsForOfferings(ctx context.Context, offerings []enrollment.CourseOfferingReference, effectiveOn timezone.Date) (map[int64][]enrollment.CourseGroup, error) {
	if r.courseGroups == nil {
		return repositories.NewManualPlanningQuery(r.db).CourseGroupsForOfferings(ctx, offerings)
	}
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
	result := make(map[int64][]enrollment.CourseGroup, len(offerings))
	for _, group := range groups {
		var participantLimit *int
		if group.MaxParticipants > 0 {
			limit := group.MaxParticipants
			participantLimit = &limit
		}
		course := enrollment.CourseGroup{
			ID:                  group.ID,
			Active:              group.Active,
			ParticipantLimit:    participantLimit,
			ScheduledWeekdays:   append([]int(nil), group.ScheduledWeekdays...),
			SourceGradeLevels:   append([]int(nil), group.SourceGradeLevels...),
			SourceSchoolClasses: append([]string(nil), group.SourceSchoolClasses...),
		}
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

func (r manualPlanningReader) LockCourseGroups(ctx context.Context, groupIDs []int64) ([]enrollment.CourseGroup, error) {
	return repositories.NewManualPlanningQuery(r.db).LockCourseGroups(ctx, groupIDs)
}

func (r manualPlanningReader) CountActiveCourseEnrollments(ctx context.Context, groupIDs []int64, from, until timezone.Date, excludeStudentID int64) (map[int64]int, error) {
	return repositories.NewManualPlanningQuery(r.db).CountActiveCourseEnrollments(ctx, groupIDs, from, until, excludeStudentID)
}
