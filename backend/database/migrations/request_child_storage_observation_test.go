package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestChildStorageObservationSQLDetectsCompatibilityDrift(t *testing.T) {
	t.Parallel()
	db := setupRequestChildStorageBeforeCutover(t)
	fixture := createRequestChildStorageFixture(t, db)
	runBackfill(t, db, RequestChildStorageBackfillOptions{})
	require.NoError(t, finalizeRequestChildStorage(t.Context(), db, installRequestChildStorageCompatibility))
	script, err := os.ReadFile("../../../docs/operations/enrollment-storage-cutover-2714.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), string(script))
	require.NoError(t, err, "the complete operator script must execute against the cutover schema")
	_, query, found := strings.Cut(string(script), "WITH rows AS (")
	require.True(t, found)
	query, _, found = strings.Cut(query, "\nSELECT clock_timestamp() AS sampled_at, datname")
	require.True(t, found)
	query = "WITH rows AS (" + query
	readEqual := func() bool {
		var rows []struct {
			TenantID                int64
			TargetRows              int64
			CompatibilityRows       int64
			TargetChecksum          string
			CompatibilityChecksum   string
			EffectiveChecksumsEqual bool
		}
		require.NoError(t, db.NewRaw(query).Scan(t.Context(), &rows))
		for _, row := range rows {
			if row.TenantID == fixture.tenant {
				return row.EffectiveChecksumsEqual
			}
		}
		t.Fatal("observation omitted the fixture tenant")
		return false
	}
	require.True(t, readEqual())
	_, err = db.NewRaw(`UPDATE enrollment.request_child_offerings_legacy SET selected_days = '["sun"]'::jsonb WHERE id = ?`, fixture.legacyIDs[0]).Exec(t.Context())
	require.NoError(t, err)
	require.False(t, readEqual(), "drift in rollback-visible effective days must be detected")
}
