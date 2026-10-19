package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// Engine serves the Workforce capability over the application service: it
// maps the public vocabulary onto the domain and the domain errors onto the
// contract (#4084).
type Engine struct {
	service *Service
	workforce.StaffPlanningProfilesQuery
}

// NewEngine serves the capability over service.
func NewEngine(service *Service) Engine {
	return Engine{service: service, StaffPlanningProfilesQuery: service}
}

func (e Engine) ListWorkTimeModels(ctx context.Context) ([]workforce.WorkTimeModel, error) {
	values, err := e.service.ListWorkTimeModels(ctx)
	return modelsToPublic(values), MapError(err)
}

func (e Engine) FindWorkTimeModel(ctx context.Context, id int64) (workforce.WorkTimeModel, error) {
	value, err := e.service.FindWorkTimeModel(ctx, id)
	return modelToPublic(value), MapError(err)
}

func (e Engine) ListWorkTimeModelsByIDs(ctx context.Context, ids []int64) ([]workforce.WorkTimeModel, error) {
	values, err := e.service.ListWorkTimeModelsByIDs(ctx, ids)
	return modelsToPublic(values), MapError(err)
}

func (e Engine) CreateWorkTimeModel(ctx context.Context, input workforce.CreateWorkTimeModel) (workforce.WorkTimeModel, error) {
	value, err := e.service.CreateWorkTimeModel(ctx, modelFieldsToDomain(input.WorkTimeModelFields))
	return modelToPublic(value), MapError(err)
}

func (e Engine) UpdateWorkTimeModel(ctx context.Context, input workforce.UpdateWorkTimeModel) (workforce.WorkTimeModel, error) {
	value, err := e.service.UpdateWorkTimeModel(ctx, input.ID, modelFieldsToDomain(input.WorkTimeModelFields))
	return modelToPublic(value), MapError(err)
}

func (e Engine) DeleteWorkTimeModel(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteWorkTimeModel(ctx, id))
}

func (e Engine) ReplaceStaffSchedule(ctx context.Context, input workforce.ReplaceStaffSchedule) error {
	entries := make([]domain.StaffWorkScheduleFields, 0, len(input.Entries))
	for _, entry := range input.Entries {
		entries = append(entries, domain.StaffWorkScheduleFields{
			WeekIndex: entry.WeekIndex, RotationLength: entry.RotationLength,
			DayOfWeek: entry.DayOfWeek, TargetMinutes: entry.TargetMinutes, StartTime: entry.StartTime,
		})
	}
	return MapError(e.service.ReplaceStaffSchedule(ctx, input.StaffID, entries, input.RotationAnchorDate, input.ValidFrom))
}

func (e Engine) CurrentStaffSchedule(ctx context.Context, staffID int64) ([]workforce.StaffWorkSchedule, error) {
	values, err := e.service.CurrentStaffSchedule(ctx, staffID)
	return schedulesToPublic(values), MapError(err)
}

func (e Engine) StaffScheduleOn(ctx context.Context, staffID int64, date string) ([]workforce.StaffWorkSchedule, error) {
	values, err := e.service.StaffScheduleOn(ctx, staffID, date)
	return schedulesToPublic(values), MapError(err)
}

func (e Engine) StaffSchedulesInRange(ctx context.Context, staffIDs []int64, from, to string) ([]workforce.StaffWorkSchedule, error) {
	values, err := e.service.StaffSchedulesInRange(ctx, staffIDs, from, to)
	return schedulesToPublic(values), MapError(err)
}

func (e Engine) HasStaffScheduleHistory(ctx context.Context, staffID int64) (bool, error) {
	value, err := e.service.HasStaffScheduleHistory(ctx, staffID)
	return value, MapError(err)
}

func (e Engine) StaffIDsWithScheduleHistory(ctx context.Context, staffIDs []int64) (map[int64]bool, error) {
	value, err := e.service.StaffIDsWithScheduleHistory(ctx, staffIDs)
	return value, MapError(err)
}

func modelFieldsToDomain(fields workforce.WorkTimeModelFields) domain.WorkTimeModelFields {
	entries := make([]domain.WorkTimeModelEntryFields, 0, len(fields.Entries))
	for _, entry := range fields.Entries {
		entries = append(entries, domain.WorkTimeModelEntryFields{
			WeekIndex: entry.WeekIndex, DayOfWeek: entry.DayOfWeek,
			TargetMinutes: entry.TargetMinutes, StartTime: entry.StartTime,
		})
	}
	return domain.WorkTimeModelFields{
		Name: fields.Name, RotationLength: fields.RotationLength,
		RotationAnchorDate: fields.RotationAnchorDate, Entries: entries,
	}
}

