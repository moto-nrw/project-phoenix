package base

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

func TestNewFilter(t *testing.T) {
	t.Parallel()

	f := NewFilter()

	if f == nil {
		t.Fatal("NewFilter() should not return nil")
		return
	}

	if f.conditions == nil {
		t.Error("NewFilter().conditions should be initialized")
	}

	if f.or == nil {
		t.Error("NewFilter().or should be initialized")
	}

	if f.tableAlias != "" {
		t.Errorf("NewFilter().tableAlias = %q, want empty string", f.tableAlias)
	}
}

func TestFilter_WithTableAlias(t *testing.T) {
	t.Parallel()

	f := NewFilter().WithTableAlias("users")

	if f.tableAlias != "users" {
		t.Errorf("Filter.WithTableAlias() = %q, want users", f.tableAlias)
	}
}

func TestFilter_Where(t *testing.T) {
	t.Parallel()

	f := NewFilter().Where("status", OpEqual, "active")

	if len(f.conditions) != 1 {
		t.Fatalf("Filter.Where() should add one condition, got %d", len(f.conditions))
	}

	cond := f.conditions[0]
	if cond.Field != "status" {
		t.Errorf("condition.Field = %q, want status", cond.Field)
	}
	if cond.Operator != OpEqual {
		t.Errorf("condition.Operator = %v, want %v", cond.Operator, OpEqual)
	}
	if cond.Value != "active" {
		t.Errorf("condition.Value = %v, want active", cond.Value)
	}
}

func TestFilter_Equal(t *testing.T) {
	t.Parallel()

	f := NewFilter().Equal("id", 42)

	if len(f.conditions) != 1 {
		t.Fatalf("Filter.Equal() should add one condition, got %d", len(f.conditions))
	}

	if f.conditions[0].Operator != OpEqual {
		t.Errorf("Filter.Equal() operator = %v, want %v", f.conditions[0].Operator, OpEqual)
	}
}

func TestFilter_Comparisons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filter   *Filter
		expected Operator
	}{
		{
			name:     "GreaterThan",
			filter:   NewFilter().GreaterThan("age", 18),
			expected: OpGreaterThan,
		},
		{
			name:     "GreaterThanOrEqual",
			filter:   NewFilter().GreaterThanOrEqual("age", 18),
			expected: OpGreaterThanOrEqual,
		},
		{
			name:     "LessThan",
			filter:   NewFilter().LessThan("age", 65),
			expected: OpLessThan,
		},
		{
			name:     "LessThanOrEqual",
			filter:   NewFilter().LessThanOrEqual("age", 65),
			expected: OpLessThanOrEqual,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.filter.conditions) != 1 {
				t.Fatalf("Filter should have one condition, got %d", len(tt.filter.conditions))
			}

			if tt.filter.conditions[0].Operator != tt.expected {
				t.Errorf("Filter operator = %v, want %v", tt.filter.conditions[0].Operator, tt.expected)
			}
		})
	}
}

func TestFilter_Like(t *testing.T) {
	t.Parallel()

	f := NewFilter().Like("name", "%john%")

	if len(f.conditions) != 1 {
		t.Fatalf("Filter.Like() should add one condition, got %d", len(f.conditions))
	}

	cond := f.conditions[0]
	if cond.Operator != OpLike {
		t.Errorf("Filter.Like() operator = %v, want %v", cond.Operator, OpLike)
	}
	if cond.Value != "%john%" {
		t.Errorf("Filter.Like() value = %v, want %%john%%", cond.Value)
	}
}

func TestFilter_ILike(t *testing.T) {
	t.Parallel()

	f := NewFilter().ILike("email", "%@example.com")

	if len(f.conditions) != 1 {
		t.Fatalf("Filter.ILike() should add one condition, got %d", len(f.conditions))
	}

	if f.conditions[0].Operator != OpILike {
		t.Errorf("Filter.ILike() operator = %v, want %v", f.conditions[0].Operator, OpILike)
	}
}

