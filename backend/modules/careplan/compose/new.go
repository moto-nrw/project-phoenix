// Package compose wires the Care Plan module over the shared tenant runtime
// and Bun database.
package compose

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation
type AmbientDatabase func(context.Context) bun.IDB
type StatusStudentDirectory = postgres.StatusStudentDirectory
type StatusStudent = postgres.StatusStudent
type StatusSlotDirectory = postgres.StatusSlotDirectory

// StudentName is the display projection Care Plan needs for companion links.
// Composition translates the People Directory result at its boundary.
type StudentName struct {
	StudentID int64
	FirstName string
	LastName  string
}

type StudentNameFinder interface {
	ListStudentNamesByID(context.Context, []int64) ([]StudentName, error)
}

type StudentNameFinderFunc func(context.Context, []int64) ([]StudentName, error)

func (f StudentNameFinderFunc) ListStudentNamesByID(ctx context.Context, ids []int64) ([]StudentName, error) {
	return f(ctx, ids)
}

type Dependencies struct {
	DB              *bun.DB
	Observe         func(Observation)
	AmbientDB       AmbientDatabase
	StatusStudents  StatusStudentDirectory
	StatusSlots     StatusSlotDirectory
	People          StudentNameFinder
	StudentLock     careplanning.StudentLock
	StudentNotFound error
}

func New(dependencies Dependencies) (*careplan.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil || dependencies.AmbientDB == nil || dependencies.StatusStudents == nil || dependencies.StatusSlots == nil ||
		dependencies.People == nil || dependencies.StudentLock == nil || dependencies.StudentNotFound == nil {
		return nil, errors.New("care plan compose: database, observer, ambient database resolver, people directory, status-student directory, status-slot directory, and student lock are required")
	}
	bindCareLocks(dependencies)
	store := postgres.New(carePlanDatabase(dependencies.DB))
	service := application.New(store, func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	})
	statusDays := postgres.NewStatusDayStore(store, dependencies.StatusStudents, dependencies.StatusSlots)
	module := careplan.NewModule(engine{
		service: service, requests: postgres.NewRequestStore(store), statusDays: statusDays,
		observe: dependencies.Observe, people: dependencies.People, database: dependencies.DB,
	})
	return module, nil
}

func bindCareLocks(dependencies Dependencies) {
	careplanning.BindStudentLockForDB(dependencies.DB, dependencies.StudentLock, dependencies.StudentNotFound)
	careplanning.BindExceptionDayLockForDB(dependencies.DB, func(ctx context.Context, studentID int64, date string) error {
		tenantID := tenant.FromContext(ctx)
		if tenantID <= 0 {
			return errors.New("care plan: tenant is required for exception-day lock")
		}
		return postgres.LockExceptionDay(ctx, dependencies.AmbientDB(ctx), tenantID, studentID, date)
	})
}

func carePlanDatabase(db *bun.DB) postgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID := tenant.FromContext(ctx)
		if tenantID <= 0 {
			return nil, 0, errors.New("care plan: tenant is required")
		}
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return db, tenantID, nil
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, tenantID, nil
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID, nil
			}
			return db, tenantID, nil
		default:
			return nil, 0, fmt.Errorf("care plan postgres: unsupported transaction %T", transaction)
		}
	}
}

type engine struct {
	service    *application.Service
	requests   RequestStore
	statusDays StatusDayStore
	observe    func(Observation)
	people     StudentNameFinder
	database   *bun.DB
}

func (e engine) LockStudentAndExceptionDay(ctx context.Context, studentID int64, date string) error {
	return careplanning.LockStudentAndExceptionDay(ctx, e.database, studentID, date)
}

func (e engine) LockExceptionDay(ctx context.Context, studentID int64, date string) error {
	return careplanning.LockExceptionDay(ctx, e.database, studentID, date)
}

func (e engine) FindCareOffering(ctx context.Context, id int64) (careplan.CareOffering, error) {
	value, err := e.service.FindCareOffering(ctx, id)
	return offeringToPublic(value), mapError(err)
}

func (e engine) ListCareOfferings(ctx context.Context, filter careplan.CareOfferingFilter) ([]careplan.CareOffering, error) {
	values, err := e.service.ListCareOfferings(ctx, domain.CareOfferingFilter{
		IDs: filter.IDs, PhaseIDs: filter.PhaseIDs, ActivityGroupIDs: filter.ActivityGroupIDs,
		ActiveOnly: filter.ActiveOnly, LockForUpdate: filter.LockForUpdate, Order: filter.Order,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]careplan.CareOffering, 0, len(values))
	for _, value := range values {
		result = append(result, offeringToPublic(value))
	}
	return result, nil
}

