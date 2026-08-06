package users_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Child documents (#777): per-category authority (Attest/Impfnachweis/
// Medikamentenplan → student_documents:health, Sorgerecht →
// student_documents:legal, rest → users:update) is enforced in the service —
// uploads, downloads, deletes AND list visibility. Every upload and delete
// writes a change-history row; sensitive downloads write a data-access log row.

type studentDocumentScenario struct {
	db        *bun.DB
	svc       usersSvc.StudentDocumentService
	ctx       context.Context
	studentID int64
	account   int64
}

func newStudentDocumentScenario(t *testing.T) *studentDocumentScenario {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	repos := repositories.NewFactory(db)
	svc := usersSvc.NewStudentDocumentService(
		db,
		repos.StudentDocument,
		repos.Student,
		repos.StudentFieldEdit,
		repos.DataAccessLog,
		nil,
	)

	suffix := time.Now().UnixNano()
	student := testpkg.CreateTestStudent(t, db, "Dokumente", fmt.Sprintf("Kind-%d", suffix), "1a")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("kind-dokumente-%d@example.test", suffix))
	t.Cleanup(func() {
		cleanupStudentDocumentRows(t, db, student.ID)
		testpkg.CleanupActivityFixtures(t, db, student.ID)
		testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)
	})

	return &studentDocumentScenario{
		db:        db,
		svc:       svc,
		ctx:       testpkg.TenantContext(student.TenantID),
		studentID: student.ID,
		account:   account.ID,
	}
}

func cleanupStudentDocumentRows(t *testing.T, db *bun.DB, studentID int64) {
	t.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `DELETE FROM users.student_documents WHERE student_id = ?`, studentID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM users.student_document_file_cleanup WHERE owner_id = ?`, studentID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM audit.student_field_edits WHERE student_id = ?`, studentID)
	require.NoError(t, err)
}

func (s *studentDocumentScenario) actor(perms ...string) usersSvc.StudentDocumentActor {
	return usersSvc.StudentDocumentActor{
		AccountID:   s.account,
		Name:        "Test Person",
		Role:        "test",
		Permissions: perms,
	}
}

func (s *studentDocumentScenario) create(t *testing.T, category string, actor usersSvc.StudentDocumentActor) *userModels.StudentDocument {
	t.Helper()
	doc, err := s.svc.CreateStudentDocument(s.ctx, s.input(category), actor)
	require.NoError(t, err)
	return doc
}

func (s *studentDocumentScenario) input(category string) usersSvc.CreateStudentDocumentInput {
	return usersSvc.CreateStudentDocumentInput{
		StudentID:       s.studentID,
		Category:        category,
		FilenameDisplay: category + "-datei.pdf",
		FilenameStored:  fmt.Sprintf("%s-%d.pdf", category, time.Now().UnixNano()),
		SizeBytes:       42,
		ContentType:     "application/pdf",
	}
}

func (s *studentDocumentScenario) auditRows(t *testing.T) []*auditModels.StudentFieldEdit {
	t.Helper()
	var rows []*auditModels.StudentFieldEdit
	err := s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`audit.student_field_edits AS "student_field_edit"`).
		Where(`"student_field_edit".student_id = ?`, s.studentID).
		Where(`"student_field_edit".field_name = ?`, auditModels.StudentFieldDocument).
		Order("id ASC").
		Scan(context.Background())
	require.NoError(t, err)
	return rows
}

func (s *studentDocumentScenario) accessLogRows(t *testing.T) []*auditModels.DataAccessLog {
	t.Helper()
	var rows []*auditModels.DataAccessLog
	err := s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`audit.data_access_log AS "data_access_log"`).
		Where(`"data_access_log".actor_account_id = ?`, s.account).
		Order("id ASC").
		Scan(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = s.db.ExecContext(context.Background(), `DELETE FROM audit.data_access_log WHERE actor_account_id = ?`, s.account)
	})
	return rows
}

func studentDocumentCategoriesOf(docs []*userModels.StudentDocument) []string {
	out := make([]string, 0, len(docs))
	for _, doc := range docs {
		out = append(out, doc.Category)
	}
	return out
}

