package contracttest_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The Care Plan store caps its document cleanup reads. These mirror the
// shared file-storage limits (documents.CleanupBatchSize and
// documents.RequestCleanupRetryLimit), which this test package may not import.
const (
	careDocumentCleanupBatchSize      = 200
	careDocumentRequestCleanupRetries = 10
)

func careDocumentRecords(t *testing.T, db *bun.DB) careplan.Capability {
	t.Helper()
	return repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CarePlan()
}

// newTestCareDocument builds a valid document row for a student.
func newTestCareDocument(studentID, accountID int64, category, display, stored string) careplan.CareDocument {
	return careplan.CareDocument{
		StudentID:       studentID,
		Category:        category,
		FilenameDisplay: display,
		FilenameStored:  stored,
		SizeBytes:       1234,
		ContentType:     "application/pdf",
		UploadedBy:      accountID,
	}
}

func careDocumentStoredName(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("%d-%s.pdf", time.Now().UnixNano(), t.Name())
}

// TestCareDocumentRecords_CreateAndList also proves the Care Plan document row
// maps onto users.student_documents: every shared column round-trips, so a
// broken mapping fails right here rather than in production.
func TestCareDocumentRecords_CreateAndList(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Doku", "Kind", "1a")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-%d@example.test", time.Now().UnixNano()))

	records := careDocumentRecords(t, db)
	ctx := testpkg.TenantContext(student.TenantID)

	doc, err := records.CreateCareDocument(ctx, newTestCareDocument(student.ID, account.ID, careplan.StudentDocumentCategoryAttest, "attest.pdf", careDocumentStoredName(t)))
	require.NoError(t, err)

	require.NotZero(t, doc.ID, "Create must hydrate the generated ID")
	require.False(t, doc.CreatedAt.IsZero(), "Create must hydrate created_at")
	assert.Equal(t, student.TenantID, doc.TenantID, "tenant_id must be filled from context")
	assert.Equal(t, int64(1234), doc.SizeBytes)

	docs, err := records.ListCareDocuments(ctx, student.ID, []string{careplan.StudentDocumentCategoryAttest})
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, "attest.pdf", docs[0].FilenameDisplay)
	assert.Equal(t, doc.FilenameStored, docs[0].FilenameStored)

	// A category the caller cannot see must not leak through the list.
	other, err := records.ListCareDocuments(ctx, student.ID, []string{careplan.StudentDocumentCategorySonstiges})
	require.NoError(t, err)
	assert.Empty(t, other)

	// No visible category at all means no query and no rows.
	none, err := records.ListCareDocuments(ctx, student.ID, nil)
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestCareDocumentRecords_FindRejectsForeignStudent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Eigen", "Kind", "1a")
	otherStudent := testpkg.CreateTestStudent(t, db, "Fremd", "Kind", "1b")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-foreign-%d@example.test", time.Now().UnixNano()))

	records := careDocumentRecords(t, db)
	ctx := testpkg.TenantContext(student.TenantID)

	doc, err := records.CreateCareDocument(ctx, newTestCareDocument(student.ID, account.ID, careplan.StudentDocumentCategorySonstiges, "sonstiges.pdf", careDocumentStoredName(t)))
	require.NoError(t, err)

	found, err := records.FindCareDocument(ctx, student.ID, doc.ID, false)
	require.NoError(t, err)
	assert.Equal(t, doc.ID, found.ID)

	// The URL names both IDs; a mismatched pair must be a 404, not a leak.
	_, err = records.FindCareDocument(ctx, otherStudent.ID, doc.ID, false)
	require.Error(t, err)
	assert.ErrorIs(t, err, careplan.ErrCareDocumentNotFound, "foreign student lookup must read as not-found")
}

