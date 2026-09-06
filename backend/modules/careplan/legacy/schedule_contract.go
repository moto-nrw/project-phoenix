package legacy

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

type ScheduleDate = timezone.Date
type ScheduleQueryOptions = modelBase.QueryOptions
type RequestQueueFilters = modelBase.RequestQueueFilters

func CarePlanScheduleQueryOptions(options *ScheduleQueryOptions) *careplan.StudentScheduleQueryOptions {
	if options == nil {
		return nil
	}
	result := &careplan.StudentScheduleQueryOptions{}
	if options.Filter != nil {
		result.Filter = carePlanScheduleQueryFilter(*options.Filter)
	}
	if options.Pagination != nil {
		result.Limit = options.Pagination.PageSize
		result.Offset = options.Pagination.Offset()
	}
	if options.Sorting != nil {
		result.Sorting = make([]careplan.StudentScheduleSortField, 0, len(options.Sorting.Fields))
		for _, field := range options.Sorting.Fields {
			result.Sorting = append(result.Sorting, careplan.StudentScheduleSortField{Field: field.Field, Descending: field.Direction == modelBase.SortDesc})
		}
	}
	return result
}

func carePlanScheduleQueryFilter(filter modelBase.Filter) *careplan.StudentScheduleQueryFilter {
	result := &careplan.StudentScheduleQueryFilter{}
	for _, condition := range filter.Conditions() {
		result.Conditions = append(result.Conditions, careplan.StudentScheduleQueryCondition{Field: condition.Field, Operator: string(condition.Operator), Value: condition.Value})
	}
	for _, child := range filter.OrFilters() {
		result.Or = append(result.Or, *carePlanScheduleQueryFilter(child))
	}
	for _, child := range filter.AndFilters() {
		result.And = append(result.And, *carePlanScheduleQueryFilter(child))
	}
	return result
}

func ScheduleID(raw any) (int64, error) {
	switch value := raw.(type) {
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case int32:
		return int64(value), nil
	default:
		return 0, fmt.Errorf("unsupported id type %T", raw)
	}
}

func ScheduleError(op string, err error) error {
	if errors.Is(err, careplan.ErrStudentScheduleNotFound) {
		return &modelBase.DatabaseError{Op: op, Err: errors.Join(modelBase.ErrNotFound, sql.ErrNoRows)}
	}
	if err != nil {
		return &modelBase.DatabaseError{Op: op, Err: err}
	}
	return nil
}

func NotFoundError(op string) error {
	return &modelBase.DatabaseError{Op: op, Err: errors.Join(modelBase.ErrNotFound, sql.ErrNoRows)}
}

func NoRowsError() error { return sql.ErrNoRows }

func TodayScheduleDate() careplan.Date { return careplan.Date(timezone.TodayDate().String()) }
