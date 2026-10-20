package application

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

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
	case errors.Is(err, workforce.ErrOffboardingConflict):
		return workforce.ErrOffboardingConflict
	case errors.Is(err, workforce.ErrOffboardingInUse):
		return workforce.ErrOffboardingInUse
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrWorkTimeModelNotFound), errors.Is(err, workforce.ErrWorkTimeModelNotFound):
		return workforce.ErrWorkTimeModelNotFound
	case errors.Is(err, domain.ErrWorkTimeModelAssigned), errors.Is(err, workforce.ErrWorkTimeModelAssigned):
		return workforce.ErrWorkTimeModelAssigned
	case errors.Is(err, domain.ErrInvalidWorkTime), errors.Is(err, workforce.ErrInvalidWorkTime):
		return &workforce.InvalidWorkTimeError{Reason: err.Error()}
	case errors.Is(err, domain.ErrStaffAbsenceNotFound), errors.Is(err, workforce.ErrStaffAbsenceNotFound):
		return workforce.ErrStaffAbsenceNotFound
	case errors.Is(err, domain.ErrAbsenceTypeNotFound), errors.Is(err, workforce.ErrAbsenceTypeNotFound):
		return workforce.ErrAbsenceTypeNotFound
	case errors.Is(err, domain.ErrAbsenceTypeNameReserved), errors.Is(err, workforce.ErrAbsenceTypeNameReserved):
		return workforce.ErrAbsenceTypeNameReserved
	case errors.Is(err, domain.ErrAbsenceTypeInUse), errors.Is(err, workforce.ErrAbsenceTypeInUse):
		return workforce.ErrAbsenceTypeInUse
	case errors.Is(err, domain.ErrAbsenceTypeAllowanceExceeded), errors.Is(err, workforce.ErrAbsenceTypeAllowanceExceeded):
		return workforce.ErrAbsenceTypeAllowanceExceeded
	case errors.Is(err, domain.ErrAbsenceTypeAllowanceInvalid), errors.Is(err, workforce.ErrAbsenceTypeAllowanceInvalid):
		if validation, ok := errors.AsType[*domain.InvalidAbsenceAllowanceError](err); ok {
			return &workforce.InvalidAbsenceAllowanceError{Reason: validation.Reason}
		}
		if validation, ok := errors.AsType[*workforce.InvalidAbsenceAllowanceError](err); ok {
			return &workforce.InvalidAbsenceAllowanceError{Reason: validation.Reason}
		}
		return workforce.ErrAbsenceTypeAllowanceInvalid
	case errors.Is(err, domain.ErrGroupSubstitutionNotFound), errors.Is(err, workforce.ErrGroupSubstitutionNotFound):
		return workforce.ErrGroupSubstitutionNotFound
	case errors.Is(err, domain.ErrInvalidStaffAbsence), errors.Is(err, workforce.ErrInvalidStaffAbsence):
		return &workforce.InvalidStaffAbsenceError{Reason: err.Error()}
	case errors.Is(err, domain.ErrInvalidGroupSubstitution), errors.Is(err, workforce.ErrInvalidGroupSubstitution):
		return &workforce.InvalidGroupSubstitutionError{Reason: err.Error()}
	case errors.Is(err, domain.ErrAbsenceTypeInvalid), errors.Is(err, workforce.ErrAbsenceTypeInvalid):
		return &workforce.InvalidAbsenceTypeError{Reason: err.Error()}
	case errors.Is(err, domain.ErrAbsenceTypeNameTaken), errors.Is(err, workforce.ErrAbsenceTypeNameTaken):
		return &workforce.ConflictError{Kind: workforce.ErrAbsenceTypeNameTaken, Cause: conflictCause(err)}
	case errors.Is(err, domain.ErrGroupSubstitutionExists), errors.Is(err, workforce.ErrGroupSubstitutionExists):
		return &workforce.ConflictError{Kind: workforce.ErrGroupSubstitutionExists, Cause: conflictCause(err)}
	case errors.Is(err, domain.ErrStaffTargetOverrideNotFound), errors.Is(err, workforce.ErrStaffTargetOverrideNotFound), errors.Is(err, domain.ErrInvalidStaffTargetOverride), errors.Is(err, workforce.ErrInvalidStaffTargetOverride),
		errors.Is(err, domain.ErrStaffTargetOverrideRejected), errors.Is(err, workforce.ErrStaffTargetOverrideRejected):
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
	if conflict, ok := errors.AsType[*workforce.ConflictError](err); ok {
		return conflict.Cause
	}
	return err
}
