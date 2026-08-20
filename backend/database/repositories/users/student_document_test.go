package users_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	documentModels "github.com/moto-nrw/project-phoenix/models/documents"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// newTestStudentDocument builds a valid document row for a student.
func newTestStudentDocument(studentID, accountID int64, category, display, stored string) *userModels.StudentDocument {
	doc := &userModels.StudentDocument{StudentID: studentID}
	doc.Category = category
	doc.FilenameDisplay = display
	doc.FilenameStored = stored
	doc.SizeBytes = 1234
	doc.ContentType = "application/pdf"
	doc.UploadedBy = accountID
	return doc
}

func storedName(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("%d-%s.pdf", time.Now().UnixNano(), t.Name())
}

// TestStudentDocumentRepository_CreateAndList also proves the embedded
// documents.File model maps onto users.student_documents: every shared column
// comes from the embed, so a broken mapping fails right here rather than in
// production.
func TestStudentDocumentRepository_CreateAndList(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Doku", "Kind", "1a")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-%d@example.test", time.Now().UnixNano()))

	repo := repositories.NewFactory(db).StudentDocument
	ctx := testpkg.TenantContext(student.TenantID)

	doc := newTestStudentDocument(student.ID, account.ID, userModels.StudentDocumentCategoryAttest, "attest.pdf", storedName(t))
	require.NoError(t, repo.Create(ctx, doc))

	require.NotZero(t, doc.ID, "Create must hydrate the generated ID")
	require.False(t, doc.CreatedAt.IsZero(), "Create must hydrate created_at")
	assert.Equal(t, student.TenantID, doc.TenantID, "tenant_id must be filled from context")
	assert.Equal(t, int64(1234), doc.SizeBytes)

	docs, err := repo.ListByOwnerID(ctx, student.ID, []string{userModels.StudentDocumentCategoryAttest})
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, "attest.pdf", docs[0].FilenameDisplay)
	assert.Equal(t, doc.FilenameStored, docs[0].FilenameStored)

	// A category the caller cannot see must not leak through the list.
	other, err := repo.ListByOwnerID(ctx, student.ID, []string{userModels.StudentDocumentCategorySonstiges})
	require.NoError(t, err)
	assert.Empty(t, other)

	// No visible category at all means no query and no rows.
	none, err := repo.ListByOwnerID(ctx, student.ID, nil)
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestStudentDocumentRepository_FindForOwnerRejectsForeignStudent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Eigen", "Kind", "1a")
	otherStudent := testpkg.CreateTestStudent(t, db, "Fremd", "Kind", "1b")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-foreign-%d@example.test", time.Now().UnixNano()))

	repo := repositories.NewFactory(db).StudentDocument
	ctx := testpkg.TenantContext(student.TenantID)

	doc := newTestStudentDocument(student.ID, account.ID, userModels.StudentDocumentCategorySonstiges, "sonstiges.pdf", storedName(t))
	require.NoError(t, repo.Create(ctx, doc))

	found, err := repo.FindForOwner(ctx, student.ID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, doc.ID, found.ID)

	// The URL names both IDs; a mismatched pair must be a 404, not a leak.
	_, err = repo.FindForOwner(ctx, otherStudent.ID, doc.ID)
	require.Error(t, err)
	assert.True(t, modelBase.IsNoRows(err), "foreign student lookup must read as not-found")
}

func TestStudentDocumentRepository_SoftDeleteHidesRowButKeepsIt(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Lösch", "Kind", "1a")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-del-%d@example.test", time.Now().UnixNano()))

	repo := repositories.NewFactory(db).StudentDocument
	ctx := testpkg.TenantContext(student.TenantID)

	doc := newTestStudentDocument(student.ID, account.ID, userModels.StudentDocumentCategoryBetreuungsvertrag, "vertrag.pdf", storedName(t))
	require.NoError(t, repo.Create(ctx, doc))

	require.NoError(t, repo.SoftDelete(ctx, doc, account.ID))
	require.NotNil(t, doc.DeletedAt, "SoftDelete must stamp the in-memory row")
	require.NotNil(t, doc.DeletedBy)

	// Gone from the normal view...
	visible, err := repo.ListByOwnerID(ctx, student.ID, userModels.StudentDocumentCategories)
	require.NoError(t, err)
	assert.Empty(t, visible)

	// ...but still there as the record that it once existed, and still
	// pending file cleanup.
	pending, err := repo.ListDeletedPendingFileCleanupByOwnerID(ctx, student.ID, userModels.StudentDocumentCategories)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, doc.ID, pending[0].ID)

	require.NoError(t, repo.MarkFileDeleted(ctx, doc.ID))
	afterCleanup, err := repo.ListDeletedPendingFileCleanupByOwnerID(ctx, student.ID, userModels.StudentDocumentCategories)
	require.NoError(t, err)
	assert.Empty(t, afterCleanup, "a document whose bytes are gone must not be retried")

	// Deleting twice must not silently succeed — the second call has nothing
	// left to soft-delete.
	require.Error(t, repo.SoftDelete(ctx, doc, account.ID))
}

