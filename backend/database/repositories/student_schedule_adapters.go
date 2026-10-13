package repositories

import (
	"errors"
	"fmt"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// StudentScheduleRepositories contains the Care Plan compatibility adapters
// still consumed by legacy schedule services.
type StudentScheduleRepositories struct {
	ArrivalSchedule  timetableCompose.CareArrivalScheduleRows
	ArrivalException timetableCompose.CareArrivalExceptionRows
	ArrivalNote      timetableCompose.CareArrivalNoteRows
	PickupSchedule   timetableCompose.CarePickupScheduleRows
	PickupException  timetableCompose.CarePickupExceptionRows
	PickupNote       timetableCompose.CarePickupNoteRows
	StatusDay        *StudentStatusDayRepository
}

// CareScheduleRepositories are the retained schedule repositories served by
// the Care Plan owner.
type CareScheduleRepositories = timetableCompose.CareScheduleRepositories

// NewCareScheduleRepositories builds the retained schedule repositories over
// the Care Plan capability.
func NewCareScheduleRepositories(capability careplan.Capability) CareScheduleRepositories {
	return timetableCompose.NewCareScheduleRepositories(NewCarePlanRows(capability))
}

func NewArrivalScheduleRepository(capability careplan.Capability) timetableCompose.CareArrivalScheduleRows {
	return timetableCompose.NewCareArrivalScheduleRows(NewCarePlanRows(capability))
}
func NewArrivalExceptionRepository(capability careplan.Capability) timetableCompose.CareArrivalExceptionRows {
	return timetableCompose.NewCareArrivalExceptionRows(NewCarePlanRows(capability))
}
func NewArrivalNoteRepository(capability careplan.Capability) timetableCompose.CareArrivalNoteRows {
	return timetableCompose.NewCareArrivalNoteRows(NewCarePlanRows(capability))
}
func NewPickupScheduleRepository(capability careplan.Capability) timetableCompose.CarePickupScheduleRows {
	return timetableCompose.NewCarePickupScheduleRows(NewCarePlanRows(capability))
}
func NewPickupExceptionRepository(capability careplan.Capability) timetableCompose.CarePickupExceptionRows {
	return timetableCompose.NewCarePickupExceptionRows(NewCarePlanRows(capability))
}
func NewPickupNoteRepository(capability careplan.Capability) timetableCompose.CarePickupNoteRows {
	return timetableCompose.NewCarePickupNoteRows(NewCarePlanRows(capability))
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
	return wrapRetainedError(op, err)
}

// noRowsError is the bare not-found sentinel pair without an operation.
func noRowsError() error { return errors.Unwrap(retainedNotFound("")) }
