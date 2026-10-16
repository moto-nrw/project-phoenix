package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Engine serves the Timetable capability over the application service: it
// maps the public vocabulary onto the domain and the domain errors onto the
// public sentinels.
type Engine struct {
	service    *Service
	observe    ports.Observer
	provenance ports.EnrollmentProvenanceStore
}

// NewEngine serves the capability over service; observe receives the
// rejections, provenance writes the enrollment links.
func NewEngine(service *Service, observe ports.Observer, provenance ports.EnrollmentProvenanceStore) Engine {
	return Engine{service: service, observe: observe, provenance: provenance}
}

func (e Engine) ObserveRejection(operation string, duration time.Duration, err error) {
	e.observe(ports.Observation{Operation: operation, Duration: duration, Err: err})
}

func (e Engine) FindCategory(ctx context.Context, id int64) (timetable.Category, error) {
	value, err := e.service.FindCategory(ctx, id)
	return categoryToPublic(value), MapError(err)
}

func (e Engine) FindCategoryForAssignment(ctx context.Context, id int64) (timetable.Category, error) {
	value, err := e.service.FindCategoryForAssignment(ctx, id)
	return categoryToPublic(value), MapError(err)
}

func (e Engine) FindCategoryForShare(ctx context.Context, id int64) (timetable.Category, error) {
	value, err := e.service.FindCategoryForShare(ctx, id)
	return categoryToPublic(value), MapError(err)
}

func (e Engine) FindCategoryByName(ctx context.Context, name string) (timetable.Category, error) {
	value, err := e.service.FindCategoryByName(ctx, name)
	return categoryToPublic(value), MapError(err)
}

func (e Engine) FindCategoryByNameForAssignment(ctx context.Context, name string) (timetable.Category, error) {
	value, err := e.service.FindCategoryByNameForAssignment(ctx, name)
	return categoryToPublic(value), MapError(err)
}

func (e Engine) ListCategories(ctx context.Context) ([]timetable.Category, error) {
	values, err := e.service.ListCategories(ctx)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.Category, 0, len(values))
	for _, value := range values {
		result = append(result, categoryToPublic(value))
	}
	return result, nil
}

func (e Engine) CountCategoryUsage(ctx context.Context) (map[int64]int, error) {
	result, err := e.service.CountCategoryUsage(ctx)
	return result, MapError(err)
}

func (e Engine) FindGroup(ctx context.Context, id int64) (timetable.Group, error) {
	value, err := e.service.FindGroup(ctx, id)
	return groupToPublic(value), MapError(err)
}

func (e Engine) FindGroupForUpdate(ctx context.Context, id int64) (timetable.Group, error) {
	value, err := e.service.FindGroupForUpdate(ctx, id)
	return groupToPublic(value), MapError(err)
}

func (e Engine) FindGroupByName(ctx context.Context, name string) (timetable.Group, error) {
	value, err := e.service.FindGroupByName(ctx, name)
	return groupToPublic(value), MapError(err)
}

func (e Engine) ListGroups(ctx context.Context, filter timetable.GroupFilter) ([]timetable.Group, error) {
	values, err := e.service.ListGroups(ctx, domain.GroupFilter{
		Name: filter.Name, CategoryID: filter.CategoryID, IsOpen: filter.IsOpen, IsSystem: filter.IsSystem,
		IsTemplate: filter.IsTemplate, IDs: filter.IDs, SeriesForGroupID: filter.SeriesForGroupID,
		SourceOfferingIDs: filter.SourceOfferingIDs, HasOfferingSource: filter.HasOfferingSource,
		ActiveOnly: filter.ActiveOnly, OrderByName: filter.OrderByName, OrderByID: filter.OrderByID,
	})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.Group, 0, len(values))
	for _, value := range values {
		result = append(result, groupToPublic(value))
	}
	return result, nil
}

func (e Engine) ListCourseGroups(ctx context.Context, filter timetable.CourseGroupFilter) ([]timetable.CourseGroup, error) {
	values, err := e.service.ListCourseGroups(ctx, domain.CourseGroupFilter{
		LegacyGroupIDs: filter.LegacyGroupIDs, SourceOfferingIDs: filter.SourceOfferingIDs, EffectiveOn: filter.EffectiveOn,
	})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.CourseGroup, 0, len(values))
	for _, value := range values {
		result = append(result, CourseGroupToPublic(value))
	}
	return result, nil
}

func CourseGroupToPublic(value domain.CourseGroup) timetable.CourseGroup {
	return timetable.CourseGroup{
		ID:                    value.ID,
		Active:                value.Active,
		MaxParticipants:       value.MaxParticipants,
		SourceCareOfferingIDs: append([]int64(nil), value.SourceCareOfferingIDs...),
		SourceGradeLevels:     append([]int(nil), value.SourceGradeLevels...),
		SourceSchoolClasses:   append([]string(nil), value.SourceSchoolClasses...),
		ScheduledWeekdays:     append([]int(nil), value.ScheduledWeekdays...),
	}
}

func (e Engine) ListTemplateRows(ctx context.Context, templateID *int64) ([]timetable.TemplateListRow, error) {
	values, err := e.service.ListTemplateRows(ctx, templateID)
	return publicTemplateRows(values), MapError(err)
}

func (e Engine) ListTemplateRowsForTemplatePeriod(ctx context.Context, templateID, periodID int64) ([]timetable.TemplateListRow, error) {
	values, err := e.service.ListTemplateRowsForTemplatePeriod(ctx, templateID, periodID)
	return publicTemplateRows(values), MapError(err)
}

func (e Engine) ListTemplateRowsForPeriod(ctx context.Context, periodID *int64) ([]timetable.TemplateListRow, error) {
	values, err := e.service.ListTemplateRowsForPeriod(ctx, periodID)
	return publicTemplateRows(values), MapError(err)
}

func publicTemplateRows(values []domain.TemplateListRow) []timetable.TemplateListRow {
	result := make([]timetable.TemplateListRow, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.TemplateListRow(value))
	}
	return result
}

func (e Engine) ListTemplateWeekdayRoster(ctx context.Context, templateID, periodID *int64) ([]timetable.TemplateWeekdayRosterRow, error) {
	values, err := e.service.ListTemplateWeekdayRoster(ctx, templateID, periodID)
	result := make([]timetable.TemplateWeekdayRosterRow, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.TemplateWeekdayRosterRow(value))
	}
	return result, MapError(err)
}