func TestStudentDocumentRepository_CleanupIntentLifecycle(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Waise", "Kind", "1a")

	repo := repositories.NewFactory(db).StudentDocument
	ctx := testpkg.TenantContext(student.TenantID)

	name := storedName(t)
	cleanup := &userModels.StudentDocumentFileCleanup{}
	cleanup.OwnerID = student.ID
	cleanup.FilenameStored = name
	// In the future: an intent must not be eligible while its upload may
	// still be running.
	cleanup.RetryAfter = time.Now().Add(time.Hour)
	require.NoError(t, repo.QueueFileCleanup(ctx, cleanup))
	require.NotZero(t, cleanup.ID)

	queued, err := repo.ListQueuedFileCleanups(ctx)
	require.NoError(t, err)
	assert.NotContains(t, storedNames(queued), name, "an intent in the future must not be eligible yet")

	// A failed upload activates its intent for immediate retry.
	require.NoError(t, repo.ActivateQueuedFileCleanupByFilename(ctx, name))
	queued, err = repo.ListQueuedFileCleanups(ctx)
	require.NoError(t, err)
	assert.Contains(t, storedNames(queued), name)

	require.NoError(t, repo.MarkQueuedFileCleanupCompleteByFilename(ctx, name))
	queued, err = repo.ListQueuedFileCleanups(ctx)
	require.NoError(t, err)
	assert.NotContains(t, storedNames(queued), name, "a settled intent must not come back")

	// Queueing the same object again REVIVES the settled intent. Settling is
	// an update, so the row outlives the upload it belonged to; deleting the
	// child re-queues under the same stored name, and if that were a no-op the
	// bytes would stay on disk with nothing left pointing at them.
	duplicate := &userModels.StudentDocumentFileCleanup{}
	duplicate.OwnerID = student.ID
	duplicate.FilenameStored = name
	duplicate.RetryAfter = time.Now()
	require.NoError(t, repo.QueueFileCleanup(ctx, duplicate))

	queued, err = repo.ListQueuedFileCleanups(ctx)
	require.NoError(t, err)
	assert.Contains(t, storedNames(queued), name, "a re-queued object must become eligible again")
}

// TestStudentDocumentRepository_CreateRejectsInvalidRow proves the validation
// runs before the insert. A row without a stored filename would be an orphan
// from the moment it is written: nothing could ever find its bytes again, and
// no cleanup pass could reclaim them.
func TestStudentDocumentRepository_CreateRejectsInvalidRow(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Ungueltig", "Kind", "1a")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-invalid-%d@example.test", time.Now().UnixNano()))

	repo := repositories.NewFactory(db).StudentDocument
	ctx := testpkg.TenantContext(student.TenantID)

	doc := newTestStudentDocument(student.ID, account.ID, userModels.StudentDocumentCategoryAttest, "attest.pdf", "")
	require.Error(t, repo.Create(ctx, doc))
	assert.Zero(t, doc.ID, "a rejected row must never reach the table")
}