func TestCareDocumentRecords_SoftDeleteHidesRowButKeepsIt(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Lösch", "Kind", "1a")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-del-%d@example.test", time.Now().UnixNano()))

	records := careDocumentRecords(t, db)
	ctx := testpkg.TenantContext(student.TenantID)

	doc, err := records.CreateCareDocument(ctx, newTestCareDocument(student.ID, account.ID, careplan.StudentDocumentCategoryBetreuungsvertrag, "vertrag.pdf", careDocumentStoredName(t)))
	require.NoError(t, err)

	deletedAt, err := records.SoftDeleteCareDocument(ctx, doc.ID, account.ID)
	require.NoError(t, err)
	require.False(t, deletedAt.IsZero(), "SoftDelete must report the deletion time")
	stamped, err := records.FindCareDocument(ctx, student.ID, doc.ID, true)
	require.NoError(t, err)
	require.NotNil(t, stamped.DeletedAt, "SoftDelete must stamp the row")
	require.NotNil(t, stamped.DeletedBy)

	// Gone from the normal view...
	visible, err := records.ListCareDocuments(ctx, student.ID, careplan.StudentDocumentCategories)
	require.NoError(t, err)
	assert.Empty(t, visible)

	// ...but still there as the record that it once existed, and still
	// pending file cleanup.
	pending, err := records.ListDeletedCareDocuments(ctx, student.ID, careplan.StudentDocumentCategories)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, doc.ID, pending[0].ID)

	require.NoError(t, records.MarkCareDocumentFileDeleted(ctx, doc.ID))
	afterCleanup, err := records.ListDeletedCareDocuments(ctx, student.ID, careplan.StudentDocumentCategories)
	require.NoError(t, err)
	assert.Empty(t, afterCleanup, "a document whose bytes are gone must not be retried")

	// Deleting twice must not silently succeed — the second call has nothing
	// left to soft-delete.
	_, err = records.SoftDeleteCareDocument(ctx, doc.ID, account.ID)
	require.Error(t, err)
}

func TestCareDocumentRecords_CleanupIntentLifecycle(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Waise", "Kind", "1a")

	records := careDocumentRecords(t, db)
	ctx := testpkg.TenantContext(student.TenantID)

	name := careDocumentStoredName(t)
	cleanup, err := records.QueueCareDocumentCleanup(ctx, careplan.CareDocumentCleanup{
		OwnerID:        student.ID,
		FilenameStored: name,
		// In the future: an intent must not be eligible while its upload may
		// still be running.
		RetryAfter: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	require.NotZero(t, cleanup.ID)

	queued, err := records.ListCareDocumentCleanups(ctx, nil)
	require.NoError(t, err)
	assert.NotContains(t, careDocumentCleanupNames(queued), name, "an intent in the future must not be eligible yet")

	// A failed upload activates its intent for immediate retry.
	require.NoError(t, records.ActivateCareDocumentCleanup(ctx, name))
	queued, err = records.ListCareDocumentCleanups(ctx, nil)
	require.NoError(t, err)
	assert.Contains(t, careDocumentCleanupNames(queued), name)

	require.NoError(t, records.CompleteCareDocumentCleanupByFilename(ctx, name))
	queued, err = records.ListCareDocumentCleanups(ctx, nil)
	require.NoError(t, err)
	assert.NotContains(t, careDocumentCleanupNames(queued), name, "a settled intent must not come back")

	// Queueing the same object again REVIVES the settled intent. Settling is
	// an update, so the row outlives the upload it belonged to; deleting the
	// child re-queues under the same stored name, and if that were a no-op the
	// bytes would stay on disk with nothing left pointing at them.
	_, err = records.QueueCareDocumentCleanup(ctx, careplan.CareDocumentCleanup{
		OwnerID:        student.ID,
		FilenameStored: name,
		RetryAfter:     time.Now(),
	})
	require.NoError(t, err)

	queued, err = records.ListCareDocumentCleanups(ctx, nil)
	require.NoError(t, err)
	assert.Contains(t, careDocumentCleanupNames(queued), name, "a re-queued object must become eligible again")
}

// TestCareDocumentRecords_CreateRejectsInvalidRow proves the validation runs
// before the insert. A row without a stored filename would be an orphan from
// the moment it is written: nothing could ever find its bytes again, and no
// cleanup pass could reclaim them.
func TestCareDocumentRecords_CreateRejectsInvalidRow(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Ungueltig", "Kind", "1a")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-invalid-%d@example.test", time.Now().UnixNano()))

	records := careDocumentRecords(t, db)
	ctx := testpkg.TenantContext(student.TenantID)

	doc, err := records.CreateCareDocument(ctx, newTestCareDocument(student.ID, account.ID, careplan.StudentDocumentCategoryAttest, "attest.pdf", ""))
	require.Error(t, err)
	assert.Zero(t, doc.ID, "a rejected row must never reach the table")
	stored, err := records.ListCareDocuments(ctx, student.ID, careplan.StudentDocumentCategories)
	require.NoError(t, err)
	assert.Empty(t, stored, "a rejected row must never reach the table")
}

