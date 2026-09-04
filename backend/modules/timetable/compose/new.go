// Package compose wires the Timetable & Activities module over the shared
// tenant runtime and Bun database.
package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

type Dependencies struct {
	DB       *bun.DB
	Students StudentDirectory
	Observe  func(Observation)
}

func New(dependencies Dependencies) (*timetable.Module, error) {
	if dependencies.DB == nil || dependencies.Students == nil || dependencies.Observe == nil {
		return nil, errors.New("timetable compose: all dependencies are required")
	}
	store := postgres.New(databaseRuntime(dependencies.DB))
	observe := func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	}
	service := application.New(store, transaction{}, studentDirectory{query: dependencies.Students},
		func() string { return timezone.TodayDate().String() }, observe)
	return timetable.NewModule(engine{service: service, observe: observe}), nil
}

func databaseRuntime(db *bun.DB) postgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("timetable postgres: tenant is required: %w", err)
		}
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return db, tenantID.Int64(), nil
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, tenantID.Int64(), nil
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID.Int64(), nil
			}
			return db, tenantID.Int64(), nil
		default:
			return nil, 0, fmt.Errorf("timetable postgres: unsupported transaction %T", transaction)
		}
	}
}

type transaction struct{}

func (transaction) RunWrite(ctx context.Context, retry bool, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	if !retry {
		return tenant.WithinCurrentTenant(ctx, callback)
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	return tenant.WithinTenantRetry(ctx, tenantID, callback)
}

type engine struct {
	service *application.Service
	observe ports.Observer
}

func (e engine) ObserveRejection(operation string, duration time.Duration, err error) {
	e.observe(ports.Observation{Operation: operation, Duration: duration, Err: err})
}

func (e engine) FindCategory(ctx context.Context, id int64) (timetable.Category, error) {
	value, err := e.service.FindCategory(ctx, id)
	return categoryToPublic(value), mapError(err)
}

func (e engine) FindCategoryForAssignment(ctx context.Context, id int64) (timetable.Category, error) {
	value, err := e.service.FindCategoryForAssignment(ctx, id)
	return categoryToPublic(value), mapError(err)
}

func (e engine) FindCategoryByName(ctx context.Context, name string) (timetable.Category, error) {
	value, err := e.service.FindCategoryByName(ctx, name)
	return categoryToPublic(value), mapError(err)
}

func (e engine) ListCategories(ctx context.Context) ([]timetable.Category, error) {
	values, err := e.service.ListCategories(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]timetable.Category, 0, len(values))
	for _, value := range values {
		result = append(result, categoryToPublic(value))
	}
	return result, nil
}

func (e engine) CountCategoryUsage(ctx context.Context) (map[int64]int, error) {
	result, err := e.service.CountCategoryUsage(ctx)
	return result, mapError(err)
}

func (e engine) FindGroup(ctx context.Context, id int64) (timetable.Group, error) {
	value, err := e.service.FindGroup(ctx, id)
	return groupToPublic(value), mapError(err)
}

func (e engine) FindGroupForUpdate(ctx context.Context, id int64) (timetable.Group, error) {
	value, err := e.service.FindGroupForUpdate(ctx, id)
	return groupToPublic(value), mapError(err)
}

func (e engine) FindGroupByName(ctx context.Context, name string) (timetable.Group, error) {
	value, err := e.service.FindGroupByName(ctx, name)
	return groupToPublic(value), mapError(err)
}

func (e engine) ListGroups(ctx context.Context, filter timetable.GroupFilter) ([]timetable.Group, error) {
	values, err := e.service.ListGroups(ctx, domain.GroupFilter{
		Name: filter.Name, CategoryID: filter.CategoryID, IsOpen: filter.IsOpen, IsSystem: filter.IsSystem,
		IsTemplate: filter.IsTemplate, IDs: filter.IDs, SeriesForGroupID: filter.SeriesForGroupID,
		SourceOfferingIDs: filter.SourceOfferingIDs, HasOfferingSource: filter.HasOfferingSource,
		ActiveOnly: filter.ActiveOnly, OrderByName: filter.OrderByName, OrderByID: filter.OrderByID,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]timetable.Group, 0, len(values))
	for _, value := range values {
		result = append(result, groupToPublic(value))
	}
	return result, nil
}

