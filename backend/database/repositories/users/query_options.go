package users

import (
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Filter operators of users.QueryFilter, by value (models/base.Operator).
const (
	opEqual              = "="
	opGreaterThan        = ">"
	opGreaterThanOrEqual = ">="
	opLessThan           = "<"
	opLessThanOrEqual    = "<="
	opLike               = "LIKE"
	opILike              = "ILIKE"
	opTrimEqual          = "TRIM_EQUALS"
	opTrimIn             = "TRIM_IN"
	opFirstNumberIn      = "FIRST_NUMBER_IN"
	opIsNull             = "IS NULL"
	opIsNotNull          = "IS NOT NULL"
	opIn                 = "IN"
	opNotIn              = "NOT IN"
)

// applyQueryOptions translates the persistence-neutral query specification
// to Bun, as database/repositories/base.ApplyQueryOptions does.
func applyQueryOptions(query *bun.SelectQuery, options *users.QueryOptions) *bun.SelectQuery {
	if options == nil {
		return query
	}
	if options.Filter != nil {
		query = applyFilter(query, options.Filter)
	}
	if options.Sorting != nil {
		for _, field := range options.Sorting.Fields {
			if field.Direction == users.SortDesc {
				query = query.OrderExpr("? DESC", bun.Ident(field.Field))
			} else {
				query = query.OrderExpr("? ASC", bun.Ident(field.Field))
			}
		}
	}
	if options.Pagination != nil {
		query = query.Limit(options.Pagination.PageSize).Offset(options.Pagination.Offset())
	}
	return query
}

func applyFilter(query *bun.SelectQuery, filter *users.QueryFilter) *bun.SelectQuery {
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

func applyConditions(query *bun.SelectQuery, filter *users.QueryFilter) *bun.SelectQuery {
	for _, condition := range filter.Conditions() {
		field := condition.Field
		if alias := filter.TableAlias(); alias != "" {
			field = alias + "." + field
		}
		query = applyCondition(query, bun.Ident(field), string(condition.Operator), condition.Value)
	}
	return query
}

func applyCondition(query *bun.SelectQuery, identifier bun.Ident, operator string, value any) *bun.SelectQuery {
	switch operator {
	case opEqual:
		return query.Where("? = ?", identifier, value)
	case opGreaterThan:
		return query.Where("? > ?", identifier, value)
	case opGreaterThanOrEqual:
		return query.Where("? >= ?", identifier, value)
	case opLessThan:
		return query.Where("? < ?", identifier, value)
	case opLessThanOrEqual:
		return query.Where("? <= ?", identifier, value)
	case opLike:
		return query.Where("? LIKE ?", identifier, value)
	case opILike:
		return query.Where("? ILIKE ?", identifier, value)
	case opTrimEqual:
		return query.Where("LOWER(TRIM(?)) = LOWER(TRIM(?))", identifier, value)
	case opTrimIn:
		if values, ok := value.([]any); ok && len(values) > 0 {
			return query.WhereGroup(" AND ", func(group *bun.SelectQuery) *bun.SelectQuery {
				for _, value := range values {
					group = group.WhereOr("LOWER(TRIM(?)) = LOWER(TRIM(?))", identifier, value)
				}
				return group
			})
		}
	case opFirstNumberIn:
		if values, ok := value.([]any); ok && len(values) > 0 {
			return query.Where("substring(? from '[0-9]+') IN (?)", identifier, bun.List(values))
		}
	case opIsNull:
		return query.Where("? IS NULL", identifier)
	case opIsNotNull:
		return query.Where("? IS NOT NULL", identifier)
	case opIn:
		if values, ok := value.([]any); ok {
			return query.Where("? IN (?)", identifier, bun.List(values))
		}
	case opNotIn:
		if values, ok := value.([]any); ok {
			return query.Where("? NOT IN (?)", identifier, bun.List(values))
		}
	}
	return query
}

func applyLogical(query *bun.SelectQuery, filters []users.QueryFilter, operator string) *bun.SelectQuery {
	for i := range filters {
		filter := &filters[i]
		query = query.WhereGroup(operator, func(group *bun.SelectQuery) *bun.SelectQuery {
			return applyFilter(group, filter)
		})
	}
	return query
}
