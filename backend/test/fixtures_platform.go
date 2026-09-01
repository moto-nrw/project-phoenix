package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// CreateTestOperator inserts a platform operator with a unique email and
// registers cleanup via tb.Cleanup. Sites that need an int64 use the ID field.
func CreateTestOperator(tb testing.TB, db *bun.DB) *platform.Operator {
	tb.Helper()
	return CreateTestOperatorWithEmail(tb, db,
		fmt.Sprintf("op-%d@test.local", uniqueFixtureSuffix()), "Test Operator")
}

// CreateTestOperatorWithEmail inserts a platform operator with the given email
// and display name and registers cleanup via tb.Cleanup.
func CreateTestOperatorWithEmail(tb testing.TB, db *bun.DB, email, displayName string) *platform.Operator {
	tb.Helper()

	op := &platform.Operator{
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$placeholder$placeholder",
		Active:       true,
	}

	_, err := db.NewInsert().
		Model(op).
		ModelTableExpr("platform.operators").
		Returning("*").
		Exec(context.Background())
	require.NoError(tb, err, "Failed to create test operator")

	OwnTestOperator(tb, db, op.ID)

	return op
}

// CreateTestOrganization inserts an organization fixture and registers exact-ID cleanup.
func CreateTestOrganization(tb testing.TB, db *bun.DB, organization *platform.Organization) *platform.Organization {
	tb.Helper()
	require.NotNil(tb, organization)
	_, err := db.NewInsert().
		Model(organization).
		ModelTableExpr("platform.organizations").
		Returning("*").
		Exec(context.Background())
	require.NoError(tb, err, "Failed to create test organization")
	tb.Cleanup(func() {
		ctx := context.Background()
		var tenantIDs []int64
		cleanupErr := db.NewSelect().
			TableExpr("platform.schools").
			Column("id").
			Where("organization_id = ?", organization.ID).
			Scan(ctx, &tenantIDs)
		require.NoError(tb, cleanupErr, "Failed to find test organization schools")
		cleanupTenantTestData(tb, db, tenantIDs...)
		_, cleanupErr = db.NewDelete().
			Model((*platform.Organization)(nil)).
			ModelTableExpr("platform.organizations").
			Where("id = ?", organization.ID).
			Exec(ctx)
		require.NoError(tb, cleanupErr, "Failed to clean up test organization")
	})
	return organization
}

// OwnTestOperator registers exact-ID teardown for an operator created through
// a service or repository path.
func OwnTestOperator(tb testing.TB, db *bun.DB, operatorID int64) {
	tb.Helper()
	tb.Cleanup(func() { cleanupOperator(tb, db, operatorID) })
}

// cleanupOperator removes an operator and its audit-log rows. All other
// operator-scoped tables (tokens, MFA, passkeys) cascade on delete; rows in
// domain tables referencing the operator (announcements)
// must be removed by the caller's own cleanup first.
func cleanupOperator(tb testing.TB, db *bun.DB, operatorID int64) {
	tb.Helper()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(tb, err)
	defer func() { _ = tx.Rollback() }()

	// MFA audit writes run asynchronously. Lock the operator first so an audit
	// insert cannot land between deleting its logs and deleting the operator.
	_, err = tx.ExecContext(ctx, `SELECT id FROM platform.operators WHERE id = ? FOR UPDATE`, operatorID)
	require.NoError(tb, err)
	_, err = tx.ExecContext(ctx, `DELETE FROM platform.operator_audit_log WHERE operator_id = ?`, operatorID)
	require.NoError(tb, err)
	_, err = tx.ExecContext(ctx, `DELETE FROM platform.operators WHERE id = ?`, operatorID)
	require.NoError(tb, err)
	require.NoError(tb, tx.Commit())
}
