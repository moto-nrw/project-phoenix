package base

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// Operator defines valid comparison operators for filters
type Operator string

const (
	// Equality operators
	OpEqual    Operator = "="
	OpNotEqual Operator = "!="

	// Comparison operators
	OpGreaterThan        Operator = ">"
	OpGreaterThanOrEqual Operator = ">="
	OpLessThan           Operator = "<"
	OpLessThanOrEqual    Operator = "<="

	// String operators
	OpLike  Operator = "LIKE"
	OpILike Operator = "ILIKE"

	// Null checking
	OpIsNull    Operator = "IS NULL"
	OpIsNotNull Operator = "IS NOT NULL"

	// Array operators
	OpIn    Operator = "IN"
	OpNotIn Operator = "NOT IN"

	// JSON operators
	OpContains    Operator = "@>"
	OpContainedBy Operator = "<@"
	OpHasKey      Operator = "?"
)

// FilterCondition represents a single filter condition
type FilterCondition struct {
	Field    string
	Operator Operator
	Value    interface{}
}

// Filter represents a collection of filter conditions with logical operators
type Filter struct {
	conditions []FilterCondition
	or         []Filter
	and        []Filter
	tableAlias string
}

// NewFilter creates a new filter
func NewFilter() *Filter {
	return &Filter{
		conditions: make([]FilterCondition, 0),
		or:         make([]Filter, 0),
		and:        make([]Filter, 0),
		tableAlias: "",
	}
}

// WithTableAlias sets the table alias for the filter
func (f *Filter) WithTableAlias(alias string) *Filter {
	f.tableAlias = alias
	return f
}

// Where adds a new condition to the filter
func (f *Filter) Where(field string, operator Operator, value interface{}) *Filter {
	f.conditions = append(f.conditions, FilterCondition{
		Field:    field,
		Operator: operator,
		Value:    value,
	})
	return f
}

// Equal adds an equality condition
func (f *Filter) Equal(field string, value interface{}) *Filter {
	return f.Where(field, OpEqual, value)
}

// NotEqual adds an inequality condition
func (f *Filter) NotEqual(field string, value interface{}) *Filter {
	return f.Where(field, OpNotEqual, value)
}

// GreaterThan adds a greater than condition
func (f *Filter) GreaterThan(field string, value interface{}) *Filter {
	return f.Where(field, OpGreaterThan, value)
}

// GreaterThanOrEqual adds a greater than or equal condition
func (f *Filter) GreaterThanOrEqual(field string, value interface{}) *Filter {
	return f.Where(field, OpGreaterThanOrEqual, value)
}

// LessThan adds a less than condition
func (f *Filter) LessThan(field string, value interface{}) *Filter {
	return f.Where(field, OpLessThan, value)
}

// LessThanOrEqual adds a less than or equal condition
func (f *Filter) LessThanOrEqual(field string, value interface{}) *Filter {
	return f.Where(field, OpLessThanOrEqual, value)
}

// Like adds a LIKE condition
func (f *Filter) Like(field string, value string) *Filter {
	return f.Where(field, OpLike, value)
}

// ILike adds a case-insensitive LIKE condition
func (f *Filter) ILike(field string, value string) *Filter {
	return f.Where(field, OpILike, value)
}

// IsNull adds an IS NULL condition
func (f *Filter) IsNull(field string) *Filter {
	return f.Where(field, OpIsNull, nil)
}

// IsNotNull adds an IS NOT NULL condition
func (f *Filter) IsNotNull(field string) *Filter {
	return f.Where(field, OpIsNotNull, nil)
}

// In adds an IN condition
func (f *Filter) In(field string, values ...interface{}) *Filter {
	return f.Where(field, OpIn, values)
}

// NotIn adds a NOT IN condition
func (f *Filter) NotIn(field string, values ...interface{}) *Filter {
	return f.Where(field, OpNotIn, values)
}

// Or adds a logical OR condition with another filter
func (f *Filter) Or(filter Filter) *Filter {
	f.or = append(f.or, filter)
	return f
}

// And adds a logical AND condition with another filter
func (f *Filter) And(filter Filter) *Filter {
	f.and = append(f.and, filter)
	return f
}