func (e Engine) ListTemplateCapacityOccurrences(ctx context.Context, periodID *int64, templateIDs []int64, periods []timetable.TemplateCapacityPeriod) ([]timetable.TemplateCapacityOccurrence, error) {
	ownerPeriods := make([]domain.TemplateCapacityPeriod, 0, len(periods))
	for _, period := range periods {
		ownerPeriods = append(ownerPeriods, domain.TemplateCapacityPeriod(period))
	}
	values, err := e.service.ListTemplateCapacityOccurrences(ctx, periodID, templateIDs, ownerPeriods)
	result := make([]timetable.TemplateCapacityOccurrence, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.TemplateCapacityOccurrence(value))
	}
	return result, MapError(err)
}

func (e Engine) ListGroupTargets(ctx context.Context, ids []int64) (map[int64][]timetable.GroupTarget, error) {
	values, err := e.service.ListGroupTargets(ctx, ids)
	if err != nil {
		return nil, MapError(err)
	}
	result := make(map[int64][]timetable.GroupTarget, len(values))
	for groupID, targets := range values {
		for _, target := range targets {
			result[groupID] = append(result[groupID], groupTargetToPublic(target))
		}
	}
	return result, nil
}

func (e Engine) ListTargetStudentIDs(ctx context.Context, ids []int64) (map[int64][]int64, error) {
	result, err := e.service.ListTargetStudentIDs(ctx, ids)
	return result, MapError(err)
}

func (e Engine) FindSchedule(ctx context.Context, id int64) (timetable.Schedule, error) {
	value, err := e.service.FindSchedule(ctx, id)
	return scheduleToPublic(value), MapError(err)
}

func (e Engine) ListSchedules(ctx context.Context, filter timetable.ScheduleFilter) ([]timetable.Schedule, error) {
	values, err := e.service.ListSchedules(ctx, domain.ScheduleFilter{GroupIDs: filter.GroupIDs, Weekday: filter.Weekday})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.Schedule, 0, len(values))
	for _, value := range values {
		result = append(result, scheduleToPublic(value))
	}
	return result, nil
}

func (e Engine) FindTemplateStartTimes(ctx context.Context, groupIDs []int64) ([]timetable.TemplateStartTime, error) {
	values, err := e.service.FindTemplateStartTimes(ctx, groupIDs)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.TemplateStartTime, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.TemplateStartTime(value))
	}
	return result, nil
}

func (e Engine) CreateSchedule(ctx context.Context, input timetable.ScheduleInput) (timetable.Schedule, error) {
	value, err := e.service.CreateSchedule(ctx, scheduleFields(input))
	return scheduleToPublic(value), MapError(err)
}

func (e Engine) UpdateSchedule(ctx context.Context, id int64, input timetable.ScheduleInput) (timetable.Schedule, error) {
	value, err := e.service.UpdateSchedule(ctx, id, scheduleFields(input))
	return scheduleToPublic(value), MapError(err)
}

func (e Engine) DeleteSchedule(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteSchedule(ctx, id))
}

func (e Engine) DeleteSchedulesByGroup(ctx context.Context, groupID int64) error {
	return MapError(e.service.DeleteSchedulesByGroup(ctx, groupID))
}

func (e Engine) CapScheduleValidUntil(ctx context.Context, groupID int64, validUntil string) (int64, error) {
	rows, err := e.service.CapScheduleValidUntil(ctx, groupID, validUntil)
	return rows, MapError(err)
}

func (e Engine) FindPlannedSupervisor(ctx context.Context, id int64) (timetable.PlannedSupervisor, error) {
	value, err := e.service.FindPlannedSupervisor(ctx, id)
	return plannedSupervisorToPublic(value), MapError(err)
}

func (e Engine) ListPlannedSupervisors(ctx context.Context, filter timetable.PlannedSupervisorFilter) ([]timetable.PlannedSupervisor, error) {
	values, err := e.service.ListPlannedSupervisors(ctx, domain.PlannedSupervisorFilter{GroupIDs: filter.GroupIDs, StaffID: filter.StaffID})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.PlannedSupervisor, 0, len(values))
	for _, value := range values {
		result = append(result, plannedSupervisorToPublic(value))
	}
	return result, nil
}

func (e Engine) ListPlannedSupervisionBlockers(ctx context.Context, staffID, tenantID int64) ([]timetable.PlannedSupervisionBlocker, error) {
	contextTenant, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if contextTenant.Int64() != tenantID {
		return nil, errors.New("tenant context does not match requested tenant")
	}
	values, err := e.service.ListPlannedSupervisionBlockers(ctx, staffID)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.PlannedSupervisionBlocker, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.PlannedSupervisionBlocker(value))
	}
	return result, nil
}

func (e Engine) CreatePlannedSupervisor(ctx context.Context, input timetable.PlannedSupervisorInput) (timetable.PlannedSupervisor, error) {
	value, err := e.service.CreatePlannedSupervisor(ctx, plannedSupervisorFields(input))
	return plannedSupervisorToPublic(value), MapError(err)
}

func (e Engine) UpdatePlannedSupervisor(ctx context.Context, id int64, input timetable.PlannedSupervisorInput) (timetable.PlannedSupervisor, error) {
	value, err := e.service.UpdatePlannedSupervisor(ctx, id, plannedSupervisorFields(input))
	return plannedSupervisorToPublic(value), MapError(err)
}

func (e Engine) DeletePlannedSupervisor(ctx context.Context, id int64) error {
	return MapError(e.service.DeletePlannedSupervisor(ctx, id))
}

func (e Engine) SetPrimaryPlannedSupervisor(ctx context.Context, id int64) error {
	return MapError(e.service.SetPrimaryPlannedSupervisor(ctx, id))
}

func (e Engine) LockPlannedSupervisors(ctx context.Context) error {
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return errors.New("timetable: planned supervisor locks require a transaction")
	}
	return MapError(e.service.LockPlannedSupervisors(ctx))
}

func (e Engine) DeletePlannedSupervisorsByStaff(ctx context.Context, staffID int64) (int64, error) {
	rows, err := e.service.DeletePlannedSupervisorsByStaff(ctx, staffID)
	return rows, MapError(err)
}

func (e Engine) CapActivePlannedSupervisors(ctx context.Context, groupID int64, validUntil string) (int64, error) {
	rows, err := e.service.CapActivePlannedSupervisors(ctx, groupID, validUntil)
	return rows, MapError(err)
}

