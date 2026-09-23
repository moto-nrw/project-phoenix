package contracttest_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/services"
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
	svc       careplan.StudentDocuments
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

func (s stubDocumentUserContext) HasCurrentStaff(context.Context) (bool, error) {
	return s.staff != nil, nil
}

func newStudentDocumentScenario(t *testing.T) *studentDocumentScenario {
	t.Helper()

	db := testpkg.SetupTestDB(t)

	suffix := time.Now().UnixNano()
	group := testpkg.CreateTestEducationGroup(t, db, fmt.Sprintf("Dokumente-Gruppe-%d", suffix))
	student := testpkg.CreateTestStudent(t, db, "Dokumente", fmt.Sprintf("Kind-%d", suffix), "1a")
	// The child sits in a group so the fixture keeps modelling the ordinary
	// case; since #2329 the group no longer takes part in the access decision.
	testpkg.AssignStudentGroup(t, db, student.ID, group.ID)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("kind-dokumente-%d@example.test", suffix))

	return &studentDocumentScenario{
		db:        db,
		svc:       newStudentDocuments(t, db, stubDocumentUserContext{staff: &userModels.Staff{}}),
		ctx:       testpkg.TenantContext(student.TenantID),
		studentID: student.ID,
		account:   account.ID,
		groupID:   group.ID,
	}
}

// newStudentDocuments composes the production document capability with the
// caller's staff identity stood in.
func newStudentDocuments(t *testing.T, db *bun.DB, userContext stubDocumentUserContext) careplan.StudentDocuments {
	t.Helper()
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	documents, err := services.NewStudentDocumentsTestModule(db, repos.CarePlan(), repos.Student, userContext, repos.StudentFieldEdit, repos.DataAccessLog)
	require.NoError(t, err)
	return documents
}

func (s *studentDocumentScenario) actor(perms ...string) careplan.StudentDocumentActor {
	return careplan.StudentDocumentActor{
		AccountID:   s.account,
		Name:        "Test Person",
		Role:        "test",
		Permissions: perms,
	}
}

// create mirrors what the upload handler does, intent first. Skipping that
// step in tests hides the whole class of bug where a later re-queue collides
// with the settled intent this leaves behind.
func (s *studentDocumentScenario) create(t *testing.T, category string, actor careplan.StudentDocumentActor) careplan.CareDocument {
	t.Helper()
	input := s.input(category)
	require.NoError(t, s.svc.QueueStudentDocumentFileCleanup(s.ctx, s.studentID, input.FilenameStored))
	doc, err := s.svc.CreateStudentDocument(s.ctx, input, actor)
	require.NoError(t, err)
	return doc
}

func (s *studentDocumentScenario) input(category string) careplan.CreateStudentDocumentInput {
	return careplan.CreateStudentDocumentInput{
		StudentID:       s.studentID,
		Category:        category,
		FilenameDisplay: category + "-datei.pdf",
		FilenameStored:  fmt.Sprintf("%s-%d.pdf", category, testpkg.UniqueSuffix()),
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

func studentDocumentCategoriesOf(docs []careplan.CareDocument) []string {
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
	s.create(t, careplan.StudentDocumentCategoryBetreuungsvertrag, office)
	_, err := s.svc.CreateStudentDocument(s.ctx, s.input(careplan.StudentDocumentCategoryAttest), office)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentForbidden)
	_, err = s.svc.CreateStudentDocument(s.ctx, s.input(careplan.StudentDocumentCategorySorgerecht), office)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentForbidden)

	// The dedicated permissions cover exactly their tier and nothing else.
	attest := s.create(t, careplan.StudentDocumentCategoryAttest, health)
	sorgerecht := s.create(t, careplan.StudentDocumentCategorySorgerecht, legal)
	_, err = s.svc.CreateStudentDocument(s.ctx, s.input(careplan.StudentDocumentCategorySorgerecht), health)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentForbidden)
	_, err = s.svc.CreateStudentDocument(s.ctx, s.input(careplan.StudentDocumentCategoryImpfnachweis), legal)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentForbidden)

	// List visibility follows the same mapping — a health document must not
	// even appear in the office's list.
	docs, visible, err := s.svc.ListStudentDocuments(s.ctx, s.studentID, "", office)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		careplan.StudentDocumentCategoryBetreuungsvertrag,
		careplan.StudentDocumentCategoryAbholvollmacht,
		careplan.StudentDocumentCategorySchwimmerlaubnis,
		careplan.StudentDocumentCategorySonstiges,
	}, visible)
	assert.ElementsMatch(t, []string{careplan.StudentDocumentCategoryBetreuungsvertrag}, studentDocumentCategoriesOf(docs))

	docs, visible, err = s.svc.ListStudentDocuments(s.ctx, s.studentID, "", health)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		careplan.StudentDocumentCategoryAttest,
		careplan.StudentDocumentCategoryImpfnachweis,
		careplan.StudentDocumentCategoryMedikamentenplan,
	}, visible)
	assert.ElementsMatch(t, []string{careplan.StudentDocumentCategoryAttest}, studentDocumentCategoriesOf(docs))

	docs, visible, err = s.svc.ListStudentDocuments(s.ctx, s.studentID, "", admin)
	require.NoError(t, err)
	assert.Len(t, visible, len(careplan.StudentDocumentCategories))
	assert.Len(t, docs, 3)

	// A category filter outside the caller's authority is refused; inside it
	// narrows.
	_, _, err = s.svc.ListStudentDocuments(s.ctx, s.studentID, careplan.StudentDocumentCategoryAttest, office)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentForbidden)
	docs, _, err = s.svc.ListStudentDocuments(s.ctx, s.studentID, careplan.StudentDocumentCategoryAttest, health)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, attest.ID, docs[0].ID)

	// Deleting across the tier boundary is refused too — a 403 on upload would
	// be worthless if delete were open.
	_, err = s.svc.DeleteStudentDocument(s.ctx, s.studentID, sorgerecht.ID, office)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentForbidden)
}

