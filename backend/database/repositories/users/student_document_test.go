package users_test

import (
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	student := testpkg.CreateTestStudent(t, db, "Doku", "Kind", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-%d@example.test", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	repo := repositories.NewFactory(db).StudentDocument
	ctx := testpkg.TenantContext(student.TenantID)

	doc := newTestStudentDocument(student.ID, account.ID, userModels.StudentDocumentCategoryAttest, "attest.pdf", storedName(t))
	require.NoError(t, repo.Create(ctx, doc))
	defer testpkg.CleanupTableRecords(t, db, "users.student_documents", doc.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	student := testpkg.CreateTestStudent(t, db, "Eigen", "Kind", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)
	otherStudent := testpkg.CreateTestStudent(t, db, "Fremd", "Kind", "1b")
	defer testpkg.CleanupActivityFixtures(t, db, otherStudent.ID)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-foreign-%d@example.test", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	repo := repositories.NewFactory(db).StudentDocument
	ctx := testpkg.TenantContext(student.TenantID)

	doc := newTestStudentDocument(student.ID, account.ID, userModels.StudentDocumentCategorySonstiges, "sonstiges.pdf", storedName(t))
	require.NoError(t, repo.Create(ctx, doc))
	defer testpkg.CleanupTableRecords(t, db, "users.student_documents", doc.ID)

	found, err := repo.FindForOwner(ctx, student.ID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, doc.ID, found.ID)

	// The URL names both IDs; a mismatched pair must be a 404, not a leak.
	_, err = repo.FindForOwner(ctx, otherStudent.ID, doc.ID)
	require.Error(t, err)
	assert.True(t, modelBase.IsNoRows(err), "foreign student lookup must read as not-found")
}

func TestStudentDocumentRepository_SoftDeleteHidesRowButKeepsIt(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	student := testpkg.CreateTestStudent(t, db, "Lösch", "Kind", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-del-%d@example.test", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	repo := repositories.NewFactory(db).StudentDocument
	ctx := testpkg.TenantContext(student.TenantID)

	doc := newTestStudentDocument(student.ID, account.ID, userModels.StudentDocumentCategoryBetreuungsvertrag, "vertrag.pdf", storedName(t))
	require.NoError(t, repo.Create(ctx, doc))
	defer testpkg.CleanupTableRecords(t, db, "users.student_documents", doc.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	student := testpkg.CreateTestStudent(t, db, "Waise", "Kind", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

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
	defer testpkg.CleanupTableRecords(t, db, "users.student_document_file_cleanup", cleanup.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	student := testpkg.CreateTestStudent(t, db, "Viele", "Dokumente", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	repo := repositories.NewFactory(db).StudentDocument
	ctx := testpkg.TenantContext(student.TenantID)

	overBatch := documentModels.CleanupBatchSize + 5
	ids := make([]int64, 0, overBatch)
	for i := range overBatch {
		cleanup := &userModels.StudentDocumentFileCleanup{}
		cleanup.OwnerID = student.ID
		cleanup.FilenameStored = fmt.Sprintf("bounded-%d-%d.pdf", time.Now().UnixNano(), i)
		cleanup.RetryAfter = time.Now().Add(-time.Minute)
		require.NoError(t, repo.QueueFileCleanup(ctx, cleanup))
		ids = append(ids, cleanup.ID)
	}
	defer testpkg.CleanupTableRecords(t, db, "users.student_document_file_cleanup", ids...)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	student := testpkg.CreateTestStudent(t, db, "Rueckstand", "Kind", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-doc-backlog-%d@example.test", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	repo := repositories.NewFactory(db).StudentDocument
	ctx := testpkg.TenantContext(student.TenantID)

	overLimit := documentModels.RequestCleanupRetryLimit + 3
	ids := make([]int64, 0, overLimit)
	for i := range overLimit {
		doc := newTestStudentDocument(student.ID, account.ID,
			userModels.StudentDocumentCategorySonstiges,
			fmt.Sprintf("rueckstand-%d.pdf", i),
			fmt.Sprintf("rueckstand-%d-%d.pdf", time.Now().UnixNano(), i))
		require.NoError(t, repo.Create(ctx, doc))
		require.NoError(t, repo.SoftDelete(ctx, doc, account.ID))
		ids = append(ids, doc.ID)
	}
	defer testpkg.CleanupTableRecords(t, db, "users.student_documents", ids...)

	pending, err := repo.ListDeletedPendingFileCleanupByOwnerID(ctx, student.ID, userModels.StudentDocumentCategories)
	require.NoError(t, err)
	assert.Len(t, pending, documentModels.RequestCleanupRetryLimit,
		"one page view retries at most the request limit, the scheduler takes the rest")
}