func TestFilter_TrimIn(t *testing.T) {
	t.Parallel()

	t.Run("several values", func(t *testing.T) {
		f := NewFilter().TrimIn("school_class", "3a", "4b")

		if len(f.conditions) != 1 {
			t.Fatalf("Filter.TrimIn() should add one condition, got %d", len(f.conditions))
		}
		cond := f.conditions[0]
		if cond.Operator != OpTrimIn {
			t.Errorf("Filter.TrimIn() operator = %v, want %v", cond.Operator, OpTrimIn)
		}
		values, ok := cond.Value.([]interface{})
		if !ok {
			t.Fatalf("Filter.TrimIn() value should be []interface{}, got %T", cond.Value)
		}
		if len(values) != 2 {
			t.Errorf("Filter.TrimIn() should have 2 values, got %d", len(values))
		}
	})

	// One value must produce the exact condition the single-select filter
	// always produced, so the multi-value form changes no existing query.
	t.Run("single value collapses to TrimEqual", func(t *testing.T) {
		f := NewFilter().TrimIn("school_class", "3a")

		if len(f.conditions) != 1 {
			t.Fatalf("Filter.TrimIn() should add one condition, got %d", len(f.conditions))
		}
		if f.conditions[0].Operator != OpTrimEqual {
			t.Errorf("Filter.TrimIn() operator = %v, want %v", f.conditions[0].Operator, OpTrimEqual)
		}
		if f.conditions[0].Value != "3a" {
			t.Errorf("Filter.TrimIn() value = %v, want 3a", f.conditions[0].Value)
		}
	})

	// No values means "no restriction". An empty IN () would match nothing and
	// silently return an empty list instead.
	t.Run("no values adds no condition", func(t *testing.T) {
		f := NewFilter().TrimIn("school_class")

		if len(f.conditions) != 0 {
			t.Fatalf("Filter.TrimIn() with no values should add no condition, got %d", len(f.conditions))
		}
	})
}

func TestFilter_FirstNumberIn(t *testing.T) {
	t.Parallel()

	t.Run("several values", func(t *testing.T) {
		f := NewFilter().FirstNumberIn("school_class", "3", "4")

		if len(f.conditions) != 1 {
			t.Fatalf("Filter.FirstNumberIn() should add one condition, got %d", len(f.conditions))
		}
		cond := f.conditions[0]
		if cond.Operator != OpFirstNumberIn {
			t.Errorf("Filter.FirstNumberIn() operator = %v, want %v", cond.Operator, OpFirstNumberIn)
		}
		values, ok := cond.Value.([]interface{})
		if !ok {
			t.Fatalf("Filter.FirstNumberIn() value should be []interface{}, got %T", cond.Value)
		}
		if len(values) != 2 {
			t.Errorf("Filter.FirstNumberIn() should have 2 values, got %d", len(values))
		}
	})

	// A single grade keeps the same operator: unlike TrimIn there is no cheaper
	// one-value form to collapse into.
	t.Run("single value keeps the operator", func(t *testing.T) {
		f := NewFilter().FirstNumberIn("school_class", "3")

		if len(f.conditions) != 1 {
			t.Fatalf("Filter.FirstNumberIn() should add one condition, got %d", len(f.conditions))
		}
		if f.conditions[0].Operator != OpFirstNumberIn {
			t.Errorf("Filter.FirstNumberIn() operator = %v, want %v", f.conditions[0].Operator, OpFirstNumberIn)
		}
	})

	t.Run("no values adds no condition", func(t *testing.T) {
		f := NewFilter().FirstNumberIn("school_class")

		if len(f.conditions) != 0 {
			t.Fatalf("Filter.FirstNumberIn() with no values should add no condition, got %d", len(f.conditions))
		}
	})
}

func TestTrimInPlaceholders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		count int
		want  string
	}{
		{0, ""},
		{1, "LOWER(TRIM(?))"},
		{3, "LOWER(TRIM(?)), LOWER(TRIM(?)), LOWER(TRIM(?))"},
	}
	for _, tt := range tests {
		if got := trimInPlaceholders(tt.count); got != tt.want {
			t.Errorf("trimInPlaceholders(%d) = %q, want %q", tt.count, got, tt.want)
		}
	}
}

