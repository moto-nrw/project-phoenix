package base

import (
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/uptrace/bun"
)

// Operator defines valid comparison operators for filters
type Operator string

const (
	// Equality operators
	OpEqual Operator = "="

	// Comparison operators
	OpGreaterThan        Operator = ">"
	OpGreaterThanOrEqual Operator = ">="
	OpLessThan           Operator = "<"
	OpLessThanOrEqual    Operator = "<="

	// String operators
	OpLike      Operator = "LIKE"
	OpILike     Operator = "ILIKE"
	OpTrimEqual Operator = "TRIM_EQUALS"
	OpTrimIn    Operator = "TRIM_IN"

	// Null checking
	OpIsNull    Operator = "IS NULL"
	OpIsNotNull Operator = "IS NOT NULL"

	// Array operators
	OpIn    Operator = "IN"
	OpNotIn Operator = "NOT IN"
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
	for i := range f.or {
		if f.or[i].tableAlias == "" {
			f.or[i].WithTableAlias(alias)
		}
	}
	for i := range f.and {
		if f.and[i].tableAlias == "" {
			f.and[i].WithTableAlias(alias)
		}
	}
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
func (f *Filter) Like(field, value string) *Filter {
	return f.Where(field, OpLike, value)
}

// ILike adds a case-insensitive LIKE condition
func (f *Filter) ILike(field, value string) *Filter {
	return f.Where(field, OpILike, value)
}

// TrimEqual adds a case-insensitive equality condition after trimming both sides.
func (f *Filter) TrimEqual(field, value string) *Filter {
	return f.Where(field, OpTrimEqual, value)
}

// TrimIn is the multi-value form of TrimEqual: it matches when the column
// equals ANY of the values, comparing both sides trimmed and lower-cased.
// Used by filters a school may pick several values for (e.g. Klasse 3a AND
// 4b in the Kindersuche, #2218). Passing no values adds no condition at all —
// an empty IN () would match nothing and silently return an empty list where
// callers mean "no restriction".
func (f *Filter) TrimIn(field string, values ...string) *Filter {
	if len(values) == 0 {
		return f
	}
	if len(values) == 1 {
		return f.TrimEqual(field, values[0])
	}
	boxed := make([]interface{}, len(values))
	for i, value := range values {
		boxed[i] = value
	}
	return f.Where(field, OpTrimIn, boxed)
}

// trimInPlaceholders renders the `LOWER(TRIM(?)), …` list a TRIM_IN condition
// binds its values into, so each value is normalized exactly like the column
// side is (PostgreSQL LOWER, not Go's — collation stays the database's job).
func trimInPlaceholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("LOWER(TRIM(?)), ", count), ", ")
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

// NotIn adds a NOT IN condition.
func (f *Filter) NotIn(field string, values ...interface{}) *Filter {
	return f.Where(field, OpNotIn, values)
}

// Or adds a logical OR condition with another filter
func (f *Filter) Or(filter Filter) *Filter {
	if filter.tableAlias == "" && f.tableAlias != "" {
		filter.WithTableAlias(f.tableAlias)
	}
	f.or = append(f.or, filter)
	return f
}

// And adds a grouped logical AND condition with another filter.
func (f *Filter) And(filter Filter) *Filter {
	if filter.tableAlias == "" && f.tableAlias != "" {
		filter.WithTableAlias(f.tableAlias)
	}
	f.and = append(f.and, filter)
	return f
}

// DateBetween adds a date between filter for a calendar date contained within
// a [startField, endField] range of DATE columns. timezone.Date binds as a
// 'YYYY-MM-DD' literal, so no UTC shift can occur.
func (f *Filter) DateBetween(startField, endField string, date timezone.Date) *Filter {
	f.LessThanOrEqual(startField, date)
	f.GreaterThanOrEqual(endField, date)
	return f
}

// Get retrieves the value of a filter condition by field name (first match only)
// Returns the value and true if found, or nil and false if not found
func (f *Filter) Get(field string) (interface{}, bool) {
	for _, condition := range f.conditions {
		if condition.Field == field && condition.Operator == OpEqual {
			return condition.Value, true
		}
	}
	return nil, false
}

// Remove removes all conditions for a specific field from the filter
func (f *Filter) Remove(field string) *Filter {
	filtered := make([]FilterCondition, 0, len(f.conditions))
	for _, condition := range f.conditions {
		if condition.Field != field {
			filtered = append(filtered, condition)
		}
	}
	f.conditions = filtered
	return f
}

// ApplyToQuery applies the filter to a Bun query
func (f *Filter) ApplyToQuery(query *bun.SelectQuery) *bun.SelectQuery {
	// Keep this filter's OR expression inside one AND group. Besides making
	// A.Or(B).And(C) mean (A OR B) AND C, this prevents an OR branch from
	// escaping tenant or other predicates already attached to the query.
	if len(f.or) > 0 {
		query = query.WhereGroup(" AND ", func(group *bun.SelectQuery) *bun.SelectQuery {
			group = f.applyConditionsToQuery(group)
			return applyLogicalConditions(group, f.or, " OR ")
		})
	} else {
		query = f.applyConditionsToQuery(query)
	}

	// Apply grouped AND conditions
	query = applyLogicalConditions(query, f.and, " AND ")

	return query
}