func (e Engine) SetPlannedSupervisorValidUntil(ctx context.Context, id int64, validUntil string) error {
	return MapError(e.service.SetPlannedSupervisorValidUntil(ctx, id, validUntil))
}

func (e Engine) CloseOpenPlannedSupervisors(ctx context.Context, groupID int64, periodID *int64, validUntil string) error {
	return MapError(e.service.CloseOpenPlannedSupervisors(ctx, groupID, periodID, validUntil))
}

func (e Engine) FindStudentEnrollment(ctx context.Context, id int64) (timetable.StudentEnrollment, error) {
	value, err := e.service.FindStudentEnrollment(ctx, id)
	return studentEnrollmentToPublic(value), MapError(err)
}

func (e Engine) ListStudentEnrollments(ctx context.Context, filter timetable.StudentEnrollmentFilter) ([]timetable.StudentEnrollment, error) {
	values, err := e.service.ListStudentEnrollments(ctx, domain.StudentEnrollmentFilter{
		StudentIDs: filter.StudentIDs, ActivityGroupIDs: filter.ActivityGroupIDs, ActiveOn: filter.ActiveOn,
		Limit: filter.Limit, Offset: filter.Offset, OrderByValidFrom: filter.OrderByValidFrom, OrderByGroupName: filter.OrderByGroupName,
	})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.StudentEnrollment, 0, len(values))
	for _, value := range values {
		result = append(result, studentEnrollmentToPublic(value))
	}
	return result, nil
}

func (e Engine) CreateStudentEnrollment(ctx context.Context, input timetable.StudentEnrollmentInput) (timetable.StudentEnrollment, error) {
	value, err := e.service.CreateStudentEnrollment(ctx, studentEnrollmentFields(input))
	return studentEnrollmentToPublic(value), MapError(err)
}

func (e Engine) UpdateStudentEnrollment(ctx context.Context, id int64, input timetable.StudentEnrollmentInput) (timetable.StudentEnrollment, error) {
	value, err := e.service.UpdateStudentEnrollment(ctx, id, studentEnrollmentFields(input))
	return studentEnrollmentToPublic(value), MapError(err)
}

func (e Engine) DeleteStudentEnrollment(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteStudentEnrollment(ctx, id))
}

func (e Engine) BackfillStudentEnrollmentSource(ctx context.Context, studentID, requestChildID int64, groupIDs []int64) (int64, error) {
	rows, err := BackfillStudentEnrollmentSource(ctx, e.service, e.provenance, studentID, requestChildID, groupIDs)
	return rows, MapError(err)
}

func (e Engine) DeleteStudentEnrollmentsBySource(ctx context.Context, studentID, requestChildID int64) (int64, error) {
	rows, err := e.service.DeleteStudentEnrollmentsBySource(ctx, studentID, requestChildID)
	return rows, MapError(err)
}

func (e Engine) CapActiveStudentEnrollments(ctx context.Context, groupID int64, validUntil string) (int64, error) {
	rows, err := e.service.CapActiveStudentEnrollments(ctx, groupID, validUntil)
	return rows, MapError(err)
}

func (e Engine) SetStudentEnrollmentValidUntil(ctx context.Context, id int64, validUntil string) error {
	return MapError(e.service.SetStudentEnrollmentValidUntil(ctx, id, validUntil))
}

func (e Engine) CloseOpenStudentEnrollments(ctx context.Context, groupID int64, periodID *int64, validUntil string) error {
	return MapError(e.service.CloseOpenStudentEnrollments(ctx, groupID, periodID, validUntil))
}

func (e Engine) FindTimeframe(ctx context.Context, id int64) (timetable.Timeframe, error) {
	value, err := e.service.FindTimeframe(ctx, id)
	return timeframeToPublic(value), MapError(err)
}

func (e Engine) ListTimeframes(ctx context.Context, filter timetable.TimeframeFilter) ([]timetable.Timeframe, error) {
	values, err := e.service.ListTimeframes(ctx, domain.TimeframeFilter{
		DescriptionContains: filter.DescriptionContains, ActiveOnly: filter.ActiveOnly,
		OverlapsStart: filter.OverlapsStart, OverlapsEnd: filter.OverlapsEnd, Limit: filter.Limit, Offset: filter.Offset,
	})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.Timeframe, 0, len(values))
	for _, value := range values {
		result = append(result, timeframeToPublic(value))
	}
	return result, nil
}

func (e Engine) CreateTimeframe(ctx context.Context, input timetable.TimeframeInput) (timetable.Timeframe, error) {
	value, err := e.service.CreateTimeframe(ctx, timeframeFields(input))
	return timeframeToPublic(value), MapError(err)
}

func (e Engine) UpdateTimeframe(ctx context.Context, id int64, input timetable.TimeframeInput) (timetable.Timeframe, error) {
	value, err := e.service.UpdateTimeframe(ctx, id, timeframeFields(input))
	return timeframeToPublic(value), MapError(err)
}

func (e Engine) DeleteTimeframe(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteTimeframe(ctx, id))
}

func (e Engine) ListConflictAcks(ctx context.Context, accountID int64) ([]string, error) {
	values, err := e.service.ListConflictAcks(ctx, accountID)
	return values, MapError(err)
}

func (e Engine) AcknowledgeConflict(ctx context.Context, accountID int64, fingerprint string) error {
	return MapError(e.service.AcknowledgeConflict(ctx, accountID, fingerprint))
}

func (e Engine) UnacknowledgeConflict(ctx context.Context, accountID int64, fingerprint string) error {
	return MapError(e.service.UnacknowledgeConflict(ctx, accountID, fingerprint))
}

func (e Engine) FindPlanningTrack(ctx context.Context, id int64) (timetable.PlanningTrack, error) {
	value, err := e.service.FindPlanningTrack(ctx, id, "")
	return planningTrackToPublic(value), MapError(err)
}

func (e Engine) FindPlanningTrackForShare(ctx context.Context, id int64) (timetable.PlanningTrack, error) {
	value, err := e.service.FindPlanningTrack(ctx, id, "SHARE")
	return planningTrackToPublic(value), MapError(err)
}