func TestFilter_NullChecks(t *testing.T) {
	t.Parallel()

	t.Run("IsNull", func(t *testing.T) {
		f := NewFilter().IsNull("deleted_at")

		if len(f.conditions) != 1 {
			t.Fatalf("Filter.IsNull() should add one condition, got %d", len(f.conditions))
		}

		cond := f.conditions[0]
		if cond.Operator != OpIsNull {
			t.Errorf("Filter.IsNull() operator = %v, want %v", cond.Operator, OpIsNull)
		}
		if cond.Value != nil {
			t.Errorf("Filter.IsNull() value should be nil, got %v", cond.Value)
		}
	})

	t.Run("IsNotNull", func(t *testing.T) {
		f := NewFilter().IsNotNull("email")

		if len(f.conditions) != 1 {
			t.Fatalf("Filter.IsNotNull() should add one condition, got %d", len(f.conditions))
		}

		if f.conditions[0].Operator != OpIsNotNull {
			t.Errorf("Filter.IsNotNull() operator = %v, want %v", f.conditions[0].Operator, OpIsNotNull)
		}
	})
}

func TestFilter_In(t *testing.T) {
	t.Parallel()

	f := NewFilter().In("status", "active", "pending", "approved")

	if len(f.conditions) != 1 {
		t.Fatalf("Filter.In() should add one condition, got %d", len(f.conditions))
	}

	cond := f.conditions[0]
	if cond.Operator != OpIn {
		t.Errorf("Filter.In() operator = %v, want %v", cond.Operator, OpIn)
	}

	values, ok := cond.Value.([]interface{})
	if !ok {
		t.Fatalf("Filter.In() value should be []interface{}, got %T", cond.Value)
	}

	if len(values) != 3 {
		t.Errorf("Filter.In() should have 3 values, got %d", len(values))
	}
}

func TestFilter_NotIn(t *testing.T) {
	t.Parallel()

	f := NewFilter().NotIn("name", "WC", "Toilette")

	if len(f.conditions) != 1 {
		t.Fatalf("Filter.NotIn() should add one condition, got %d", len(f.conditions))
	}
	cond := f.conditions[0]
	if cond.Operator != OpNotIn {
		t.Errorf("Filter.NotIn() operator = %v, want %v", cond.Operator, OpNotIn)
	}
}

func TestFilter_Or(t *testing.T) {
	t.Parallel()

	mainFilter := NewFilter().Equal("status", "active")
	orFilter := Filter{}
	orFilter.Equal("role", "admin")

	mainFilter.Or(orFilter)

	if len(mainFilter.or) != 1 {
		t.Fatalf("Filter.Or() should add one or-filter, got %d", len(mainFilter.or))
	}
}

func TestFilter_AndGroupsNestedConditionsAndInheritsAlias(t *testing.T) {
	t.Parallel()

	mainFilter := NewFilter().WithTableAlias("room").Equal("building", "A")
	visibility := NewFilter().Equal("is_system", false)
	visibility.Or(*NewFilter().Equal("name", "Schulhof"))

	mainFilter.And(*visibility)

	if len(mainFilter.and) != 1 {
		t.Fatalf("Filter.And() should add one and-filter, got %d", len(mainFilter.and))
	}
	nested := mainFilter.and[0]
	if nested.tableAlias != "room" {
		t.Errorf("nested AND alias = %q, want room", nested.tableAlias)
	}
	if len(nested.or) != 1 || nested.or[0].tableAlias != "room" {
		t.Fatalf("nested OR filter should inherit room alias: %+v", nested.or)
	}
}

func TestFilter_DateBetween(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2024, time.June, 15)

	f := NewFilter().DateBetween("start_date", "end_date", date)

	if len(f.conditions) != 2 {
		t.Fatalf("Filter.DateBetween() should add two conditions, got %d", len(f.conditions))
	}

	// start_date <= date
	if f.conditions[0].Field != "start_date" || f.conditions[0].Operator != OpLessThanOrEqual {
		t.Errorf("DateBetween first condition = %+v, want start_date <=", f.conditions[0])
	}
	if got, ok := f.conditions[0].Value.(timezone.Date); !ok || got != date {
		t.Errorf("DateBetween first value = %T(%v), want timezone.Date(%v)", f.conditions[0].Value, f.conditions[0].Value, date)
	}

	// end_date >= date
	if f.conditions[1].Field != "end_date" || f.conditions[1].Operator != OpGreaterThanOrEqual {
		t.Errorf("DateBetween second condition = %+v, want end_date >=", f.conditions[1])
	}
	if got, ok := f.conditions[1].Value.(timezone.Date); !ok || got != date {
		t.Errorf("DateBetween second value = %T(%v), want timezone.Date(%v)", f.conditions[1].Value, f.conditions[1].Value, date)
	}
}