func (e engine) CountCareOfferingsByPhase(ctx context.Context, phaseID int64) (int, error) {
	count, err := e.service.CountCareOfferingsByPhase(ctx, phaseID)
	return count, mapError(err)
}

func (e engine) CreateCareOffering(ctx context.Context, input careplan.CreateCareOffering) (result careplan.CareOffering, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		value, commandErr := e.service.CreateCareOffering(txCtx, offeringFieldsToDomain(input.CareOfferingFields))
		result = offeringToPublic(value)
		return commandErr
	})
	return result, mapError(err)
}

func (e engine) UpdateCareOffering(ctx context.Context, input careplan.UpdateCareOffering) (result careplan.CareOffering, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		value, commandErr := e.service.UpdateCareOffering(txCtx, input.ID, offeringFieldsToDomain(input.CareOfferingFields))
		result = offeringToPublic(value)
		return commandErr
	})
	return result, mapError(err)
}

func (e engine) DeleteCareOffering(ctx context.Context, id int64) error {
	return mapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.DeleteCareOffering(txCtx, id)
	}))
}

func (e engine) ReplaceAutoAddTriggers(ctx context.Context, id int64, triggers []int64) error {
	return mapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.ReplaceAutoAddTriggers(txCtx, id, triggers)
	}))
}

func (e engine) FindOfferingChange(ctx context.Context, id int64, lock bool) (careplan.OfferingChangeRequest, error) {
	value, err := e.service.FindOfferingChange(ctx, id, lock)
	return changeToPublic(value), mapError(err)
}

func (e engine) ListOfferingChanges(ctx context.Context, filter careplan.OfferingChangeFilter) ([]careplan.OfferingChangeRequest, error) {
	values, err := e.service.ListOfferingChanges(ctx, domain.OfferingChangeFilter{
		IDs: filter.IDs, StudentID: filter.StudentID, StudentIDs: filter.StudentIDs, Statuses: filter.Statuses,
		UrgentOnly: filter.UrgentOnly, UrgentDate: filter.UrgentDate, BeforeInstant: filter.BeforeInstant,
		BeforeID: filter.BeforeID, Limit: filter.Limit, LockForUpdate: filter.LockForUpdate, Order: filter.Order,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]careplan.OfferingChangeRequest, 0, len(values))
	for _, value := range values {
		result = append(result, changeToPublic(value))
	}
	return result, nil
}

func (e engine) CreateOfferingChange(ctx context.Context, input careplan.OfferingChangeRequest) (result careplan.OfferingChangeRequest, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		value, commandErr := e.service.CreateOfferingChange(txCtx, changeToDomain(input))
		result = changeToPublic(value)
		return commandErr
	})
	return result, mapError(err)
}

func (e engine) UpdateOfferingChangeEffectiveFrom(ctx context.Context, id int64, date string) error {
	return mapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.UpdateOfferingChangeEffectiveFrom(txCtx, id, date)
	}))
}

func (e engine) UpdateApprovedCompleteWithdrawal(ctx context.Context, id int64, complete bool) error {
	return mapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.UpdateApprovedCompleteWithdrawal(txCtx, id, complete)
	}))
}

func (e engine) UpdatePendingOfferingChange(ctx context.Context, input careplan.UpdatePendingOfferingChange) error {
	return mapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.UpdatePendingOfferingChange(txCtx, domain.UpdatePendingOfferingChange{
			ID: input.ID, Payload: input.Payload, EffectiveFrom: input.EffectiveFrom, ParentNote: input.ParentNote,
		})
	}))
}

func (e engine) DecideOfferingChange(ctx context.Context, input careplan.DecideOfferingChange) error {
	return mapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.DecideOfferingChange(txCtx, domain.DecideOfferingChange{
			ID: input.ID, Status: input.Status, Reason: input.Reason, ReviewedBy: input.ReviewedBy, Applied: input.Applied,
		})
	}))
}

func (e engine) UpdateOfferingChangeSnapshot(ctx context.Context, id int64, snapshot json.RawMessage) error {
	return mapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.UpdateOfferingChangeSnapshot(txCtx, id, snapshot)
	}))
}

func (e engine) ClosePendingOfferingChanges(ctx context.Context, studentIDs []int64, reason string, reviewedBy *int64, at time.Time) (rows int64, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		var commandErr error
		rows, commandErr = e.service.ClosePendingOfferingChanges(txCtx, studentIDs, reason, reviewedBy, at)
		return commandErr
	})
	return rows, mapError(err)
}

func (e engine) withinTenant(ctx context.Context, command func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return command(ctx)
	}
	tenantID := tenant.FromContext(ctx)
	validatedTenantID, err := tenant.NewTenantID(tenantID)
	if err != nil {
		return errors.New("care plan: tenant is required for commands")
	}
	return tenant.WithinTenant(ctx, validatedTenantID, command)
}

