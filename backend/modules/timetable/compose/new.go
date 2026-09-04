// Package compose wires the Timetable & Activities module over the shared
// tenant runtime and Bun database.
package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	DB      *bun.DB
	Observe func(Observation)
}

func New(dependencies Dependencies) (*timetable.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("timetable compose: all dependencies are required")
	}
	store := postgres.New(databaseRuntime(dependencies.DB))
	observe := func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	}
	service := application.New(store, transaction{}, observe)
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
	default:
		return err
	}
}
