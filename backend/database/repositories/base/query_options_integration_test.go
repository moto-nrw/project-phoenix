package base

import (
	"context"
	"fmt"
	"testing"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// accountTableAlias is the schema-qualified table expression for auth.accounts
const accountTableAlias = `auth.accounts AS "account"`

type queryAccountTable struct{} //nolint:unused // BUN consumes this table marker through reflection.

type account struct {
	//nolint:unused // BUN consumes this table metadata through reflection.
	queryAccountTable `bun:"table:auth.accounts,alias:account"`
	ID                int64      `bun:"id"`
	Email             string     `bun:"email"`
	Active            bool       `bun:"active"`
	LastLogin         *time.Time `bun:"last_login"`
}

// =============================================================================
// FILTER APPLY TO QUERY TESTS
// =============================================================================

func TestFilter_ApplyToQuery_Equal(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	// Create filter with Equal condition using table alias
	filter := modelBase.NewFilter().WithTableAlias("account").Equal("active", true)

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplyFilter(query, filter)

	err := query.Scan(ctx)
	require.NoError(t, err)

	// All returned records should have active=true
	for _, r := range records {
		assert.True(t, r.Active, "Filter should only return active records")
	}
}

func TestFilter_ApplyToQuery_ILike(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	filter := modelBase.NewFilter().WithTableAlias("account").ILike("email", "%@example.com")

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplyFilter(query, filter)

	err := query.Scan(ctx)
	require.NoError(t, err)

	// All returned records should have emails ending with @example.com
	for _, r := range records {
		assert.Contains(t, r.Email, "@example.com", "Filter should match email pattern")
	}
}

func TestFilter_ApplyToQuery_IsNull(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	filter := modelBase.NewFilter().WithTableAlias("account").IsNull("last_login")

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplyFilter(query, filter)

	err := query.Scan(ctx)
	require.NoError(t, err)

	// All returned records should have NULL last_login
	for _, r := range records {
		assert.Nil(t, r.LastLogin, "Filter should only return records with NULL last_login")
	}
}

func TestFilter_ApplyToQuery_IsNotNull(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	filter := modelBase.NewFilter().WithTableAlias("account").IsNotNull("email")

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplyFilter(query, filter)

	err := query.Scan(ctx)
	require.NoError(t, err)

	// All returned records should have non-NULL email
	for _, r := range records {
		assert.NotEmpty(t, r.Email, "Filter should only return records with non-NULL email")
	}
}

func TestFilter_ApplyToQuery_In(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	filter := modelBase.NewFilter().WithTableAlias("account").In("active", true, false)

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplyFilter(query, filter)

	err := query.Scan(ctx)
	require.NoError(t, err)
	// Query should execute without error - IN clause with both values should return all records
}

func TestFilter_ApplyToQuery_WithTableAlias(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	// Use filter with explicit table alias
	filter := modelBase.NewFilter().
		WithTableAlias("account").
		Equal("active", true)

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplyFilter(query, filter)

	err := query.Scan(ctx)
	require.NoError(t, err)

	for _, r := range records {
		assert.True(t, r.Active, "Filter with alias should work correctly")
	}
}

func TestFilter_ApplyToQuery_MultipleConditions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	filter := modelBase.NewFilter().
		WithTableAlias("account").
		Equal("active", true).
		ILike("email", "%@example.com")

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplyFilter(query, filter)

	err := query.Scan(ctx)
	require.NoError(t, err)

	for _, r := range records {
		assert.True(t, r.Active, "Filter should return active records")
		assert.Contains(t, r.Email, "@example.com", "Filter should match email pattern")
	}
}

func TestFilter_ApplyToQuery_Comparisons(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	// Test GreaterThan on id field
	filter := modelBase.NewFilter().WithTableAlias("account").GreaterThan("id", 0)

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplyFilter(query, filter)

	err := query.Scan(ctx)
	require.NoError(t, err)

	for _, r := range records {
		assert.Greater(t, r.ID, int64(0), "Filter should return records with id > 0")
	}
}

