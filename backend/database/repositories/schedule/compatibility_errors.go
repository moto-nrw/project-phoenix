package schedule

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

// RecurrenceRuleQueryOptions keeps the legacy repository method signature
// without making the composition adapter import the base model package.
type RecurrenceRuleQueryOptions = modelBase.QueryOptions
type ActivityExceptionQueryOptions = modelBase.QueryOptions
type ActivityExceptionDate = timezone.Date

func ParseActivityExceptionDate(value string) (ActivityExceptionDate, error) {
	return timezone.ParseDate(value)
}

func WrapDatabaseError(operation string, err error) error {
	return &modelBase.DatabaseError{Op: operation, Err: err}
}

func WrapNotFoundDatabaseError(operation string) error {
	return WrapDatabaseError(operation, modelBase.ErrNotFound)
}

func TimeframeListOptions(options *scheduleModels.TimeframeQueryOptions) (string, int, int, error) {
	if options == nil {
		return "", 0, 0, nil
	}
	limit, offset := 0, 0
	if options.Pagination != nil {
		limit, offset = options.Pagination.PageSize, options.Pagination.Offset()
	}
	if options.Sorting != nil && len(options.Sorting.Fields) > 0 {
		return "", 0, 0, errors.New("timeframe sorting is unsupported")
	}
	if options.Filter == nil {
		return "", limit, offset, nil
	}
	if len(options.Filter.OrFilters()) > 0 || len(options.Filter.AndFilters()) > 0 {
		return "", 0, 0, errors.New("compound timeframe filters are unsupported")
	}
	conditions := options.Filter.Conditions()
	if len(conditions) > 1 {
		return "", 0, 0, errors.New("multiple timeframe filters are unsupported")
	}
	description := ""
	for _, condition := range conditions {
		value, ok := condition.Value.(string)
		if condition.Field != "description" || condition.Operator != modelBase.OpILike || !ok {
			return "", 0, 0, errors.New("timeframe filter is unsupported")
		}
		description = value
	}
	return description, limit, offset, nil
}

func RecurrenceRuleListOptions(options *modelBase.QueryOptions) (string, []string, string, bool, int, int, error) {
	if options == nil {
		return "", nil, "", false, 0, 0, nil
	}
	limit, offset := 0, 0
	if options.Pagination != nil {
		limit, offset = options.Pagination.PageSize, options.Pagination.Offset()
	}
	sortBy, descending, err := recurrenceRuleSort(options.Sorting)
	if err != nil {
		return "", nil, "", false, 0, 0, err
	}
	frequency, frequencies, err := recurrenceRuleFilter(options.Filter)
	if err != nil {
		return "", nil, "", false, 0, 0, err
	}
	return frequency, frequencies, sortBy, descending, limit, offset, nil
}

func recurrenceRuleSort(sorting *modelBase.Sorting) (string, bool, error) {
	if sorting == nil || len(sorting.Fields) == 0 {
		return "", false, nil
	}
	if len(sorting.Fields) > 1 {
		return "", false, errors.New("multiple recurrence rule sort fields are unsupported")
	}
	return sorting.Fields[0].Field, sorting.Fields[0].Direction == modelBase.SortDesc, nil
}

func recurrenceRuleFilter(filter *modelBase.Filter) (string, []string, error) {
	if filter == nil {
		return "", nil, nil
	}
	if len(filter.OrFilters()) > 0 || len(filter.AndFilters()) > 0 {
		return "", nil, errors.New("compound recurrence rule filters are unsupported")
	}
	conditions := filter.Conditions()
	if len(conditions) > 1 {
		return "", nil, errors.New("multiple recurrence rule filters are unsupported")
	}
	if len(conditions) == 0 {
		return "", nil, nil
	}
	return recurrenceRuleFrequencyCondition(conditions[0])
}

func recurrenceRuleFrequencyCondition(condition modelBase.FilterCondition) (string, []string, error) {
	if condition.Field != "frequency" {
		return "", nil, errors.New("recurrence rule filter is unsupported")
	}
	if condition.Operator == modelBase.OpEqual {
		value, ok := condition.Value.(string)
		if ok {
			return value, nil, nil
		}
	}
	if condition.Operator == modelBase.OpIn {
		values, ok := condition.Value.([]interface{})
		if ok {
			result := make([]string, 0, len(values))
			for _, value := range values {
				text, stringOK := value.(string)
				if !stringOK {
					return "", nil, errors.New("recurrence rule filter is unsupported")
				}
				result = append(result, text)
			}
			return "", result, nil
		}
	}
	return "", nil, errors.New("recurrence rule filter is unsupported")
}

func ActivityExceptionListOptions(options *ActivityExceptionQueryOptions) (*int64, int, int, error) {
	if options == nil {
		return nil, 0, 0, nil
	}
	limit, offset := 0, 0
	if options.Pagination != nil {
		limit, offset = options.Pagination.PageSize, options.Pagination.Offset()
	}
	if options.Sorting != nil && len(options.Sorting.Fields) > 0 {
		return nil, 0, 0, errors.New("activity exception sorting is unsupported")
	}
	if options.Filter == nil {
		return nil, limit, offset, nil
	}
	if len(options.Filter.OrFilters()) > 0 || len(options.Filter.AndFilters()) > 0 {
		return nil, 0, 0, errors.New("compound activity exception filters are unsupported")
	}
	conditions := options.Filter.Conditions()
	if len(conditions) == 0 {
		return nil, limit, offset, nil
	}
	if len(conditions) != 1 {
		return nil, 0, 0, errors.New("multiple activity exception filters are unsupported")
	}
	condition := conditions[0]
	groupID, ok := condition.Value.(int64)
	if condition.Field != "activity_group_id" || condition.Operator != modelBase.OpEqual || !ok {
		return nil, 0, 0, errors.New("activity exception filter is unsupported")
	}
	return &groupID, limit, offset, nil
}

func ActivityExceptionBefore(options *ActivityExceptionQueryOptions) (*string, error) {
	if options == nil || options.Filter == nil || len(options.Filter.Conditions()) == 0 {
		return nil, nil
	}
	if len(options.Filter.OrFilters()) > 0 || len(options.Filter.AndFilters()) > 0 ||
		len(options.Filter.Conditions()) != 1 {
		return nil, errors.New("activity exception count filter is unsupported")
	}
	condition := options.Filter.Conditions()[0]
	value, ok := condition.Value.(timezone.Date)
	if condition.Field != "exception_date" || condition.Operator != modelBase.OpLessThan || !ok {
		return nil, errors.New("activity exception count filter is unsupported")
	}
	date := value.String()
	return &date, nil
}
