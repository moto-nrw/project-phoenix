package users_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Staff documents (#1424): per-category authority (AU → staff_documents:health,
// Lohn → staff:financial, rest → users:update) is enforced in the service —
// uploads, downloads, deletes AND list visibility. Every upload/delete writes
// a Stammdaten audit row; sensitive downloads write a data-access log row.

type staffDocumentScenario struct {
	db      *bun.DB
	repos   *repositories.Factory
	svc     usersSvc.StaffDocumentService
	ctx     context.Context
	staffID int64
	account int64
}

func newStaffDocumentScenario(t *testing.T) *staffDocumentScenario {
	t.Helper()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)
	svc := usersSvc.NewStaffDocumentService(
		db,
		repos.StaffDocument,
		repos.Staff,
		repos.StaffMasterData,
		repos.StaffMasterDataChange,
		repos.DataAccessLog,
		nil,
	)

	suffix := time.Now().UnixNano()
	staff := testpkg.CreateTestStaff(t, db, "Dokumente", fmt.Sprintf("Service-%d", suffix))
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("dokumente-%d@example.test", suffix))
	t.Cleanup(func() {
		cleanupStaffDocumentRows(t, db, staff.ID)
		testpkg.CleanupStaffFixtures(t, db, staff.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
	})

	return &staffDocumentScenario{
		db:      db,
		repos:   repos,
		svc:     svc,
		ctx:     testpkg.Ctx(t),
		staffID: staff.ID,
		account: account.ID,
	}
}

