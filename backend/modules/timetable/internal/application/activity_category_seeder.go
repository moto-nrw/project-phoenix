package application

import (
	"context"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
)

// ActivityCategorySeeder creates the default activity categories the
// operator provisioning seeds into a new school.
type ActivityCategorySeeder struct {
	categories activitiesModels.CategoryRepository
}

// NewActivityCategorySeeder binds the seeder to the retained category rows.
func NewActivityCategorySeeder(categories activitiesModels.CategoryRepository) ActivityCategorySeeder {
	return ActivityCategorySeeder{categories: categories}
}

// CreateCategory creates one category at the school in ctx. A category the
// school already has fails with the repository's unique violation.
func (s ActivityCategorySeeder) CreateCategory(ctx context.Context, name, description, color string) error {
	return s.categories.Create(ctx, &activityContract.Category{Name: name, Description: description, Color: color})
}
