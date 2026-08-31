package base

import (
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// ApplyQueryOptions translates the persistence-neutral query specification to Bun.
func ApplyQueryOptions(query *bun.SelectQuery, options *modelBase.QueryOptions) *bun.SelectQuery {
	if options == nil {
		return query
	}
	if options.Filter != nil {
		query = ApplyFilter(query, options.Filter)
	}
	if options.Sorting != nil {
		query = ApplySorting(query, *options.Sorting)
	}
	if options.Pagination != nil {
		query = ApplyPagination(query, *options.Pagination)
	}
	return query
}

func ApplyFilter(query *bun.SelectQuery, filter *modelBase.Filter) *bun.SelectQuery {
	if filter == nil {
		return query
	}
	if len(filter.OrFilters()) > 0 {
		query = query.WhereGroup(" AND ", func(group *bun.SelectQuery) *bun.SelectQuery {
			group = applyConditions(group, filter)
			return applyLogical(group, filter.OrFilters(), " OR ")
		})
	} else {
		query = applyConditions(query, filter)
	}
	return applyLogical(query, filter.AndFilters(), " AND ")
}

func applyConditions(query *bun.SelectQuery, filter *modelBase.Filter) *bun.SelectQuery {
	for _, condition := range filter.Conditions() {
		query = applyCondition(query, filter.TableAlias(), condition)
	}
	return query
}

func applyCondition(query *bun.SelectQuery, alias string, condition modelBase.FilterCondition) *bun.SelectQuery {
	field := condition.Field
	if alias != "" {
		field = alias + "." + field
	}
	identifier := bun.Ident(field)
	switch condition.Operator {
	case modelBase.OpEqual:
		return query.Where("? = ?", identifier, condition.Value)
	case modelBase.OpGreaterThan:
		return query.Where("? > ?", identifier, condition.Value)
	case modelBase.OpGreaterThanOrEqual:
		return query.Where("? >= ?", identifier, condition.Value)
	case modelBase.OpLessThan:
		return query.Where("? < ?", identifier, condition.Value)
	case modelBase.OpLessThanOrEqual:
		return query.Where("? <= ?", identifier, condition.Value)
	case modelBase.OpLike:
		return query.Where("? LIKE ?", identifier, condition.Value)
	case modelBase.OpILike:
		return query.Where("? ILIKE ?", identifier, condition.Value)
	case modelBase.OpTrimEqual:
		return query.Where("LOWER(TRIM(?)) = LOWER(TRIM(?))", identifier, condition.Value)
	case modelBase.OpTrimIn:
		if values, ok := condition.Value.([]interface{}); ok && len(values) > 0 {
			return query.WhereGroup(" AND ", func(group *bun.SelectQuery) *bun.SelectQuery {
				for _, value := range values {
					group = group.WhereOr("LOWER(TRIM(?)) = LOWER(TRIM(?))", identifier, value)
				}
				return group
			})
		}
	case modelBase.OpFirstNumberIn:
		if values, ok := condition.Value.([]interface{}); ok && len(values) > 0 {
			return query.Where("substring(? from '[0-9]+') IN (?)", identifier, bun.List(values))
		}
	case modelBase.OpIsNull:
		return query.Where("? IS NULL", identifier)
	case modelBase.OpIsNotNull:
		return query.Where("? IS NOT NULL", identifier)
	case modelBase.OpIn:
		if values, ok := condition.Value.([]interface{}); ok {
			return query.Where("? IN (?)", identifier, bun.List(values))
		}
	case modelBase.OpNotIn:
		if values, ok := condition.Value.([]interface{}); ok {
			return query.Where("? NOT IN (?)", identifier, bun.List(values))
		}
	}
	return query
}

func applyLogical(query *bun.SelectQuery, filters []modelBase.Filter, operator string) *bun.SelectQuery {
	for i := range filters {
		filter := &filters[i]
		query = query.WhereGroup(operator, func(group *bun.SelectQuery) *bun.SelectQuery {
			return ApplyFilter(group, filter)
		})
	}
	return query
}

func ApplyPagination(query *bun.SelectQuery, pagination modelBase.Pagination) *bun.SelectQuery {
	return query.Limit(pagination.PageSize).Offset(pagination.Offset())
}

func ApplySorting(query *bun.SelectQuery, sorting modelBase.Sorting) *bun.SelectQuery {
	for _, field := range sorting.Fields {
		if field.Direction == modelBase.SortDesc {
			query = query.OrderExpr("? DESC", bun.Ident(field.Field))
		} else {
			query = query.OrderExpr("? ASC", bun.Ident(field.Field))
		}
	}
	return query
}
