package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestManagedUploadCleanupMigrationPreservesLegacyRowsAndRefusesLossyRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	require.NoError(t, photoCleanupOutboxDown(ctx, db))
	tenantID := testpkg.Tenant(t)
	_, err := db.NewRaw(`INSERT INTO documents.file_cleanup (tenant_id,owner_id,filename_stored,retry_after) VALUES (?,1,'shared.jpg',NOW())`, tenantID).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, photoCleanupOutboxUp(ctx, db))
	var kind string
	require.NoError(t, db.NewRaw(`SELECT object_kind FROM documents.file_cleanup WHERE tenant_id=? AND filename_stored='shared.jpg'`, tenantID).Scan(ctx, &kind))
	require.Equal(t, "files", kind)
	_, err = db.NewRaw(`INSERT INTO documents.file_cleanup (tenant_id,object_kind,owner_id,filename_stored,retry_after) VALUES (?,'student-photos',1,'shared.jpg',NOW())`, tenantID).Exec(ctx)
	require.NoError(t, err, "the same basename in distinct upload namespaces must not collide")
	require.ErrorContains(t, photoCleanupOutboxDown(ctx, db), "draining and purging")
	_, err = db.NewRaw(`DELETE FROM documents.file_cleanup WHERE tenant_id=? AND object_kind='student-photos'`, tenantID).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, photoCleanupOutboxDown(ctx, db))
	var count int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM documents.file_cleanup WHERE tenant_id=? AND filename_stored='shared.jpg'`, tenantID).Scan(ctx, &count))
	require.Equal(t, 1, count)
	require.NoError(t, photoCleanupOutboxUp(ctx, db))
}
