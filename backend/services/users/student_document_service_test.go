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
	groupID   int64
}

// stubDocumentUserContext answers the only identity question the document
// service asks since #2329: does the caller hold a staff record in this tenant.
// A nil staff models a guest or guardian account — which children a staff
// member may reach is no longer a group question.
type stubDocumentUserContext struct {
	staff *userModels.Staff
}

func (s stubDocumentUserContext) GetCurrentStaff(context.Context) (*userModels.Staff, error) {
	return s.staff, nil
}

func newStudentDocumentScenario(t *testing.T) *studentDocumentScenario {
	t.Helper()

	db := testpkg.SetupTestDB(t)

	suffix := time.Now().UnixNano()
	group := testpkg.CreateTestEducationGroup(t, db, fmt.Sprintf("Dokumente-Gruppe-%d", suffix))
	student := testpkg.CreateTestStudent(t, db, "Dokumente", fmt.Sprintf("Kind-%d", suffix), "1a")
	// The child sits in a group so the fixture keeps modelling the ordinary
	// case; since #2329 the group no longer takes part in the access decision.
	assignStudentGroup(t, db, student.ID, group.ID)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("kind-dokumente-%d@example.test", suffix))
	t.Cleanup(func() {
		cleanupStudentDocumentRows(t, db, student.ID)
		testpkg.CleanupActivityFixtures(t, db, student.ID)
		testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)
		testpkg.CleanupTableRecords(t, db, "education.groups", group.ID)
	})

	repos := repositories.NewFactory(db)
	svc := usersSvc.NewStudentDocumentService(
		db,
		repos.StudentDocument,
		repos.Student,
		repos.StudentFieldEdit,
		repos.DataAccessLog,
		stubDocumentUserContext{staff: &userModels.Staff{}},
		nil,
	)

	return &studentDocumentScenario{
		db:        db,
		svc:       svc,
		ctx:       testpkg.TenantContext(student.TenantID),
		studentID: student.ID,
		account:   account.ID,
		groupID:   group.ID,
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

// create mirrors what the upload handler does, intent first. Skipping that
// step in tests hides the whole class of bug where a later re-queue collides
// with the settled intent this leaves behind.
func (s *studentDocumentScenario) create(t *testing.T, category string, actor usersSvc.StudentDocumentActor) *userModels.StudentDocument {
	t.Helper()
	input := s.input(category)
	require.NoError(t, s.svc.QueueStudentDocumentFileCleanup(s.ctx, s.studentID, input.FilenameStored))
	doc, err := s.svc.CreateStudentDocument(s.ctx, input, actor)
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
		Where(`"student_field_edit".field_name LIKE ?`, auditModels.StudentFieldDocumentPrefix+"%").
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
	t.Parallel()

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
	t.Parallel()

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

	// The child belongs in the column, which is what a per-child disclosure
	// report reads and what the FK detaches on deletion. A copy in the JSONB
	// would outlive the child and defeat that.
	for _, entry := range logs {
		require.NotNil(t, entry.StudentID, "a child document download must name its child")
		assert.Equal(t, s.studentID, *entry.StudentID)
		assert.NotContains(t, entry.Metadata, "student_id",
			"the child must not be duplicated into metadata, which no deletion can reach")
	}
}

func TestStudentDocumentService_AuditTrailAndSoftDelete(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

// TestStudentDocumentService_NonStaffCallerIsUnreachable covers the per-child
// gate. The route permissions only say the caller may open documents at all;
// whether the caller is staff of this tenant is a separate question, and
// without this a guest or guardian account holding users:update could read and
// delete the paperwork of every child in the school.
func TestStudentDocumentService_NonStaffCallerIsUnreachable(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)

	// Same permissions, but no staff record in the tenant.
	repos := repositories.NewFactory(s.db)
	outsider := usersSvc.NewStudentDocumentService(
		s.db,
		repos.StudentDocument,
		repos.Student,
		repos.StudentFieldEdit,
		repos.DataAccessLog,
		stubDocumentUserContext{},
		nil,
	)

	office := s.actor("users:update")
	doc := s.create(t, userModels.StudentDocumentCategoryBetreuungsvertrag, office)

	_, _, err := outsider.ListStudentDocuments(s.ctx, s.studentID, "", office)
	require.ErrorIs(t, err, usersSvc.ErrStudentDocumentNoAccess)

	_, err = outsider.ResolveStudentDocumentDownload(s.ctx, s.studentID, doc.ID, office)
	require.ErrorIs(t, err, usersSvc.ErrStudentDocumentNoAccess)

	_, err = outsider.DeleteStudentDocument(s.ctx, s.studentID, doc.ID, office)
	require.ErrorIs(t, err, usersSvc.ErrStudentDocumentNoAccess)

	_, err = outsider.CreateStudentDocument(s.ctx, s.input(userModels.StudentDocumentCategorySonstiges), office)
	require.ErrorIs(t, err, usersSvc.ErrStudentDocumentNoAccess)

	// The document is untouched: the refusals are refusals, not silent no-ops.
	docs, _, err := s.svc.ListStudentDocuments(s.ctx, s.studentID, "", office)
	require.NoError(t, err)
	require.Len(t, docs, 1)

	// An admin reaches every child without a staff record at all — that is the
	// office role the tab exists for.
	adminDocs, _, err := outsider.ListStudentDocuments(s.ctx, s.studentID, "", s.actor("admin:*"))
	require.NoError(t, err)
	assert.Len(t, adminDocs, 1)
}

// TestStudentDocumentService_AuditFieldCarriesCategory pins the property the
// change-history filter depends on. Without the category in the field name a
// reader cannot tell a Sorgerechtsnachweis row from a Betreuungsvertrag row,
// and the history becomes a way around the document permissions: a supervisor
// without student_documents:health would read "Ärztliches Attest:
// Attest_Epilepsie.pdf" in the Historie tab while the Dokumente tab hides that
// very document.
func TestStudentDocumentService_AuditFieldCarriesCategory(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)

	s.create(t, userModels.StudentDocumentCategoryAttest, s.actor("student_documents:health"))
	s.create(t, userModels.StudentDocumentCategoryBetreuungsvertrag, s.actor("users:update"))

	rows := s.auditRows(t)
	require.Len(t, rows, 2)

	fields := []string{rows[0].FieldName, rows[1].FieldName}
	assert.Contains(t, fields, auditModels.StudentDocumentField(userModels.StudentDocumentCategoryAttest))
	assert.Contains(t, fields, auditModels.StudentDocumentField(userModels.StudentDocumentCategoryBetreuungsvertrag))

	// And the category survives the round trip a reader performs.
	for _, row := range rows {
		category := auditModels.StudentDocumentCategoryFromField(row.FieldName)
		require.NotEmpty(t, category, "every document row must name its category")
		assert.True(t, userModels.IsValidStudentDocumentCategory(category))
	}

	// The office permission covers the contract but not the medical note.
	office := []string{"users:update"}
	assert.True(t, usersSvc.CanSeeStudentDocumentCategory(userModels.StudentDocumentCategoryBetreuungsvertrag, office))
	assert.False(t, usersSvc.CanSeeStudentDocumentCategory(userModels.StudentDocumentCategoryAttest, office))
	assert.False(t, usersSvc.CanSeeStudentDocumentCategory(userModels.StudentDocumentCategorySorgerecht, office))
	assert.False(t, usersSvc.CanSeeEveryStudentDocumentCategory(office))
	assert.True(t, usersSvc.CanSeeEveryStudentDocumentCategory([]string{"admin:*"}))
}