func (e Engine) ListPlanningTracks(ctx context.Context, filter timetable.PlanningTrackFilter) ([]timetable.PlanningTrack, error) {
	values, err := e.service.ListPlanningTracks(ctx, domain.PlanningTrackFilter{IDs: filter.IDs, Ordered: filter.Ordered})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.PlanningTrack, 0, len(values))
	for _, value := range values {
		result = append(result, planningTrackToPublic(value))
	}
	return result, nil
}

func (e Engine) CreatePlanningTrack(ctx context.Context, input timetable.PlanningTrackInput) (timetable.PlanningTrack, error) {
	value, err := e.service.CreatePlanningTrack(ctx, planningTrackFields(input))
	return planningTrackToPublic(value), MapError(err)
}

func (e Engine) UpdatePlanningTrack(ctx context.Context, id int64, input timetable.PlanningTrackInput) (timetable.PlanningTrack, error) {
	value, _, err := e.service.UpdatePlanningTrack(ctx, id, planningTrackFields(input), false)
	return planningTrackToPublic(value), MapError(err)
}

func (e Engine) UpdateActivePlanningTrack(ctx context.Context, id int64, input timetable.PlanningTrackInput) (timetable.PlanningTrack, bool, error) {
	value, updated, err := e.service.UpdatePlanningTrack(ctx, id, planningTrackFields(input), true)
	return planningTrackToPublic(value), updated, MapError(err)
}

func (e Engine) DeletePlanningTrack(ctx context.Context, id int64) error {
	return MapError(e.service.DeletePlanningTrack(ctx, id))
}

func (e Engine) SetPlanningTrackArchivedAt(ctx context.Context, id int64, value *time.Time) (timetable.PlanningTrack, bool, error) {
	track, updated, err := e.service.SetPlanningTrackArchivedAt(ctx, id, value)
	return planningTrackToPublic(track), updated, MapError(err)
}

func (e Engine) ReorderPlanningTracks(ctx context.Context, ids []int64) error {
	return MapError(e.service.ReorderPlanningTracks(ctx, ids))
}

func (e Engine) RestorePlanningTrackAtEnd(ctx context.Context, id int64) (timetable.PlanningTrack, bool, error) {
	track, restored, err := e.service.RestorePlanningTrackAtEnd(ctx, id)
	return planningTrackToPublic(track), restored, MapError(err)
}

func (e Engine) FindRecurrenceRule(ctx context.Context, id int64) (timetable.RecurrenceRule, error) {
	value, err := e.service.FindRecurrenceRule(ctx, id)
	return recurrenceRuleToPublic(value), MapError(err)
}

func (e Engine) ListRecurrenceRules(ctx context.Context, filter timetable.RecurrenceRuleFilter) ([]timetable.RecurrenceRule, error) {
	values, err := e.service.ListRecurrenceRules(ctx, domain.RecurrenceRuleFilter{
		Frequency: filter.Frequency, Frequencies: filter.Frequencies, Weekday: filter.Weekday, ActiveAt: filter.ActiveAt,
		SortBy: filter.SortBy, SortDescending: filter.SortDescending, Limit: filter.Limit, Offset: filter.Offset,
	})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.RecurrenceRule, 0, len(values))
	for _, value := range values {
		result = append(result, recurrenceRuleToPublic(value))
	}
	return result, nil
}

func (e Engine) CreateRecurrenceRule(ctx context.Context, input timetable.RecurrenceRuleInput) (timetable.RecurrenceRule, error) {
	value, err := e.service.CreateRecurrenceRule(ctx, recurrenceRuleFields(input))
	return recurrenceRuleToPublic(value), MapError(err)
}

func (e Engine) UpdateRecurrenceRule(ctx context.Context, id int64, input timetable.RecurrenceRuleInput) (timetable.RecurrenceRule, error) {
	value, err := e.service.UpdateRecurrenceRule(ctx, id, recurrenceRuleFields(input))
	return recurrenceRuleToPublic(value), MapError(err)
}

func (e Engine) DeleteRecurrenceRule(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteRecurrenceRule(ctx, id))
}

func (e Engine) FindActivityException(ctx context.Context, id int64) (timetable.ActivityException, error) {
	value, err := e.service.FindActivityException(ctx, id)
	return activityExceptionToPublic(value), MapError(err)
}

func (e Engine) ListActivityExceptions(ctx context.Context, filter timetable.ActivityExceptionFilter) ([]timetable.ActivityException, error) {
	values, err := e.service.ListActivityExceptions(ctx, domain.ActivityExceptionFilter{
		ActivityGroupID: filter.ActivityGroupID, ExceptionDate: filter.ExceptionDate,
		FromDate: filter.FromDate, ToDate: filter.ToDate, BeforeDate: filter.BeforeDate,
		OrderByDate: filter.OrderByDate, Limit: filter.Limit, Offset: filter.Offset,
	})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.ActivityException, 0, len(values))
	for _, value := range values {
		result = append(result, activityExceptionToPublic(value))
	}
	return result, nil
}

func (e Engine) CountActivityExceptions(ctx context.Context, before *string) (int, error) {
	result, err := e.service.CountActivityExceptions(ctx, before)
	return result, MapError(err)
}

func (e Engine) OldestActivityExceptionBefore(ctx context.Context, before *string) (*string, error) {
	result, err := e.service.OldestActivityExceptionBefore(ctx, before)
	return result, MapError(err)
}

func (e Engine) CreateActivityException(ctx context.Context, input timetable.ActivityExceptionInput) (timetable.ActivityException, error) {
	value, err := e.service.CreateActivityException(ctx, activityExceptionFields(input))
	return activityExceptionToPublic(value), MapError(err)
}

func (e Engine) UpdateActivityException(ctx context.Context, id int64, input timetable.ActivityExceptionInput) (timetable.ActivityException, error) {
	value, err := e.service.UpdateActivityException(ctx, id, activityExceptionFields(input))
	return activityExceptionToPublic(value), MapError(err)
}

func (e Engine) DeleteActivityException(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteActivityException(ctx, id))
}

func (e Engine) DeleteActivityExceptionsBefore(ctx context.Context, before string) (int64, error) {
	rows, err := e.service.DeleteActivityExceptionsBefore(ctx, before)
	return rows, MapError(err)
}

func (e Engine) FindActivityInstance(ctx context.Context, id int64) (timetable.ActivityInstance, error) {
	value, err := e.service.FindActivityInstance(ctx, id)
	return activityInstanceToPublic(value), MapError(err)
}