func (f *Filter) applyConditionsToQuery(query *bun.SelectQuery) *bun.SelectQuery {
	for _, condition := range f.conditions {
		query = f.applyConditionToQuery(query, condition)
	}
	return query
}

// applyConditionToQuery applies a single filter condition to the query
func (f *Filter) applyConditionToQuery(query *bun.SelectQuery, condition FilterCondition) *bun.SelectQuery {
	if f.tableAlias != "" {
		columnRef := fmt.Sprintf(`"%s"."%s"`, f.tableAlias, condition.Field)
		return applyOperatorWithColumnRef(query, columnRef, condition)
	}
	return applyOperatorWithIdent(query, condition.Field, condition)
}

// applyOperatorWithColumnRef applies operator with direct column reference (for aliased tables)
func applyOperatorWithColumnRef(query *bun.SelectQuery, columnRef string, condition FilterCondition) *bun.SelectQuery {
	switch condition.Operator {
	case OpEqual:
		return query.Where(columnRef+" = ?", condition.Value)
	case OpGreaterThan:
		return query.Where(columnRef+" > ?", condition.Value)
	case OpGreaterThanOrEqual:
		return query.Where(columnRef+" >= ?", condition.Value)
	case OpLessThan:
		return query.Where(columnRef+" < ?", condition.Value)
	case OpLessThanOrEqual:
		return query.Where(columnRef+" <= ?", condition.Value)
	case OpLike:
		return query.Where(columnRef+" LIKE ?", condition.Value)
	case OpILike:
		return query.Where(columnRef+" ILIKE ?", condition.Value)
	case OpTrimEqual:
		return query.Where("LOWER(TRIM("+columnRef+")) = LOWER(TRIM(?))", condition.Value)
	case OpTrimIn:
		if values, ok := condition.Value.([]interface{}); ok && len(values) > 0 {
			return query.Where("LOWER(TRIM("+columnRef+")) IN ("+trimInPlaceholders(len(values))+")", values...)
		}
	case OpIsNull:
		return query.Where(columnRef + " IS NULL")
	case OpIsNotNull:
		return query.Where(columnRef + " IS NOT NULL")
	case OpIn:
		if values, ok := condition.Value.([]interface{}); ok {
			return query.Where(columnRef+" IN (?)", bun.List(values))
		}
	case OpNotIn:
		if values, ok := condition.Value.([]interface{}); ok {
			return query.Where(columnRef+" NOT IN (?)", bun.List(values))
		}
	}
	return query
}

// applyOperatorWithIdent applies operator with bun.Ident (for non-aliased tables)
func applyOperatorWithIdent(query *bun.SelectQuery, field string, condition FilterCondition) *bun.SelectQuery {
	fieldIdent := bun.Ident(field)
	switch condition.Operator {
	case OpEqual:
		return query.Where("? = ?", fieldIdent, condition.Value)
	case OpGreaterThan:
		return query.Where("? > ?", fieldIdent, condition.Value)
	case OpGreaterThanOrEqual:
		return query.Where("? >= ?", fieldIdent, condition.Value)
	case OpLessThan:
		return query.Where("? < ?", fieldIdent, condition.Value)
	case OpLessThanOrEqual:
		return query.Where("? <= ?", fieldIdent, condition.Value)
	case OpLike:
		return query.Where("? LIKE ?", fieldIdent, condition.Value)
	case OpILike:
		return query.Where("? ILIKE ?", fieldIdent, condition.Value)
	case OpTrimEqual:
		return query.Where("LOWER(TRIM(?)) = LOWER(TRIM(?))", fieldIdent, condition.Value)
	case OpTrimIn:
		if values, ok := condition.Value.([]interface{}); ok && len(values) > 0 {
			args := append([]interface{}{fieldIdent}, values...)
			return query.Where("LOWER(TRIM(?)) IN ("+trimInPlaceholders(len(values))+")", args...)
		}
	case OpIsNull:
		return query.Where("? IS NULL", fieldIdent)
	case OpIsNotNull:
		return query.Where("? IS NOT NULL", fieldIdent)
	case OpIn:
		if values, ok := condition.Value.([]interface{}); ok {
			return query.Where("? IN (?)", fieldIdent, bun.List(values))
		}
	case OpNotIn:
		if values, ok := condition.Value.([]interface{}); ok {
			return query.Where("? NOT IN (?)", fieldIdent, bun.List(values))
		}
	}
	return query
}

// applyLogicalConditions applies OR or AND conditions to the query
func applyLogicalConditions(query *bun.SelectQuery, filters []Filter, operator string) *bun.SelectQuery {
	for _, filter := range filters {
		localFilter := filter
		query = query.WhereGroup(operator, func(q *bun.SelectQuery) *bun.SelectQuery {
			return localFilter.ApplyToQuery(q)
		})
	}
	return query
}

// Pagination defaults applied when a caller omits valid values.
const (
	// minPage is the lowest valid (and fallback) page number.
	minPage = 1
	// minPageSize is the lowest valid page size; below it the default applies.
	minPageSize = 1
	// defaultPageSize is the fallback result-page size.
	defaultPageSize = 20
)

// Pagination defines a structure for pagination parameters
type Pagination struct {
	Page     int
	PageSize int
}

// NewPagination creates a new pagination with default values
func NewPagination(page, pageSize int) Pagination {
	if page < minPage {
		page = minPage
	}
	if pageSize < minPageSize {
		pageSize = defaultPageSize
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