func TestFilter_ApplyToQuery_LessThan(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	filter := modelBase.NewFilter().WithTableAlias("account").LessThan("id", 999999)

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplyFilter(query, filter)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	// Create fixture accounts to guarantee at least 2 records exist
	testpkg.CreateTestAccount(t, db, "pagination-test-1")
	testpkg.CreateTestAccount(t, db, "pagination-test-2")

	// Test page 1 with size 1
	pagination := modelBase.NewPagination(1, 1)

	var page1Records []*account
	query := db.NewSelect().
		Model(&page1Records).
		ModelTableExpr(accountTableAlias).
		Order("id ASC")

	query = ApplyPagination(query, pagination)

	err := query.Scan(ctx)
	require.NoError(t, err)
	assert.Len(t, page1Records, 1, "Page 1 should have 1 record")

	// Test page 2 with size 1
	pagination2 := modelBase.NewPagination(2, 1)

	var page2Records []*account
	query2 := db.NewSelect().
		Model(&page2Records).
		ModelTableExpr(accountTableAlias).
		Order("id ASC")

	query2 = ApplyPagination(query2, pagination2)

	err = query2.Scan(ctx)
	require.NoError(t, err)
	assert.Len(t, page2Records, 1, "Page 2 should have 1 record")

	// Records should be different
	if len(page1Records) > 0 && len(page2Records) > 0 {
		assert.NotEqual(t, page1Records[0].ID, page2Records[0].ID, "Different pages should have different records")
	}
}

func TestPagination_ApplyToQuery_LargePageSize(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	// Test with large page size
	pagination := modelBase.NewPagination(1, 1000)

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplyPagination(query, pagination)

	err := query.Scan(ctx)
	require.NoError(t, err)
	// Should work without error, returning all available records
}

// =============================================================================
// SORTING APPLY TO QUERY TESTS
// Note: Sorting.ApplyToQuery uses bun.Ident which works with BUN's model alias
// =============================================================================

func TestSorting_ApplyToQuery_Ascending(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	sorting := &modelBase.Sorting{}
	sorting.AddField("id", modelBase.SortAsc)

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplySorting(query, *sorting)

	err := query.Scan(ctx)
	require.NoError(t, err)

	// Verify records are sorted ascending by ID
	for i := 1; i < len(records); i++ {
		assert.GreaterOrEqual(t, records[i].ID, records[i-1].ID, "Records should be sorted ascending by ID")
	}
}

func TestSorting_ApplyToQuery_Descending(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	sorting := &modelBase.Sorting{}
	sorting.AddField("id", modelBase.SortDesc)

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplySorting(query, *sorting)

	err := query.Scan(ctx)
	require.NoError(t, err)

	// Verify records are sorted descending by ID
	for i := 1; i < len(records); i++ {
		assert.LessOrEqual(t, records[i].ID, records[i-1].ID, "Records should be sorted descending by ID")
	}
}

func TestSorting_ApplyToQuery_MultipleFields(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	sorting := &modelBase.Sorting{}
	sorting.AddField("active", modelBase.SortDesc).AddField("id", modelBase.SortAsc)

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplySorting(query, *sorting)

	err := query.Scan(ctx)
	require.NoError(t, err)
	// Query should execute without error - multi-field sorting works
}

// =============================================================================
// QUERY OPTIONS APPLY TO QUERY TESTS
// =============================================================================

func TestQueryOptions_ApplyToQuery_FilterOnly(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	qo := modelBase.NewQueryOptions()
	qo.Filter.WithTableAlias("account").Equal("active", true)

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplyQueryOptions(query, qo)

	err := query.Scan(ctx)
	require.NoError(t, err)

	for _, r := range records {
		assert.True(t, r.Active, "Filter should return active records only")
	}
}

func TestQueryOptions_ApplyToQuery_Empty(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	qo := modelBase.NewQueryOptions()

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplyQueryOptions(query, qo)

	err := query.Scan(ctx)
	require.NoError(t, err)
	// Empty options should not affect the query - returns all records
}

// =============================================================================
// LOGICAL OPERATORS TESTS
// =============================================================================

func TestFilter_ApplyToQuery_OrCondition(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	// Create main filter
	mainFilter := modelBase.NewFilter().WithTableAlias("account").Equal("active", true)

	// Create OR filter
	orFilter := modelBase.Filter{}
	orFilter.WithTableAlias("account").Equal("active", false)
	mainFilter.Or(orFilter)

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplyFilter(query, mainFilter)

	err := query.Scan(ctx)
	require.NoError(t, err)
	// Should return all records (active=true OR active=false covers all)
}

func TestFilter_ApplyToQuery_MixedOrAndKeepsExpressionGrouped(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	type logicalRow struct {
		Active bool   `bun:"active"`
		Name   string `bun:"name"`
	}

	filter := modelBase.NewFilter().WithTableAlias("item").Equal("active", true)
	filter.Or(*modelBase.NewFilter().Equal("name", "drop"))
	filter.And(*modelBase.NewFilter().Equal("name", "drop"))

	var rows []logicalRow
	query := db.NewSelect().
		TableExpr(`(VALUES (TRUE, 'keep'), (FALSE, 'drop')) AS "item"("active", "name")`).
		ColumnExpr(`"item"."active"`).
		ColumnExpr(`"item"."name"`)
	query = ApplyFilter(query, filter)

	require.NoError(t, query.Scan(context.Background(), &rows))
	require.Len(t, rows, 1, "(A OR B) AND C must not degrade to A OR (B AND C)")
	assert.Equal(t, "drop", rows[0].Name)
}