func TestStudentDocumentService_CategoryAuthority(t *testing.T) {
	s := newStudentDocumentScenario(t)

	office := s.actor("users:update")
	health := s.actor("student_documents:health")
	legal := s.actor("student_documents:legal")
	admin := s.actor("admin:*")

	// The office covers the everyday paperwork only.
	s.create(t, userModels.StudentDocumentCategoryBetreuungsvertrag, office)
	_, err := s.svc.CreateStudentDocument(s.ctx, s.input(userModels.StudentDocumentCategoryAttest), office)
	require.ErrorIs(t, err, usersSvc.ErrStudentDocumentForbidden)
	_, err = s.svc.CreateStudentDocument(s.ctx, s.input(userModels.StudentDocumentCategorySorgerecht), office)
	require.ErrorIs(t, err, usersSvc.ErrStudentDocumentForbidden)

	// The dedicated permissions cover exactly their tier and nothing else.
	attest := s.create(t, userModels.StudentDocumentCategoryAttest, health)
	sorgerecht := s.create(t, userModels.StudentDocumentCategorySorgerecht, legal)
	_, err = s.svc.CreateStudentDocument(s.ctx, s.input(userModels.StudentDocumentCategorySorgerecht), health)
	require.ErrorIs(t, err, usersSvc.ErrStudentDocumentForbidden)
	_, err = s.svc.CreateStudentDocument(s.ctx, s.input(userModels.StudentDocumentCategoryImpfnachweis), legal)
	require.ErrorIs(t, err, usersSvc.ErrStudentDocumentForbidden)

	// List visibility follows the same mapping — a health document must not
	// even appear in the office's list.
	docs, visible, err := s.svc.ListStudentDocuments(s.ctx, s.studentID, "", office)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		userModels.StudentDocumentCategoryBetreuungsvertrag,
		userModels.StudentDocumentCategoryAbholvollmacht,
		userModels.StudentDocumentCategorySchwimmerlaubnis,
		userModels.StudentDocumentCategorySonstiges,
	}, visible)
	assert.ElementsMatch(t, []string{userModels.StudentDocumentCategoryBetreuungsvertrag}, studentDocumentCategoriesOf(docs))

	docs, visible, err = s.svc.ListStudentDocuments(s.ctx, s.studentID, "", health)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		userModels.StudentDocumentCategoryAttest,
		userModels.StudentDocumentCategoryImpfnachweis,
		userModels.StudentDocumentCategoryMedikamentenplan,
	}, visible)
	assert.ElementsMatch(t, []string{userModels.StudentDocumentCategoryAttest}, studentDocumentCategoriesOf(docs))

	docs, visible, err = s.svc.ListStudentDocuments(s.ctx, s.studentID, "", admin)
	require.NoError(t, err)
	assert.Len(t, visible, len(userModels.StudentDocumentCategories))
	assert.Len(t, docs, 3)

	// A category filter outside the caller's authority is refused; inside it
	// narrows.
	_, _, err = s.svc.ListStudentDocuments(s.ctx, s.studentID, userModels.StudentDocumentCategoryAttest, office)
	require.ErrorIs(t, err, usersSvc.ErrStudentDocumentForbidden)
	docs, _, err = s.svc.ListStudentDocuments(s.ctx, s.studentID, userModels.StudentDocumentCategoryAttest, health)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, attest.ID, docs[0].ID)

	// Deleting across the tier boundary is refused too — a 403 on upload would
	// be worthless if delete were open.
	_, err = s.svc.DeleteStudentDocument(s.ctx, s.studentID, sorgerecht.ID, office)
	require.ErrorIs(t, err, usersSvc.ErrStudentDocumentForbidden)
}

func TestStudentDocumentService_SensitiveDownloadsAreLogged(t *testing.T) {
	s := newStudentDocumentScenario(t)

	office := s.actor("users:update")
	health := s.actor("student_documents:health")
	legal := s.actor("student_documents:legal")

	everyday := s.create(t, userModels.StudentDocumentCategoryAbholvollmacht, office)
	attest := s.create(t, userModels.StudentDocumentCategoryAttest, health)
	custody := s.create(t, userModels.StudentDocumentCategorySorgerecht, legal)

	// Foreign permissions never reach the bytes.
	_, err := s.svc.ResolveStudentDocumentDownload(s.ctx, s.studentID, attest.ID, office)
	require.ErrorIs(t, err, usersSvc.ErrStudentDocumentForbidden)

	// An ordinary category is served without an access-log row.
	_, err = s.svc.ResolveStudentDocumentDownload(s.ctx, s.studentID, everyday.ID, office)
	require.NoError(t, err)
	assert.Empty(t, s.accessLogRows(t), "an everyday document must not inflate the access log")

	// Both sensitive tiers write one row each, naming the document.
	_, err = s.svc.ResolveStudentDocumentDownload(s.ctx, s.studentID, attest.ID, health)
	require.NoError(t, err)
	_, err = s.svc.ResolveStudentDocumentDownload(s.ctx, s.studentID, custody.ID, legal)
	require.NoError(t, err)

	logs := s.accessLogRows(t)
	require.Len(t, logs, 2)
	for _, entry := range logs {
		assert.Equal(t, auditModels.ResourceTypeStudentDocumentDownload, entry.ResourceType)
	}
	assert.EqualValues(t, attest.ID, logs[0].Metadata["document_id"])
	assert.EqualValues(t, custody.ID, logs[1].Metadata["document_id"])
}

