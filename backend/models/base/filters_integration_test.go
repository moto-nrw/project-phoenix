package base_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// accountTableAlias is the schema-qualified table expression for auth.accounts
const accountTableAlias = `auth.accounts AS "account"`

// =============================================================================
// FILTER APPLY TO QUERY TESTS
// =============================================================================

func TestFilter_ApplyToQuery_Equal(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Create filter with Equal condition using table alias
	filter := base.NewFilter().WithTableAlias("account").Equal("active", true)

	// Build and execute query using real auth.Account model with explicit table
	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = filter.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)

	// All returned records should have active=true
	for _, r := range records {
		assert.True(t, r.Active, "Filter should only return active records")
	}
}

func TestFilter_ApplyToQuery_NotEqual(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	filter := base.NewFilter().WithTableAlias("account").NotEqual("active", false)

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = filter.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)

	// All returned records should NOT have active=false
	for _, r := range records {
		assert.True(t, r.Active, "Filter should exclude inactive records")
	}
}

func TestFilter_ApplyToQuery_ILike(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	filter := base.NewFilter().WithTableAlias("account").ILike("email", "%@example.com")

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = filter.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)

	// All returned records should have emails ending with @example.com
	for _, r := range records {
		assert.Contains(t, r.Email, "@example.com", "Filter should match email pattern")
	}
}

func TestFilter_ApplyToQuery_IsNull(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	filter := base.NewFilter().WithTableAlias("account").IsNull("last_login")

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = filter.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)

	// All returned records should have NULL last_login
	for _, r := range records {
		assert.Nil(t, r.LastLogin, "Filter should only return records with NULL last_login")
	}
}

func TestFilter_ApplyToQuery_IsNotNull(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	filter := base.NewFilter().WithTableAlias("account").IsNotNull("email")

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = filter.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)

	// All returned records should have non-NULL email
	for _, r := range records {
		assert.NotEmpty(t, r.Email, "Filter should only return records with non-NULL email")
	}
}

func TestFilter_ApplyToQuery_In(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	filter := base.NewFilter().WithTableAlias("account").In("active", true, false)

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = filter.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)
	// Query should execute without error - IN clause with both values should return all records
}

func TestFilter_ApplyToQuery_WithTableAlias(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Use filter with explicit table alias
	filter := base.NewFilter().
		WithTableAlias("account").
		Equal("active", true)

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = filter.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)

	for _, r := range records {
		assert.True(t, r.Active, "Filter with alias should work correctly")
	}
}

func TestFilter_ApplyToQuery_MultipleConditions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	filter := base.NewFilter().
		WithTableAlias("account").
		Equal("active", true).
		ILike("email", "%@example.com")

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = filter.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)

	for _, r := range records {
		assert.True(t, r.Active, "Filter should return active records")
		assert.Contains(t, r.Email, "@example.com", "Filter should match email pattern")
	}
}

func TestFilter_ApplyToQuery_Comparisons(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Test GreaterThan on id field
	filter := base.NewFilter().WithTableAlias("account").GreaterThan("id", 0)

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = filter.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)

	for _, r := range records {
		assert.Greater(t, r.ID, int64(0), "Filter should return records with id > 0")
	}
}

func TestFilter_ApplyToQuery_LessThan(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	filter := base.NewFilter().WithTableAlias("account").LessThan("id", 999999)

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = filter.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)

	for _, r := range records {
		assert.Less(t, r.ID, int64(999999), "Filter should return records with id < 999999")
	}
}

// =============================================================================
// PAGINATION APPLY TO QUERY TESTS
// =============================================================================

func TestPagination_ApplyToQuery(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Get total count first
	var totalCount int
	err := db.NewSelect().
		Model((*auth.Account)(nil)).
		ModelTableExpr(accountTableAlias).
		ColumnExpr("COUNT(*)").
		Scan(ctx, &totalCount)
	require.NoError(t, err)

	if totalCount < 2 {
		t.Skip("Need at least 2 records to test pagination")
	}

	// Test page 1 with size 1
	pagination := base.NewPagination(1, 1)

	var page1Records []*auth.Account
	query := db.NewSelect().
		Model(&page1Records).
		ModelTableExpr(accountTableAlias).
		Order("id ASC")

	query = pagination.ApplyToQuery(query)

	err = query.Scan(ctx)
	require.NoError(t, err)
	assert.Len(t, page1Records, 1, "Page 1 should have 1 record")

	// Test page 2 with size 1
	pagination2 := base.NewPagination(2, 1)

	var page2Records []*auth.Account
	query2 := db.NewSelect().
		Model(&page2Records).
		ModelTableExpr(accountTableAlias).
		Order("id ASC")

	query2 = pagination2.ApplyToQuery(query2)

	err = query2.Scan(ctx)
	require.NoError(t, err)
	assert.Len(t, page2Records, 1, "Page 2 should have 1 record")

	// Records should be different
	if len(page1Records) > 0 && len(page2Records) > 0 {
		assert.NotEqual(t, page1Records[0].ID, page2Records[0].ID, "Different pages should have different records")
	}
}