// TestStudentDocumentRepository_FindIncludingDeletedFeedsCleanupRetry covers the
// lookup the cleanup retry depends on: once a document is soft-deleted the
// ordinary find stops seeing it, but the handler still has to authorize a retry
// of the unlink against the very row whose bytes are pending.
func TestStudentDocumentRepository_FindIncludingDeletedFeedsCleanupRetry(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Nachlauf", "Kind", "1a")
	otherStudent := testpkg.CreateTestStudent(t, db, "Fremd", "Nachlauf", "1b")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-retry-%d@example.test", time.Now().UnixNano()))

	repo := repositories.NewFactory(db).StudentDocument
	ctx := testpkg.TenantContext(student.TenantID)

	doc := newTestStudentDocument(student.ID, account.ID, userModels.StudentDocumentCategorySonstiges, "nachlauf.pdf", storedName(t))
	require.NoError(t, repo.Create(ctx, doc))
	require.NoError(t, repo.SoftDelete(ctx, doc, account.ID))

	_, err := repo.FindForOwner(ctx, student.ID, doc.ID)
	require.Error(t, err, "the ordinary lookup must not resurrect a deleted document")

	found, err := repo.FindForOwnerIncludingDeleted(ctx, student.ID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, doc.ID, found.ID)
	require.NotNil(t, found.DeletedAt)

	// The owner check still holds: seeing deleted rows must not become a way
	// to read another child's paperwork.
	_, err = repo.FindForOwnerIncludingDeleted(ctx, otherStudent.ID, doc.ID)
	require.Error(t, err)
	assert.True(t, modelBase.IsNoRows(err), "foreign lookup must read as not-found")
}

// TestStudentDocumentRepository_PendingCleanupCoversLiveAndDeletedRows covers
// what the child-deletion path reads. It has to see documents that are still
// live, because the cascade is about to remove their rows while the bytes stay
// on disk.
func TestStudentDocumentRepository_PendingCleanupCoversLiveAndDeletedRows(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Abbau", "Kind", "1a")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-teardown-%d@example.test", time.Now().UnixNano()))

	repo := repositories.NewFactory(db).StudentDocument
	ctx := testpkg.TenantContext(student.TenantID)

	live := newTestStudentDocument(student.ID, account.ID, userModels.StudentDocumentCategorySonstiges, "aktiv.pdf", storedName(t)+"-live")
	require.NoError(t, repo.Create(ctx, live))

	gone := newTestStudentDocument(student.ID, account.ID, userModels.StudentDocumentCategoryAbholvollmacht, "geloescht.pdf", storedName(t)+"-gone")
	require.NoError(t, repo.Create(ctx, gone))
	require.NoError(t, repo.SoftDelete(ctx, gone, account.ID))

	pending, err := repo.ListPendingFileCleanupByOwnerID(ctx, student.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{live.ID, gone.ID}, documentIDs(pending),
		"deleting a child must reclaim the bytes of live documents too")

	// The tenant-wide scheduler sweep only picks up soft-deleted rows: a live
	// document's bytes are still in use.
	deleted, err := repo.ListDeletedPendingFileCleanups(ctx)
	require.NoError(t, err)
	assert.Contains(t, documentIDs(deleted), gone.ID)
	assert.NotContains(t, documentIDs(deleted), live.ID)

	// Once the bytes are gone neither list may offer the row again.
	require.NoError(t, repo.MarkFileDeleted(ctx, gone.ID))
	deleted, err = repo.ListDeletedPendingFileCleanups(ctx)
	require.NoError(t, err)
	assert.NotContains(t, documentIDs(deleted), gone.ID)
	pending, err = repo.ListPendingFileCleanupByOwnerID(ctx, student.ID)
	require.NoError(t, err)
	assert.NotContains(t, documentIDs(pending), gone.ID)
}

// TestStudentDocumentRepository_QueuedCleanupsAreScopedAndSettleable covers the
// per-owner view of the intent queue and settling an intent by its ID, which is
// how the scheduler retires an object it has just unlinked.
func TestStudentDocumentRepository_QueuedCleanupsAreScopedAndSettleable(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Warteschlange", "Kind", "1a")
	otherStudent := testpkg.CreateTestStudent(t, db, "Warteschlange", "Andere", "1b")

	repo := repositories.NewFactory(db).StudentDocument
	ctx := testpkg.TenantContext(student.TenantID)

	mine := queueCleanupIntent(t, db, repo, ctx, student.ID, storedName(t)+"-mine")
	theirs := queueCleanupIntent(t, db, repo, ctx, otherStudent.ID, storedName(t)+"-theirs")

	scoped, err := repo.ListQueuedFileCleanupByOwnerID(ctx, student.ID)
	require.NoError(t, err)
	assert.Contains(t, storedNames(scoped), mine.FilenameStored)
	assert.NotContains(t, storedNames(scoped), theirs.FilenameStored)

	require.NoError(t, repo.MarkQueuedFileCleanupComplete(ctx, mine.ID))
	scoped, err = repo.ListQueuedFileCleanupByOwnerID(ctx, student.ID)
	require.NoError(t, err)
	assert.NotContains(t, storedNames(scoped), mine.FilenameStored,
		"a settled intent must not be handed out again")
}

