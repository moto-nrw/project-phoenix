package ports

import (
	"errors"
	"slices"
	"strings"
	"time"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
)

// FilterOperator is a comparison operator of a QueryFilter condition.
type FilterOperator string

// Filter operators of a QueryFilter condition.
const (
	FilterEqual              FilterOperator = "="
	FilterGreaterThan        FilterOperator = ">"
	FilterGreaterThanOrEqual FilterOperator = ">="
	FilterLessThan           FilterOperator = "<"
	FilterLessThanOrEqual    FilterOperator = "<="
	FilterLike               FilterOperator = "LIKE"
	FilterILike              FilterOperator = "ILIKE"
	FilterTrimEqual          FilterOperator = "TRIM_EQUALS"
	FilterTrimIn             FilterOperator = "TRIM_IN"
	FilterFirstNumberIn      FilterOperator = "FIRST_NUMBER_IN"
	FilterIsNull             FilterOperator = "IS NULL"
	FilterIsNotNull          FilterOperator = "IS NOT NULL"
	FilterIn                 FilterOperator = "IN"
	FilterNotIn              FilterOperator = "NOT IN"
)

// FilterCondition is one condition of a QueryFilter.
type FilterCondition struct {
	Field    string
	Operator FilterOperator
	Value    any
}

// QueryFilter is a set of conditions joined by AND, with nested OR and AND
// groups.
type QueryFilter struct {
	conditions []FilterCondition
	or         []QueryFilter
	and        []QueryFilter
	tableAlias string
}