func cleanupStaffDocumentRows(t *testing.T, db *bun.DB, staffID int64) {
	t.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `DELETE FROM users.staff_documents WHERE staff_id = ?`, staffID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM audit.staff_master_data_changes WHERE staff_id = ?`, staffID)
	require.NoError(t, err)
}

func (s *staffDocumentScenario) actor(perms ...string) usersSvc.StaffDocumentActor {
	return usersSvc.StaffDocumentActor{AccountID: s.account, Role: "test", Permissions: perms}
}

func (s *staffDocumentScenario) create(t *testing.T, category string, actor usersSvc.StaffDocumentActor) *usersSvc.StaffDocumentInfo {
	t.Helper()
	info, err := s.svc.CreateStaffDocument(s.ctx, usersSvc.CreateStaffDocumentInput{
		StaffID:         s.staffID,
		Category:        category,
		FilenameDisplay: category + "-datei.pdf",
		FilenameStored:  fmt.Sprintf("%s-%d.pdf", category, time.Now().UnixNano()),
		SizeBytes:       42,
		ContentType:     "application/pdf",
	}, actor)
	require.NoError(t, err)
	return info
}

func (s *staffDocumentScenario) auditRows(t *testing.T) []*auditModels.StaffMasterDataChange {
	t.Helper()
	var rows []*auditModels.StaffMasterDataChange
	err := s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`audit.staff_master_data_changes AS "staff_master_data_change"`).
		Where(`"staff_master_data_change".staff_id = ?`, s.staffID).
		Order("id ASC").
		Scan(context.Background())
	require.NoError(t, err)
	return rows
}

func documentCategories(infos []*usersSvc.StaffDocumentInfo) []string {
	out := make([]string, 0, len(infos))
	for _, info := range infos {
		out = append(out, info.Document.Category)
	}
	return out
}

func TestStaffDocumentService_CategoryAuthority(t *testing.T) {
	t.Parallel()

	s := newStaffDocumentScenario(t)

	directory := s.actor("users:update")
	health := s.actor("staff_documents:health")
	payroll := s.actor("staff:financial")
	admin := s.actor("admin:*")

	// Directory maintainers cover the four general categories only.
	s.create(t, userModels.StaffDocumentCategoryZeugnis, directory)
	_, err := s.svc.CreateStaffDocument(s.ctx, usersSvc.CreateStaffDocumentInput{
		StaffID:         s.staffID,
		Category:        userModels.StaffDocumentCategoryAUBescheinigung,
		FilenameDisplay: "au.pdf",
		FilenameStored:  fmt.Sprintf("au-%d.pdf", time.Now().UnixNano()),
		SizeBytes:       1,
		ContentType:     "application/pdf",
	}, directory)
	require.ErrorIs(t, err, usersSvc.ErrStaffDocumentForbidden)

	// The dedicated permissions cover exactly their category.
	auInfo := s.create(t, userModels.StaffDocumentCategoryAUBescheinigung, health)
	lohnInfo := s.create(t, userModels.StaffDocumentCategoryLohnabrechnung, payroll)
	_, err = s.svc.CreateStaffDocument(s.ctx, usersSvc.CreateStaffDocumentInput{
		StaffID:         s.staffID,
		Category:        userModels.StaffDocumentCategoryZeugnis,
		FilenameDisplay: "zeugnis.pdf",
		FilenameStored:  fmt.Sprintf("z-%d.pdf", time.Now().UnixNano()),
		SizeBytes:       1,
		ContentType:     "application/pdf",
	}, health)
	require.ErrorIs(t, err, usersSvc.ErrStaffDocumentForbidden)

	// List visibility follows the same mapping.
	infos, visible, err := s.svc.ListStaffDocuments(s.ctx, s.staffID, "", directory)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		userModels.StaffDocumentCategoryArbeitsvertrag,
		userModels.StaffDocumentCategoryZeugnis,
		userModels.StaffDocumentCategoryBewerbung,
		userModels.StaffDocumentCategorySonstiges,
	}, visible)
	assert.ElementsMatch(t, []string{userModels.StaffDocumentCategoryZeugnis}, documentCategories(infos))

	infos, visible, err = s.svc.ListStaffDocuments(s.ctx, s.staffID, "", payroll)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{userModels.StaffDocumentCategoryLohnabrechnung}, visible)
	assert.ElementsMatch(t, []string{userModels.StaffDocumentCategoryLohnabrechnung}, documentCategories(infos))

	infos, visible, err = s.svc.ListStaffDocuments(s.ctx, s.staffID, "", admin)
	require.NoError(t, err)
	assert.Len(t, visible, len(userModels.StaffDocumentCategories))
	assert.Len(t, infos, 3)

	// A category filter outside the caller's authority is refused, inside it
	// narrows.
	_, _, err = s.svc.ListStaffDocuments(s.ctx, s.staffID, userModels.StaffDocumentCategoryAUBescheinigung, directory)
	require.ErrorIs(t, err, usersSvc.ErrStaffDocumentForbidden)
	infos, _, err = s.svc.ListStaffDocuments(s.ctx, s.staffID, userModels.StaffDocumentCategoryAUBescheinigung, health)
	require.NoError(t, err)
	require.Len(t, infos, 1)
	assert.Equal(t, auInfo.Document.ID, infos[0].Document.ID)

	// Downloads: the general category serves without an access-log row, the
	// sensitive ones refuse foreign permissions and log for the right ones.
	_, err = s.svc.ResolveStaffDocumentDownload(s.ctx, s.staffID, auInfo.Document.ID, directory)
	require.ErrorIs(t, err, usersSvc.ErrStaffDocumentForbidden)

	before := len(s.accessLogRowsFor(t))
	_, err = s.svc.ResolveStaffDocumentDownload(s.ctx, s.staffID, lohnInfo.Document.ID, payroll)
	require.NoError(t, err)
	logs := s.accessLogRowsFor(t)
	require.Len(t, logs, before+1)
	last := logs[len(logs)-1]
	assert.Equal(t, auditModels.ResourceTypeStaffDocumentDownload, last.ResourceType)
	assert.EqualValues(t, lohnInfo.Document.ID, last.Metadata["document_id"])
}

func (s *staffDocumentScenario) accessLogRowsFor(t *testing.T) []*auditModels.DataAccessLog {
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

func TestStaffDocumentService_AuditTrailAndSoftDelete(t *testing.T) {
	t.Parallel()

	s := newStaffDocumentScenario(t)
	directory := s.actor("users:update")

	info := s.create(t, userModels.StaffDocumentCategoryZeugnis, directory)

	rows := s.auditRows(t)
	require.Len(t, rows, 1)
	assert.Equal(t, auditModels.StammdatenSectionDokumente, rows[0].Section)
	assert.Equal(t, userModels.StaffDocumentCategoryZeugnis, rows[0].FieldName)
	assert.Equal(t, "", rows[0].OldValue)
	assert.Equal(t, "zeugnis-datei.pdf", rows[0].NewValue)
	assert.Equal(t, s.account, rows[0].ChangedBy)

	deleted, err := s.svc.DeleteStaffDocument(s.ctx, s.staffID, info.Document.ID, directory)
	require.NoError(t, err)
	assert.Equal(t, info.Document.FilenameStored, deleted.FilenameStored)

	rows = s.auditRows(t)
	require.Len(t, rows, 2)
	assert.Equal(t, "zeugnis-datei.pdf", rows[1].OldValue)
	assert.Equal(t, "", rows[1].NewValue)

	// Gone from the list, but the metadata row survives soft-deleted.
	infos, _, err := s.svc.ListStaffDocuments(s.ctx, s.staffID, "", directory)
	require.NoError(t, err)
	assert.Empty(t, infos)

	var count int
	err = s.db.NewRaw(
		`SELECT COUNT(*) FROM users.staff_documents WHERE id = ? AND deleted_at IS NOT NULL AND deleted_by = ?`,
		info.Document.ID, s.account,
	).Scan(context.Background(), &count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Deleting again is a 404-shaped error, not a second audit row.
	_, err = s.svc.DeleteStaffDocument(s.ctx, s.staffID, info.Document.ID, directory)
	require.Error(t, err)
	assert.Len(t, s.auditRows(t), 2)
}

func TestStaffDocumentService_CreateHydratesGeneratedTimestamps(t *testing.T) {
	t.Parallel()

	s := newStaffDocumentScenario(t)
	info := s.create(t, userModels.StaffDocumentCategoryLohnabrechnung, s.actor("staff:financial"))

	assert.False(t, info.Document.CreatedAt.IsZero())
	require.NotNil(t, info.RetainUntil)
	assert.Equal(t, timezone.DateFromTime(info.Document.CreatedAt).Year+10, info.RetainUntil.Year)
}

func TestStaffDocumentService_RefusesDownloadsAfterOffboarding(t *testing.T) {
	t.Parallel()

	s := newStaffDocumentScenario(t)
	actor := s.actor("users:update")
	info := s.create(t, userModels.StaffDocumentCategoryZeugnis, actor)

	_, err := s.db.ExecContext(s.ctx, `UPDATE users.staff SET deleted_at = NOW() WHERE id = ?`, s.staffID)
	require.NoError(t, err)

	_, err = s.svc.ResolveStaffDocumentDownload(s.ctx, s.staffID, info.Document.ID, actor)
	require.Error(t, err)
}

func TestStaffDocumentService_RetentionSchedule(t *testing.T) {
	t.Parallel()

	s := newStaffDocumentScenario(t)
	admin := s.actor("admin:*")

	lohn := s.create(t, userModels.StaffDocumentCategoryLohnabrechnung, admin)
	uploaded := timezone.DateFromTime(lohn.Document.CreatedAt)
	require.NotNil(t, lohn.RetainUntil)
	assert.Equal(t, timezone.NewDate(uploaded.Year+10, uploaded.Month, uploaded.Day), *lohn.RetainUntil)
	assert.Nil(t, lohn.ReviewDue)

	au := s.create(t, userModels.StaffDocumentCategoryAUBescheinigung, admin)
	require.NotNil(t, au.RetainUntil)
	assert.Equal(t, uploaded.Year+4, au.RetainUntil.Year)

	bewerbung := s.create(t, userModels.StaffDocumentCategoryBewerbung, admin)
	assert.Nil(t, bewerbung.RetainUntil)
	require.NotNil(t, bewerbung.ReviewDue)
	assert.Equal(t, timezone.NewDate(uploaded.Year, uploaded.Month+6, uploaded.Day), *bewerbung.ReviewDue)

	// Arbeitsvertrag: open without a contract end, contract end + 6 years
	// once the Stammdaten row carries one.
	vertrag := s.create(t, userModels.StaffDocumentCategoryArbeitsvertrag, admin)
	assert.Nil(t, vertrag.RetainUntil)

	end := timezone.NewDate(2027, 7, 31)
	require.NoError(t, s.repos.StaffMasterData.Create(s.ctx, &userModels.StaffMasterData{
		StaffID:         s.staffID,
		ContractEndDate: &end,
	}))
	t.Cleanup(func() {
		_, _ = s.db.ExecContext(context.Background(), `DELETE FROM users.staff_master_data WHERE staff_id = ?`, s.staffID)
	})
	vertragWithEnd := s.create(t, userModels.StaffDocumentCategoryArbeitsvertrag, admin)
	require.NotNil(t, vertragWithEnd.RetainUntil)
	assert.Equal(t, timezone.NewDate(2033, 7, 31), *vertragWithEnd.RetainUntil)

	infos, _, err := s.svc.ListStaffDocuments(s.ctx, s.staffID, userModels.StaffDocumentCategoryArbeitsvertrag, admin)
	require.NoError(t, err)
	require.Len(t, infos, 2)
	for _, info := range infos {
		require.NotNil(t, info.RetainUntil)
		assert.Equal(t, timezone.NewDate(2033, 7, 31), *info.RetainUntil)
	}

	zeugnis := s.create(t, userModels.StaffDocumentCategoryZeugnis, admin)
	assert.Nil(t, zeugnis.RetainUntil)
	assert.Nil(t, zeugnis.ReviewDue)
}