// TestCareDocumentRecords_FindIncludingDeletedFeedsCleanupRetry covers the
// lookup the cleanup retry depends on: once a document is soft-deleted the
// ordinary find stops seeing it, but the handler still has to authorize a retry
// of the unlink against the very row whose bytes are pending.
func TestCareDocumentRecords_FindIncludingDeletedFeedsCleanupRetry(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Nachlauf", "Kind", "1a")
	otherStudent := testpkg.CreateTestStudent(t, db, "Fremd", "Nachlauf", "1b")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-retry-%d@example.test", time.Now().UnixNano()))

	records := careDocumentRecords(t, db)
	ctx := testpkg.TenantContext(student.TenantID)

	doc, err := records.CreateCareDocument(ctx, newTestCareDocument(student.ID, account.ID, careplan.StudentDocumentCategorySonstiges, "nachlauf.pdf", careDocumentStoredName(t)))
	require.NoError(t, err)
	_, err = records.SoftDeleteCareDocument(ctx, doc.ID, account.ID)
	require.NoError(t, err)

	_, err = records.FindCareDocument(ctx, student.ID, doc.ID, false)
	require.Error(t, err, "the ordinary lookup must not resurrect a deleted document")

	found, err := records.FindCareDocument(ctx, student.ID, doc.ID, true)
	require.NoError(t, err)
	assert.Equal(t, doc.ID, found.ID)
	require.NotNil(t, found.DeletedAt)

	// The owner check still holds: seeing deleted rows must not become a way
	// to read another child's paperwork.
	_, err = records.FindCareDocument(ctx, otherStudent.ID, doc.ID, true)
	require.Error(t, err)
	assert.ErrorIs(t, err, careplan.ErrCareDocumentNotFound, "foreign lookup must read as not-found")
}

// TestCareDocumentRecords_PendingCleanupCoversLiveAndDeletedRows covers what
// the child-deletion path reads. It has to see documents that are still live,
// because the cascade is about to remove their rows while the bytes stay on
// disk.
func TestCareDocumentRecords_PendingCleanupCoversLiveAndDeletedRows(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Abbau", "Kind", "1a")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-teardown-%d@example.test", time.Now().UnixNano()))

	records := careDocumentRecords(t, db)
	ctx := testpkg.TenantContext(student.TenantID)

	live, err := records.CreateCareDocument(ctx, newTestCareDocument(student.ID, account.ID, careplan.StudentDocumentCategorySonstiges, "aktiv.pdf", careDocumentStoredName(t)+"-live"))
	require.NoError(t, err)

	gone, err := records.CreateCareDocument(ctx, newTestCareDocument(student.ID, account.ID, careplan.StudentDocumentCategoryAbholvollmacht, "geloescht.pdf", careDocumentStoredName(t)+"-gone"))
	require.NoError(t, err)
	_, err = records.SoftDeleteCareDocument(ctx, gone.ID, account.ID)
	require.NoError(t, err)

	pending, err := records.ListPendingCareDocumentCleanup(ctx, student.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{live.ID, gone.ID}, careDocumentIDs(pending),
		"deleting a child must reclaim the bytes of live documents too")

	// The tenant-wide scheduler sweep only picks up soft-deleted rows: a live
	// document's bytes are still in use.
	deleted, err := records.ListDeletedCareDocuments(ctx, 0, nil)
	require.NoError(t, err)
	assert.Contains(t, careDocumentIDs(deleted), gone.ID)
	assert.NotContains(t, careDocumentIDs(deleted), live.ID)

	// Once the bytes are gone neither list may offer the row again.
	require.NoError(t, records.MarkCareDocumentFileDeleted(ctx, gone.ID))
	deleted, err = records.ListDeletedCareDocuments(ctx, 0, nil)
	require.NoError(t, err)
	assert.NotContains(t, careDocumentIDs(deleted), gone.ID)
	pending, err = records.ListPendingCareDocumentCleanup(ctx, student.ID)
	require.NoError(t, err)
	assert.NotContains(t, careDocumentIDs(pending), gone.ID)
}

