package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type CategoryCountRecords interface {
	ListCategories(context.Context) ([]timetable.Category, error)
}

// CategoryCount supplies the category total from the tenant's category directory.
type CategoryCount struct {
	records CategoryCountRecords
}

func NewCategoryCount(records CategoryCountRecords) *CategoryCount {
	return &CategoryCount{records: records}
}

func (r *CategoryCount) CountCategories(ctx context.Context) (int, error) {
	rows, err := r.records.ListCategories(ctx)
	if err != nil {
		return 0, legacyDatabaseError("list all", err)
	}
	return len(rows), nil
}