func queueCleanupIntent(t *testing.T, db *bun.DB, repo userModels.StudentDocumentRepository, ctx context.Context, ownerID int64, name string) *userModels.StudentDocumentFileCleanup {
	t.Helper()
	cleanup := &userModels.StudentDocumentFileCleanup{}
	cleanup.OwnerID = ownerID
	cleanup.FilenameStored = name
	cleanup.RetryAfter = time.Now().Add(-time.Minute)
	require.NoError(t, repo.QueueFileCleanup(ctx, cleanup))
	return cleanup
}

func documentIDs(docs []*userModels.StudentDocument) []int64 {
	ids := make([]int64, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}
	return ids
}

func storedNames(cleanups []*userModels.StudentDocumentFileCleanup) []string {
	names := make([]string, 0, len(cleanups))
	for _, cleanup := range cleanups {
		names = append(names, cleanup.FilenameStored)
	}
	return names
}

// TestStudentDocumentRepository_CleanupSweepIsBounded pins the cap the
// scheduler depends on. The sweep runs inside one tenant transaction, so an
// uncapped pass after a cohort deletion would hold a connection through
// thousands of unlink-and-mark pairs, and a deadline firing mid-pass would roll
// back every completion mark it had written.
func TestStudentDocumentRepository_CleanupSweepIsBounded(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Viele", "Dokumente", "1a")

	repo := repositories.NewFactory(db).StudentDocument
	ctx := testpkg.TenantContext(student.TenantID)

	overBatch := documentModels.CleanupBatchSize + 5
	for i := range overBatch {
		cleanup := &userModels.StudentDocumentFileCleanup{}
		cleanup.OwnerID = student.ID
		cleanup.FilenameStored = fmt.Sprintf("bounded-%d-%d.pdf", time.Now().UnixNano(), i)
		cleanup.RetryAfter = time.Now().Add(-time.Minute)
		require.NoError(t, repo.QueueFileCleanup(ctx, cleanup))
	}

	queued, err := repo.ListQueuedFileCleanups(ctx)
	require.NoError(t, err)
	assert.Len(t, queued, documentModels.CleanupBatchSize,
		"one pass must stop at the batch size, leaving the rest for the next tick")
}

// TestStudentDocumentRepository_RequestRetryIsBounded pins the tighter cap on
// the query that feeds the request path. Each returned row becomes an unlink
// plus a transaction after the response, so a page view must never inherit a
// backlog left by an unreachable storage backend.
func TestStudentDocumentRepository_RequestRetryIsBounded(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Rueckstand", "Kind", "1a")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-backlog-%d@example.test", time.Now().UnixNano()))

	repo := repositories.NewFactory(db).StudentDocument
	ctx := testpkg.TenantContext(student.TenantID)

	overLimit := documentModels.RequestCleanupRetryLimit + 3
	for i := range overLimit {
		doc := newTestStudentDocument(student.ID, account.ID,
			userModels.StudentDocumentCategorySonstiges,
			fmt.Sprintf("rueckstand-%d.pdf", i),
			fmt.Sprintf("rueckstand-%d-%d.pdf", time.Now().UnixNano(), i))
		require.NoError(t, repo.Create(ctx, doc))
		require.NoError(t, repo.SoftDelete(ctx, doc, account.ID))
	}

	pending, err := repo.ListDeletedPendingFileCleanupByOwnerID(ctx, student.ID, userModels.StudentDocumentCategories)
	require.NoError(t, err)
	assert.Len(t, pending, documentModels.RequestCleanupRetryLimit,
		"one page view retries at most the request limit, the scheduler takes the rest")
}
