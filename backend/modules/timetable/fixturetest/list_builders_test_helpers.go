//go:build testhooks

package fixturetest

// The builders below serve only the tests that pin how the retained
// repositories narrow or reject list options.

func (f *ListFilter) GreaterThan(field string, value any) *ListFilter {
	return f.Where(field, ListOpGreaterThan, value)
}

func (f *ListFilter) ILike(field, value string) *ListFilter {
	return f.Where(field, ListOpILike, value)
}

// Or adds an alternative filter.
func (f *ListFilter) Or(filter ListFilter) *ListFilter {
	f.or = append(f.or, filter)
	return f
}

// AddField appends a field to the sort order.
func (s *ListSorting) AddField(field string, direction ListSortDirection) *ListSorting {
	s.Fields = append(s.Fields, ListSortField{Field: field, Direction: direction})
	return s
}