func (e Engine) ListActivityInstances(ctx context.Context, filter timetable.ActivityInstanceFilter) ([]timetable.ActivityInstance, error) {
	values, err := e.service.ListActivityInstances(ctx, domain.ActivityInstanceFilter(filter))
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.ActivityInstance, 0, len(values))
	for _, value := range values {
		result = append(result, activityInstanceToPublic(value))
	}
	return result, nil
}

func (e Engine) MaxActivityInstanceID(ctx context.Context) (int64, error) {
	result, err := e.service.MaxActivityInstanceID(ctx)
	return result, MapError(err)
}

func (e Engine) CountActivityInstances(ctx context.Context, before *string) (int, error) {
	result, err := e.service.CountActivityInstances(ctx, before)
	return result, MapError(err)
}

func (e Engine) OldestActivityInstanceBefore(ctx context.Context, before *string) (*string, error) {
	result, err := e.service.OldestActivityInstanceBefore(ctx, before)
	return result, MapError(err)
}

func (e Engine) CreateActivityInstance(ctx context.Context, input timetable.ActivityInstanceInput) (timetable.ActivityInstance, error) {
	value, err := e.service.CreateActivityInstance(ctx, domain.ActivityInstanceFields(input))
	return activityInstanceToPublic(value), MapError(err)
}

func (e Engine) CreateTemplateBackedActivityInstanceIfAbsent(ctx context.Context, input timetable.ActivityInstanceInput) (timetable.ActivityInstance, bool, error) {
	value, inserted, err := e.service.CreateTemplateBackedActivityInstanceIfAbsent(ctx, domain.ActivityInstanceFields(input))
	return activityInstanceToPublic(value), inserted, MapError(err)
}

func (e Engine) CreateIdempotentActivityInstance(ctx context.Context, input timetable.ActivityInstanceInput) (timetable.ActivityInstance, bool, error) {
	value, inserted, err := e.service.CreateIdempotentActivityInstance(ctx, domain.ActivityInstanceFields(input))
	return activityInstanceToPublic(value), inserted, MapError(err)
}

func (e Engine) UpdateActivityInstance(ctx context.Context, id int64, input timetable.ActivityInstanceInput) (timetable.ActivityInstance, error) {
	value, err := e.service.UpdateActivityInstance(ctx, id, domain.ActivityInstanceFields(input))
	return activityInstanceToPublic(value), MapError(err)
}

func (e Engine) PatchActivityInstance(ctx context.Context, id int64, input timetable.ActivityInstanceInput, columns []string) (int64, error) {
	rows, err := e.service.PatchActivityInstance(ctx, id, domain.ActivityInstanceFields(input), columns)
	return rows, MapError(err)
}

func (e Engine) DeleteActivityInstance(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteActivityInstance(ctx, id))
}

func (e Engine) DeletePlannedActivityInstances(ctx context.Context, from string, to *string, groupID *int64, preserveDeviations bool) (int64, error) {
	rows, err := e.service.DeletePlannedActivityInstances(ctx, from, to, groupID, preserveDeviations)
	return rows, MapError(err)
}

func (e Engine) DeleteRemovedWeekendActivityInstances(ctx context.Context, groupID int64, weekdays []int) (int64, error) {
	rows, err := e.service.DeleteRemovedWeekendActivityInstances(ctx, groupID, weekdays)
	return rows, MapError(err)
}

func (e Engine) PropagateActivityInstanceListKind(ctx context.Context, groupID int64, previousKind, newKind *string, after string) (int64, error) {
	rows, err := e.service.PropagateActivityInstanceListKind(ctx, groupID, previousKind, newKind, after)
	return rows, MapError(err)
}

func (e Engine) DeleteActivityInstancesBefore(ctx context.Context, before string) (int64, error) {
	rows, err := e.service.DeleteActivityInstancesBefore(ctx, before)
	return rows, MapError(err)
}

func (e Engine) FindInstanceStaff(ctx context.Context, id int64) (timetable.InstanceStaff, error) {
	value, err := e.service.FindInstanceStaff(ctx, id)
	return instanceStaffToPublic(value), MapError(err)
}

func (e Engine) ListInstanceStaff(ctx context.Context, filter timetable.InstanceStaffFilter) ([]timetable.InstanceStaff, error) {
	values, err := e.service.ListInstanceStaff(ctx, domain.InstanceStaffFilter(filter))
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.InstanceStaff, 0, len(values))
	for _, value := range values {
		result = append(result, instanceStaffToPublic(value))
	}
	return result, nil
}

func (e Engine) CountNonAbsentInstanceStaff(ctx context.Context, instanceIDs []int64) (map[int64]int, error) {
	result, err := e.service.CountNonAbsentInstanceStaff(ctx, instanceIDs)
	return result, MapError(err)
}

func (e Engine) CreateInstanceStaff(ctx context.Context, input timetable.InstanceStaffInput) (timetable.InstanceStaff, error) {
	value, err := e.service.CreateInstanceStaff(ctx, domain.InstanceStaffFields(input))
	return instanceStaffToPublic(value), MapError(err)
}

func (e Engine) UpdateInstanceStaff(ctx context.Context, id int64, input timetable.InstanceStaffInput) (timetable.InstanceStaff, error) {
	value, err := e.service.UpdateInstanceStaff(ctx, id, domain.InstanceStaffFields(input))
	return instanceStaffToPublic(value), MapError(err)
}

func (e Engine) PatchInstanceStaff(ctx context.Context, id int64, input timetable.InstanceStaffInput, columns []string) (int64, error) {
	rows, err := e.service.PatchInstanceStaff(ctx, id, domain.InstanceStaffFields(input), columns)
	return rows, MapError(err)
}

func (e Engine) DeleteInstanceStaff(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteInstanceStaff(ctx, id))
}

func (e Engine) DeleteInstanceStaffByInstance(ctx context.Context, instanceID int64) error {
	return MapError(e.service.DeleteInstanceStaffByInstance(ctx, instanceID))
}

func (e Engine) DeleteUpcomingInstanceStaff(ctx context.Context, staffID int64, after string) (int64, error) {
	rows, err := e.service.DeleteUpcomingInstanceStaff(ctx, staffID, after)
	return rows, MapError(err)
}

func (e Engine) FindInstanceStudent(ctx context.Context, id int64) (timetable.InstanceStudent, error) {
	value, err := e.service.FindInstanceStudent(ctx, id)
	return instanceStudentToPublic(value), MapError(err)
}

