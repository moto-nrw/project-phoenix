package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TokenFixture is test-owned setup data for persisted refresh sessions. It
// deliberately supports expired tokens and unknown portal scopes. Token is
// the raw handle; the row stores TokenHash(Token).
type TokenFixture struct {
	ID          int64 `bun:"id,pk,autoincrement"`
	TenantID    int64
	AccountID   int64
	Token       string
	Expiry      time.Time
	Mobile      bool
	FamilyID    string
	Generation  int
	PortalScope string `bun:"portal_scope,notnull,default:'unknown'"`
}

// insertTokenFixture stores the session under TokenHash(token.Token) and
// leaves the raw handle in the fixture.
func insertTokenFixture(tb testing.TB, ctx context.Context, db *bun.DB, token *TokenFixture) {
	tb.Helper()
	raw := token.Token
	token.Token = TokenHash(tb, db, raw)
	err := db.NewInsert().
		Model(token).
		ModelTableExpr(`auth.tokens`).
		Scan(ctx)
	token.Token = raw
	require.NoError(tb, err, "Failed to create test token")
}
