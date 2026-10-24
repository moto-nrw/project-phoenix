package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	fileStorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type photoCleanupRuntime struct {
	enqueue func(context.Context, string) error
}

func (r photoCleanupRuntime) PhotoFeatureEnabled(context.Context) (bool, error) { return true, nil }
func (r photoCleanupRuntime) ActingAccountID(context.Context) int64             { return 0 }
func (r photoCleanupRuntime) QueueUnlinkStoredPhoto(ctx context.Context, url string) error {
	return r.enqueue(ctx, url)
}
func (photoCleanupRuntime) UnlinkStoredPhoto(string)                  {} // simulate death before the optional fast path
func (photoCleanupRuntime) BroadcastPhotoChange(int64, int64, string) {}
func (photoCleanupRuntime) RecordPhotoConsent(context.Context, peopleCompose.StudentPhotoConsentSnapshot, peopleCompose.StudentPhotoConsentSnapshot, *int64, time.Time) error {
	return nil
}

func TestPhotoCleanupOutboxSharesEveryDetachingTransaction(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"replace", "clear", "consent", "purge", "unlink"} {
		t.Run(action, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
			cleanup := fileStorageCompose.NewUploadCleanup(db, "student-photos")
			photos, err := peopleCompose.New(peopleCompose.Dependencies{DB: db, StudentDirectory: testpkg.StudentDirectory{}, Observe: func(peopleCompose.Observation) {}, StudentPhotos: photoCleanupRuntime{enqueue: cleanup.Queue}}, nil)
			require.NoError(t, err)
			student := testpkg.CreateTestStudent(t, db, "Atomic", "Photo", "1a")
			oldURL := "/uploads/student-photos/old-photo.jpg"
			consent := time.Now()
			_, err = db.NewRaw(`UPDATE users.student_profiles SET photo_path = ?, photo_consent_given_at = ? WHERE id = ? AND tenant_id = ?`, oldURL, consent, student.ID, testpkg.Tenant(t)).Exec(ctx)
			require.NoError(t, err)
			count := func(source bun.IDB, queryCtx context.Context, want int) {
				t.Helper()
				n, err := source.NewSelect().TableExpr("documents.file_cleanup").Where("tenant_id = ? AND object_kind = ?", testpkg.Tenant(t), "student-photos").Count(queryCtx)
				require.NoError(t, err)
				assert.EqualValues(t, want, n)
			}
			rollback := errors.New("rollback photo change")
			trigger := func(commit bool) error {
				return testpkg.WithinTenantTransaction(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
					var err error
					switch action {
					case "replace":
						err = photos.CommitStudentPhoto(txCtx, student.ID, "/uploads/student-photos/new-photo.jpg", false)
					case "clear":
						_, err = photos.ClearStudentPhoto(txCtx, student.ID)
					case "consent":
						granting := false
						var updated peopledirectory.StudentPhotoState
						updated, err = photos.ApplyStudentPhotoConsent(txCtx, peopledirectory.StudentPhotoState{StudentID: student.ID, PhotoPath: &oldURL, PhotoConsentGivenAt: &consent}, &granting)
						if err == nil {
							_, err = tx.NewRaw(`UPDATE users.student_profiles SET photo_path = ?, photo_consent_given_at = ? WHERE id = ? AND tenant_id = ?`, updated.PhotoPath, updated.PhotoConsentGivenAt, student.ID, testpkg.Tenant(t)).Exec(txCtx)
						}
					case "purge":
						_, err = photos.PurgeStudentPhotos(txCtx)
					case "unlink":
						_, err = tx.NewRaw(`UPDATE users.student_profiles SET photo_path = NULL WHERE id = ? AND tenant_id = ?`, student.ID, testpkg.Tenant(t)).Exec(txCtx)
						if err == nil {
							err = photos.ScheduleStudentPhotoUnlink(txCtx, oldURL)
						}
					}
					if err != nil {
						return err
					}
					count(tx, txCtx, 1)
					if !commit {
						return rollback
					}
					return nil
				})
			}
			require.ErrorIs(t, trigger(false), rollback)
			count(db, ctx, 0)
			require.NoError(t, trigger(true))
			count(db, ctx, 1)
		})
	}
}