func (e engine) ListGroupTargets(ctx context.Context, ids []int64) (map[int64][]timetable.GroupTarget, error) {
	values, err := e.service.ListGroupTargets(ctx, ids)
	if err != nil {
		return nil, mapError(err)
	}
	result := make(map[int64][]timetable.GroupTarget, len(values))
	for groupID, targets := range values {
		for _, target := range targets {
			result[groupID] = append(result[groupID], groupTargetToPublic(target))
		}
	}
	return result, nil
}

func (e engine) ListTargetStudentIDs(ctx context.Context, ids []int64) (map[int64][]int64, error) {
	result, err := e.service.ListTargetStudentIDs(ctx, ids)
	return result, mapError(err)
}

func (e engine) FindSchedule(ctx context.Context, id int64) (timetable.Schedule, error) {
	value, err := e.service.FindSchedule(ctx, id)
	return scheduleToPublic(value), mapError(err)
}

func (e engine) ListSchedules(ctx context.Context, filter timetable.ScheduleFilter) ([]timetable.Schedule, error) {
	values, err := e.service.ListSchedules(ctx, domain.ScheduleFilter{GroupIDs: filter.GroupIDs, Weekday: filter.Weekday})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]timetable.Schedule, 0, len(values))
	for _, value := range values {
		result = append(result, scheduleToPublic(value))
	}
	return result, nil
}

func (e engine) FindTemplateStartTimes(ctx context.Context, groupIDs []int64) ([]timetable.TemplateStartTime, error) {
	values, err := e.service.FindTemplateStartTimes(ctx, groupIDs)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]timetable.TemplateStartTime, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.TemplateStartTime(value))
	}
	return result, nil
}

func (e engine) CreateSchedule(ctx context.Context, input timetable.ScheduleInput) (timetable.Schedule, error) {
	value, err := e.service.CreateSchedule(ctx, scheduleFields(input))
	return scheduleToPublic(value), mapError(err)
}

func (e engine) UpdateSchedule(ctx context.Context, id int64, input timetable.ScheduleInput) (timetable.Schedule, error) {
	value, err := e.service.UpdateSchedule(ctx, id, scheduleFields(input))
	return scheduleToPublic(value), mapError(err)
}

func (e engine) DeleteSchedule(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteSchedule(ctx, id))
}

func (e engine) DeleteSchedulesByGroup(ctx context.Context, groupID int64) error {
	return mapError(e.service.DeleteSchedulesByGroup(ctx, groupID))
}

func (e engine) CapScheduleValidUntil(ctx context.Context, groupID int64, validUntil string) (int64, error) {
	rows, err := e.service.CapScheduleValidUntil(ctx, groupID, validUntil)
	return rows, mapError(err)
}

func (e engine) FindPlannedSupervisor(ctx context.Context, id int64) (timetable.PlannedSupervisor, error) {
	value, err := e.service.FindPlannedSupervisor(ctx, id)
	return plannedSupervisorToPublic(value), mapError(err)
}

func (e engine) ListPlannedSupervisors(ctx context.Context, filter timetable.PlannedSupervisorFilter) ([]timetable.PlannedSupervisor, error) {
	values, err := e.service.ListPlannedSupervisors(ctx, domain.PlannedSupervisorFilter{GroupIDs: filter.GroupIDs, StaffID: filter.StaffID})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]timetable.PlannedSupervisor, 0, len(values))
	for _, value := range values {
		result = append(result, plannedSupervisorToPublic(value))
	}
	return result, nil
}

func (e engine) ListPlannedSupervisionBlockers(ctx context.Context, staffID int64) ([]timetable.PlannedSupervisionBlocker, error) {
	values, err := e.service.ListPlannedSupervisionBlockers(ctx, staffID)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]timetable.PlannedSupervisionBlocker, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.PlannedSupervisionBlocker(value))
	}
	return result, nil
}

func (e engine) CreatePlannedSupervisor(ctx context.Context, input timetable.PlannedSupervisorInput) (timetable.PlannedSupervisor, error) {
	value, err := e.service.CreatePlannedSupervisor(ctx, plannedSupervisorFields(input))
	return plannedSupervisorToPublic(value), mapError(err)
}

func (e engine) UpdatePlannedSupervisor(ctx context.Context, id int64, input timetable.PlannedSupervisorInput) (timetable.PlannedSupervisor, error) {
	value, err := e.service.UpdatePlannedSupervisor(ctx, id, plannedSupervisorFields(input))
	return plannedSupervisorToPublic(value), mapError(err)
}

func (e engine) DeletePlannedSupervisor(ctx context.Context, id int64) error {
	return mapError(e.service.DeletePlannedSupervisor(ctx, id))
}