func TestStudentDocumentService_SensitiveDownloadsAreLogged(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)

	office := s.actor("users:update")
	health := s.actor("student_documents:health")
	legal := s.actor("student_documents:legal")

	everyday := s.create(t, careplan.StudentDocumentCategoryAbholvollmacht, office)
	attest := s.create(t, careplan.StudentDocumentCategoryAttest, health)
	custody := s.create(t, careplan.StudentDocumentCategorySorgerecht, legal)

	// Foreign permissions never reach the bytes.
	_, err := s.svc.ResolveStudentDocumentDownload(s.ctx, s.studentID, attest.ID, office)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentForbidden)

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

	doc := s.create(t, careplan.StudentDocumentCategorySonstiges, office)

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

	input := s.input(careplan.StudentDocumentCategorySchwimmerlaubnis)
	require.NoError(t, s.svc.QueueStudentDocumentFileCleanup(s.ctx, s.studentID, input.FilenameStored))

	_, err := s.svc.CreateStudentDocument(s.ctx, input, office)
	require.NoError(t, err)

	// Make every intent eligible, then confirm the settled one stays out.
	require.NoError(t, s.svc.ActivateQueuedCleanup(s.ctx, input.FilenameStored))
	removed, _ := s.sweepFiles(t)
	assert.NotContains(t, removed, input.FilenameStored,
		"a committed upload must not be reclaimed by the cleanup pass")
}

// sweepFiles runs the scheduler's recovery pass with a recording remover.
// Names in failing are refused, as a storage hiccup would refuse them.
func (s *studentDocumentScenario) sweepFiles(t *testing.T, failing ...string) ([]string, careplan.StudentDocumentFileSweep) {
	t.Helper()
	var removed []string
	sweep, err := s.svc.SweepStudentDocumentFiles(s.ctx, func(_ context.Context, tenantID int64, storedName string) error {
		assert.Positive(t, tenantID, "every object is addressed within its school")
		removed = append(removed, storedName)
		if slices.Contains(failing, storedName) {
			return errors.New("storage unavailable")
		}
		return nil
	})
	require.NoError(t, err)
	return removed, sweep
}

func TestStudentDocumentService_NonStaffCallerIsUnreachable(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)

	// Same permissions, but no staff record in the tenant.
	outsider := newStudentDocuments(t, s.db, stubDocumentUserContext{})

	office := s.actor("users:update")
	doc := s.create(t, careplan.StudentDocumentCategoryBetreuungsvertrag, office)

	_, _, err := outsider.ListStudentDocuments(s.ctx, s.studentID, "", office)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentNoAccess)

	_, err = outsider.ResolveStudentDocumentDownload(s.ctx, s.studentID, doc.ID, office)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentNoAccess)

	_, err = outsider.DeleteStudentDocument(s.ctx, s.studentID, doc.ID, office)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentNoAccess)

	_, err = outsider.CreateStudentDocument(s.ctx, s.input(careplan.StudentDocumentCategorySonstiges), office)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentNoAccess)

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

	s.create(t, careplan.StudentDocumentCategoryAttest, s.actor("student_documents:health"))
	s.create(t, careplan.StudentDocumentCategoryBetreuungsvertrag, s.actor("users:update"))

	rows := s.auditRows(t)
	require.Len(t, rows, 2)

	fields := []string{rows[0].FieldName, rows[1].FieldName}
	assert.Contains(t, fields, auditModels.StudentDocumentField(careplan.StudentDocumentCategoryAttest))
	assert.Contains(t, fields, auditModels.StudentDocumentField(careplan.StudentDocumentCategoryBetreuungsvertrag))

	// And the category survives the round trip a reader performs.
	for _, row := range rows {
		category := auditModels.StudentDocumentCategoryFromField(row.FieldName)
		require.NotEmpty(t, category, "every document row must name its category")
		assert.True(t, careplan.IsValidStudentDocumentCategory(category))
	}

	// The office permission covers the contract but not the medical note.
	office := []string{"users:update"}
	assert.True(t, s.svc.CanSeeStudentDocumentCategory(careplan.StudentDocumentCategoryBetreuungsvertrag, office))
	assert.False(t, s.svc.CanSeeStudentDocumentCategory(careplan.StudentDocumentCategoryAttest, office))
	assert.False(t, s.svc.CanSeeStudentDocumentCategory(careplan.StudentDocumentCategorySorgerecht, office))
	assert.False(t, s.svc.CanSeeEveryStudentDocumentCategory(office))
	assert.True(t, s.svc.CanSeeEveryStudentDocumentCategory([]string{"admin:*"}))
}

