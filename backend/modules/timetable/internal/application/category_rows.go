package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
)

// categoryCreator is the category write of the owner the legacy rows use.
type categoryCreator interface {
	CreateCategory(context.Context, timetable.CreateCategory) (timetable.Category, error)
}

// createLegacyCategory creates a category through the owner's category
// command with the legacy row checks, and copies the stored row back.
func createLegacyCategory(ctx context.Context, categories categoryCreator, value *activityContract.Category) error {
	if value == nil {
		return errors.New("category cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	created, err := categories.CreateCategory(ctx, timetable.CreateCategory{
		Name: value.Name, Description: value.Description, Color: value.Color, IsSystem: value.IsSystem,
	})
	if err != nil {
		return legacyCategoryError("create", err)
	}
	replaceLegacyCategory(value, created)
	return nil
}

func replaceLegacyCategory(result *activityContract.Category, value timetable.Category) {
	*result = *legacyCategory(value)
}

func legacyCategoryError(operation string, err error) error {
	if errors.Is(err, timetable.ErrCategoryNotFound) || errors.Is(err, timetable.ErrInvalidCategory) {
		return legacyNotFoundError(operation)
	}
	return legacyDatabaseError(operation, err)
}
