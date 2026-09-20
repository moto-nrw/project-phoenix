package compose

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// enabledPhotoRuntime is the smallest runtime the purge needs: the feature is
// on, and nothing else about it is under test here.
type enabledPhotoRuntime struct{ unlinked []string }

func (r *enabledPhotoRuntime) PhotoFeatureEnabled(context.Context) (bool, error) { return true, nil }
func (r *enabledPhotoRuntime) ActingAccountID(context.Context) int64             { return 0 }
func (r *enabledPhotoRuntime) UnlinkStoredPhoto(storedURL string) {
	r.unlinked = append(r.unlinked, storedURL)
}
func (r *enabledPhotoRuntime) BroadcastPhotoChange(int64, int64, string) {}
func (r *enabledPhotoRuntime) RecordPhotoConsent(
	context.Context, StudentPhotoConsentSnapshot, StudentPhotoConsentSnapshot, *int64, time.Time,
) error {
	return nil
}

func buildPhotoModule(t *testing.T, db *bun.DB) (*peopledirectory.Module, *enabledPhotoRuntime) {
	t.Helper()
	runtime := &enabledPhotoRuntime{}
	module, err := New(Dependencies{
		DB:                  db,
		Observe:             func(Observation) {},
		StudentPhotoRuntime: func() StudentPhotoRuntime { return runtime },
	})
	require.NoError(t, err)
	return module, runtime
}

func setStudentPhotoPath(t *testing.T, db *bun.DB, studentID int64, path *string) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("users.student_profiles").
		Set("photo_path = ?", path).
		Where("id = ?", studentID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
}

func studentPhotoPath(t *testing.T, db *bun.DB, studentID int64) *string {
	t.Helper()
	var row struct {
		PhotoPath *string `bun:"photo_path"`
	}
	require.NoError(t, db.NewSelect().
		TableExpr("users.student_profiles").
		Column("photo_path").
		Where("id = ?", studentID).
		Scan(testpkg.Ctx(t), &row))
	return row.PhotoPath
}

// TestPurgeStudentPhotosReturnsOldURLsAndClearsTheColumn pins the contract the
// feature-disable path depends on: the purge returns the OLD photo_path values
// (the files the post-commit unlink has to remove) and clears the column on the
// same rows in one statement.
//
// The single statement is what closes the race: a SELECT-then-UPDATE captures a
// snapshot and then clears rows that were not in it, leaving a concurrently
// uploaded file orphaned on disk. A plain UPDATE … RETURNING returns the
// post-update values, which are always NULL here — so this also asserts the
// join that surfaces the pre-image is in place.
func TestPurgeStudentPhotosReturnsOldURLsAndClearsTheColumn(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, _ := buildPhotoModule(t, db)
	ctx := testpkg.Ctx(t)

	first := testpkg.CreateTestStudent(t, db, "Purge", "One", "1a")
	second := testpkg.CreateTestStudent(t, db, "Purge", "Two", "1a")
	// Left NULL, so the purge is also shown not to touch rows without a photo.
	third := testpkg.CreateTestStudent(t, db, "Purge", "Three", "1a")

	firstURL := "/uploads/student-photos/p1.jpg"
	secondURL := "/uploads/student-photos/p2.jpg"
	setStudentPhotoPath(t, db, first.ID, &firstURL)
	setStudentPhotoPath(t, db, second.ID, &secondURL)

	urls, err := module.PurgeStudentPhotos(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{firstURL, secondURL}, urls,
		"the purge must return the photo paths as they were before the clear")

	assert.Nil(t, studentPhotoPath(t, db, first.ID))
	assert.Nil(t, studentPhotoPath(t, db, second.ID))
	assert.Nil(t, studentPhotoPath(t, db, third.ID))
}

func TestPurgeStudentPhotosWithoutStoredPhotos(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, _ := buildPhotoModule(t, db)

	testpkg.CreateTestStudent(t, db, "Empty", "Purge", "1a")

	urls, err := module.PurgeStudentPhotos(testpkg.Ctx(t))
	require.NoError(t, err)
	assert.Empty(t, urls, "no stored photo means no file to unlink and no error")
}

// TestLockStudentPhotoFeatureIsVisibleToOtherTransactions verifies the
// per-tenant advisory lock is really observable elsewhere. Without that
// serialization the in-transaction feature recheck on the upload path still
// races a concurrent disable: the upload commits a fresh photo_path AFTER the
// disable's purge captured its rows, and the file is orphaned because the
// disable's post-commit unlinks never saw it.
//
// The probe uses pg_try_advisory_xact_lock on a second connection, so a lock
// that silently no-oped fails the assertion instead of deadlocking the test.
func TestLockStudentPhotoFeatureIsVisibleToOtherTransactions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, _ := buildPhotoModule(t, db)
	ctx := testpkg.Ctx(t)

	holding, err := db.BeginTx(ctx, nil)
	require.NoError(t, err, "begin the holding transaction")
	require.NoError(t, module.LockStudentPhotoFeature(tenant.WithTransactionForTest(ctx, holding)),
		"the first transaction must acquire the per-tenant photo gate")

	// The advisory-lock class of the photo gate. Spelled out rather than
	// imported: the probe must fail if the owner ever changes the key silently.
	const lockClass int32 = 0x70686F74

	probe, err := db.BeginTx(ctx, nil)
	require.NoError(t, err, "begin the probing transaction")
	var acquired bool
	require.NoError(t, probe.NewRaw(`SELECT pg_try_advisory_xact_lock(?, ?)`,
		lockClass, int32(testpkg.Tenant(t))).Scan(ctx, &acquired))
	assert.False(t, acquired, "a second transaction must not take the gate while the first holds it")
	require.NoError(t, probe.Rollback())

	require.NoError(t, holding.Rollback(), "releasing the gate")

	after, err := db.BeginTx(ctx, nil)
	require.NoError(t, err, "begin the second probing transaction")
	defer func() { _ = after.Rollback() }()
	require.NoError(t, after.NewRaw(`SELECT pg_try_advisory_xact_lock(?, ?)`,
		lockClass, int32(testpkg.Tenant(t))).Scan(ctx, &acquired))
	assert.True(t, acquired, "the gate is bound to the tenant, not the connection, and is free again")
}
