package services

import (
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

func NewArrivalScheduleRepository(capability careplan.Capability) timetableCompose.CareArrivalScheduleRows {
	return timetableCompose.NewCareArrivalScheduleRows(NewCarePlanRows(capability))
}
func NewArrivalExceptionRepository(capability careplan.Capability) timetableCompose.CareArrivalExceptionRows {
	return timetableCompose.NewCareArrivalExceptionRows(NewCarePlanRows(capability))
}

func NewPickupScheduleRepository(capability careplan.Capability) timetableCompose.CarePickupScheduleRows {
	return timetableCompose.NewCarePickupScheduleRows(NewCarePlanRows(capability))
}
func NewPickupExceptionRepository(capability careplan.Capability) timetableCompose.CarePickupExceptionRows {
	return timetableCompose.NewCarePickupExceptionRows(NewCarePlanRows(capability))
}

func legacyScheduleID(raw any) (int64, error) {
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

// legacyScheduleQueryOptions translates the retained repository query shape
// into the owner's schedule query options.
func legacyScheduleQueryOptions(options *userModels.QueryOptions) *careplan.StudentScheduleQueryOptions {
	if options == nil {
		return nil
	}
	result := &careplan.StudentScheduleQueryOptions{}
	if options.Filter != nil {
		result.Filter = legacyScheduleQueryFilter(*options.Filter)
	}
	if options.Pagination != nil {
		result.Limit = options.Pagination.PageSize
		result.Offset = options.Pagination.Offset()
	}
	if options.Sorting != nil {
		result.Sorting = make([]careplan.StudentScheduleSortField, 0, len(options.Sorting.Fields))
		for _, field := range options.Sorting.Fields {
			result.Sorting = append(result.Sorting, careplan.StudentScheduleSortField{Field: field.Field, Descending: field.Direction == userModels.SortDesc})
		}
	}
	return result
}

func legacyScheduleQueryFilter(filter userModels.QueryFilter) *careplan.StudentScheduleQueryFilter {
	result := &careplan.StudentScheduleQueryFilter{}
	for _, condition := range filter.Conditions() {
		result.Conditions = append(result.Conditions, careplan.StudentScheduleQueryCondition{Field: condition.Field, Operator: string(condition.Operator), Value: condition.Value})
	}
	for _, child := range filter.OrFilters() {
		result.Or = append(result.Or, *legacyScheduleQueryFilter(child))
	}
	for _, child := range filter.AndFilters() {
		result.And = append(result.And, *legacyScheduleQueryFilter(child))
	}
	return result
}

// legacyScheduleError keeps the retained repository error shape: a missing
// schedule row classifies as not found, everything else is wrapped with its
// operation.
func legacyScheduleError(op string, err error) error {
	if errors.Is(err, careplan.ErrStudentScheduleNotFound) {
		return retainedNotFound(op)
	}
	return WrapRetainedError(op, err)
}

// noRowsError is the bare not-found sentinel pair without an operation.
func noRowsError() error { return errors.Unwrap(retainedNotFound("")) }
