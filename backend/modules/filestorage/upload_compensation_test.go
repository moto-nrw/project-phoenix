package filestorage_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/filestorage"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The object port alone injects storage failures; all metadata, transactions
// and cleanup intents use the real module and PostgreSQL.
type compensationObjects struct {
	content    []byte
	storedName string
	kind       string
	tenantID   int64
	removes    int
	failRemove bool
}

func (o *compensationObjects) SavePrivate(_ context.Context, kind string, tenantID int64, name string, source io.Reader) (int64, error) {
	var err error
	o.content, err = io.ReadAll(source)
	o.storedName = name
	o.kind, o.tenantID = kind, tenantID
	return int64(len(o.content)), err
}
func (*compensationObjects) OpenPrivate(context.Context, string, int64, string) (interface {
	io.ReadSeekCloser
	ModTime() time.Time
}, error) {
	return nil, errors.New("unexpected object open")
}
func (o *compensationObjects) RemovePrivate(_ context.Context, kind string, tenantID int64, name string) error {
	o.removes++
	if o.failRemove {
		return errors.New("object store unavailable")
	}
	if name != o.storedName || kind != o.kind || tenantID != o.tenantID {
		return errors.New("wrong cleanup key")
	}
	o.content = nil
	return nil
}

func TestUploadDatabaseFailureKeepsIntentUntilObjectCleanupSucceeds(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	objects := &compensationObjects{}
	module, _, err := api.NewFileStoreTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), objects)
	require.NoError(t, err)
	_, account := testpkg.CreateTestPersonWithAccount(t, db, "File", "Manager")
	actor := filestorage.Actor{AccountID: account.ID, Permissions: []string{"files:manage"}}
	folder, err := module.CreateFolder(ctx, filestorage.FolderInput{Name: "Compensation", Visibility: "all_staff"}, actor)
	require.NoError(t, err)
	require.Positive(t, folder.ID)
	_, err = db.ExecContext(ctx, `ALTER TABLE documents.files ADD CONSTRAINT refuse_upload_metadata CHECK (filename_display <> 'refused.pdf')`)
	require.NoError(t, err)
	payload := []byte("%PDF-1.4 compensation")
	_, err = module.UploadFile(ctx, folder.ID, filestorage.Upload{Filename: "refused.pdf", ContentType: "application/pdf", Extension: ".pdf", Content: bytes.NewReader(payload)}, actor)
	require.ErrorContains(t, err, "refuse_upload_metadata")
	require.Equal(t, payload, objects.content, "the object write preceded the real metadata failure")
	require.Equal(t, "files", objects.kind)
	require.Equal(t, testpkg.Tenant(t), objects.tenantID)
	require.Zero(t, objects.removes, "an uncertain database outcome must not eagerly delete bytes")
	list, err := module.ListFiles(ctx, folder.ID, actor)
	require.NoError(t, err)
	require.Empty(t, list.Files, "the metadata transaction rolled back")
	var pending int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM documents.file_cleanup WHERE tenant_id = ? AND owner_id = ? AND filename_stored = ? AND cleaned_at IS NULL`, testpkg.Tenant(t), folder.ID, objects.storedName).Scan(ctx, &pending))
	require.Equal(t, 1, pending, "the independently committed intent survives the metadata failure")
	// Make this test's intent eligible without waiting for the upload grace period.
	_, err = db.ExecContext(ctx, `UPDATE documents.file_cleanup SET retry_after = ? WHERE tenant_id = ? AND owner_id = ?`, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), testpkg.Tenant(t), folder.ID)
	require.NoError(t, err)
	sweep := func() (int, error) {
		removed := 0
		err := testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
			var err error
			removed, err = module.CleanupOrphanedFiles(txCtx)
			return err
		})
		return removed, err
	}
	objects.failRemove = true
	removed, err := sweep()
	require.ErrorContains(t, err, "object store unavailable")
	require.Zero(t, removed)
	require.Equal(t, payload, objects.content)
	objects.failRemove = false
	removed, err = sweep()
	require.NoError(t, err)
	require.Equal(t, 1, removed)
	require.Empty(t, objects.content)
	require.Equal(t, 2, objects.removes)
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		var completed bool
		require.NoError(t, tx.NewRaw(`SELECT cleaned_at IS NOT NULL FROM documents.file_cleanup WHERE tenant_id = ? AND filename_stored = ?`, testpkg.Tenant(t), objects.storedName).Scan(txCtx, &completed))
		require.True(t, completed)
		return nil
	}))
	removed, err = sweep()
	require.NoError(t, err)
	require.Zero(t, removed, "a completed intent is not retried")
	require.Equal(t, 2, objects.removes)
}
