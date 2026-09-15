package shiftplanning

import (
	"context"
	"errors"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// CategoryLinker syncs the optional Kategorie↔Schichtart mapping. The FK
// lives on activities.categories, so the write belongs to the Timetable
// owner; a nil linker leaves every mapping untouched.
type CategoryLinker func(ctx context.Context, shiftTypeID int64, categoryIDs []int64) error

type shiftTypeAdministration struct {
	types  scheduleSvc.ShiftTypeService
	linker CategoryLinker
}

// NewShiftTypeAdministration binds the /api/shift-types capability to the
// retained shift type service and the timetable category linker.
func NewShiftTypeAdministration(types scheduleSvc.ShiftTypeService, linker CategoryLinker) workforce.ShiftTypeAdministration {
	if types == nil {
		panic("shift type administration: shift type service is required")
	}
	return shiftTypeAdministration{types: types, linker: linker}
}

func (a shiftTypeAdministration) ListShiftTypes(ctx context.Context) ([]workforce.ShiftType, error) {
	types, err := a.types.ListShiftTypes(ctx)
	if err != nil {
		return nil, mapShiftTypeError(err)
	}
	return shiftTypesToCapability(types), nil
}

func (a shiftTypeAdministration) CreateShiftType(ctx context.Context, input workforce.ShiftTypeInput) (workforce.ShiftType, error) {
	shiftType := shiftTypeInputToModel(input)
	if input.IsActive == nil {
		shiftType.IsActive = true
	}
	saved, err := a.types.CreateShiftType(ctx, shiftType)
	if err != nil {
		return workforce.ShiftType{}, mapShiftTypeError(err)
	}
	if err := a.syncCategoryLinks(ctx, saved.ID, input.CategoryIDs); err != nil {
		return workforce.ShiftType{}, err
	}
	return shiftTypeToCapability(saved), nil
}

func (a shiftTypeAdministration) CreateDefaultShiftTypes(ctx context.Context) ([]workforce.ShiftType, error) {
	types, err := a.types.CreateDefaultShiftTypes(ctx)
	if err != nil {
		return nil, mapShiftTypeError(err)
	}
	return shiftTypesToCapability(types), nil
}

// UpdateShiftType preserves the stored active flag when the client omitted
// it, so a rename cannot silently reactivate a retired type.
func (a shiftTypeAdministration) UpdateShiftType(ctx context.Context, input workforce.ShiftTypeInput) (workforce.ShiftType, error) {
	shiftType := shiftTypeInputToModel(input)
	if input.IsActive == nil {
		existing, err := a.types.GetShiftType(ctx, input.ID)
		if err != nil {
			return workforce.ShiftType{}, mapShiftTypeError(err)
		}
		shiftType.IsActive = existing.IsActive
	}
	saved, err := a.types.UpdateShiftType(ctx, shiftType)
	if err != nil {
		return workforce.ShiftType{}, mapShiftTypeError(err)
	}
	if err := a.syncCategoryLinks(ctx, saved.ID, input.CategoryIDs); err != nil {
		return workforce.ShiftType{}, err
	}
	return shiftTypeToCapability(saved), nil
}

func (a shiftTypeAdministration) DeleteShiftType(ctx context.Context, id int64) error {
	return mapShiftTypeError(a.types.DeleteShiftType(ctx, id))
}

// syncCategoryLinks applies the optional mapping after a write. A nil set
// (field omitted) leaves the mapping untouched; a present set, including an
// empty one, replaces it. It runs in the same tenant transaction as the
// shift-type write, so the caller must roll the request back when unknown
// category ids reject it.
func (a shiftTypeAdministration) syncCategoryLinks(ctx context.Context, shiftTypeID int64, categoryIDs []int64) error {
	if categoryIDs == nil || a.linker == nil {
		return nil
	}
	if err := a.linker(ctx, shiftTypeID, categoryIDs); err != nil {
		if errors.Is(err, activitiesModels.ErrUnknownCategoryIDs) {
			return &capabilityError{kind: workforce.ErrShiftTypeCategoryUnknown, cause: err}
		}
		return err
	}
	return nil
}

func shiftTypeInputToModel(input workforce.ShiftTypeInput) *scheduleModels.ShiftType {
	shiftType := &scheduleModels.ShiftType{Name: input.Name, Color: input.Color, Description: input.Description}
	shiftType.ID = input.ID
	if input.IsActive != nil {
		shiftType.IsActive = *input.IsActive
	}
	return shiftType
}

func shiftTypeToCapability(shiftType *scheduleModels.ShiftType) workforce.ShiftType {
	if shiftType == nil {
		return workforce.ShiftType{}
	}
	return workforce.ShiftType{
		ID: shiftType.ID, TenantID: shiftType.TenantID, Name: shiftType.Name, Color: shiftType.Color,
		Description: shiftType.Description, IsActive: shiftType.IsActive, CreatedAt: shiftType.CreatedAt, UpdatedAt: shiftType.UpdatedAt,
	}
}

func shiftTypesToCapability(types []*scheduleModels.ShiftType) []workforce.ShiftType {
	result := make([]workforce.ShiftType, 0, len(types))
	for _, shiftType := range types {
		if shiftType == nil {
			continue
		}
		result = append(result, shiftTypeToCapability(shiftType))
	}
	return result
}

var shiftTypeErrorKinds = []struct {
	service    error
	capability error
}{
	{scheduleSvc.ErrShiftTypeNameTaken, workforce.ErrShiftTypeNameTaken},
	{scheduleSvc.ErrShiftTypeNotFound, workforce.ErrShiftTypeNotFound},
	{scheduleSvc.ErrShiftTypeInvalid, workforce.ErrInvalidShiftType},
}

func mapShiftTypeError(err error) error {
	if err == nil {
		return nil
	}
	for _, kind := range shiftTypeErrorKinds {
		if errors.Is(err, kind.service) {
			return &capabilityError{kind: kind.capability, cause: err}
		}
	}
	return err
}