// TestStudentDocumentService_RefusesToWriteWithoutAnAuditTrail pins the rule
// that makes the Dokumente tab defensible: a capability without its change
// history or data-access log must not exist at all. An unlogged upload,
// deletion or sensitive download of a child's paperwork is worse than a
// server that refuses to start, so the composition fails before any request
// can reach it.
func TestStudentDocumentService_RefusesToWriteWithoutAnAuditTrail(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	staff := stubDocumentUserContext{staff: &userModels.Staff{}}

	unaudited, err := services.NewStudentDocumentsTestModule(db, repos.CarePlan(), repos.Student, staff, nil, repos.DataAccessLog)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing unaudited changes")
	assert.Nil(t, unaudited)

	unlogged, err := services.NewStudentDocumentsTestModule(db, repos.CarePlan(), repos.Student, staff, repos.StudentFieldEdit, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing unaudited changes")
	assert.Nil(t, unlogged)
}

// TestStudentDocumentService_RequiresAnActingAccount covers the other half of
// the audit contract: every write and every logged download has to name the
// person who performed it, so an anonymous actor is refused up front.
func TestStudentDocumentService_RequiresAnActingAccount(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)
	office := s.actor("users:update")
	anonymous := careplan.StudentDocumentActor{Permissions: []string{"admin:*"}}

	_, err := s.svc.CreateStudentDocument(s.ctx, s.input(careplan.StudentDocumentCategorySonstiges), anonymous)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "actor account id is required")

	doc := s.create(t, careplan.StudentDocumentCategoryAttest, s.actor("student_documents:health"))
	_, err = s.svc.DeleteStudentDocument(s.ctx, s.studentID, doc.ID, anonymous)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "actor account id is required")

	_, err = s.svc.ResolveStudentDocumentDownload(s.ctx, s.studentID, doc.ID, anonymous)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "actor account id is required")

	// An everyday document is downloadable without an account, because nothing
	// has to be logged for it.
	everyday := s.create(t, careplan.StudentDocumentCategoryAbholvollmacht, office)
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

	unknown := s.input(careplan.StudentDocumentCategorySonstiges)
	unknown.Category = "erfundene_kategorie"
	_, err := s.svc.CreateStudentDocument(s.ctx, unknown, office)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentInvalid)

	blank := s.input(careplan.StudentDocumentCategorySonstiges)
	blank.FilenameDisplay = "   "
	_, err = s.svc.CreateStudentDocument(s.ctx, blank, office)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentInvalid)

	// Passes the input checks but fails the model's own validation, which is
	// the last guard before the insert.
	nameless := s.input(careplan.StudentDocumentCategorySonstiges)
	nameless.FilenameStored = ""
	_, err = s.svc.CreateStudentDocument(s.ctx, nameless, office)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentInvalid)

	// The list filter validates its category too, before it decides whether the
	// caller may see it.
	_, _, err = s.svc.ListStudentDocuments(s.ctx, s.studentID, "erfundene_kategorie", office)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentInvalid)

	// A cleanup intent needs both halves or it can never be acted on.
	require.ErrorIs(t, s.svc.QueueStudentDocumentFileCleanup(s.ctx, 0, "x.pdf"), careplan.ErrStudentDocumentInvalid)
	require.ErrorIs(t, s.svc.QueueStudentDocumentFileCleanup(s.ctx, s.studentID, "  "), careplan.ErrStudentDocumentInvalid)

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

	attest := s.create(t, careplan.StudentDocumentCategoryAttest, health)
	_, err := s.svc.DeleteStudentDocument(s.ctx, s.studentID, attest.ID, health)
	require.NoError(t, err)

	resolved, err := s.svc.ResolveStudentDocumentCleanup(s.ctx, s.studentID, attest.ID, health)
	require.NoError(t, err)
	assert.Equal(t, attest.FilenameStored, resolved.FilenameStored)

	_, err = s.svc.ResolveStudentDocumentCleanup(s.ctx, s.studentID, attest.ID, office)
	require.ErrorIs(t, err, careplan.ErrStudentDocumentForbidden)

	_, err = s.svc.ResolveStudentDocumentCleanup(s.ctx, s.studentID, attest.ID+9_000_000, health)
	require.Error(t, err)

	// The tenant-wide sweep the scheduler runs sees the same row without any
	// actor at all — it works on behalf of nobody. A refused removal leaves
	// the row for the next pass and is reported, not swallowed.
	removed, sweep := s.sweepFiles(t, attest.FilenameStored)
	assert.Contains(t, removed, attest.FilenameStored)
	require.Len(t, sweep.Failures, 1)
	assert.Equal(t, attest.ID, sweep.Failures[0].DocumentID)
	assert.Equal(t, careplan.StudentDocumentSweepRemove, sweep.Failures[0].Stage)
	retry, err := s.svc.ListDeletedStudentDocumentsPendingFileCleanup(s.ctx, s.studentID, health)
	require.NoError(t, err)
	require.Len(t, retry, 1, "a failed removal stays pending")

	// Marking the bytes gone retires it from both views.
	require.NoError(t, s.svc.MarkFileDeleted(s.ctx, attest.ID))
	removed, _ = s.sweepFiles(t)
	assert.NotContains(t, removed, attest.FilenameStored)
	retry, err = s.svc.ListDeletedStudentDocumentsPendingFileCleanup(s.ctx, s.studentID, health)
	require.NoError(t, err)
	assert.Empty(t, retry)
}

