package test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// SetTestChildQuota books a Kinderkontingent of bundles × bundleSize children
// for the test's tenant (#3567). The operator UI writes the same two columns.
func SetTestChildQuota(tb testing.TB, db *bun.DB, bundles, bundleSize int) {
	tb.Helper()
	_, err := db.NewUpdate().TableExpr("platform.schools").
		Set("child_quota_bundles = ?", bundles).Set("child_quota_bundle_size = ?", bundleSize).
		Where("id = ?", fixtureTenantID(tb)).Exec(Ctx(tb))
	require.NoError(tb, err)
}