// TestCareDocumentRecords_QueuedCleanupsAreScopedAndSettleable covers the
// per-owner view of the intent queue and settling an intent by its ID, which is
// how the scheduler retires an object it has just unlinked.
func TestCareDocumentRecords_QueuedCleanupsAreScopedAndSettleable(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Warteschlange", "Kind", "1a")
	otherStudent := testpkg.CreateTestStudent(t, db, "Warteschlange", "Andere", "1b")

	records := careDocumentRecords(t, db)
	ctx := testpkg.TenantContext(student.TenantID)

	mine := queueCareDocumentCleanupIntent(t, records, ctx, student.ID, careDocumentStoredName(t)+"-mine")
	theirs := queueCareDocumentCleanupIntent(t, records, ctx, otherStudent.ID, careDocumentStoredName(t)+"-theirs")

	scoped, err := records.ListCareDocumentCleanups(ctx, &student.ID)
	require.NoError(t, err)
	assert.Contains(t, careDocumentCleanupNames(scoped), mine.FilenameStored)
	assert.NotContains(t, careDocumentCleanupNames(scoped), theirs.FilenameStored)

	require.NoError(t, records.CompleteCareDocumentCleanup(ctx, mine.ID))
	scoped, err = records.ListCareDocumentCleanups(ctx, &student.ID)
	require.NoError(t, err)
	assert.NotContains(t, careDocumentCleanupNames(scoped), mine.FilenameStored,
		"a settled intent must not be handed out again")
}

func queueCareDocumentCleanupIntent(t *testing.T, records careplan.Capability, ctx context.Context, ownerID int64, name string) careplan.CareDocumentCleanup {
	t.Helper()
	cleanup, err := records.QueueCareDocumentCleanup(ctx, careplan.CareDocumentCleanup{
		OwnerID:        ownerID,
		FilenameStored: name,
		RetryAfter:     time.Now().Add(-time.Minute),
	})
	require.NoError(t, err)
	return cleanup
}

func careDocumentIDs(docs []careplan.CareDocument) []int64 {
	ids := make([]int64, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}
	return ids
}

func careDocumentCleanupNames(cleanups []careplan.CareDocumentCleanup) []string {
	names := make([]string, 0, len(cleanups))
	for _, cleanup := range cleanups {
		names = append(names, cleanup.FilenameStored)
	}
	return names
}

// TestCareDocumentRecords_CleanupSweepIsBounded pins the cap the scheduler
// depends on. The sweep runs inside one tenant transaction, so an uncapped pass
// after a cohort deletion would hold a connection through thousands of
// unlink-and-mark pairs, and a deadline firing mid-pass would roll back every
// completion mark it had written.
func TestCareDocumentRecords_CleanupSweepIsBounded(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Viele", "Dokumente", "1a")

	records := careDocumentRecords(t, db)
	ctx := testpkg.TenantContext(student.TenantID)

	overBatch := careDocumentCleanupBatchSize + 5
	for i := range overBatch {
		_, err := records.QueueCareDocumentCleanup(ctx, careplan.CareDocumentCleanup{
			OwnerID:        student.ID,
			FilenameStored: fmt.Sprintf("bounded-%d-%d.pdf", time.Now().UnixNano(), i),
			RetryAfter:     time.Now().Add(-time.Minute),
		})
		require.NoError(t, err)
	}

	queued, err := records.ListCareDocumentCleanups(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, queued, careDocumentCleanupBatchSize,
		"one pass must stop at the batch size, leaving the rest for the next tick")
}

// TestCareDocumentRecords_RequestRetryIsBounded pins the tighter cap on the
// query that feeds the request path. Each returned row becomes an unlink plus a
// transaction after the response, so a page view must never inherit a backlog
// left by an unreachable storage backend.
func TestCareDocumentRecords_RequestRetryIsBounded(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Rueckstand", "Kind", "1a")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-backlog-%d@example.test", time.Now().UnixNano()))

	records := careDocumentRecords(t, db)
	ctx := testpkg.TenantContext(student.TenantID)

	overLimit := careDocumentRequestCleanupRetries + 3
	for i := range overLimit {
		doc, err := records.CreateCareDocument(ctx, newTestCareDocument(student.ID, account.ID,
			careplan.StudentDocumentCategorySonstiges,
			fmt.Sprintf("rueckstand-%d.pdf", i),
			fmt.Sprintf("rueckstand-%d-%d.pdf", time.Now().UnixNano(), i)))
		require.NoError(t, err)
		_, err = records.SoftDeleteCareDocument(ctx, doc.ID, account.ID)
		require.NoError(t, err)
	}

	pending, err := records.ListDeletedCareDocuments(ctx, student.ID, careplan.StudentDocumentCategories)
	require.NoError(t, err)
	assert.Len(t, pending, careDocumentRequestCleanupRetries,
		"one page view retries at most the request limit, the scheduler takes the rest")
}