// DateRange adds a date range filter between start and end dates
func (f *Filter) DateRange(field string, start, end time.Time) *Filter {
	return f.GreaterThanOrEqual(field, start).LessThanOrEqual(field, end)
}

// DateBetween adds a date between filter for a date contained within a range
func (f *Filter) DateBetween(startField, endField string, date time.Time) *Filter {
	f.LessThanOrEqual(startField, date)
	f.GreaterThanOrEqual(endField, date)
	return f
}

// ToMap converts a filter to a simple map for repository use
func (f *Filter) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	// Apply basic conditions
	for _, condition := range f.conditions {
		// Currently, we only add Equality conditions to the map
		// as this is mostly what the repositories expect
		if condition.Operator == OpEqual {
			result[condition.Field] = condition.Value
		}
		// For LIKE/ILIKE, we could add them too but repository would need
		// to know how to handle them
	}

	// Note: OR and AND conditions are not supported in the simple map format
	// Complex filtering should use the ApplyToQuery method directly

	return result
}

// ApplyToQuery applies the filter to a Bun query
func (f *Filter) ApplyToQuery(query *bun.SelectQuery) *bun.SelectQuery {
	// Apply basic conditions
	for _, condition := range f.conditions {
		// When we have a table alias, we need to use quoted column references
		// like "alias"."column" instead of bun.Ident
		if f.tableAlias != "" {
			columnRef := fmt.Sprintf(`"%s"."%s"`, f.tableAlias, condition.Field)
			switch condition.Operator {
			case OpEqual:
				query = query.Where(columnRef+" = ?", condition.Value)
		case OpNotEqual:
			query = query.Where(columnRef+" != ?", condition.Value)
		case OpGreaterThan:
			query = query.Where(columnRef+" > ?", condition.Value)
		case OpGreaterThanOrEqual:
			query = query.Where(columnRef+" >= ?", condition.Value)
		case OpLessThan:
			query = query.Where(columnRef+" < ?", condition.Value)
		case OpLessThanOrEqual:
			query = query.Where(columnRef+" <= ?", condition.Value)
		case OpLike:
			query = query.Where(columnRef+" LIKE ?", condition.Value)
		case OpILike:
			query = query.Where(columnRef+" ILIKE ?", condition.Value)
		case OpIsNull:
			query = query.Where(columnRef+" IS NULL")
		case OpIsNotNull:
			query = query.Where(columnRef+" IS NOT NULL")
		case OpIn:
			if values, ok := condition.Value.([]interface{}); ok {
				query = query.Where(columnRef+" IN (?)", bun.In(values))
			}
		case OpNotIn:
			if values, ok := condition.Value.([]interface{}); ok {
				query = query.Where(columnRef+" NOT IN (?)", bun.In(values))
			}
		case OpContains:
			query = query.Where(columnRef+" @> ?", condition.Value)
		case OpContainedBy:
			query = query.Where(columnRef+" <@ ?", condition.Value)
		case OpHasKey:
			query = query.Where(columnRef+" ? ?", condition.Value)
		}
	} else {
		// Original behavior for queries without table alias
		fieldIdent := bun.Ident(condition.Field)
		switch condition.Operator {
		case OpEqual:
			query = query.Where("? = ?", fieldIdent, condition.Value)
		case OpNotEqual:
			query = query.Where("? != ?", fieldIdent, condition.Value)
		case OpGreaterThan:
			query = query.Where("? > ?", fieldIdent, condition.Value)
		case OpGreaterThanOrEqual:
			query = query.Where("? >= ?", fieldIdent, condition.Value)
		case OpLessThan:
			query = query.Where("? < ?", fieldIdent, condition.Value)
		case OpLessThanOrEqual:
			query = query.Where("? <= ?", fieldIdent, condition.Value)
		case OpLike:
			query = query.Where("? LIKE ?", fieldIdent, condition.Value)
		case OpILike:
			query = query.Where("? ILIKE ?", fieldIdent, condition.Value)
		case OpIsNull:
			query = query.Where("? IS NULL", fieldIdent)
		case OpIsNotNull:
			query = query.Where("? IS NOT NULL", fieldIdent)
		case OpIn:
			if values, ok := condition.Value.([]interface{}); ok {
				query = query.Where("? IN (?)", fieldIdent, bun.In(values))
			}
		case OpNotIn:
			if values, ok := condition.Value.([]interface{}); ok {
				query = query.Where("? NOT IN (?)", fieldIdent, bun.In(values))
			}
		case OpContains:
			query = query.Where("? @> ?", fieldIdent, condition.Value)
		case OpContainedBy:
			query = query.Where("? <@ ?", fieldIdent, condition.Value)
		case OpHasKey:
			query = query.Where("? ? ?", fieldIdent, condition.Value)
		}
	}
	}

	// Apply OR conditions
	for _, orFilter := range f.or {
		query = query.WhereGroup(" OR ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return orFilter.ApplyToQuery(q)
		})
	}

	// Apply AND conditions
	for _, andFilter := range f.and {
		query = query.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return andFilter.ApplyToQuery(q)
		})
	}

	return query
}