// TestStudentDocumentService_RefusesToWriteWithoutAnAuditTrail pins the rule
// that makes the Dokumente tab defensible: a service wired without its audit
// repository must refuse the change rather than perform it silently. An
// unlogged upload or deletion of a child's paperwork is worse than a failed
// request.
func TestStudentDocumentService_RefusesToWriteWithoutAnAuditTrail(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)
	repos := repositories.NewFactory(s.db)
	unaudited := usersSvc.NewStudentDocumentService(
		s.db,
		repos.StudentDocument,
		repos.Student,
		nil,
		repos.DataAccessLog,
		stubDocumentUserContext{staff: &userModels.Staff{}},
		nil,
	)
	office := s.actor("users:update")

	_, err := unaudited.CreateStudentDocument(s.ctx, s.input(userModels.StudentDocumentCategorySonstiges), office)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing unaudited change")

	doc := s.create(t, userModels.StudentDocumentCategorySonstiges, office)
	_, err = unaudited.DeleteStudentDocument(s.ctx, s.studentID, doc.ID, office)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing unaudited change")

	// The document survived the refused deletion.
	docs, _, err := s.svc.ListStudentDocuments(s.ctx, s.studentID, "", office)
	require.NoError(t, err)
	require.Len(t, docs, 1)

	// The same holds for a sensitive download without a data-access log: no
	// row, no file.
	unlogged := usersSvc.NewStudentDocumentService(
		s.db,
		repos.StudentDocument,
		repos.Student,
		repos.StudentFieldEdit,
		nil,
		stubDocumentUserContext{staff: &userModels.Staff{}},
		nil,
	)
	health := s.actor("student_documents:health")
	attest := s.create(t, userModels.StudentDocumentCategoryAttest, health)
	_, err = unlogged.ResolveStudentDocumentDownload(s.ctx, s.studentID, attest.ID, health)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing unaudited document download")

	// An everyday category needs no access-log row, so it is still served.
	everyday := s.create(t, userModels.StudentDocumentCategoryAbholvollmacht, office)
	_, err = unlogged.ResolveStudentDocumentDownload(s.ctx, s.studentID, everyday.ID, office)
	require.NoError(t, err)
}

