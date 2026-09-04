package schedule

import (
	"errors"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

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
