package base_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// accountTableAlias is the schema-qualified table expression for auth.accounts
const accountTableAlias = `auth.accounts AS "account"`

// =============================================================================
// FILTER APPLY TO QUERY TESTS
// =============================================================================

func TestFilter_ApplyToQuery_Equal(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)

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

func TestFilter_ApplyToQuery_ILike(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)

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

	ctx := testpkg.TenantContext(1)

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

	ctx := testpkg.TenantContext(1)

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

	ctx := testpkg.TenantContext(1)

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

	ctx := testpkg.TenantContext(1)

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

	ctx := testpkg.TenantContext(1)

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

	ctx := testpkg.TenantContext(1)

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

	ctx := testpkg.TenantContext(1)

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

	ctx := testpkg.TenantContext(1)

	// Create fixture accounts to guarantee at least 2 records exist
	acct1 := testpkg.CreateTestAccount(t, db, "pagination-test-1")
	acct2 := testpkg.CreateTestAccount(t, db, "pagination-test-2")
	defer testpkg.CleanupAuthFixtures(t, db, acct1.ID)
	defer testpkg.CleanupAuthFixtures(t, db, acct2.ID)

	// Test page 1 with size 1
	pagination := base.NewPagination(1, 1)

	var page1Records []*auth.Account
	query := db.NewSelect().
		Model(&page1Records).
		ModelTableExpr(accountTableAlias).
		Order("id ASC")

	query = pagination.ApplyToQuery(query)

	err := query.Scan(ctx)
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

	ctx := testpkg.TenantContext(1)

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

	ctx := testpkg.TenantContext(1)

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

	ctx := testpkg.TenantContext(1)

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

	ctx := testpkg.TenantContext(1)

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

func TestQueryOptions_ApplyToQuery_FilterOnly(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)

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

	ctx := testpkg.TenantContext(1)

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

	ctx := testpkg.TenantContext(1)

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

func TestFilter_ApplyToQuery_MixedOrAndKeepsExpressionGrouped(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	type logicalRow struct {
		Active bool   `bun:"active"`
		Name   string `bun:"name"`
	}

	filter := base.NewFilter().WithTableAlias("item").Equal("active", true)
	filter.Or(*base.NewFilter().Equal("name", "drop"))
	filter.And(*base.NewFilter().Equal("name", "drop"))

	var rows []logicalRow
	query := db.NewSelect().
		TableExpr(`(VALUES (TRUE, 'keep'), (FALSE, 'drop')) AS "item"("active", "name")`).
		ColumnExpr(`"item"."active"`).
		ColumnExpr(`"item"."name"`)
	query = filter.ApplyToQuery(query)

	require.NoError(t, query.Scan(context.Background(), &rows))
	require.Len(t, rows, 1, "(A OR B) AND C must not degrade to A OR (B AND C)")
	assert.Equal(t, "drop", rows[0].Name)
}

func TestFilter_ApplyToQuery_Like(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)

	filter := base.NewFilter().WithTableAlias("account").Like("email", "%@%")

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = filter.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)

	for _, r := range records {
		assert.Contains(t, r.Email, "@", "Email should contain @")
	}
}

func TestFilter_ApplyToQuery_GreaterThanOrEqual(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)

	filter := base.NewFilter().WithTableAlias("account").GreaterThanOrEqual("id", 1)

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = filter.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)

	for _, r := range records {
		assert.GreaterOrEqual(t, r.ID, int64(1))
	}
}

func TestFilter_ApplyToQuery_LessThanOrEqual(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)

	filter := base.NewFilter().WithTableAlias("account").LessThanOrEqual("id", 999999)

	var records []*auth.Account
	query := db.NewSelect().
		Model(&records).
		ModelTableExpr(accountTableAlias)

	query = filter.ApplyToQuery(query)

	err := query.Scan(ctx)
	require.NoError(t, err)

	for _, r := range records {
		assert.LessOrEqual(t, r.ID, int64(999999))
	}
}

// =============================================================================
// TRANSACTION TESTS
// =============================================================================

func TestTxHandler_NewTxHandler(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	handler := base.NewTxHandler(db)
	require.NotNil(t, handler)
}

func TestTxHandler_RunInTx_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	handler := base.NewTxHandler(db)

	executed := false
	err := handler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		executed = true
		// Verify tx is usable - use schema-qualified table
		var count int
		err := tx.NewSelect().
			TableExpr("auth.accounts").
			ColumnExpr("COUNT(*)").
			Scan(ctx, &count)
		return err
	})

	require.NoError(t, err)
	assert.True(t, executed, "Transaction function should have been executed")
}

func TestTxHandler_RunInTx_Rollback(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	handler := base.NewTxHandler(db)

	expectedErr := errors.New("intentional error for rollback")
	err := handler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		return expectedErr
	})

	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestTxHandler_GetTx_NewTransaction(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	handler := base.NewTxHandler(db)

	tx, isNew, err := handler.GetTx(ctx)
	require.NoError(t, err)
	assert.True(t, isNew, "Should create a new transaction")

	// Clean up - rollback the transaction
	_ = tx.Rollback()
}

func TestTxHandler_RunInTx_ReusesContextTransaction(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	ctxWithTx := base.ContextWithTx(ctx, &tx)
	handler := base.NewTxHandler(db)

	err = handler.RunInTx(ctxWithTx, func(runCtx context.Context, runTx bun.Tx) error {
		contextTx, ok := base.TxFromContext(runCtx)
		require.True(t, ok)
		require.NotNil(t, contextTx)
		assert.Same(t, tx.Tx, contextTx.Tx)

		var count int
		return runTx.NewSelect().
			TableExpr("auth.accounts").
			ColumnExpr("COUNT(*)").
			Scan(runCtx, &count)
	})

	require.NoError(t, err)
}

func TestContextWithTx_NoTxInContext(t *testing.T) {
	// Test that TxFromContext returns false when no tx in context
	ctx := testpkg.TenantContext(1)
	tx, ok := base.TxFromContext(ctx)
	assert.False(t, ok, "Should return false when no tx in context")
	assert.Nil(t, tx)
}

func TestIsRetryableTxError(t *testing.T) {
	assert.False(t, base.IsRetryableTxError(nil), "nil is not retryable")
	assert.False(t, base.IsRetryableTxError(errors.New("some error")), "plain errors are not retryable")
	assert.False(t, base.IsRetryableTxError(fmt.Errorf("wrap: %w", errors.New("inner"))), "non-pg wrapped errors are not retryable")
}

func TestTxHandler_RunInTxWithRetry_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	handler := base.NewTxHandler(db)

	calls := 0
	err := handler.RunInTxWithRetry(ctx, func(ctx context.Context, _ bun.Tx) error {
		calls++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "a successful transaction runs exactly once")
}

func TestTxHandler_RunInTxWithRetry_NonRetryableRunsOnce(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	handler := base.NewTxHandler(db)

	expected := errors.New("business rule violation")
	calls := 0
	err := handler.RunInTxWithRetry(ctx, func(ctx context.Context, _ bun.Tx) error {
		calls++
		return expected
	})
	require.ErrorIs(t, err, expected)
	assert.Equal(t, 1, calls, "a non-retryable error must NOT be retried")
}