// TestStudentDocumentService_RequiresAnActingAccount covers the other half of
// the audit contract: every write and every logged download has to name the
// person who performed it, so an anonymous actor is refused up front.
func TestStudentDocumentService_RequiresAnActingAccount(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)
	office := s.actor("users:update")
	anonymous := usersSvc.StudentDocumentActor{Permissions: []string{"admin:*"}}

	_, err := s.svc.CreateStudentDocument(s.ctx, s.input(userModels.StudentDocumentCategorySonstiges), anonymous)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "actor account id is required")

	doc := s.create(t, userModels.StudentDocumentCategoryAttest, s.actor("student_documents:health"))
	_, err = s.svc.DeleteStudentDocument(s.ctx, s.studentID, doc.ID, anonymous)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "actor account id is required")

	_, err = s.svc.ResolveStudentDocumentDownload(s.ctx, s.studentID, doc.ID, anonymous)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "actor account id is required")

	// An everyday document is downloadable without an account, because nothing
	// has to be logged for it.
	everyday := s.create(t, userModels.StudentDocumentCategoryAbholvollmacht, office)
	_, err = s.svc.ResolveStudentDocumentDownload(s.ctx, s.studentID, everyday.ID, anonymous)
	require.NoError(t, err)
}

// TestStudentDocumentService_RejectsMalformedInput covers the payload checks
// that keep an unusable row out of the table. A document whose display name is
// blank cannot be identified in the UI, and a category outside the enum would
// map to the weakest of the three permissions.
func TestStudentDocumentService_RejectsMalformedInput(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)
	office := s.actor("users:update")

	unknown := s.input(userModels.StudentDocumentCategorySonstiges)
	unknown.Category = "erfundene_kategorie"
	_, err := s.svc.CreateStudentDocument(s.ctx, unknown, office)
	require.ErrorIs(t, err, usersSvc.ErrStudentDocumentInvalid)

	blank := s.input(userModels.StudentDocumentCategorySonstiges)
	blank.FilenameDisplay = "   "
	_, err = s.svc.CreateStudentDocument(s.ctx, blank, office)
	require.ErrorIs(t, err, usersSvc.ErrStudentDocumentInvalid)

	// Passes the input checks but fails the model's own validation, which is
	// the last guard before the insert.
	nameless := s.input(userModels.StudentDocumentCategorySonstiges)
	nameless.FilenameStored = ""
	_, err = s.svc.CreateStudentDocument(s.ctx, nameless, office)
	require.ErrorIs(t, err, usersSvc.ErrStudentDocumentInvalid)

	// The list filter validates its category too, before it decides whether the
	// caller may see it.
	_, _, err = s.svc.ListStudentDocuments(s.ctx, s.studentID, "erfundene_kategorie", office)
	require.ErrorIs(t, err, usersSvc.ErrStudentDocumentInvalid)

	// A cleanup intent needs both halves or it can never be acted on.
	require.ErrorIs(t, s.svc.QueueStudentDocumentFileCleanup(s.ctx, 0, "x.pdf"), usersSvc.ErrStudentDocumentInvalid)
	require.ErrorIs(t, s.svc.QueueStudentDocumentFileCleanup(s.ctx, s.studentID, "  "), usersSvc.ErrStudentDocumentInvalid)

	// None of it reached the table.
	docs, _, err := s.svc.ListStudentDocuments(s.ctx, s.studentID, "", s.actor("admin:*"))
	require.NoError(t, err)
	assert.Empty(t, docs)
}

