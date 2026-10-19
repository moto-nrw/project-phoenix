package application

import (
	"context"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
)

type CategoryCountRecords interface {
	ListAll(context.Context) ([]*activityContract.Category, error)
}

// CategoryCount supplies the category total from the tenant's category directory.
type CategoryCount struct {
	records CategoryCountRecords
}

func NewCategoryCount(records CategoryCountRecords) *CategoryCount {
	return &CategoryCount{records: records}
}

func (r *CategoryCount) CountCategories(ctx context.Context) (int, error) {
	rows, err := r.records.ListAll(ctx)
	return len(rows), err
}
