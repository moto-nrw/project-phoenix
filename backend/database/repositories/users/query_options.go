package users

import (
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// applyQueryOptions translates the persistence-neutral query specification
// to Bun, as database/repositories/base.ApplyQueryOptions does.
func applyQueryOptions(query *bun.SelectQuery, options *ports.QueryOptions) *bun.SelectQuery {
	if options == nil {
		return query
	}
	if options.Filter != nil {
		query = applyFilter(query, options.Filter)
	}
	if options.Sorting != nil {
		for _, field := range options.Sorting.Fields {
			if field.Direction == ports.SortDesc {
				query = query.OrderExpr("? DESC", bun.Ident(field.Field))
			} else {
				query = query.OrderExpr("? ASC", bun.Ident(field.Field))
			}
		}
	}
	if options.Pagination != nil {
		query = query.Limit(int64(options.Pagination.PageSize)).Offset(int64(options.Pagination.Offset()))
	}
	return query
}

func applyFilter(query *bun.SelectQuery, filter *ports.QueryFilter) *bun.SelectQuery {
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

func applyConditions(query *bun.SelectQuery, filter *ports.QueryFilter) *bun.SelectQuery {
	for _, condition := range filter.Conditions() {
		field := condition.Field
		if alias := filter.TableAlias(); alias != "" {
			field = alias + "." + field
		}
		query = applyCondition(query, bun.Ident(field), condition.Operator, condition.Value)
	}
	return query
}

func applyCondition(query *bun.SelectQuery, identifier bun.Ident, operator ports.FilterOperator, value any) *bun.SelectQuery {
	switch operator {
	case ports.FilterEqual:
		return query.Where("? = ?", identifier, value)
	case ports.FilterGreaterThan:
		return query.Where("? > ?", identifier, value)
	case ports.FilterGreaterThanOrEqual:
		return query.Where("? >= ?", identifier, value)
	case ports.FilterLessThan:
		return query.Where("? < ?", identifier, value)
	case ports.FilterLessThanOrEqual:
		return query.Where("? <= ?", identifier, value)
	case ports.FilterLike:
		return query.Where("? LIKE ?", identifier, value)
	case ports.FilterILike:
		return query.Where("? ILIKE ?", identifier, value)
	case ports.FilterTrimEqual:
		return query.Where("LOWER(TRIM(?)) = LOWER(TRIM(?))", identifier, value)
	case ports.FilterTrimIn:
		if values, ok := value.([]any); ok && len(values) > 0 {
			return query.WhereGroup(" AND ", func(group *bun.SelectQuery) *bun.SelectQuery {
				for _, value := range values {
					group = group.WhereOr("LOWER(TRIM(?)) = LOWER(TRIM(?))", identifier, value)
				}
				return group
			})
		}
	case ports.FilterFirstNumberIn:
		if values, ok := value.([]any); ok && len(values) > 0 {
			return query.Where("substring(? from '[0-9]+') IN (?)", identifier, bun.List(values))
		}
	case ports.FilterIsNull:
		return query.Where("? IS NULL", identifier)
	case ports.FilterIsNotNull:
		return query.Where("? IS NOT NULL", identifier)
	case ports.FilterIn:
		if values, ok := value.([]any); ok {
			return query.Where("? IN (?)", identifier, bun.List(values))
		}
	case ports.FilterNotIn:
		if values, ok := value.([]any); ok {
			return query.Where("? NOT IN (?)", identifier, bun.List(values))
		}
	}
	return query
}

func applyLogical(query *bun.SelectQuery, filters []ports.QueryFilter, operator string) *bun.SelectQuery {
	for i := range filters {
		filter := &filters[i]
		query = query.WhereGroup(operator, func(group *bun.SelectQuery) *bun.SelectQuery {
			return applyFilter(group, filter)
		})
	}
	return query
}
