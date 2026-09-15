package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/activities"
)

type CategoryCountRecords interface {
	List(context.Context, *activities.QueryOptions) ([]*activities.Category, error)
}

// CategoryCount supplies the category total from the tenant's category directory.
type CategoryCount struct {
	records CategoryCountRecords
}

func NewCategoryCount(records CategoryCountRecords) *CategoryCount {
	return &CategoryCount{records: records}
}

func (r *CategoryCount) CountCategories(ctx context.Context) (int, error) {
	rows, err := r.records.List(ctx, nil)
	return len(rows), err
}