func (e Engine) ListInstanceStudents(ctx context.Context, filter timetable.InstanceStudentFilter) ([]timetable.InstanceStudent, error) {
	values, err := e.service.ListInstanceStudents(ctx, domain.InstanceStudentFilter(filter))
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.InstanceStudent, 0, len(values))
	for _, value := range values {
		result = append(result, instanceStudentToPublic(value))
	}
	return result, nil
}

func (e Engine) CreateInstanceStudent(ctx context.Context, input timetable.InstanceStudentInput) (timetable.InstanceStudent, error) {
	value, err := e.service.CreateInstanceStudent(ctx, domain.InstanceStudentFields(input))
	return instanceStudentToPublic(value), MapError(err)
}

func (e Engine) UpdateInstanceStudent(ctx context.Context, id int64, input timetable.InstanceStudentInput) (timetable.InstanceStudent, error) {
	value, err := e.service.UpdateInstanceStudent(ctx, id, domain.InstanceStudentFields(input))
	return instanceStudentToPublic(value), MapError(err)
}

func (e Engine) DeleteInstanceStudent(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteInstanceStudent(ctx, id))
}

func (e Engine) DeleteInstanceStudentsByInstance(ctx context.Context, instanceID int64) error {
	return MapError(e.service.DeleteInstanceStudentsByInstance(ctx, instanceID))
}

func (e Engine) ListStudentInstanceRefsBefore(ctx context.Context, cutoff string) ([]timetable.StudentInstanceRef, error) {
	values, err := e.service.ListStudentInstanceRefsBefore(ctx, cutoff)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.StudentInstanceRef, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.StudentInstanceRef(value))
	}
	return result, nil
}

func (e Engine) ListPlannedStudentIDs(ctx context.Context, studentIDs []int64, date string) ([]int64, error) {
	result, err := e.service.ListPlannedStudentIDs(ctx, studentIDs, date)
	return result, MapError(err)
}

func (e Engine) ReplaceGroupTargets(ctx context.Context, groupID int64, targets []timetable.GroupTargetInput) error {
	values := make([]domain.GroupTargetFields, 0, len(targets))
	for _, target := range targets {
		values = append(values, domain.GroupTargetFields{
			TargetGroupType: target.TargetGroupType, TargetGradeLevel: target.TargetGradeLevel,
			TargetSchoolClass: target.TargetSchoolClass, EducationGroupID: target.EducationGroupID,
		})
	}
	return MapError(e.service.ReplaceGroupTargets(ctx, groupID, values))
}

func (e Engine) CreateGroup(ctx context.Context, input timetable.GroupInput) (timetable.Group, error) {
	value, err := e.service.CreateGroup(ctx, groupFields(input))
	return groupToPublic(value), MapError(err)
}

func (e Engine) UpdateGroup(ctx context.Context, id int64, input timetable.GroupInput) (timetable.Group, error) {
	value, err := e.service.UpdateGroup(ctx, id, groupFields(input))
	return groupToPublic(value), MapError(err)
}

func (e Engine) DeleteGroup(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteGroup(ctx, id))
}

func (e Engine) UpdateTemplate(ctx context.Context, id int64, input timetable.TemplateUpdate) (int64, error) {
	rows, err := e.service.UpdateTemplate(ctx, id, templateFields(input))
	return rows, MapError(err)
}

func (e Engine) ArchiveTemplate(ctx context.Context, id int64) (int64, error) {
	rows, err := e.service.ArchiveTemplate(ctx, id)
	return rows, MapError(err)
}

func (e Engine) UpdateGroupOfferingSource(ctx context.Context, id int64, input timetable.OfferingSourceInput) error {
	return MapError(e.service.UpdateGroupOfferingSource(ctx, id, domain.OfferingSourceFields{
		CareOfferingIDs: input.CareOfferingIDs, GradeLevels: input.GradeLevels, SchoolClasses: input.SchoolClasses,
	}))
}

func (e Engine) CreateCategory(ctx context.Context, input timetable.CreateCategory) (timetable.Category, error) {
	value, err := e.service.CreateCategory(ctx, domain.CategoryFields{
		Name: input.Name, Description: input.Description, Color: input.Color, IsSystem: input.IsSystem,
	})
	return categoryToPublic(value), MapError(err)
}

func (e Engine) UpdateCategory(ctx context.Context, input timetable.UpdateCategory) (timetable.Category, error) {
	value, err := e.service.UpdateCategory(ctx, input.ID, domain.CategoryFields{
		Name: input.Name, Description: input.Description, Color: input.Color,
	})
	return categoryToPublic(value), MapError(err)
}

func (e Engine) ArchiveCategory(ctx context.Context, id int64) (timetable.Category, error) {
	value, err := e.service.ArchiveCategory(ctx, id)
	return categoryToPublic(value), MapError(err)
}

func (e Engine) RestoreCategory(ctx context.Context, id int64) (timetable.Category, error) {
	value, err := e.service.RestoreCategory(ctx, id)
	return categoryToPublic(value), MapError(err)
}

func (e Engine) DeleteCategory(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteCategory(ctx, id))
}

func (e Engine) SetCategoryShiftTypeID(ctx context.Context, id int64, shiftTypeID *int64) error {
	return MapError(e.service.SetCategoryShiftTypeID(ctx, id, shiftTypeID))
}

func (e Engine) SetCategoryShiftTypeLinks(ctx context.Context, shiftTypeID int64, categoryIDs []int64) error {
	return MapError(e.service.SetCategoryShiftTypeLinks(ctx, shiftTypeID, categoryIDs))
}

func (e Engine) LockStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string) error {
	return MapError(e.service.LockStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil))
}

func (e Engine) EndStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string) (timetable.CareExitEnrollmentChanges, error) {
	changes, err := e.service.EndStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil)
	return careExitEnrollmentChangesToPublic(changes), MapError(err)
}

func (e Engine) RestoreStudentEnrollmentsForCareExit(ctx context.Context, studentIDs, periodIDs []int64, removals []timetable.CareExitEnrollmentRemoval) (int, error) {
	result, err := RestoreStudentEnrollmentsForCareExit(ctx, e.service, e.provenance, studentIDs, periodIDs, careExitEnrollmentRemovalsToDomain(removals))
	return result, MapError(err)
}