func (e engine) SetPrimaryPlannedSupervisor(ctx context.Context, id int64) error {
	return mapError(e.service.SetPrimaryPlannedSupervisor(ctx, id))
}

func (e engine) DeletePlannedSupervisorsByStaff(ctx context.Context, staffID int64) (int64, error) {
	rows, err := e.service.DeletePlannedSupervisorsByStaff(ctx, staffID)
	return rows, mapError(err)
}

func (e engine) CapActivePlannedSupervisors(ctx context.Context, groupID int64, validUntil string) (int64, error) {
	rows, err := e.service.CapActivePlannedSupervisors(ctx, groupID, validUntil)
	return rows, mapError(err)
}

func (e engine) SetPlannedSupervisorValidUntil(ctx context.Context, id int64, validUntil string) error {
	return mapError(e.service.SetPlannedSupervisorValidUntil(ctx, id, validUntil))
}

func (e engine) CloseOpenPlannedSupervisors(ctx context.Context, groupID int64, periodID *int64, validUntil string) error {
	return mapError(e.service.CloseOpenPlannedSupervisors(ctx, groupID, periodID, validUntil))
}

func (e engine) FindStudentEnrollment(ctx context.Context, id int64) (timetable.StudentEnrollment, error) {
	value, err := e.service.FindStudentEnrollment(ctx, id)
	return studentEnrollmentToPublic(value), mapError(err)
}

func (e engine) ListStudentEnrollments(ctx context.Context, filter timetable.StudentEnrollmentFilter) ([]timetable.StudentEnrollment, error) {
	values, err := e.service.ListStudentEnrollments(ctx, domain.StudentEnrollmentFilter{
		StudentIDs: filter.StudentIDs, ActivityGroupIDs: filter.ActivityGroupIDs, ActiveOn: filter.ActiveOn,
		Limit: filter.Limit, Offset: filter.Offset, OrderByValidFrom: filter.OrderByValidFrom, OrderByGroupName: filter.OrderByGroupName,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]timetable.StudentEnrollment, 0, len(values))
	for _, value := range values {
		result = append(result, studentEnrollmentToPublic(value))
	}
	return result, nil
}

func (e engine) CreateStudentEnrollment(ctx context.Context, input timetable.StudentEnrollmentInput) (timetable.StudentEnrollment, error) {
	value, err := e.service.CreateStudentEnrollment(ctx, studentEnrollmentFields(input))
	return studentEnrollmentToPublic(value), mapError(err)
}

func (e engine) UpdateStudentEnrollment(ctx context.Context, id int64, input timetable.StudentEnrollmentInput) (timetable.StudentEnrollment, error) {
	value, err := e.service.UpdateStudentEnrollment(ctx, id, studentEnrollmentFields(input))
	return studentEnrollmentToPublic(value), mapError(err)
}

func (e engine) DeleteStudentEnrollment(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteStudentEnrollment(ctx, id))
}

func (e engine) BackfillStudentEnrollmentSource(ctx context.Context, studentID, requestChildID int64, groupIDs []int64) (int64, error) {
	rows, err := e.service.BackfillStudentEnrollmentSource(ctx, studentID, requestChildID, groupIDs)
	return rows, mapError(err)
}

func (e engine) DeleteStudentEnrollmentsBySource(ctx context.Context, studentID, requestChildID int64) (int64, error) {
	rows, err := e.service.DeleteStudentEnrollmentsBySource(ctx, studentID, requestChildID)
	return rows, mapError(err)
}

func (e engine) CapActiveStudentEnrollments(ctx context.Context, groupID int64, validUntil string) (int64, error) {
	rows, err := e.service.CapActiveStudentEnrollments(ctx, groupID, validUntil)
	return rows, mapError(err)
}

func (e engine) SetStudentEnrollmentValidUntil(ctx context.Context, id int64, validUntil string) error {
	return mapError(e.service.SetStudentEnrollmentValidUntil(ctx, id, validUntil))
}

func (e engine) CloseOpenStudentEnrollments(ctx context.Context, groupID int64, periodID *int64, validUntil string) error {
	return mapError(e.service.CloseOpenStudentEnrollments(ctx, groupID, periodID, validUntil))
}

func (e engine) FindTimeframe(ctx context.Context, id int64) (timetable.Timeframe, error) {
	value, err := e.service.FindTimeframe(ctx, id)
	return timeframeToPublic(value), mapError(err)
}