func TestPagination_ApplyToQuery_LargePageSize(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Test with large page size
	pagination := base.NewPagination(1, 1000)

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = pagination.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)
	// Should work without error, returning all available records
}

// =============================================================================
// SORTING APPLY TO QUERY TESTS
// Note: Sorting.ApplyToQuery uses bun.Ident which works with BUN's model alias
// =============================================================================

func TestSorting_ApplyToQuery_Ascending(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	sorting := &base.Sorting{}
	sorting.AddField("id", base.SortAsc)

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = sorting.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)

	// Verify records are sorted ascending by ID
	for i := 1; i < len(records); i++ {
		assert.GreaterOrEqual(t, records[i].ID, records[i-1].ID, "Records should be sorted ascending by ID")
	}
}

func TestSorting_ApplyToQuery_Descending(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	sorting := &base.Sorting{}
	sorting.AddField("id", base.SortDesc)

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = sorting.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)

	// Verify records are sorted descending by ID
	for i := 1; i < len(records); i++ {
		assert.LessOrEqual(t, records[i].ID, records[i-1].ID, "Records should be sorted descending by ID")
	}
}

func TestSorting_ApplyToQuery_MultipleFields(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	sorting := &base.Sorting{}
	sorting.AddField("active", base.SortDesc).AddField("id", base.SortAsc)

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = sorting.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)
	// Query should execute without error - multi-field sorting works
}

// =============================================================================
// QUERY OPTIONS APPLY TO QUERY TESTS
// =============================================================================

func TestQueryOptions_ApplyToQuery_Full(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	sorting := base.Sorting{}
	sorting.AddField("id", base.SortAsc)

	qo := base.NewQueryOptions().
		WithPagination(1, 10).
		WithSorting(sorting)
	qo.Filter.WithTableAlias("account").Equal("active", true)

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = qo.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)

	// Should have at most 10 records (pagination limit)
	assert.LessOrEqual(t, len(records), 10, "Pagination should limit results")

	// All records should be active
	for _, r := range records {
		assert.True(t, r.Active, "Filter should return active records only")
	}

	// Records should be sorted ascending
	for i := 1; i < len(records); i++ {
		assert.GreaterOrEqual(t, records[i].ID, records[i-1].ID, "Records should be sorted ascending")
	}
}

func TestQueryOptions_ApplyToQuery_FilterOnly(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	qo := base.NewQueryOptions()
	qo.Filter.WithTableAlias("account").Equal("active", true)

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = qo.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)

	for _, r := range records {
		assert.True(t, r.Active, "Filter should return active records only")
	}
}

func TestQueryOptions_ApplyToQuery_Empty(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	qo := base.NewQueryOptions()

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = qo.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)
	// Empty options should not affect the query - returns all records
}

// =============================================================================
// LOGICAL OPERATORS TESTS
// =============================================================================

func TestFilter_ApplyToQuery_OrCondition(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Create main filter
	mainFilter := base.NewFilter().WithTableAlias("account").Equal("active", true)

	// Create OR filter
	orFilter := base.Filter{}
	orFilter.WithTableAlias("account").Equal("active", false)
	mainFilter.Or(orFilter)

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = mainFilter.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)
	// Should return all records (active=true OR active=false covers all)
}

func TestFilter_ApplyToQuery_AndCondition(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	mainFilter := base.NewFilter().WithTableAlias("account").Equal("active", true)

	andFilter := base.Filter{}
	andFilter.WithTableAlias("account").IsNotNull("email")
	mainFilter.And(andFilter)

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = mainFilter.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)

	for _, r := range records {
		assert.True(t, r.Active, "Should be active")
		assert.NotEmpty(t, r.Email, "Email should not be null")
	}
}