func careExitEnrollmentChangesToPublic(value domain.CareExitEnrollmentChanges) timetable.CareExitEnrollmentChanges {
	result := timetable.CareExitEnrollmentChanges{
		Deleted: make([]timetable.CareExitEnrollment, 0, len(value.Deleted)),
		Capped:  make([]timetable.CareExitEnrollmentCap, 0, len(value.Capped)),
	}
	for _, enrollment := range value.Deleted {
		result.Deleted = append(result.Deleted, careExitEnrollmentToPublic(enrollment))
	}
	for _, cap := range value.Capped {
		result.Capped = append(result.Capped, timetable.CareExitEnrollmentCap{
			TenantID: cap.TenantID, StudentID: cap.StudentID, ID: cap.ID, PreviousValidUntil: cap.PreviousValidUntil,
		})
	}
	return result
}

func careExitEnrollmentToPublic(value domain.CareExitEnrollment) timetable.CareExitEnrollment {
	return timetable.CareExitEnrollment{
		ID: value.ID, TenantID: value.TenantID, StudentID: value.StudentID,
		ActivityGroupID: value.ActivityGroupID, ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil,
		CalendarPeriodID: value.CalendarPeriodID, EnrollmentRequestChildID: value.EnrollmentRequestChildID,
		SelectedWeekdays: value.SelectedWeekdays, AttendanceStatus: value.AttendanceStatus, Weekday: value.Weekday,
	}
}

func careExitEnrollmentRemovalsToDomain(values []timetable.CareExitEnrollmentRemoval) []domain.CareExitEnrollmentRemoval {
	result := make([]domain.CareExitEnrollmentRemoval, 0, len(values))
	for _, value := range values {
		result = append(result, domain.CareExitEnrollmentRemoval{
			CareExitEnrollment: domain.CareExitEnrollment{
				ID: value.ID, TenantID: value.TenantID, StudentID: value.StudentID,
				ActivityGroupID: value.ActivityGroupID, ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil,
				CalendarPeriodID: value.CalendarPeriodID, EnrollmentRequestChildID: value.EnrollmentRequestChildID,
				SelectedWeekdays: value.SelectedWeekdays, AttendanceStatus: value.AttendanceStatus, Weekday: value.Weekday,
			},
			WasDeleted: value.WasDeleted, PreviousValidUntil: value.PreviousValidUntil,
		})
	}
	return result
}

func categoryToPublic(value domain.Category) timetable.Category {
	return timetable.Category{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Name: value.Name, Description: value.Description, Color: value.Color, IsSystem: value.IsSystem,
		ShiftTypeID: value.ShiftTypeID, ArchivedAt: value.ArchivedAt,
	}
}

func groupTargetToPublic(value domain.GroupTarget) timetable.GroupTarget {
	return timetable.GroupTarget{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		ActivityGroupID: value.ActivityGroupID,
		TargetGroupType: value.TargetGroupType, TargetGradeLevel: value.TargetGradeLevel,
		TargetSchoolClass: value.TargetSchoolClass, EducationGroupID: value.EducationGroupID,
		EducationGroupName: value.EducationGroupName,
	}
}

func scheduleToPublic(value domain.Schedule) timetable.Schedule {
	return timetable.Schedule{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Weekday: value.Weekday, TimeframeID: value.TimeframeID, ActivityGroupID: value.ActivityGroupID,
		WeekPattern: value.WeekPattern, CalendarPeriodID: value.CalendarPeriodID,
		ValidUntil: value.ValidUntil, ValidFrom: value.ValidFrom,
	}
}

func scheduleFields(value timetable.ScheduleInput) domain.ScheduleFields {
	return domain.ScheduleFields{
		Weekday: value.Weekday, TimeframeID: value.TimeframeID, ActivityGroupID: value.ActivityGroupID,
		WeekPattern: value.WeekPattern, CalendarPeriodID: value.CalendarPeriodID,
		ValidUntil: value.ValidUntil, ValidFrom: value.ValidFrom,
	}
}

func plannedSupervisorToPublic(value domain.PlannedSupervisor) timetable.PlannedSupervisor {
	return timetable.PlannedSupervisor{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		StaffID: value.StaffID, GroupID: value.GroupID, IsPrimary: value.IsPrimary, ValidFrom: value.ValidFrom,
		ValidUntil: value.ValidUntil, CalendarPeriodID: value.CalendarPeriodID, Weekday: value.Weekday}
}

func plannedSupervisorFields(value timetable.PlannedSupervisorInput) domain.PlannedSupervisorFields {
	return domain.PlannedSupervisorFields{StaffID: value.StaffID, GroupID: value.GroupID, IsPrimary: value.IsPrimary,
		ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil, CalendarPeriodID: value.CalendarPeriodID, Weekday: value.Weekday}
}

func studentEnrollmentToPublic(value domain.StudentEnrollment) timetable.StudentEnrollment {
	return timetable.StudentEnrollment{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		StudentID: value.StudentID, ActivityGroupID: value.ActivityGroupID, ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil,
		CalendarPeriodID: value.CalendarPeriodID, EnrollmentRequestChildID: value.EnrollmentRequestChildID,
		SelectedWeekdays: value.SelectedWeekdays, AttendanceStatus: value.AttendanceStatus, Weekday: value.Weekday}
}

func studentEnrollmentFields(value timetable.StudentEnrollmentInput) domain.StudentEnrollmentFields {
	return domain.StudentEnrollmentFields{StudentID: value.StudentID, ActivityGroupID: value.ActivityGroupID,
		ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil, CalendarPeriodID: value.CalendarPeriodID,
		EnrollmentRequestChildID: value.EnrollmentRequestChildID, SelectedWeekdays: value.SelectedWeekdays,
		AttendanceStatus: value.AttendanceStatus, Weekday: value.Weekday}
}

func timeframeToPublic(value domain.Timeframe) timetable.Timeframe {
	return timetable.Timeframe{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		StartTime: value.StartTime, EndTime: value.EndTime, IsActive: value.IsActive, Description: value.Description}
}

func timeframeFields(value timetable.TimeframeInput) domain.TimeframeFields {
	return domain.TimeframeFields{StartTime: value.StartTime, EndTime: value.EndTime,
		IsActive: value.IsActive, Description: value.Description}
}

func planningTrackToPublic(value domain.PlanningTrack) timetable.PlanningTrack {
	return timetable.PlanningTrack{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Name: value.Name, Color: value.Color, SortOrder: value.SortOrder, ArchivedAt: value.ArchivedAt}
}