// WithTableAlias qualifies the fields of the filter and of its groups that
// have no alias of their own.
func (f *QueryFilter) WithTableAlias(alias string) *QueryFilter {
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

// Where adds a condition.
func (f *QueryFilter) Where(field string, operator FilterOperator, value any) *QueryFilter {
	f.conditions = append(f.conditions, FilterCondition{Field: field, Operator: operator, Value: value})
	return f
}

// Equal adds an equality condition.
func (f *QueryFilter) Equal(field string, value any) *QueryFilter {
	return f.Where(field, FilterEqual, value)
}

// GreaterThan adds a greater-than condition.
func (f *QueryFilter) GreaterThan(field string, value any) *QueryFilter {
	return f.Where(field, FilterGreaterThan, value)
}

// GreaterThanOrEqual adds a greater-than-or-equal condition.
func (f *QueryFilter) GreaterThanOrEqual(field string, value any) *QueryFilter {
	return f.Where(field, FilterGreaterThanOrEqual, value)
}

// LessThan adds a less-than condition.
func (f *QueryFilter) LessThan(field string, value any) *QueryFilter {
	return f.Where(field, FilterLessThan, value)
}

// LessThanOrEqual adds a less-than-or-equal condition.
func (f *QueryFilter) LessThanOrEqual(field string, value any) *QueryFilter {
	return f.Where(field, FilterLessThanOrEqual, value)
}

// Like adds a LIKE condition.
func (f *QueryFilter) Like(field, value string) *QueryFilter {
	return f.Where(field, FilterLike, value)
}

// ILike adds a case-insensitive LIKE condition.
func (f *QueryFilter) ILike(field, value string) *QueryFilter {
	return f.Where(field, FilterILike, value)
}

// TrimEqual adds a case-insensitive equality condition after trimming both
// sides.
func (f *QueryFilter) TrimEqual(field, value string) *QueryFilter {
	return f.Where(field, FilterTrimEqual, value)
}

// TrimIn matches when the column equals any of the values, both sides trimmed
// and lower-cased. No values add no condition: an empty IN () would match
// nothing where callers mean "no restriction" (#2218).
func (f *QueryFilter) TrimIn(field string, values ...string) *QueryFilter {
	if len(values) == 0 {
		return f
	}
	if len(values) == 1 {
		return f.TrimEqual(field, values[0])
	}
	return f.Where(field, FilterTrimIn, boxStrings(values))
}

// FirstNumberIn matches when the first run of digits inside a free-text
// column equals any of the values: "3" out of "3a" and "Klasse 3a" alike,
// "13" out of "13a" (#2218). No values add no condition.
func (f *QueryFilter) FirstNumberIn(field string, values ...string) *QueryFilter {
	if len(values) == 0 {
		return f
	}
	return f.Where(field, FilterFirstNumberIn, boxStrings(values))
}

// IsNull adds an IS NULL condition.
func (f *QueryFilter) IsNull(field string) *QueryFilter {
	return f.Where(field, FilterIsNull, nil)
}

// IsNotNull adds an IS NOT NULL condition.
func (f *QueryFilter) IsNotNull(field string) *QueryFilter {
	return f.Where(field, FilterIsNotNull, nil)
}

// In adds an IN condition.
func (f *QueryFilter) In(field string, values ...any) *QueryFilter {
	return f.Where(field, FilterIn, values)
}

// NotIn adds a NOT IN condition.
func (f *QueryFilter) NotIn(field string, values ...any) *QueryFilter {
	return f.Where(field, FilterNotIn, values)
}

// Or adds a group joined by OR.
func (f *QueryFilter) Or(filter QueryFilter) *QueryFilter {
	if filter.tableAlias == "" && f.tableAlias != "" {
		filter.WithTableAlias(f.tableAlias)
	}
	f.or = append(f.or, filter)
	return f
}

// And adds a group joined by AND.
func (f *QueryFilter) And(filter QueryFilter) *QueryFilter {
	if filter.tableAlias == "" && f.tableAlias != "" {
		filter.WithTableAlias(f.tableAlias)
	}
	f.and = append(f.and, filter)
	return f
}

// Get returns the value of the first equality condition on field.
func (f *QueryFilter) Get(field string) (any, bool) {
	for _, condition := range f.conditions {
		if condition.Field == field && condition.Operator == FilterEqual {
			return condition.Value, true
		}
	}
	return nil, false
}

// Remove drops every condition on field.
func (f *QueryFilter) Remove(field string) *QueryFilter {
	filtered := make([]FilterCondition, 0, len(f.conditions))
	for _, condition := range f.conditions {
		if condition.Field != field {
			filtered = append(filtered, condition)
		}
	}
	f.conditions = filtered
	return f
}

func (f QueryFilter) Conditions() []FilterCondition { return slices.Clone(f.conditions) }

func (f QueryFilter) OrFilters() []QueryFilter { return slices.Clone(f.or) }

func (f QueryFilter) AndFilters() []QueryFilter { return slices.Clone(f.and) }

func (f QueryFilter) TableAlias() string { return f.tableAlias }

// defaultPageSize is the page size a pagination below one row falls back to.
const defaultPageSize = 20

// Pagination selects one page of a list.
type Pagination struct {
	Page     int
	PageSize int
}

// Offset is the number of rows before the page.
func (p Pagination) Offset() int { return (p.Page - 1) * p.PageSize }

// SortDirection is the direction of a QuerySortField.
type SortDirection string

// SortAsc and SortDesc are the directions of a QuerySortField.
const (
	SortAsc  SortDirection = "ASC"
	SortDesc SortDirection = "DESC"
)

// QuerySortField is one sort key.
type QuerySortField struct {
	Field     string
	Direction SortDirection
}

// QuerySorting is the ordered list of sort keys.
type QuerySorting struct {
	Fields []QuerySortField
}

// AddField appends a sort key.
func (s *QuerySorting) AddField(field string, direction SortDirection) *QuerySorting {
	s.Fields = append(s.Fields, QuerySortField{Field: field, Direction: direction})
	return s
}

// QueryOptions combines filter, pagination and sorting of a list.
type QueryOptions struct {
	Filter     *QueryFilter
	Pagination *Pagination
	Sorting    *QuerySorting
}

// WithPagination selects one page; a page below one becomes the first page
// and a page size below one the default size.
func (qo *QueryOptions) WithPagination(page, pageSize int) *QueryOptions {
	qo.Pagination = &Pagination{Page: max(page, 1), PageSize: pageSize}
	if pageSize < 1 {
		qo.Pagination.PageSize = defaultPageSize
	}
	return qo
}

// RequestQueueFilters narrows one change-request queue in SQL: which
// children, where to resume, and how many rows (#2432). The zero value is the
// whole queue from the top.
type RequestQueueFilters struct {
	// UrgentOnly selects the open queue's urgency phase. Nil leaves urgency
	// unrestricted; true/false select exactly one phase.
	UrgentOnly *bool
	UrgentDate string
	// StudentIDs limits the queue to a set of children. Empty means no
	// restriction; it composes with StudentID.
	StudentIDs []int64
	// StudentID limits the queue to one child (#2437). Zero means every child.
	StudentID int64
	// Search matches the child's name case-insensitively as a substring of
	// "first last".
	Search string
	// BeforeInstant and BeforeID are the keyset position of the last row the
	// caller consumed. A zero instant returns the first page.
	BeforeInstant time.Time
	BeforeID      int64
	// Limit caps the page. Zero or less means unbounded.
	Limit int
}

// ErrRepositoryNotFound is the persistence-neutral not-found sentinel of the
// People Directory repositories. Callers that may not import this package
// recognise it by its RepositoryNotFound marker, as they recognise the
// generic sentinel of models/base.
var ErrRepositoryNotFound error = repositoryNotFound{}

type repositoryNotFound struct{}

func (repositoryNotFound) Error() string { return "repository: not found" }

// RepositoryNotFound marks the not-found sentinel.
func (repositoryNotFound) RepositoryNotFound() {}

// DatabaseError is the failure of one repository operation.
type DatabaseError struct {
	Op  string // Operation that failed (e.g., "create", "update")
	Err error  // Original error
}

// Error returns the error message.
func (e *DatabaseError) Error() string {
	if e.Err == nil {
		return "database error during " + e.Op
	}
	return "database error during " + e.Op + ": " + e.Err.Error()
}

// Unwrap returns the original error.
func (e *DatabaseError) Unwrap() error {
	return e.Err
}

// StoreFailure marks the error as the store's failure rather than a refusal
// of the request, for callers that may not name this type (#2736).
func (e *DatabaseError) StoreFailure() bool {
	return true
}

// ErrGuardianEmailTaken marks a guardian profile write the e-mail index
// refused: another profile of the school already uses the address.
var ErrGuardianEmailTaken = errors.New("guardian e-mail is already used in this school")

// NewQueryFilter returns an empty filter.
func NewQueryFilter() *QueryFilter {
	return &QueryFilter{}
}

func boxStrings(values []string) []any {
	boxed := make([]any, len(values))
	for i, value := range values {
		boxed[i] = value
	}
	return boxed
}

// NewQueryOptions returns options with an empty filter.
func NewQueryOptions() *QueryOptions {
	return &QueryOptions{Filter: NewQueryFilter()}
}

// IsRepositoryNoRows reports whether err carries a repository not-found
// sentinel: this package's or any other that has the marker.
func IsRepositoryNoRows(err error) bool {
	var marker interface{ RepositoryNotFound() }
	return errors.As(err, &marker)
}

// RowMissing reports the "no row" outcomes the retained repositories use: the
// translated not-found sentinel and the guardian sentinels of the row package.
// Those repositories translate a driver no-rows result into one of these
// before it gets here. The composition root classifies repository results
// with it when it binds the Identity & Access seams (#3364).
func RowMissing(err error) bool {
	return errors.Is(err, ErrRepositoryNotFound) ||
		errors.Is(err, peopleValues.ErrGuardianProfileNotFound) || errors.Is(err, peopleValues.ErrStudentGuardianNotFound)
}

// MissingStudentError is the not-found shape of a child that is not there,
// with the no-rows sentinel the retained callers match (#3349).
func MissingStudentError(op string) error {
	return &DatabaseError{Op: op, Err: errors.Join(ErrRepositoryNotFound, transactionruntime.ErrNoRows, peopleValues.ErrStudentRowMissing)}
}

// postgresError is the driver error shape the constraint classification
// reads: the SQLSTATE and the constraint name by field code.
type postgresError interface {
	error
	Field(byte) string
}

// IsConstraintViolation reports whether err carries PostgreSQL error code
// 23503 (foreign_key_violation) or 23502 (not_null_violation), such as the
// refusal of a guardian delete that still has links.
func IsConstraintViolation(err error) bool {
	var pgErr postgresError
	if errors.As(err, &pgErr) {
		code := pgErr.Field('C')
		return code == "23503" || code == "23502"
	}
	return hasTextualSQLState(err, "23503") || hasTextualSQLState(err, "23502")
}

func hasTextualSQLState(err error, sqlState string) bool {
	return err != nil && strings.Contains(err.Error(), "SQLSTATE="+sqlState)
}