func (e engine) ListTimeframes(ctx context.Context, filter timetable.TimeframeFilter) ([]timetable.Timeframe, error) {
	values, err := e.service.ListTimeframes(ctx, domain.TimeframeFilter{
		DescriptionContains: filter.DescriptionContains, ActiveOnly: filter.ActiveOnly,
		OverlapsStart: filter.OverlapsStart, OverlapsEnd: filter.OverlapsEnd, Limit: filter.Limit, Offset: filter.Offset,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]timetable.Timeframe, 0, len(values))
	for _, value := range values {
		result = append(result, timeframeToPublic(value))
	}
	return result, nil
}

func (e engine) CreateTimeframe(ctx context.Context, input timetable.TimeframeInput) (timetable.Timeframe, error) {
	value, err := e.service.CreateTimeframe(ctx, timeframeFields(input))
	return timeframeToPublic(value), mapError(err)
}

func (e engine) UpdateTimeframe(ctx context.Context, id int64, input timetable.TimeframeInput) (timetable.Timeframe, error) {
	value, err := e.service.UpdateTimeframe(ctx, id, timeframeFields(input))
	return timeframeToPublic(value), mapError(err)
}

func (e engine) DeleteTimeframe(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteTimeframe(ctx, id))
}

func (e engine) FindPlanningTrack(ctx context.Context, id int64) (timetable.PlanningTrack, error) {
	value, err := e.service.FindPlanningTrack(ctx, id, "")
	return planningTrackToPublic(value), mapError(err)
}

func (e engine) FindPlanningTrackForShare(ctx context.Context, id int64) (timetable.PlanningTrack, error) {
	value, err := e.service.FindPlanningTrack(ctx, id, "SHARE")
	return planningTrackToPublic(value), mapError(err)
}

func (e engine) ListPlanningTracks(ctx context.Context, filter timetable.PlanningTrackFilter) ([]timetable.PlanningTrack, error) {
	values, err := e.service.ListPlanningTracks(ctx, domain.PlanningTrackFilter{IDs: filter.IDs, Ordered: filter.Ordered})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]timetable.PlanningTrack, 0, len(values))
	for _, value := range values {
		result = append(result, planningTrackToPublic(value))
	}
	return result, nil
}

func (e engine) CreatePlanningTrack(ctx context.Context, input timetable.PlanningTrackInput) (timetable.PlanningTrack, error) {
	value, err := e.service.CreatePlanningTrack(ctx, planningTrackFields(input))
	return planningTrackToPublic(value), mapError(err)
}

func (e engine) UpdatePlanningTrack(ctx context.Context, id int64, input timetable.PlanningTrackInput) (timetable.PlanningTrack, error) {
	value, _, err := e.service.UpdatePlanningTrack(ctx, id, planningTrackFields(input), false)
	return planningTrackToPublic(value), mapError(err)
}

func (e engine) UpdateActivePlanningTrack(ctx context.Context, id int64, input timetable.PlanningTrackInput) (timetable.PlanningTrack, bool, error) {
	value, updated, err := e.service.UpdatePlanningTrack(ctx, id, planningTrackFields(input), true)
	return planningTrackToPublic(value), updated, mapError(err)
}

func (e engine) DeletePlanningTrack(ctx context.Context, id int64) error {
	return mapError(e.service.DeletePlanningTrack(ctx, id))
}

func (e engine) SetPlanningTrackArchivedAt(ctx context.Context, id int64, value *time.Time) (timetable.PlanningTrack, bool, error) {
	track, updated, err := e.service.SetPlanningTrackArchivedAt(ctx, id, value)
	return planningTrackToPublic(track), updated, mapError(err)
}

func (e engine) ReorderPlanningTracks(ctx context.Context, ids []int64) error {
	return mapError(e.service.ReorderPlanningTracks(ctx, ids))
}

func (e engine) RestorePlanningTrackAtEnd(ctx context.Context, id int64) (timetable.PlanningTrack, bool, error) {
	track, restored, err := e.service.RestorePlanningTrackAtEnd(ctx, id)
	return planningTrackToPublic(track), restored, mapError(err)
}

func (e engine) FindRecurrenceRule(ctx context.Context, id int64) (timetable.RecurrenceRule, error) {
	value, err := e.service.FindRecurrenceRule(ctx, id)
	return recurrenceRuleToPublic(value), mapError(err)
}