// TestStudentDocumentService_UnknownChildAndGrouplessChild covers the two ends
// of the per-child gate for a non-admin staff caller: an unknown child is
// refused, while a child without an education group is ordinary paperwork
// (#2329 — group-less children used to be admin-only territory).
func TestStudentDocumentService_UnknownChildAndGrouplessChild(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)
	office := s.actor("users:update")

	_, _, err := s.svc.ListStudentDocuments(s.ctx, s.studentID+9_000_000, "", office)
	require.Error(t, err, "an unknown child must not read as an empty document list")

	groupless := testpkg.CreateTestStudent(t, s.db, "Ohne", fmt.Sprintf("Gruppe-%d", time.Now().UnixNano()), "1c")
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, s.db, groupless.ID) })

	_, _, err = s.svc.ListStudentDocuments(s.ctx, groupless.ID, "", office)
	require.NoError(t, err, "a staff member reaches a child that has no group yet")

	_, _, err = s.svc.ListStudentDocuments(s.ctx, groupless.ID, "", s.actor("admin:*"))
	require.NoError(t, err)
}

// TestStudentDocumentService_CleanupRetryPathIsAuthorized covers the retry a
// page view performs after a failed unlink. It has to reach a soft-deleted row,
// and it has to apply the same category authority the tab does — otherwise
// "retry the cleanup" becomes a way to confirm that a document exists.
func TestStudentDocumentService_CleanupRetryPathIsAuthorized(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)
	office := s.actor("users:update")
	health := s.actor("student_documents:health")

	attest := s.create(t, userModels.StudentDocumentCategoryAttest, health)
	_, err := s.svc.DeleteStudentDocument(s.ctx, s.studentID, attest.ID, health)
	require.NoError(t, err)

	resolved, err := s.svc.ResolveStudentDocumentCleanup(s.ctx, s.studentID, attest.ID, health)
	require.NoError(t, err)
	assert.Equal(t, attest.FilenameStored, resolved.FilenameStored)

	_, err = s.svc.ResolveStudentDocumentCleanup(s.ctx, s.studentID, attest.ID, office)
	require.ErrorIs(t, err, usersSvc.ErrStudentDocumentForbidden)

	_, err = s.svc.ResolveStudentDocumentCleanup(s.ctx, s.studentID, attest.ID+9_000_000, health)
	require.Error(t, err)

	// The tenant-wide sweep the scheduler runs sees the same row without any
	// actor at all — it works on behalf of nobody.
	sweep, err := s.svc.ListDeletedStudentDocumentsPendingFileCleanups(s.ctx)
	require.NoError(t, err)
	assert.Contains(t, documentIDsOf(sweep), attest.ID)

	// Marking the bytes gone retires it from both views.
	require.NoError(t, s.svc.MarkFileDeleted(s.ctx, attest.ID))
	sweep, err = s.svc.ListDeletedStudentDocumentsPendingFileCleanups(s.ctx)
	require.NoError(t, err)
	assert.NotContains(t, documentIDsOf(sweep), attest.ID)
	retry, err := s.svc.ListDeletedStudentDocumentsPendingFileCleanup(s.ctx, s.studentID, health)
	require.NoError(t, err)
	assert.Empty(t, retry)
}