func TestPhotoCleanupWorkerRecoversCommittedIntentAndRetriesRemovalFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	cleanup := fileStorageCompose.NewUploadCleanup(db, "student-photos")
	publicDir := t.TempDir()
	photoDir := filepath.Join(publicDir, "uploads", "student-photos")
	require.NoError(t, os.MkdirAll(photoDir, 0700))
	photoPath := filepath.Join(photoDir, "recover.jpg")
	require.NoError(t, os.WriteFile(photoPath, []byte("photo fixture"), 0600))
	rollback := errors.New("rollback removal intent")
	err := testpkg.WithinTenantTransaction(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		require.NoError(t, cleanup.Queue(txCtx, "/uploads/student-photos/recover.jpg"))
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	removed, err := cleanup.Sweep(ctx, func(context.Context, string) error { t.Fatal("rolled-back cleanup reached filesystem"); return nil })
	require.NoError(t, err)
	assert.Zero(t, removed)
	require.NoError(t, testpkg.WithinTenantTransaction(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		return cleanup.Queue(txCtx, "/uploads/student-photos/recover.jpg")
	}))
	_, err = os.Stat(photoPath)
	require.NoError(t, err, "process death leaves committed work for the worker")
	refusal := errors.New("filesystem temporarily unavailable")
	removed, err = cleanup.Sweep(ctx, func(context.Context, string) error { return refusal })
	require.ErrorIs(t, err, refusal)
	assert.Zero(t, removed)
	remover := peopledirectory.NewPhotoUnlinker(nil, publicDir, nativeUploads.UploadFiles{})
	removed, err = cleanup.Sweep(ctx, remover.RemoveStored)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	_, err = os.Stat(photoPath)
	require.True(t, os.IsNotExist(err))
	removed, err = cleanup.Sweep(ctx, remover.RemoveStored)
	require.NoError(t, err)
	assert.Zero(t, removed, "a completed cleanup is not delivered again")
}

func TestPhotoCleanupNamespaceCannotConsumeFolderIntents(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	cleanup := fileStorageCompose.NewUploadCleanup(db, "student-photos")
	require.NoError(t, testpkg.WithinTenantTransaction(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		_, err := tx.NewRaw(`INSERT INTO documents.file_cleanup (tenant_id,owner_id,filename_stored,retry_after) VALUES (?, ?, ?, NOW())`, testpkg.Tenant(t), testpkg.Tenant(t), "shared-name.jpg").Exec(txCtx)
		if err != nil {
			return err
		}
		return cleanup.Queue(txCtx, "/uploads/student-photos/shared-name.jpg")
	}))
	removed, err := cleanup.Sweep(ctx, func(_ context.Context, url string) error {
		assert.Equal(t, "/uploads/student-photos/shared-name.jpg", url)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	var pending int64
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM documents.file_cleanup WHERE tenant_id = ? AND object_kind = 'files' AND cleaned_at IS NULL`, testpkg.Tenant(t)).Scan(ctx, &pending))
	assert.EqualValues(t, 1, pending, "the folder namespace remains pending for its own sweeper")
}

func TestLegalDocumentCleanupOutboxIsAtomicWithSetting(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	module, _, err := NewSettingsCallbacksTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), nil)
	require.NoError(t, err)
	actor := testpkg.CreateTestAccount(t, db, "legal-cleanup-actor")
	tenantID := testpkg.Tenant(t)
	oldURL := fmt.Sprintf("/uploads/enrollment-legal-documents/%d_old.pdf", tenantID)
	newURL := fmt.Sprintf("/uploads/enrollment-legal-documents/%d_new.pdf", tenantID)
	cleanup := fileStorageCompose.NewUploadCleanup(db, "enrollment-legal-documents")
	enqueue := func(txCtx context.Context, _ int64, oldURL, newURL string) (func(), error) {
		if oldURL == "" || oldURL == newURL {
			return nil, nil
		}
		return nil, cleanup.Queue(txCtx, oldURL)
	}
	require.NoError(t, module.TenantSettings.SetLegalDocument(ctx, tenantID, actor.ID, []string{"config:manage"}, oldURL, enqueue))
	rollback := errors.New("rollback managed legal document setting")
	trigger := func(commit bool) error {
		return testpkg.WithinTenantTransaction(ctx, db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
			if err := module.TenantSettings.SetLegalDocument(txCtx, tenantID, actor.ID, []string{"config:manage"}, newURL, enqueue); err != nil {
				return err
			}
			n, err := tx.NewSelect().TableExpr("documents.file_cleanup").Where("tenant_id = ? AND object_kind = ?", tenantID, "enrollment-legal-documents").Count(txCtx)
			require.NoError(t, err)
			assert.EqualValues(t, 1, n)
			if !commit {
				return rollback
			}
			return nil
		})
	}
	require.ErrorIs(t, trigger(false), rollback)
	n, err := db.NewSelect().TableExpr("documents.file_cleanup").Where("tenant_id = ? AND object_kind = ?", tenantID, "enrollment-legal-documents").Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, n)
	require.NoError(t, trigger(true))
	n, err = db.NewSelect().TableExpr("documents.file_cleanup").Where("tenant_id = ? AND object_kind = ?", tenantID, "enrollment-legal-documents").Count(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
}
