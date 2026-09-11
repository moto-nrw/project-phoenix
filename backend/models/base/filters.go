package base

import "slices"

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
	// OpFirstNumberIn matches the first run of digits inside a free-text column
	// against a set of numbers (see Filter.FirstNumberIn).
	OpFirstNumberIn Operator = "FIRST_NUMBER_IN"

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

// FirstNumberIn matches when the first run of digits inside a free-text column
// equals ANY of the values — the SQL twin of schoolclass.GradePrefix, which
// reads "3" out of "3a" and "Klasse 3a" alike and "13" out of "13a". A plain
// LIKE '3%' cannot express that: it would count 13a as a third-graders' class.
//
// It exists so a grade filter can be answered by the database instead of by
// fetching every child and dropping most of them again in memory (#2218
// review); the in-memory form loses SQL pagination, so each page repeats the
// whole query, enrichment and filtering pass.
//
// Values are the plain numbers ("3", "4"). Passing none adds no condition, for
// the same reason as TrimIn.
func (f *Filter) FirstNumberIn(field string, values ...string) *Filter {
	if len(values) == 0 {
		return f
	}
	boxed := make([]interface{}, len(values))
	for i, value := range values {
		boxed[i] = value
	}
	return f.Where(field, OpFirstNumberIn, boxed)
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

func (f Filter) Conditions() []FilterCondition { return slices.Clone(f.conditions) }
func (f Filter) OrFilters() []Filter           { return slices.Clone(f.or) }
func (f Filter) AndFilters() []Filter          { return slices.Clone(f.and) }
func (f Filter) TableAlias() string            { return f.tableAlias }

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

func (p Pagination) Offset() int { return (p.Page - 1) * p.PageSize }

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
