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
	default:
		return err
	}
}
