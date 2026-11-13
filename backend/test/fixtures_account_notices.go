package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// OwnAccountSecurityNotices registers teardown for the notice an
// administrator's reset of an account's second factor queues (#6862). It is
// a global identity mail without a school, so no tenant of the test owns it.
func OwnAccountSecurityNotices(tb testing.TB, db *bun.DB, accountID int64) {
	tb.Helper()
	tb.Cleanup(func() {
		_, err := db.NewDelete().
			Table("platform.email_outbox").
			Where("tenant_id IS NULL AND related_entity_type = ? AND related_entity_id = ?", "account_mfa_reset", accountID).
			Exec(context.Background())
		require.NoError(tb, err)
	})
}