func TestFilter_ApplyToQuery_Like(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	filter := modelBase.NewFilter().WithTableAlias("account").Like("email", "%@%")

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplyFilter(query, filter)

	err := query.Scan(ctx)
	require.NoError(t, err)

	for _, r := range records {
		assert.Contains(t, r.Email, "@", "Email should contain @")
	}
}

func TestFilter_ApplyToQuery_GreaterThanOrEqual(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	filter := modelBase.NewFilter().WithTableAlias("account").GreaterThanOrEqual("id", 1)

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplyFilter(query, filter)

	err := query.Scan(ctx)
	require.NoError(t, err)

	for _, r := range records {
		assert.GreaterOrEqual(t, r.ID, int64(1))
	}
}

func TestFilter_ApplyToQuery_LessThanOrEqual(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	filter := modelBase.NewFilter().WithTableAlias("account").LessThanOrEqual("id", 999999)

	var records []*account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = ApplyFilter(query, filter)

	err := query.Scan(ctx)
	require.NoError(t, err)

	for _, r := range records {
		assert.LessOrEqual(t, r.ID, int64(999999))
	}
}

func TestFilter_ApplyToQuery_FirstNumberIn(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	third := testpkg.CreateTestStudent(t, db, "Grade", "Third", "3a-"+suffix)
	thirteenth := testpkg.CreateTestStudent(t, db, "Grade", "Thirteenth", "13a-"+suffix)
	labelled := testpkg.CreateTestStudent(t, db, "Grade", "Labelled", "Klasse 3b-"+suffix)
	named := testpkg.CreateTestStudent(t, db, "Grade", "Named", "Bienen-"+suffix)

	assertGradeThree := func(t *testing.T, ids []int64) {
		t.Helper()
		assert.Contains(t, ids, third.ID)
		assert.Contains(t, ids, labelled.ID, `"Klasse 3b" is a third-graders' class too`)
		assert.NotContains(t, ids, thirteenth.ID, "grade 13 must not be read as grade 1 or 3")
		assert.NotContains(t, ids, named.ID, "a class without a number is in no year")
	}

	t.Run("aliased column", func(t *testing.T) {
		query := db.NewSelect().
			ColumnExpr(`"student".id`).
			TableExpr(`users.students AS "student"`)
		filter := modelBase.NewFilter().
			WithTableAlias("student").
			FirstNumberIn("school_class", "3")
		query = ApplyFilter(query, filter)

		var ids []int64
		require.NoError(t, query.Scan(ctx, &ids))
		assertGradeThree(t, ids)
	})

	t.Run("plain column identifier", func(t *testing.T) {
		query := db.NewSelect().
			ColumnExpr("id").
			TableExpr("users.students")
		filter := modelBase.NewFilter().FirstNumberIn("school_class", "3")
		query = ApplyFilter(query, filter)

		var ids []int64
		require.NoError(t, query.Scan(ctx, &ids))
		assertGradeThree(t, ids)
	})

	t.Run("several years", func(t *testing.T) {
		query := db.NewSelect().
			ColumnExpr(`"student".id`).
			TableExpr(`users.students AS "student"`)
		filter := modelBase.NewFilter().
			WithTableAlias("student").
			FirstNumberIn("school_class", "3", "13")
		query = ApplyFilter(query, filter)

		var ids []int64
		require.NoError(t, query.Scan(ctx, &ids))
		assert.Contains(t, ids, third.ID)
		assert.Contains(t, ids, thirteenth.ID)
		assert.NotContains(t, ids, named.ID)
	})
}

func TestFilter_ApplyToQuery_TrimOperatorsUseDatabaseNormalization(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	first := testpkg.CreateTestStudent(t, db, "Trim", "First", "  MiXeD-"+suffix+"  ")
	second := testpkg.CreateTestStudent(t, db, "Trim", "Second", "Other-"+suffix)

	selectIDs := func(filter *modelBase.Filter) []int64 {
		t.Helper()
		query := db.NewSelect().ColumnExpr("id").TableExpr("users.students")
		var ids []int64
		require.NoError(t, ApplyFilter(query, filter).Scan(ctx, &ids))
		return ids
	}

	assert.Contains(t, selectIDs(modelBase.NewFilter().TrimEqual("school_class", "mixed-"+suffix)), first.ID)
	ids := selectIDs(modelBase.NewFilter().TrimIn("school_class", "absent", " OTHER-"+suffix+" "))
	assert.Contains(t, ids, second.ID)
	assert.NotContains(t, ids, first.ID)
}