// TestStudentDocumentService_SettlesIntentsByIDAndName covers the two ways an
// intent is retired: by the scheduler's sweep once it unlinked the object, and
// by stored name when the upload that owns it committed.
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

	require.NoError(t, s.svc.MarkQueuedCleanupCompleteByFilename(s.ctx, byName))
	removed, sweep := s.sweepFiles(t)
	assert.Equal(t, []string{byID}, removed, "the settled upload's object must not be touched")
	assert.Equal(t, 1, sweep.Removed)
	assert.Empty(t, sweep.Failures)

	removed, sweep = s.sweepFiles(t)
	assert.Empty(t, removed, "a swept intent is settled, not retried")
	assert.Zero(t, sweep.OrphansListed)
}

// TestStudentDocumentService_AuditNamesAnUnknownActor pins the fallback for an
// actor whose display name never reached the service. The trail must still say
// who acted, and "" in a history column reads as a bug rather than as a person.
func TestStudentDocumentService_AuditNamesAnUnknownActor(t *testing.T) {
	t.Parallel()

	s := newStudentDocumentScenario(t)
	nameless := careplan.StudentDocumentActor{
		AccountID:   s.account,
		Permissions: []string{"users:update"},
	}

	s.create(t, careplan.StudentDocumentCategorySonstiges, nameless)

	rows := s.auditRows(t)
	require.Len(t, rows, 1)
	assert.Equal(t, "Unbekannt", rows[0].EditedByName)
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
		s.svc.AuthorizeStudentDocumentUpload(s.ctx, s.studentID, careplan.StudentDocumentCategoryAttest, office),
		careplan.ErrStudentDocumentForbidden)
	require.ErrorIs(t,
		s.svc.AuthorizeStudentDocumentUpload(s.ctx, s.studentID, "erfundene_kategorie", office),
		careplan.ErrStudentDocumentInvalid)

	outsider := newStudentDocuments(t, s.db, stubDocumentUserContext{})
	require.ErrorIs(t,
		outsider.AuthorizeStudentDocumentUpload(s.ctx, s.studentID, careplan.StudentDocumentCategorySonstiges, office),
		careplan.ErrStudentDocumentNoAccess)

	// None of those refusals left a trace — no document row, no cleanup intent,
	// no audit entry.
	docs, _, err := s.svc.ListStudentDocuments(s.ctx, s.studentID, "", s.actor("admin:*"))
	require.NoError(t, err)
	assert.Empty(t, docs)
	_, sweep := s.sweepFiles(t)
	assert.Zero(t, sweep.OrphansListed)
	assert.Empty(t, s.auditRows(t))

	// And the permitted combination passes, so the guard is not simply closed.
	require.NoError(t,
		s.svc.AuthorizeStudentDocumentUpload(s.ctx, s.studentID, careplan.StudentDocumentCategoryBetreuungsvertrag, office))
}
