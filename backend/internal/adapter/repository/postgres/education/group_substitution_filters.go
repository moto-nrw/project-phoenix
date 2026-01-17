package education

import (
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
)

// applySubstitutionFilter applies a single filter based on field name
func applySubstitutionFilter(filter *modelBase.Filter, field string, value interface{}) {
	switch field {
	case "active":
		applyActiveFilter(filter, value)
	case "date":
		applyDateFilter(filter, value)
	case "reason_like":
		applyReasonLikeFilter(filter, value)
	default:
		filter.Equal(field, value)
	}
}

// applyActiveFilter applies active date filter using current time
func applyActiveFilter(filter *modelBase.Filter, value interface{}) {
	if boolValue, ok := value.(bool); ok && boolValue {
		filter.DateBetween("start_date", "end_date", time.Now())
	}
}

// applyDateFilter applies date filter for a specific date
func applyDateFilter(filter *modelBase.Filter, value interface{}) {
	if dateValue, ok := value.(time.Time); ok {
		filter.DateBetween("start_date", "end_date", dateValue)
	}
}

// applyReasonLikeFilter applies LIKE filter for reason field
func applyReasonLikeFilter(filter *modelBase.Filter, value interface{}) {
	if strValue, ok := value.(string); ok {
		filter.ILike("reason", "%"+strValue+"%")
	}
}