func planningTrackFields(value timetable.PlanningTrackInput) domain.PlanningTrackFields {
	return domain.PlanningTrackFields{Name: value.Name, Color: value.Color, SortOrder: value.SortOrder, ArchivedAt: value.ArchivedAt}
}

func recurrenceRuleToPublic(value domain.RecurrenceRule) timetable.RecurrenceRule {
	return timetable.RecurrenceRule{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Frequency: value.Frequency, IntervalCount: value.IntervalCount, Weekdays: value.Weekdays, MonthDays: value.MonthDays,
		EndDate: value.EndDate, Count: value.Count}
}

func recurrenceRuleFields(value timetable.RecurrenceRuleInput) domain.RecurrenceRuleFields {
	return domain.RecurrenceRuleFields{Frequency: value.Frequency, IntervalCount: value.IntervalCount,
		Weekdays: value.Weekdays, MonthDays: value.MonthDays, EndDate: value.EndDate, Count: value.Count}
}

func activityExceptionToPublic(value domain.ActivityException) timetable.ActivityException {
	return timetable.ActivityException{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		ActivityGroupID: value.ActivityGroupID, ExceptionDate: value.ExceptionDate, ExceptionType: value.ExceptionType,
		StartTime: value.StartTime, EndTime: value.EndTime, RoomID: value.RoomID, Reason: value.Reason, CreatedBy: value.CreatedBy,
	}
}

func activityExceptionFields(value timetable.ActivityExceptionInput) domain.ActivityExceptionFields {
	return domain.ActivityExceptionFields{
		ActivityGroupID: value.ActivityGroupID, ExceptionDate: value.ExceptionDate, ExceptionType: value.ExceptionType,
		StartTime: value.StartTime, EndTime: value.EndTime, RoomID: value.RoomID, Reason: value.Reason, CreatedBy: value.CreatedBy,
	}
}

func instanceStaffToPublic(value domain.InstanceStaff) timetable.InstanceStaff {
	return timetable.InstanceStaff{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		InstanceID: value.InstanceID, StaffID: value.StaffID, RoomID: value.RoomID, IsPrimary: value.IsPrimary,
		IsSubstitute: value.IsSubstitute, IsAbsent: value.IsAbsent, AbsenceReason: value.AbsenceReason,
		SickAbsenceID: value.SickAbsenceID,
	}
}

func MapError(err error) error {
	if errors.Is(err, domain.ErrOffboardingConflict) {
		return timetable.ErrOffboardingConflict
	}
	if errors.Is(err, domain.ErrInvalidRecurrenceRange) {
		return timetable.ErrInvalidRecurrenceRange
	}
	switch {
	case errors.Is(err, domain.ErrCategoryNotFound):
		return timetable.ErrCategoryNotFound
	case errors.Is(err, domain.ErrCategoryNameConflict):
		return timetable.ErrCategoryNameExists
	case errors.Is(err, domain.ErrUnknownCategoryIDs):
		return timetable.ErrUnknownCategoryIDs
	case errors.Is(err, domain.ErrSystemCategoryProtected):
		return timetable.ErrSystemCategoryProtected
	case errors.Is(err, domain.ErrSystemCategoryName):
		return timetable.ErrSystemCategoryName
	case errors.Is(err, domain.ErrCategoryArchived):
		return timetable.ErrCategoryArchived
	case errors.Is(err, domain.ErrGroupNotFound):
		return timetable.ErrGroupNotFound
	case errors.Is(err, domain.ErrScheduleNotFound):
		return timetable.ErrScheduleNotFound
	case errors.Is(err, domain.ErrPlannedSupervisorNotFound):
		return timetable.ErrPlannedSupervisorNotFound
	case errors.Is(err, domain.ErrStudentEnrollmentNotFound):
		return timetable.ErrStudentEnrollmentNotFound
	case errors.Is(err, domain.ErrTimeframeNotFound):
		return timetable.ErrTimeframeNotFound
	case errors.Is(err, domain.ErrPlanningTrackNotFound):
		return timetable.ErrPlanningTrackNotFound
	case errors.Is(err, domain.ErrPlanningTrackNameConflict):
		return fmt.Errorf("%w: %w", timetable.ErrPlanningTrackNameExists, err)
	case errors.Is(err, domain.ErrRecurrenceRuleNotFound):
		return timetable.ErrRecurrenceRuleNotFound
	case errors.Is(err, domain.ErrActivityExceptionNotFound):
		return timetable.ErrActivityExceptionNotFound
	case errors.Is(err, domain.ErrActivityInstanceNotFound):
		return timetable.ErrActivityInstanceNotFound
	case errors.Is(err, domain.ErrInstanceStaffNotFound):
		return timetable.ErrInstanceStaffNotFound
	case errors.Is(err, domain.ErrInstanceStudentNotFound):
		return timetable.ErrInstanceStudentNotFound
	case errors.Is(err, domain.ErrPickupExtensionNotFound):
		return timetable.ErrPickupExtensionNotFound
	case errors.Is(err, domain.ErrPickupExtensionBlockGone):
		return timetable.ErrPickupExtensionBlockGone
	default:
		return err
	}
}

func activityInstanceToPublic(value domain.ActivityInstance) timetable.ActivityInstance {
	return timetable.ActivityInstance{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Date: value.Date, ActivityGroupID: value.ActivityGroupID, CalendarPeriodID: value.CalendarPeriodID,
		Title: value.Title, Description: value.Description, StartTime: value.StartTime, EndTime: value.EndTime,
		RoomID: value.RoomID, RequiredStaff: value.RequiredStaff, Status: value.Status,
		ListKind: value.ListKind, IsSpontaneous: value.IsSpontaneous, UnderstaffedAck: value.UnderstaffedAck,
		UnderstaffedNote: value.UnderstaffedNote, CancelReason: value.CancelReason, Notes: value.Notes,
		IdempotencyKey: value.IdempotencyKey, IdempotencyFingerprint: value.IdempotencyFingerprint,
		CreatedBy: value.CreatedBy,
	}
}

func instanceStudentToPublic(value domain.InstanceStudent) timetable.InstanceStudent {
	return timetable.InstanceStudent{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		InstanceID: value.InstanceID, StudentID: value.StudentID, RoomID: value.RoomID}
}