func (e engine) ListRecurrenceRules(ctx context.Context, filter timetable.RecurrenceRuleFilter) ([]timetable.RecurrenceRule, error) {
	values, err := e.service.ListRecurrenceRules(ctx, domain.RecurrenceRuleFilter{
		Frequency: filter.Frequency, Frequencies: filter.Frequencies, Weekday: filter.Weekday, ActiveAt: filter.ActiveAt,
		SortBy: filter.SortBy, SortDescending: filter.SortDescending, Limit: filter.Limit, Offset: filter.Offset,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]timetable.RecurrenceRule, 0, len(values))
	for _, value := range values {
		result = append(result, recurrenceRuleToPublic(value))
	}
	return result, nil
}

func (e engine) CreateRecurrenceRule(ctx context.Context, input timetable.RecurrenceRuleInput) (timetable.RecurrenceRule, error) {
	value, err := e.service.CreateRecurrenceRule(ctx, recurrenceRuleFields(input))
	return recurrenceRuleToPublic(value), mapError(err)
}

func (e engine) UpdateRecurrenceRule(ctx context.Context, id int64, input timetable.RecurrenceRuleInput) (timetable.RecurrenceRule, error) {
	value, err := e.service.UpdateRecurrenceRule(ctx, id, recurrenceRuleFields(input))
	return recurrenceRuleToPublic(value), mapError(err)
}

func (e engine) DeleteRecurrenceRule(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteRecurrenceRule(ctx, id))
}

func (e engine) ReplaceGroupTargets(ctx context.Context, groupID int64, targets []timetable.GroupTargetInput) error {
	values := make([]domain.GroupTargetFields, 0, len(targets))
	for _, target := range targets {
		values = append(values, domain.GroupTargetFields{
			TargetGroupType: target.TargetGroupType, TargetGradeLevel: target.TargetGradeLevel,
			TargetSchoolClass: target.TargetSchoolClass, EducationGroupID: target.EducationGroupID,
		})
	}
	return mapError(e.service.ReplaceGroupTargets(ctx, groupID, values))
}

func (e engine) CreateGroup(ctx context.Context, input timetable.GroupInput) (timetable.Group, error) {
	value, err := e.service.CreateGroup(ctx, groupFields(input))
	return groupToPublic(value), mapError(err)
}

func (e engine) UpdateGroup(ctx context.Context, id int64, input timetable.GroupInput) (timetable.Group, error) {
	value, err := e.service.UpdateGroup(ctx, id, groupFields(input))
	return groupToPublic(value), mapError(err)
}

func (e engine) DeleteGroup(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteGroup(ctx, id))
}

func (e engine) UpdateTemplate(ctx context.Context, id int64, input timetable.TemplateUpdate) (int64, error) {
	rows, err := e.service.UpdateTemplate(ctx, id, templateFields(input))
	return rows, mapError(err)
}

func (e engine) ArchiveTemplate(ctx context.Context, id int64) (int64, error) {
	rows, err := e.service.ArchiveTemplate(ctx, id)
	return rows, mapError(err)
}

func (e engine) UpdateGroupOfferingSource(ctx context.Context, id int64, input timetable.OfferingSourceInput) error {
	return mapError(e.service.UpdateGroupOfferingSource(ctx, id, domain.OfferingSourceFields{
		CareOfferingIDs: input.CareOfferingIDs, GradeLevels: input.GradeLevels, SchoolClasses: input.SchoolClasses,
	}))
}

func (e engine) CreateCategory(ctx context.Context, input timetable.CreateCategory) (timetable.Category, error) {
	value, err := e.service.CreateCategory(ctx, domain.CategoryFields{
		Name: input.Name, Description: input.Description, Color: input.Color, IsSystem: input.IsSystem,
	})
	return categoryToPublic(value), mapError(err)
}

func (e engine) UpdateCategory(ctx context.Context, input timetable.UpdateCategory) (timetable.Category, error) {
	value, err := e.service.UpdateCategory(ctx, input.ID, domain.CategoryFields{
		Name: input.Name, Description: input.Description, Color: input.Color,
	})
	return categoryToPublic(value), mapError(err)
}

func (e engine) ArchiveCategory(ctx context.Context, id int64) (timetable.Category, error) {
	value, err := e.service.ArchiveCategory(ctx, id)
	return categoryToPublic(value), mapError(err)
}

func (e engine) RestoreCategory(ctx context.Context, id int64) (timetable.Category, error) {
	value, err := e.service.RestoreCategory(ctx, id)
	return categoryToPublic(value), mapError(err)
}

