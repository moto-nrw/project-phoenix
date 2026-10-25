package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TokenHash is the value Identity & Access stores for a session, invitation,
// password-reset or e-mail-change token: its hex SHA-256 (#4902). PostgreSQL
// computes it, independently of the Go code under test. Fixtures write it
// and return the raw token, the way a link or refresh JWT carries it; a test
// that reads a token column back compares against TokenHash.
func TokenHash(tb testing.TB, db bun.IDB, raw string) string {
	tb.Helper()
	var hash string
	require.NoError(tb, db.NewRaw(`SELECT encode(sha256(convert_to(?, 'UTF8')), 'hex')`, raw).Scan(context.Background(), &hash))
	return hash
}
