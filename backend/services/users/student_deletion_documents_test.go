package users_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStudentDeletionService_QueuesDocumentCleanupInsideTheTransaction covers
// the only thing standing between deleting a child and their paperwork sitting
// on disk forever (#777).
//
// The document rows carry a foreign key to the child and cascade away with
// them. The cleanup intents deliberately do not, which is what lets them
// outlive the cascade — but only if they are written before it, inside the same
// transaction. Written afterwards, a crash strands the bytes with nothing left
// pointing at them; written beforehand and outside, a deletion that then fails
// leaves the scheduler about to delete a living child's documents.
func TestStudentDeletionService_QueuesDocumentCleanupInsideTheTransaction(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db)
	service := newStudentDeletionTestService(db, repos.DataDeletion, repos.StudentDeletionAudit)
	usersService.WireStudentDocumentCleanup(service, repos.StudentDocument)

	suffix := time.Now().UnixNano()
	student := testpkg.CreateTestStudent(t, db, "Dokumente", fmt.Sprintf("Loeschung-%d", suffix), "1a")
	actor := testpkg.CreateTestAccount(t, db, fmt.Sprintf("student-delete-docs-%d@example.test", suffix))

	live := storeStudentDocument(t, ctx, repos.StudentDocument, student.ID, actor.ID,
		userModels.StudentDocumentCategoryBetreuungsvertrag, fmt.Sprintf("vertrag-%d.pdf", suffix))
	// Already soft-deleted, bytes not yet unlinked: still pending cleanup.
	deleted := storeStudentDocument(t, ctx, repos.StudentDocument, student.ID, actor.ID,
		userModels.StudentDocumentCategoryAttest, fmt.Sprintf("attest-%d.pdf", suffix))
	require.NoError(t, repos.StudentDocument.SoftDelete(ctx, deleted, actor.ID))
	// Bytes already gone: nothing left to reclaim, so no intent must appear.
	settled := storeStudentDocument(t, ctx, repos.StudentDocument, student.ID, actor.ID,
		userModels.StudentDocumentCategorySonstiges, fmt.Sprintf("erledigt-%d.pdf", suffix))
	require.NoError(t, repos.StudentDocument.MarkFileDeleted(ctx, settled.ID))

	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(),
			`DELETE FROM users.student_document_file_cleanup WHERE owner_id = ?`, student.ID)
		require.NoError(t, err)
		_, err = db.ExecContext(context.Background(),
			`DELETE FROM audit.student_deletions WHERE student_id = ?`, student.ID)
		require.NoError(t, err)
		cleanupStudentDeletionTest(t, db,
			[]int64{student.ID}, []int64{student.PersonID},
			nil, nil, nil, []int64{actor.ID}, nil)
	})

	preview, err := service.Preview(ctx, student.ID)
	require.NoError(t, err)

	_, err = service.Delete(ctx, usersService.StudentDeletionInput{
		StudentID:           student.ID,
		ActorAccountID:      actor.ID,
		ExpectedFingerprint: preview.Fingerprint,
		ConfirmationName:    preview.ConfirmationName,
		Reason:              usersService.StudentDeletionReasonTestData,
		Acknowledged:        true,
	})
	require.NoError(t, err)

	// The child and their document rows are gone...
	assert.Zero(t, tableRowCount(t, db, "users.students", student.ID))
	assert.Zero(t, tableRowCount(t, db, "users.student_documents", live.ID))
	assert.Zero(t, tableRowCount(t, db, "users.student_documents", deleted.ID))

	// ...and exactly the objects that still exist on disk are left in the
	// queue, immediately eligible: no upload for them can still be in flight.
	queued, err := repos.StudentDocument.ListQueuedFileCleanupByOwnerID(ctx, student.ID)
	require.NoError(t, err)
	names := make([]string, 0, len(queued))
	for _, cleanup := range queued {
		names = append(names, cleanup.FilenameStored)
	}
	assert.Contains(t, names, live.FilenameStored)
	assert.Contains(t, names, deleted.FilenameStored)
	assert.NotContains(t, names, settled.FilenameStored,
		"an object whose bytes are already gone must not be queued again")
}

func storeStudentDocument(
	t *testing.T,
	ctx context.Context,
	repo userModels.StudentDocumentRepository,
	studentID, accountID int64,
	category, stored string,
) *userModels.StudentDocument {
	t.Helper()
	doc := &userModels.StudentDocument{StudentID: studentID}
	doc.Category = category
	doc.FilenameDisplay = category + ".pdf"
	doc.FilenameStored = stored
	doc.SizeBytes = 512
	doc.ContentType = "application/pdf"
	doc.UploadedBy = accountID
	require.NoError(t, repo.Create(ctx, doc))
	return doc
}