func modelsToPublic(values []domain.WorkTimeModel) []workforce.WorkTimeModel {
	if values == nil {
		return nil
	}
	result := make([]workforce.WorkTimeModel, 0, len(values))
	for _, value := range values {
		result = append(result, modelToPublic(value))
	}
	return result
}

func modelToPublic(value domain.WorkTimeModel) workforce.WorkTimeModel {
	model := workforce.WorkTimeModel{
		ID: value.ID, TenantID: value.TenantID, Name: value.Name,
		RotationLength: value.RotationLength, RotationAnchorDate: value.RotationAnchorDate,
	}
	if value.Entries == nil {
		return model
	}
	model.Entries = make([]workforce.WorkTimeModelEntry, 0, len(value.Entries))
	for _, entry := range value.Entries {
		model.Entries = append(model.Entries, workforce.WorkTimeModelEntry{
			WeekIndex: entry.WeekIndex, DayOfWeek: entry.DayOfWeek,
			TargetMinutes: entry.TargetMinutes, StartTime: entry.StartTime,
		})
	}
	return model
}

func schedulesToPublic(values []domain.StaffWorkSchedule) []workforce.StaffWorkSchedule {
	if values == nil {
		return nil
	}
	result := make([]workforce.StaffWorkSchedule, 0, len(values))
	for _, value := range values {
		result = append(result, workforce.StaffWorkSchedule{
			ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID,
			WeekIndex: value.WeekIndex, RotationLength: value.RotationLength,
			DayOfWeek: value.DayOfWeek, TargetMinutes: value.TargetMinutes,
			StartTime: value.StartTime, RotationAnchorDate: value.RotationAnchorDate,
			ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil,
		})
	}
	return result
}

// MapError maps the owner errors onto the Workforce contract.
func MapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrOffboardingConflict):
		return workforce.ErrOffboardingConflict
	case errors.Is(err, domain.ErrOffboardingInUse):
		return workforce.ErrOffboardingInUse
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrWorkTimeModelNotFound):
		return workforce.ErrWorkTimeModelNotFound
	case errors.Is(err, domain.ErrWorkTimeModelAssigned):
		return workforce.ErrWorkTimeModelAssigned
	case errors.Is(err, domain.ErrInvalidWorkTime):
		return &workforce.InvalidWorkTimeError{Reason: err.Error()}
	case errors.Is(err, domain.ErrStaffAbsenceNotFound):
		return workforce.ErrStaffAbsenceNotFound
	case errors.Is(err, domain.ErrAbsenceTypeNotFound):
		return workforce.ErrAbsenceTypeNotFound
	case errors.Is(err, domain.ErrAbsenceTypeNameReserved):
		return workforce.ErrAbsenceTypeNameReserved
	case errors.Is(err, domain.ErrAbsenceTypeInUse):
		return workforce.ErrAbsenceTypeInUse
	case errors.Is(err, domain.ErrAbsenceTypeAllowanceExceeded):
		return workforce.ErrAbsenceTypeAllowanceExceeded
	case errors.Is(err, domain.ErrAbsenceTypeAllowanceInvalid):
		if validation, ok := errors.AsType[*domain.InvalidAbsenceAllowanceError](err); ok {
			return &workforce.InvalidAbsenceAllowanceError{Reason: validation.Reason}
		}
		return workforce.ErrAbsenceTypeAllowanceInvalid
	case errors.Is(err, domain.ErrGroupSubstitutionNotFound):
		return workforce.ErrGroupSubstitutionNotFound
	case errors.Is(err, domain.ErrInvalidStaffAbsence):
		return &workforce.InvalidStaffAbsenceError{Reason: err.Error()}
	case errors.Is(err, domain.ErrInvalidGroupSubstitution):
		return &workforce.InvalidGroupSubstitutionError{Reason: err.Error()}
	case errors.Is(err, domain.ErrAbsenceTypeInvalid):
		return &workforce.InvalidAbsenceTypeError{Reason: err.Error()}
	case errors.Is(err, domain.ErrAbsenceTypeNameTaken):
		return &workforce.ConflictError{Kind: workforce.ErrAbsenceTypeNameTaken, Cause: conflictCause(err)}
	case errors.Is(err, domain.ErrGroupSubstitutionExists):
		return &workforce.ConflictError{Kind: workforce.ErrGroupSubstitutionExists, Cause: conflictCause(err)}
	case errors.Is(err, domain.ErrStaffTargetOverrideNotFound), errors.Is(err, domain.ErrInvalidStaffTargetOverride),
		errors.Is(err, domain.ErrStaffTargetOverrideRejected):
		return targetOverrideError(err)
	default:
		return err
	}
}

// conflictCause keeps the driver error of a duplicate reachable through the
// public error, so a caller inspecting the violated constraint still can.
func conflictCause(err error) error {
	if conflict, ok := errors.AsType[*domain.ConflictError](err); ok {
		return conflict.Cause
	}
	return err
}