// TestStudentDocumentService_SettlesIntentsByIDAndName covers the two ways the
// coordinator retires an intent: by ID after the scheduler unlinked the object,
// and by stored name when the upload that owns it committed.
func TestStudentDocumentService_SettlesIntentsByIDAndName(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)

	byName := fmt.Sprintf("settle-name-%d.pdf", time.Now().UnixNano())
	byID := fmt.Sprintf("settle-id-%d.pdf", time.Now().UnixNano())
	require.NoError(t, s.svc.QueueStudentDocumentFileCleanup(s.ctx, s.studentID, byName))
	require.NoError(t, s.svc.QueueStudentDocumentFileCleanup(s.ctx, s.studentID, byID))
	// Both are queued five minutes out; activating makes them visible to a
	// sweep, which is the state the coordinator settles them from.
	require.NoError(t, s.svc.ActivateQueuedCleanup(s.ctx, byName))
	require.NoError(t, s.svc.ActivateQueuedCleanup(s.ctx, byID))

	queued, err := s.svc.ListQueuedStudentDocumentFileCleanups(s.ctx)
	require.NoError(t, err)
	require.Contains(t, cleanupNamesOf(queued), byName)
	var target int64
	for _, cleanup := range queued {
		if cleanup.FilenameStored == byID {
			target = cleanup.ID
		}
	}
	require.NotZero(t, target)

	require.NoError(t, s.svc.MarkQueuedCleanupCompleteByFilename(s.ctx, byName))
	require.NoError(t, s.svc.MarkQueuedCleanupComplete(s.ctx, target))

	queued, err = s.svc.ListQueuedStudentDocumentFileCleanups(s.ctx)
	require.NoError(t, err)
	assert.NotContains(t, cleanupNamesOf(queued), byName)
	assert.NotContains(t, cleanupNamesOf(queued), byID)
}

// TestStudentDocumentService_AuditNamesAnUnknownActor pins the fallback for an
// actor whose display name never reached the service. The trail must still say
// who acted, and "" in a history column reads as a bug rather than as a person.
func TestStudentDocumentService_AuditNamesAnUnknownActor(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)
	nameless := usersSvc.StudentDocumentActor{
		AccountID:   s.account,
		Permissions: []string{"users:update"},
	}

	s.create(t, userModels.StudentDocumentCategorySonstiges, nameless)

	rows := s.auditRows(t)
	require.Len(t, rows, 1)
	assert.Equal(t, "Unbekannt", rows[0].EditedByName)
}

func documentIDsOf(docs []*userModels.StudentDocument) []int64 {
	ids := make([]int64, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}
	return ids
}

func cleanupNamesOf(cleanups []*userModels.StudentDocumentFileCleanup) []string {
	names := make([]string, 0, len(cleanups))
	for _, cleanup := range cleanups {
		names = append(names, cleanup.FilenameStored)
	}
	return names
}

// TestStudentDocumentService_AuthorizeUploadWritesNothing pins the guard the
// upload handler relies on: it must answer before any byte or cleanup row
// exists, and it must answer the same way the transaction later does.
func TestStudentDocumentService_AuthorizeUploadWritesNothing(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)
	office := s.actor("users:update")

	// Wrong category for these permissions, unknown category, and a caller
	// without a staff record — all refused up front.
	require.ErrorIs(t,
		s.svc.AuthorizeStudentDocumentUpload(s.ctx, s.studentID, userModels.StudentDocumentCategoryAttest, office),
		usersSvc.ErrStudentDocumentForbidden)
	require.ErrorIs(t,
		s.svc.AuthorizeStudentDocumentUpload(s.ctx, s.studentID, "erfundene_kategorie", office),
		usersSvc.ErrStudentDocumentInvalid)

	repos := repositories.NewFactory(s.db)
	outsider := usersSvc.NewStudentDocumentService(
		s.db,
		repos.StudentDocument,
		repos.Student,
		repos.StudentFieldEdit,
		repos.DataAccessLog,
		stubDocumentUserContext{},
		nil,
	)
	require.ErrorIs(t,
		outsider.AuthorizeStudentDocumentUpload(s.ctx, s.studentID, userModels.StudentDocumentCategorySonstiges, office),
		usersSvc.ErrStudentDocumentNoAccess)

	// None of those refusals left a trace — no document row, no cleanup intent,
	// no audit entry.
	docs, _, err := s.svc.ListStudentDocuments(s.ctx, s.studentID, "", s.actor("admin:*"))
	require.NoError(t, err)
	assert.Empty(t, docs)
	queued, err := s.svc.ListQueuedStudentDocumentFileCleanups(s.ctx)
	require.NoError(t, err)
	assert.Empty(t, queued)
	assert.Empty(t, s.auditRows(t))

	// And the permitted combination passes, so the guard is not simply closed.
	require.NoError(t,
		s.svc.AuthorizeStudentDocumentUpload(s.ctx, s.studentID, userModels.StudentDocumentCategoryBetreuungsvertrag, office))
}