func TestNewPagination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		page         int
		pageSize     int
		expectedPage int
		expectedSize int
	}{
		{
			name:         "valid values",
			page:         2,
			pageSize:     50,
			expectedPage: 2,
			expectedSize: 50,
		},
		{
			name:         "zero page defaults to 1",
			page:         0,
			pageSize:     20,
			expectedPage: 1,
			expectedSize: 20,
		},
		{
			name:         "negative page defaults to 1",
			page:         -5,
			pageSize:     20,
			expectedPage: 1,
			expectedSize: 20,
		},
		{
			name:         "zero pageSize defaults to 20",
			page:         1,
			pageSize:     0,
			expectedPage: 1,
			expectedSize: 20,
		},
		{
			name:         "negative pageSize defaults to 20",
			page:         1,
			pageSize:     -10,
			expectedPage: 1,
			expectedSize: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPagination(tt.page, tt.pageSize)

			if p.Page != tt.expectedPage {
				t.Errorf("NewPagination().Page = %v, want %v", p.Page, tt.expectedPage)
			}

			if p.PageSize != tt.expectedSize {
				t.Errorf("NewPagination().PageSize = %v, want %v", p.PageSize, tt.expectedSize)
			}
		})
	}
}

func TestSorting_AddField(t *testing.T) {
	t.Parallel()

	s := &Sorting{}

	s.AddField("created_at", SortDesc).AddField("name", SortAsc)

	if len(s.Fields) != 2 {
		t.Fatalf("Sorting.AddField() should add fields, got %d", len(s.Fields))
	}

	if s.Fields[0].Field != "created_at" || s.Fields[0].Direction != SortDesc {
		t.Errorf("First sort field = %+v, want created_at DESC", s.Fields[0])
	}

	if s.Fields[1].Field != "name" || s.Fields[1].Direction != SortAsc {
		t.Errorf("Second sort field = %+v, want name ASC", s.Fields[1])
	}
}

func TestSortDirectionConstants(t *testing.T) {
	t.Parallel()

	if SortAsc != "ASC" {
		t.Errorf("SortAsc = %q, want ASC", SortAsc)
	}
	if SortDesc != "DESC" {
		t.Errorf("SortDesc = %q, want DESC", SortDesc)
	}
}

func TestNewQueryOptions(t *testing.T) {
	t.Parallel()

	qo := NewQueryOptions()

	if qo == nil {
		t.Fatal("NewQueryOptions() should not return nil")
		return
	}

	if qo.Filter == nil {
		t.Error("NewQueryOptions().Filter should be initialized")
	}

	if qo.Pagination != nil {
		t.Error("NewQueryOptions().Pagination should be nil by default")
	}

	if qo.Sorting != nil {
		t.Error("NewQueryOptions().Sorting should be nil by default")
	}
}

func TestQueryOptions_WithPagination(t *testing.T) {
	t.Parallel()

	qo := NewQueryOptions().WithPagination(3, 25)

	if qo.Pagination == nil {
		t.Fatal("WithPagination() should set Pagination")
	}

	if qo.Pagination.Page != 3 {
		t.Errorf("WithPagination().Page = %v, want 3", qo.Pagination.Page)
	}

	if qo.Pagination.PageSize != 25 {
		t.Errorf("WithPagination().PageSize = %v, want 25", qo.Pagination.PageSize)
	}
}