func TestStudentDocumentService_AuditTrailAndSoftDelete(t *testing.T) {
	s := newStudentDocumentScenario(t)
	office := s.actor("users:update")

	doc := s.create(t, userModels.StudentDocumentCategorySonstiges, office)

	rows := s.auditRows(t)
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].OldValue)
	require.NotNil(t, rows[0].NewValue)
	assert.Contains(t, *rows[0].NewValue, "sonstiges-datei.pdf")
	assert.Equal(t, "Test Person", rows[0].EditedByName)

	deleted, err := s.svc.DeleteStudentDocument(s.ctx, s.studentID, doc.ID, office)
	require.NoError(t, err)
	require.NotNil(t, deleted.DeletedAt)

	rows = s.auditRows(t)
	require.Len(t, rows, 2, "the delete must leave its own trail entry")
	require.NotNil(t, rows[1].OldValue)
	assert.Contains(t, *rows[1].OldValue, "sonstiges-datei.pdf")
	assert.Nil(t, rows[1].NewValue)

	// The row survives as the record that the document existed, but it is gone
	// from the normal view and pending byte removal.
	docs, _, err := s.svc.ListStudentDocuments(s.ctx, s.studentID, "", office)
	require.NoError(t, err)
	assert.Empty(t, docs)

	pending, err := s.svc.ListDeletedStudentDocumentsPendingFileCleanup(s.ctx, s.studentID, office)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, doc.ID, pending[0].ID)
}

// TestStudentDocumentService_UploadIntentIsSettledOnSuccess pins the ordering
// that makes an interrupted upload recoverable: the intent exists before the
// object is written and is settled by the metadata transaction, so a
// successful upload leaves nothing for the cleanup scheduler to delete.
func TestStudentDocumentService_UploadIntentIsSettledOnSuccess(t *testing.T) {
	s := newStudentDocumentScenario(t)
	office := s.actor("users:update")

	input := s.input(userModels.StudentDocumentCategorySchwimmerlaubnis)
	require.NoError(t, s.svc.QueueStudentDocumentFileCleanup(s.ctx, s.studentID, input.FilenameStored))

	_, err := s.svc.CreateStudentDocument(s.ctx, input, office)
	require.NoError(t, err)

	// Make every intent eligible, then confirm the settled one stays out.
	require.NoError(t, s.svc.ActivateQueuedCleanup(s.ctx, input.FilenameStored))
	queued, err := s.svc.ListQueuedStudentDocumentFileCleanups(s.ctx)
	require.NoError(t, err)
	for _, cleanup := range queued {
		assert.NotEqual(t, input.FilenameStored, cleanup.FilenameStored,
			"a committed upload must not be reclaimed by the cleanup pass")
	}
}

// TestStudentDocumentService_QueueCleanupForAllDocuments covers the child
// deletion path. Documents cascade away with the child, so the intents queued
// here are the only thing left that can get the bytes off disk.
func TestStudentDocumentService_QueueCleanupForAllDocuments(t *testing.T) {
	s := newStudentDocumentScenario(t)
	office := s.actor("users:update")

	first := s.create(t, userModels.StudentDocumentCategorySonstiges, office)
	second := s.create(t, userModels.StudentDocumentCategoryAbholvollmacht, office)

	require.NoError(t, s.svc.QueueCleanupForAllDocuments(s.ctx, s.studentID))

	queued, err := s.svc.ListQueuedStudentDocumentFileCleanups(s.ctx)
	require.NoError(t, err)
	names := make([]string, 0, len(queued))
	for _, cleanup := range queued {
		names = append(names, cleanup.FilenameStored)
	}
	assert.Contains(t, names, first.FilenameStored)
	assert.Contains(t, names, second.FilenameStored)
}