// Pagination defines a structure for pagination parameters
type Pagination struct {
	Page     int
	PageSize int
}

// NewPagination creates a new pagination with default values
func NewPagination(page, pageSize int) Pagination {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return Pagination{
		Page:     page,
		PageSize: pageSize,
	}
}

// ApplyToQuery applies pagination to a query
func (p Pagination) ApplyToQuery(query *bun.SelectQuery) *bun.SelectQuery {
	offset := (p.Page - 1) * p.PageSize
	return query.Limit(p.PageSize).Offset(offset)
}

// SortDirection defines the direction for sorting
type SortDirection string

const (
	SortAsc  SortDirection = "ASC"
	SortDesc SortDirection = "DESC"
)

// SortField defines a field to sort by and its direction
type SortField struct {
	Field     string
	Direction SortDirection
}

// Sorting defines a structure for sorting parameters
type Sorting struct {
	Fields []SortField
}

// NewSorting creates a new sorting configuration
func NewSorting(fields ...SortField) Sorting {
	return Sorting{
		Fields: fields,
	}
}

// AddField adds a sort field
func (s *Sorting) AddField(field string, direction SortDirection) *Sorting {
	s.Fields = append(s.Fields, SortField{
		Field:     field,
		Direction: direction,
	})
	return s
}

// ApplyToQuery applies sorting to a query
func (s Sorting) ApplyToQuery(query *bun.SelectQuery) *bun.SelectQuery {
	for _, field := range s.Fields {
		if field.Direction == SortDesc {
			query = query.OrderExpr("? DESC", bun.Ident(field.Field))
		} else {
			query = query.OrderExpr("? ASC", bun.Ident(field.Field))
		}
	}
	return query
}

// QueryOptions combines filtering, pagination, and sorting
type QueryOptions struct {
	Filter     *Filter
	Pagination *Pagination
	Sorting    *Sorting
}

// NewQueryOptions creates a new QueryOptions instance
func NewQueryOptions() *QueryOptions {
	return &QueryOptions{
		Filter: NewFilter(),
	}
}

// WithPagination adds pagination to query options
func (qo *QueryOptions) WithPagination(page, pageSize int) *QueryOptions {
	pagination := NewPagination(page, pageSize)
	qo.Pagination = &pagination
	return qo
}

// WithSorting adds sorting to query options
func (qo *QueryOptions) WithSorting(sorting Sorting) *QueryOptions {
	qo.Sorting = &sorting
	return qo
}

// ApplyToQuery applies all options to a query
func (qo *QueryOptions) ApplyToQuery(query *bun.SelectQuery) *bun.SelectQuery {
	if qo.Filter != nil {
		query = qo.Filter.ApplyToQuery(query)
	}

	if qo.Sorting != nil {
		query = qo.Sorting.ApplyToQuery(query)
	}

	if qo.Pagination != nil {
		query = qo.Pagination.ApplyToQuery(query)
	}

	return query
}

// CountFromQuery executes a count query based on a select query
func CountFromQuery(ctx context.Context, db bun.IDB, query *bun.SelectQuery) (int, error) {
	count, err := query.Count(ctx)
	if err != nil {
		return 0, &DatabaseError{
			Op:  "count",
			Err: fmt.Errorf("error counting records: %w", err),
		}
	}
	return count, nil
}