func TestFilter_Get(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setup         func() *Filter
		field         string
		expectedValue interface{}
		expectedFound bool
	}{
		{
			name: "get existing field",
			setup: func() *Filter {
				return NewFilter().Equal("status", "active")
			},
			field:         "status",
			expectedValue: "active",
			expectedFound: true,
		},
		{
			name: "get non-existing field",
			setup: func() *Filter {
				return NewFilter().Equal("status", "active")
			},
			field:         "role",
			expectedValue: nil,
			expectedFound: false,
		},
		{
			name: "get from empty filter",
			setup: func() *Filter {
				return NewFilter()
			},
			field:         "anything",
			expectedValue: nil,
			expectedFound: false,
		},
		{
			name: "get first match when multiple conditions exist",
			setup: func() *Filter {
				return NewFilter().Equal("status", "first").Equal("status", "second")
			},
			field:         "status",
			expectedValue: "first",
			expectedFound: true,
		},
		{
			name: "get only works for Equal operator",
			setup: func() *Filter {
				return NewFilter().Like("name", "%test%")
			},
			field:         "name",
			expectedValue: nil,
			expectedFound: false,
		},
		{
			name: "get boolean value",
			setup: func() *Filter {
				return NewFilter().Equal("active_only", true)
			},
			field:         "active_only",
			expectedValue: true,
			expectedFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.setup()
			value, found := f.Get(tt.field)

			if found != tt.expectedFound {
				t.Errorf("Get() found = %v, want %v", found, tt.expectedFound)
			}

			if value != tt.expectedValue {
				t.Errorf("Get() value = %v, want %v", value, tt.expectedValue)
			}
		})
	}
}

func TestFilter_Remove(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		setup               func() *Filter
		fieldToRemove       string
		expectedConditions  int
		remainingFieldCheck string
	}{
		{
			name: "remove existing field",
			setup: func() *Filter {
				return NewFilter().Equal("status", "active").Equal("role", "admin")
			},
			fieldToRemove:       "status",
			expectedConditions:  1,
			remainingFieldCheck: "role",
		},
		{
			name: "remove non-existing field",
			setup: func() *Filter {
				return NewFilter().Equal("status", "active")
			},
			fieldToRemove:       "role",
			expectedConditions:  1,
			remainingFieldCheck: "status",
		},
		{
			name: "remove from empty filter",
			setup: func() *Filter {
				return NewFilter()
			},
			fieldToRemove:      "anything",
			expectedConditions: 0,
		},
		{
			name: "remove all occurrences of field",
			setup: func() *Filter {
				return NewFilter().Equal("status", "first").Equal("role", "admin").Equal("status", "second")
			},
			fieldToRemove:       "status",
			expectedConditions:  1,
			remainingFieldCheck: "role",
		},
		{
			name: "remove returns filter for chaining",
			setup: func() *Filter {
				return NewFilter().Equal("a", 1).Equal("b", 2).Equal("c", 3)
			},
			fieldToRemove:      "b",
			expectedConditions: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.setup()
			result := f.Remove(tt.fieldToRemove)

			// Verify chaining works
			if result != f {
				t.Error("Remove() should return the same filter for chaining")
			}

			if len(f.conditions) != tt.expectedConditions {
				t.Errorf("Remove() conditions count = %d, want %d", len(f.conditions), tt.expectedConditions)
			}

			// Verify the field was removed
			for _, cond := range f.conditions {
				if cond.Field == tt.fieldToRemove {
					t.Errorf("Remove() did not remove field %q", tt.fieldToRemove)
				}
			}

			// Verify remaining field exists if specified
			if tt.remainingFieldCheck != "" {
				found := false
				for _, cond := range f.conditions {
					if cond.Field == tt.remainingFieldCheck {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Remove() accidentally removed field %q", tt.remainingFieldCheck)
				}
			}
		})
	}
}

// =============================================================================
// APPLY TO QUERY TESTS
// These tests verify that filters can be applied to BUN queries without panicking
// =============================================================================

func TestPagination_Offset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		page           int
		pageSize       int
		expectedOffset int
	}{
		{
			name:           "page 1",
			page:           1,
			pageSize:       20,
			expectedOffset: 0,
		},
		{
			name:           "page 2",
			page:           2,
			pageSize:       20,
			expectedOffset: 20,
		},
		{
			name:           "page 3 with custom size",
			page:           3,
			pageSize:       50,
			expectedOffset: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPagination(tt.page, tt.pageSize)
			offset := (p.Page - 1) * p.PageSize
			if offset != tt.expectedOffset {
				t.Errorf("Pagination offset = %d, want %d", offset, tt.expectedOffset)
			}
		})
	}
}
