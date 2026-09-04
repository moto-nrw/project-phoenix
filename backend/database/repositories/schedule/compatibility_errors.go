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
type ActivityInstanceQueryOptions = modelBase.QueryOptions
type ActivityExceptionDate = timezone.Date
type ActivityInstanceDate = timezone.Date

func ParseActivityExceptionDate(value string) (ActivityExceptionDate, error) {
	return timezone.ParseDate(value)
}

func ParseActivityInstanceDate(value string) (ActivityInstanceDate, error) {
	return timezone.ParseDate(value)
}

type ActivityInstanceListFilter struct {
	IDs              []int64
	Date             *string
	Dates            []string
	ActivityGroupIDs []int64
	ActiveGroupIDs   []int64
	Status           string
	IsSpontaneous    *bool
	IdempotencyKey   string
	Limit            int
	Offset           int
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

func ActivityInstanceListOptions(options *ActivityInstanceQueryOptions) (ActivityInstanceListFilter, error) {
	result := ActivityInstanceListFilter{}
	if options == nil {
		return result, nil
	}
	if options.Pagination != nil {
		result.Limit, result.Offset = options.Pagination.PageSize, options.Pagination.Offset()
	}
	if options.Sorting != nil && len(options.Sorting.Fields) > 0 {
		return ActivityInstanceListFilter{}, errors.New("activity instance sorting is unsupported")
	}
	if options.Filter == nil {
		return result, nil
	}
	if len(options.Filter.OrFilters()) > 0 || len(options.Filter.AndFilters()) > 0 {
		return ActivityInstanceListFilter{}, errors.New("compound activity instance filters are unsupported")
	}
	for _, condition := range options.Filter.Conditions() {
		if err := applyActivityInstanceCondition(&result, condition); err != nil {
			return ActivityInstanceListFilter{}, err
		}
	}
	return result, nil
}

func applyActivityInstanceCondition(result *ActivityInstanceListFilter, condition modelBase.FilterCondition) error {
	switch condition.Operator {
	case modelBase.OpEqual:
		return applyActivityInstanceEqual(result, condition.Field, condition.Value)
	case modelBase.OpIn:
		return applyActivityInstanceIn(result, condition.Field, condition.Value)
	default:
		return errors.New("activity instance filter is unsupported")
	}
}

func applyActivityInstanceEqual(result *ActivityInstanceListFilter, field string, value any) error {
	switch field {
	case "id":
		return assignActivityInstanceID(&result.IDs, value)
	case "activity_group_id":
		return assignActivityInstanceID(&result.ActivityGroupIDs, value)
	case "active_group_id":
		return assignActivityInstanceID(&result.ActiveGroupIDs, value)
	case "date":
		date, ok := value.(timezone.Date)
		if ok {
			text := date.String()
			result.Date = &text
			return nil
		}
	case "status":
		status, ok := value.(string)
		if ok {
			result.Status = status
			return nil
		}
	case "is_spontaneous":
		spontaneous, ok := value.(bool)
		if ok {
			result.IsSpontaneous = &spontaneous
			return nil
		}
	case "idempotency_key":
		key, ok := value.(string)
		if ok {
			result.IdempotencyKey = key
			return nil
		}
	}
	return errors.New("activity instance filter is unsupported")
}

func assignActivityInstanceID(target *[]int64, value any) error {
	id, ok := value.(int64)
	if !ok {
		return errors.New("activity instance id filter is unsupported")
	}
	*target = append(*target, id)
	return nil
}

func applyActivityInstanceIn(result *ActivityInstanceListFilter, field string, value any) error {
	values, ok := value.([]interface{})
	if !ok {
		return errors.New("activity instance IN filter is unsupported")
	}
	switch field {
	case "id":
		return appendActivityInstanceIDs(&result.IDs, values)
	case "activity_group_id":
		return appendActivityInstanceIDs(&result.ActivityGroupIDs, values)
	case "active_group_id":
		return appendActivityInstanceIDs(&result.ActiveGroupIDs, values)
	case "date":
		for _, value := range values {
			date, dateOK := value.(timezone.Date)
			if !dateOK {
				return errors.New("activity instance date filter is unsupported")
			}
			result.Dates = append(result.Dates, date.String())
		}
		return nil
	default:
		return errors.New("activity instance IN filter is unsupported")
	}
}

func appendActivityInstanceIDs(target *[]int64, values []interface{}) error {
	for _, value := range values {
		if err := assignActivityInstanceID(target, value); err != nil {
			return err
		}
	}
	return nil
}

func ActivityInstanceBefore(options *ActivityInstanceQueryOptions) (*string, error) {
	if options == nil || options.Filter == nil || len(options.Filter.Conditions()) == 0 {
		return nil, nil
	}
	if len(options.Filter.OrFilters()) > 0 || len(options.Filter.AndFilters()) > 0 ||
		len(options.Filter.Conditions()) != 1 {
		return nil, errors.New("activity instance count filter is unsupported")
	}
	condition := options.Filter.Conditions()[0]
	value, ok := condition.Value.(timezone.Date)
	if condition.Field != "date" || condition.Operator != modelBase.OpLessThan || !ok {
		return nil, errors.New("activity instance count filter is unsupported")
	}
	date := value.String()
	return &date, nil
}
