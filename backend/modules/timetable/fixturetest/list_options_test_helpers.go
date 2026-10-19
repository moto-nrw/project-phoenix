//go:build testhooks

package fixturetest

import "slices"

// QueryOptions is the list query of the retained schedule repositories: a
// filter, a page and a sort order. The repositories narrow it to their
// bounded filters and reject what they cannot serve. It replaces the generic
// query options of models/base for this owner (#4084).
type QueryOptions struct {
	Filter     *ListFilter
	Pagination *ListPagination
	Sorting    *ListSorting
}

// NewQueryOptions returns options with an empty filter.

// WithPagination sets the page; a page below 1 becomes 1 and a page size
// below 1 becomes 20.
func (o *QueryOptions) WithPagination(page, pageSize int) *QueryOptions {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	o.Pagination = &ListPagination{Page: page, PageSize: pageSize}
	return o
}

// ListOperator is the comparison of one list condition.
type ListOperator string

const (
	ListOpEqual       ListOperator = "="
	ListOpGreaterThan ListOperator = ">"
	ListOpLessThan    ListOperator = "<"
	ListOpILike       ListOperator = "ILIKE"
	ListOpIn          ListOperator = "IN"
)

// ListCondition is one field comparison of a list filter.
type ListCondition struct {
	Field    string
	Operator ListOperator
	Value    any
}

// ListFilter holds the conditions of a list query and the alternative and
// grouped filters the repositories reject as compound. Nothing builds a
// grouped filter any more; the repositories still reject one.
type ListFilter struct {
	conditions []ListCondition
	or         []ListFilter
	and        []ListFilter
}

// NewListFilter returns an empty filter.

// Where adds a condition.
func (f *ListFilter) Where(field string, operator ListOperator, value any) *ListFilter {
	f.conditions = append(f.conditions, ListCondition{Field: field, Operator: operator, Value: value})
	return f
}

func (f *ListFilter) Equal(field string, value any) *ListFilter {
	return f.Where(field, ListOpEqual, value)
}

func (f *ListFilter) In(field string, values ...any) *ListFilter {
	return f.Where(field, ListOpIn, values)
}

func (f ListFilter) Conditions() []ListCondition { return slices.Clone(f.conditions) }
func (f ListFilter) OrFilters() []ListFilter     { return slices.Clone(f.or) }
func (f ListFilter) AndFilters() []ListFilter    { return slices.Clone(f.and) }

// ListPagination is a 1-based page.
type ListPagination struct {
	Page     int
	PageSize int
}

func (p ListPagination) Offset() int { return (p.Page - 1) * p.PageSize }

// ListSortDirection is ascending or descending.
type ListSortDirection string

const (
	ListSortAsc  ListSortDirection = "ASC"
	ListSortDesc ListSortDirection = "DESC"
)

// ListSortField is one field of a sort order.
type ListSortField struct {
	Field     string
	Direction ListSortDirection
}

// ListSorting is a sort order.
type ListSorting struct {
	Fields []ListSortField
}
