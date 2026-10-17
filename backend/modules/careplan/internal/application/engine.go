package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	careplanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

// EngineRuntime is what the engine needs from the tenant runtime and the
// database: commands join the caller's tenant transaction or open one, and
// the student and exception-day locks serialize the schedule writes. The
// composition binds it.
type EngineRuntime interface {
	EngineLocks
	WithinTenant(ctx context.Context, command func(context.Context) error) error
	LockStudent(ctx context.Context, studentID int64) error
}

// EngineLocks are the student and exception-day locks the capability serves
// as they are.
type EngineLocks interface {
	LockStudentAndExceptionDay(ctx context.Context, studentID int64, date string) error
	LockExceptionDay(ctx context.Context, studentID int64, date string) error
}

// EngineDependencies are the stores and seams the engine serves the
// capability over.
type EngineDependencies struct {
	OfferingBookings *careplan.OfferingBookings
	Exceptions       *ExceptionQueryService
	Requests         RequestStore
	StatusDays       StatusDayStore
	Withdrawals      ports.WithdrawalList
	Observe          func(ports.Observation)
	People           careplanPorts.StudentNameFinder
	Runtime          EngineRuntime
}

// NewEngine serves the Care Plan capability over dependencies (#4084).
func NewEngine(dependencies EngineDependencies) Engine {
	return Engine{
		OfferingBookings: dependencies.OfferingBookings, ExceptionQueryService: dependencies.Exceptions, EngineLocks: dependencies.Runtime,
		requests: dependencies.Requests, statusDays: dependencies.StatusDays, withdrawals: dependencies.Withdrawals,
		observe: dependencies.Observe, people: dependencies.People, runtime: dependencies.Runtime,
	}
}

// Engine serves the Care Plan capability: it maps the public vocabulary onto
// the stores and the domain errors onto the contract.
type Engine struct {
	*careplan.OfferingBookings
	*ExceptionQueryService
	EngineLocks
	requests    RequestStore
	statusDays  StatusDayStore
	withdrawals ports.WithdrawalList
	observe     func(ports.Observation)
	people      careplanPorts.StudentNameFinder
	runtime     EngineRuntime
}

func (e Engine) FindCareOffering(ctx context.Context, id int64) (careplan.CareOffering, error) {
	value, err := e.service.FindCareOffering(ctx, id)
	return offeringToPublic(value), MapError(err)
}

func (e Engine) ListCareOfferings(ctx context.Context, filter careplan.CareOfferingFilter) ([]careplan.CareOffering, error) {
	values, err := e.service.ListCareOfferings(ctx, domain.CareOfferingFilter{
		IDs: filter.IDs, PhaseIDs: filter.PhaseIDs, ActivityGroupIDs: filter.ActivityGroupIDs,
		ActiveOnly: filter.ActiveOnly, LockForUpdate: filter.LockForUpdate, Order: filter.Order,
	})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]careplan.CareOffering, 0, len(values))
	for _, value := range values {
		result = append(result, offeringToPublic(value))
	}
	return result, nil
}

func (e Engine) CountCareOfferingsByPhase(ctx context.Context, phaseID int64) (int, error) {
	count, err := e.service.CountCareOfferingsByPhase(ctx, phaseID)
	return count, MapError(err)
}

func (e Engine) CreateCareOffering(ctx context.Context, input careplan.CreateCareOffering) (result careplan.CareOffering, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		value, commandErr := e.service.CreateCareOffering(txCtx, offeringFieldsToDomain(input.CareOfferingFields))
		result = offeringToPublic(value)
		return commandErr
	})
	return result, MapError(err)
}

func (e Engine) UpdateCareOffering(ctx context.Context, input careplan.UpdateCareOffering) (result careplan.CareOffering, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		value, commandErr := e.service.UpdateCareOffering(txCtx, input.ID, offeringFieldsToDomain(input.CareOfferingFields))
		result = offeringToPublic(value)
		return commandErr
	})
	return result, MapError(err)
}

func (e Engine) DeleteCareOffering(ctx context.Context, id int64) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.DeleteCareOffering(txCtx, id)
	}))
}

func (e Engine) ReplaceAutoAddTriggers(ctx context.Context, id int64, triggers []int64) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.ReplaceAutoAddTriggers(txCtx, id, triggers)
	}))
}

func (e Engine) FindOfferingChange(ctx context.Context, id int64, lock bool) (careplan.OfferingChangeRequest, error) {
	value, err := e.service.FindOfferingChange(ctx, id, lock)
	return changeToPublic(value), MapError(err)
}

func (e Engine) ListOfferingChanges(ctx context.Context, filter careplan.OfferingChangeFilter) ([]careplan.OfferingChangeRequest, error) {
	return (offeringChangeRequestQueries{service: e.service}).ListOfferingChanges(ctx, filter)
}

func (e Engine) CreateOfferingChange(ctx context.Context, input careplan.OfferingChangeRequest) (result careplan.OfferingChangeRequest, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		value, commandErr := e.service.CreateOfferingChange(txCtx, changeToDomain(input))
		result = changeToPublic(value)
		return commandErr
	})
	return result, MapError(err)
}

func (e Engine) UpdateOfferingChangeEffectiveFrom(ctx context.Context, id int64, date string) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.UpdateOfferingChangeEffectiveFrom(txCtx, id, date)
	}))
}

func (e Engine) UpdateApprovedCompleteWithdrawal(ctx context.Context, id int64, complete bool) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.UpdateApprovedCompleteWithdrawal(txCtx, id, complete)
	}))
}

func (e Engine) UpdatePendingOfferingChange(ctx context.Context, input careplan.UpdatePendingOfferingChange) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.UpdatePendingOfferingChange(txCtx, domain.UpdatePendingOfferingChange{
			ID: input.ID, Payload: input.Payload, EffectiveFrom: input.EffectiveFrom, ParentNote: input.ParentNote,
		})
	}))
}

func (e Engine) DecideOfferingChange(ctx context.Context, input careplan.DecideOfferingChange) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.DecideOfferingChange(txCtx, domain.DecideOfferingChange{
			ID: input.ID, Status: input.Status, Reason: input.Reason, ReviewedBy: input.ReviewedBy, Applied: input.Applied,
		})
	}))
}

func (e Engine) UpdateOfferingChangeSnapshot(ctx context.Context, id int64, snapshot json.RawMessage) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.UpdateOfferingChangeSnapshot(txCtx, id, snapshot)
	}))
}

func (e Engine) ClosePendingOfferingChanges(ctx context.Context, studentIDs []int64, reason string, reviewedBy *int64, at time.Time) (rows int64, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		var commandErr error
		rows, commandErr = e.service.ClosePendingOfferingChanges(txCtx, studentIDs, reason, reviewedBy, at)
		return commandErr
	})
	return rows, MapError(err)
}

func (e Engine) withinTenant(ctx context.Context, command func(context.Context) error) error {
	return e.runtime.WithinTenant(ctx, command)
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
		Translations: fields.Translations,
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
		Translations: value.Translations,
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

// MapError maps the owner errors onto the Care Plan contract.
func MapError(err error) error {
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
	case errors.Is(err, domain.ErrWithdrawalNotFound):
		return careplan.ErrWithdrawalNotFound
	case errors.Is(err, domain.ErrWithdrawalNotPending):
		return careplan.ErrWithdrawalNotPending
	default:
		return err
	}
}
