package api

import (
	"context"
	"errors"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	presenceProjection "github.com/moto-nrw/project-phoenix/modules/presenceprojection/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableProjection "github.com/moto-nrw/project-phoenix/modules/timetableprojection/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ManualPlanningReader serves Care Plan's offering-change planning port: the
// manual planning a switch leaves uncovered and the course groups an offering
// feeds, through the presence and timetable legacy views and the Timetable
// owner's course-group read.
type ManualPlanningReader struct {
	Query        PlanningQuery
	CourseGroups timetable.CourseGroupQuery
}

// PlanningQuery reads the planned care blocks and course groups an offering
// change affects from the presence and timetable legacy views. It runs in the
// caller's transaction, or on the pool when the caller has none.
type PlanningQuery struct {
	DB transactionruntime.Database
}

// ListManualPlanningOccurrences validates the child and the date window and
// lists the child's manually planned care blocks in it.
func (q PlanningQuery) ListManualPlanningOccurrences(ctx context.Context, studentID int64, from, to string) ([]carePlanCompose.ManualPlanningOccurrence, error) {
	if studentID <= 0 {
		return nil, errors.New("student id must be positive")
	}
	fromDate, fromErr := calendar.ParseDate(from)
	toDate, toErr := calendar.ParseDate(to)
	if fromErr != nil || toErr != nil || toDate.Before(fromDate) {
		return nil, errors.New("valid planning date range is required")
	}
	db := carePlanCompose.TenantAmbientDatabase(q.DB.Pool())(ctx)
	rows, err := presenceProjection.Reader{}.ListManualPlanningOccurrences(ctx, db, tenant.FromContext(ctx), studentID, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	result := make([]carePlanCompose.ManualPlanningOccurrence, 0, len(rows))
	for _, row := range rows {
		result = append(result, carePlanCompose.ManualPlanningOccurrence{ActivityGroupID: row.ActivityGroupID, ActivityGroupName: row.ActivityGroupName, InstanceID: row.InstanceID, Date: row.Date})
	}
	return result, nil
}

// LockCourseGroups locks the course groups for the caller's capacity check.
func (q PlanningQuery) LockCourseGroups(ctx context.Context, groupIDs []int64) ([]carePlanCompose.CourseGroup, error) {
	db := carePlanCompose.TenantAmbientDatabase(q.DB.Pool())(ctx)
	groups, err := timetableProjection.LockCourseGroups(ctx, db, tenant.FromContext(ctx), groupIDs)
	if err != nil {
		return nil, err
	}
	result := make([]carePlanCompose.CourseGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, carePlanCompose.CourseGroup{ID: group.ID, Active: group.Active, ParticipantLimit: group.ParticipantLimit})
	}
	return result, nil
}

// CountActiveCourseEnrollments counts the active enrollments per course group
// in the window, without the excluded child.
func (q PlanningQuery) CountActiveCourseEnrollments(ctx context.Context, groupIDs []int64, from, until calendar.Date, excludeStudentID int64) (map[int64]int, error) {
	db := carePlanCompose.TenantAmbientDatabase(q.DB.Pool())(ctx)
	return timetableProjection.CountActiveCourseEnrollments(ctx, db, tenant.FromContext(ctx), groupIDs, from, until, excludeStudentID)
}

var _ carePlanCompose.OfferingChangePlanning = ManualPlanningReader{}

func (r ManualPlanningReader) ListManualPlanningOccurrences(ctx context.Context, studentID int64, from, to string) ([]carePlanCompose.ManualPlanningOccurrence, error) {
	return r.Query.ListManualPlanningOccurrences(ctx, studentID, from, to)
}

// CourseGroupsForOfferings resolves both link shapes through the Timetable
// owner: the legacy group on the offering and the templates sourcing it.
func (r ManualPlanningReader) CourseGroupsForOfferings(ctx context.Context, offerings []carePlanCompose.CourseOfferingReference, effectiveOn calendar.Date) (map[int64][]carePlanCompose.CourseGroup, error) {
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
	groups, err := r.CourseGroups.ListCourseGroups(ctx, filter)
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

func (r ManualPlanningReader) LockCourseGroups(ctx context.Context, groupIDs []int64) ([]carePlanCompose.CourseGroup, error) {
	return r.Query.LockCourseGroups(ctx, groupIDs)
}

func (r ManualPlanningReader) CountActiveCourseEnrollments(ctx context.Context, groupIDs []int64, from, until calendar.Date, excludeStudentID int64) (map[int64]int, error) {
	return r.Query.CountActiveCourseEnrollments(ctx, groupIDs, from, until, excludeStudentID)
}