func (e engine) SetCategoryShiftTypeLinks(ctx context.Context, shiftTypeID int64, categoryIDs []int64) error {
	return mapError(e.service.SetCategoryShiftTypeLinks(ctx, shiftTypeID, categoryIDs))
}

func (e engine) LockStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string) error {
	return mapError(e.service.LockStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil))
}

func (e engine) EndStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string) (timetable.CareExitEnrollmentChanges, error) {
	changes, err := e.service.EndStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil)
	return careExitEnrollmentChangesToPublic(changes), mapError(err)
}

func (e engine) RestoreStudentEnrollmentsForCareExit(ctx context.Context, studentIDs, periodIDs []int64, removals []timetable.CareExitEnrollmentRemoval) (int, error) {
	result, err := e.service.RestoreStudentEnrollmentsForCareExit(ctx, studentIDs, periodIDs, careExitEnrollmentRemovalsToDomain(removals))
	return result, mapError(err)
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

func groupToPublic(value domain.Group) timetable.Group {
	result := timetable.Group{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Name: value.Name, MaxParticipants: value.MaxParticipants, RequiredStaff: value.RequiredStaff, IsOpen: value.IsOpen,
		CategoryID: value.CategoryID, PlanningTrackID: value.PlanningTrackID, PlannedRoomID: value.PlannedRoomID,
		CreatedBy: value.CreatedBy, Type: value.Type, EducationGroupID: value.EducationGroupID, ListKind: value.ListKind,
		IsTemplate: value.IsTemplate, IsSystem: value.IsSystem, ArchivedAt: value.ArchivedAt, SeriesRootID: value.SeriesRootID,
		CalendarPeriodID: value.CalendarPeriodID, TargetGroupType: value.TargetGroupType,
		TargetGradeLevel: value.TargetGradeLevel, TargetSchoolClass: value.TargetSchoolClass,
		SourceCareOfferingIDs: value.SourceCareOfferingIDs, SourceGradeLevels: value.SourceGradeLevels,
		SourceSchoolClasses: value.SourceSchoolClasses, Notes: value.Notes,
	}
	if value.Category != nil {
		category := categoryToPublic(*value.Category)
		result.Category = &category
	}
	return result
}

func groupFields(value timetable.GroupInput) domain.GroupFields {
	return domain.GroupFields{
		Name: value.Name, MaxParticipants: value.MaxParticipants, RequiredStaff: value.RequiredStaff, IsOpen: value.IsOpen,
		CategoryID: value.CategoryID, PlanningTrackID: value.PlanningTrackID, PlannedRoomID: value.PlannedRoomID,
		CreatedBy: value.CreatedBy, Type: value.Type, EducationGroupID: value.EducationGroupID, ListKind: value.ListKind,
		IsTemplate: value.IsTemplate, IsSystem: value.IsSystem, ArchivedAt: value.ArchivedAt, SeriesRootID: value.SeriesRootID,
		CalendarPeriodID: value.CalendarPeriodID, TargetGroupType: value.TargetGroupType,
		TargetGradeLevel: value.TargetGradeLevel, TargetSchoolClass: value.TargetSchoolClass,
		SourceCareOfferingIDs: value.SourceCareOfferingIDs, SourceGradeLevels: value.SourceGradeLevels,
		SourceSchoolClasses: value.SourceSchoolClasses, Notes: value.Notes,
	}
}

func templateFields(value timetable.TemplateUpdate) domain.TemplateFields {
	return domain.TemplateFields{
		Name: value.Name, Type: value.Type, CategoryID: value.CategoryID,
		PlanningTrackID: value.PlanningTrackID, PlanningTrackIDProvided: value.PlanningTrackIDProvided,
		RoomID: value.RoomID, EducationGroupID: value.EducationGroupID,
		MaxParticipants: value.MaxParticipants, MaxParticipantsProvided: value.MaxParticipantsProvided,
		RequiredStaff: value.RequiredStaff, CalendarPeriodID: value.CalendarPeriodID,
		TargetGroupType: value.TargetGroupType, TargetGradeLevel: value.TargetGradeLevel,
		TargetSchoolClass: value.TargetSchoolClass, ListKind: value.ListKind, Notes: value.Notes,
		SourceCareOfferingIDs: value.SourceCareOfferingIDs, SourceGradeLevels: value.SourceGradeLevels,
		SourceSchoolClasses: value.SourceSchoolClasses,
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

func mapError(err error) error {
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
	default:
		return err
	}
}
