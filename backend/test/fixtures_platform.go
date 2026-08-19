package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// CreateTestOperator inserts a platform operator with a unique email and
// registers cleanup via tb.Cleanup. Sites that need an int64 use the ID field.
func CreateTestOperator(tb testing.TB, db *bun.DB) *platform.Operator {
	tb.Helper()
	return CreateTestOperatorWithEmail(tb, db,
		fmt.Sprintf("op-%d@test.local", time.Now().UnixNano()), "Test Operator")
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

	tb.Cleanup(func() { CleanupOperator(tb, db, op.ID) })

	return op
}

// CleanupOperator removes an operator and its audit-log rows. All other
// operator-scoped tables (tokens, MFA, passkeys) cascade on delete; rows in
// domain tables referencing the operator (announcements)
// must be removed by the caller's own cleanup first.
func CleanupOperator(tb testing.TB, db *bun.DB, operatorID int64) {
	tb.Helper()
	ctx := context.Background()

	_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_audit_log WHERE operator_id = ?`, operatorID)
	_, _ = db.ExecContext(ctx, `DELETE FROM platform.operators WHERE id = ?`, operatorID)
}
