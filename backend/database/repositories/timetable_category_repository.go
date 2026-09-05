package repositories

import (
	"context"
	"errors"
	"fmt"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type timetableActivityCategoryRepository struct{ timetable timetable.Capability }

func (r timetableActivityCategoryRepository) Create(ctx context.Context, value *activitiesModels.Category) error {
	if value == nil {
		return errors.New("category cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	created, err := r.timetable.CreateCategory(ctx, timetable.CreateCategory{
		Name: value.Name, Description: value.Description, Color: value.Color, IsSystem: value.IsSystem,
	})
	if err != nil {
		return legacyCategoryError("create", err)
	}
	replaceLegacyCategory(value, created)
	return nil
}

func (r timetableActivityCategoryRepository) FindByID(ctx context.Context, id any) (*activitiesModels.Category, error) {
	categoryID, ok := legacyGroupID(id)
	if !ok {
		return nil, legacyDatabaseError("find by id", fmt.Errorf("invalid category id %T", id))
	}
	value, err := r.timetable.FindCategory(ctx, categoryID)
	if err != nil {
		return nil, legacyCategoryError("find by id", err)
	}
	return legacyCategory(value), nil
}

func (r timetableActivityCategoryRepository) FindByName(ctx context.Context, name string) (*activitiesModels.Category, error) {
	value, err := r.timetable.FindCategoryByName(ctx, name)
	if err != nil {
		return nil, legacyCategoryError("find by name", err)
	}
	return legacyCategory(value), nil
}

func (r timetableActivityCategoryRepository) FindByNameIncludingArchivedForShare(ctx context.Context, name string) (*activitiesModels.Category, error) {
	value, err := r.timetable.FindCategoryByNameForAssignment(ctx, name)
	if err != nil {
		return nil, legacyCategoryError("find by name", err)
	}
	return legacyCategory(value), nil
}

func (r timetableActivityCategoryRepository) FindByIDForShare(ctx context.Context, id int64) (*activitiesModels.Category, error) {
	value, err := r.timetable.FindCategoryForShare(ctx, id)
	if err != nil {
		return nil, legacyCategoryError("find category by id for share", err)
	}
	return legacyCategory(value), nil
}

func (r timetableActivityCategoryRepository) List(ctx context.Context, options *activitiesModels.QueryOptions) ([]*activitiesModels.Category, error) {
	if options != nil && (len(options.StudentIDs) > 0 || options.Limit != 0 || options.Offset != 0) {
		return nil, legacyDatabaseError("list", errors.New("category list options are unsupported"))
	}
	return r.ListAll(ctx)
}

func (r timetableActivityCategoryRepository) ListAll(ctx context.Context) ([]*activitiesModels.Category, error) {
	values, err := r.timetable.ListCategories(ctx)
	if err != nil {
		return nil, legacyDatabaseError("list all", err)
	}
	result := make([]*activitiesModels.Category, 0, len(values))
	for _, value := range values {
		result = append(result, legacyCategory(value))
	}
	return result, nil
}

func (r timetableActivityCategoryRepository) Update(ctx context.Context, value *activitiesModels.Category) error {
	if value == nil {
		return errors.New("category cannot be nil")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	updated, err := r.timetable.UpdateCategory(ctx, timetable.UpdateCategory{
		ID: value.ID, Name: value.Name, Description: value.Description, Color: value.Color,
	})
	if err != nil {
		return legacyCategoryError("update", err)
	}
	replaceLegacyCategory(value, updated)
	return nil
}

func (r timetableActivityCategoryRepository) UpdateIfActive(ctx context.Context, value *activitiesModels.Category) (bool, error) {
	err := r.Update(ctx, value)
	if errors.Is(err, timetable.ErrCategoryArchived) {
		return false, nil
	}
	return err == nil, err
}

func (r timetableActivityCategoryRepository) UpdateColumns(ctx context.Context, value *activitiesModels.Category, columns ...string) (int64, error) {
	if value == nil {
		return 0, errors.New("category cannot be nil or zero value")
	}
	if len(columns) != 1 {
		return 0, legacyDatabaseError("update columns", errors.New("category update requires one supported column"))
	}
	var err error
	switch columns[0] {
	case "archived_at":
		if value.ArchivedAt == nil {
			_, err = r.timetable.RestoreCategory(ctx, value.ID)
		} else {
			_, err = r.timetable.ArchiveCategory(ctx, value.ID)
		}
	case "shift_type_id":
		err = r.timetable.SetCategoryShiftTypeID(ctx, value.ID, value.ShiftTypeID)
	default:
		return 0, legacyDatabaseError("update columns", fmt.Errorf("unsupported category column %q", columns[0]))
	}
	if err != nil {
		return 0, legacyCategoryError("update columns", err)
	}
	return 1, nil
}

func (r timetableActivityCategoryRepository) Delete(ctx context.Context, id any) error {
	categoryID, ok := legacyGroupID(id)
	if !ok {
		return legacyDatabaseError("delete", fmt.Errorf("invalid category id %T", id))
	}
	if err := r.timetable.DeleteCategory(ctx, categoryID); err != nil {
		return legacyCategoryError("delete", err)
	}
	return nil
}

func replaceLegacyCategory(result *activitiesModels.Category, value timetable.Category) {
	*result = *legacyCategory(value)
}

func legacyCategoryError(operation string, err error) error {
	if errors.Is(err, timetable.ErrCategoryNotFound) || errors.Is(err, timetable.ErrInvalidCategory) {
		return legacyNotFoundError(operation)
	}
	return legacyDatabaseError(operation, err)
}