func offeringFieldsToDomain(fields careplan.CareOfferingFields) domain.CareOfferingFields {
	return domain.CareOfferingFields{
		PhaseID: fields.PhaseID, ActivityGroupID: fields.ActivityGroupID, Name: fields.Name, Description: fields.Description,
		DaysOfWeekMode: fields.DaysOfWeekMode, AvailableDays: fields.AvailableDays,
		IncludesHolidayCare: fields.IncludesHolidayCare, IncludesLunch: fields.IncludesLunch,
		Capacity: fields.Capacity, PriceCents: fields.PriceCents, IsActive: fields.IsActive, IsRequired: fields.IsRequired,
		CountsAsCare: fields.CountsAsCare, AutoAddGradeLevels: fields.AutoAddGradeLevels, AvailabilityRule: fields.AvailabilityRule,
		SortOrder: fields.SortOrder, SelectionGroup: fields.SelectionGroup, SelectionRule: fields.SelectionRule,
		PickupTimes: fields.PickupTimes, AutoAddTriggerOfferingIDs: fields.AutoAddTriggerOfferingIDs,
	}
}

func offeringToPublic(value domain.CareOffering) careplan.CareOffering {
	return careplan.CareOffering{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		PhaseID: value.PhaseID, ActivityGroupID: value.ActivityGroupID, Name: value.Name, Description: value.Description,
		DaysOfWeekMode: value.DaysOfWeekMode, AvailableDays: value.AvailableDays,
		IncludesHolidayCare: value.IncludesHolidayCare, IncludesLunch: value.IncludesLunch,
		Capacity: value.Capacity, PriceCents: value.PriceCents, IsActive: value.IsActive, IsRequired: value.IsRequired,
		CountsAsCare: value.CountsAsCare, AutoAddGradeLevels: value.AutoAddGradeLevels, AvailabilityRule: value.AvailabilityRule,
		SortOrder: value.SortOrder, SelectionGroup: value.SelectionGroup, SelectionRule: value.SelectionRule,
		PickupTimes: value.PickupTimes, AutoAddTriggerOfferingIDs: value.AutoAddTriggerOfferingIDs,
	}
}

func changeToDomain(value careplan.OfferingChangeRequest) domain.OfferingChangeRequest {
	return domain.OfferingChangeRequest{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		StudentID: value.StudentID, RequestChildID: value.RequestChildID, SubmittedBy: value.SubmittedBy,
		CompleteWithdrawalConfirmed: value.CompleteWithdrawalConfirmed,
		WithdrawalConfirmedBy:       value.WithdrawalConfirmedBy, WithdrawalConfirmedAt: value.WithdrawalConfirmedAt,
		ApprovedCompleteWithdrawal: value.ApprovedCompleteWithdrawal, Payload: value.Payload,
		EffectiveFrom: value.EffectiveFrom, ParentNote: value.ParentNote, Status: value.Status,
		DecisionReason: value.DecisionReason, DecisionSnapshot: value.DecisionSnapshot,
		ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt,
	}
}

func changeToPublic(value domain.OfferingChangeRequest) careplan.OfferingChangeRequest {
	return careplan.OfferingChangeRequest{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		StudentID: value.StudentID, RequestChildID: value.RequestChildID, SubmittedBy: value.SubmittedBy,
		CompleteWithdrawalConfirmed: value.CompleteWithdrawalConfirmed,
		WithdrawalConfirmedBy:       value.WithdrawalConfirmedBy, WithdrawalConfirmedAt: value.WithdrawalConfirmedAt,
		ApprovedCompleteWithdrawal: value.ApprovedCompleteWithdrawal, Payload: value.Payload,
		EffectiveFrom: value.EffectiveFrom, ParentNote: value.ParentNote, Status: value.Status,
		DecisionReason: value.DecisionReason, DecisionSnapshot: value.DecisionSnapshot,
		ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt,
	}
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrCareOfferingNotFound):
		return careplan.ErrCareOfferingNotFound
	case errors.Is(err, domain.ErrOfferingChangeNotFound):
		return careplan.ErrOfferingChangeNotFound
	case errors.Is(err, domain.ErrOfferingChangeNotPending):
		return careplan.ErrOfferingChangeNotPending
	case errors.Is(err, domain.ErrOfferingChangeAlreadyOpen):
		return fmt.Errorf("%w: %w", careplan.ErrOfferingChangeAlreadyOpen, err)
	case errors.Is(err, domain.ErrCareOfferingTriggerInvalid):
		return fmt.Errorf("%w: %w", careplan.ErrCareOfferingTriggerInvalid, err)
	case errors.Is(err, domain.ErrCareDocumentNotFound):
		return careplan.ErrCareDocumentNotFound
	default:
		return err
	}
}
