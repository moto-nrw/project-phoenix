package active_test

import (
	"context"
	"testing"
	"time"

	repoFactory "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/audit"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Each test creates its own staff + session/absence rows with controlled
// timestamps. We bypass BUN's BeforeAppendModel hook (which sets
// CreatedAt = NOW()) by issuing raw UPSERTs after the insert.

func TestTimeTrackingCleanup_DeletesOldSessions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Cleanup", "Old")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	// Three sessions: two old (one 800d, one 1500d) and one fresh (10d).
	// The two old ones get audit edits and a break each to prove CASCADE
	// removes them transitively.
	oldSessionID := insertSession(t, db, staff.ID, daysAgo(800))
	insertBreak(t, db, oldSessionID, daysAgo(800))
	insertEdit(t, db, oldSessionID, staff.ID, daysAgo(800))
	ancientSessionID := insertSession(t, db, staff.ID, daysAgo(1500))
	insertBreak(t, db, ancientSessionID, daysAgo(1500))
	freshSessionID := insertSession(t, db, staff.ID, daysAgo(10))

	repos := repoFactory.NewFactory(db)
	svc := activeSvc.NewTimeTrackingCleanupService(repos.WorkSession, repos.StaffAbsence, repos.DataDeletion, nil, nil)

	result, err := svc.CleanupExpiredTimeTrackingData(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, result.SessionsDeleted, "two old sessions deleted")
	assert.Equal(t, 1, result.StaffAffected)
	assert.Equal(t, 730, result.RetentionDays)
	assert.True(t, result.Success)

	// Fresh session survives.
	assert.True(t, sessionExists(t, db, freshSessionID), "fresh session must remain")
	assert.False(t, sessionExists(t, db, oldSessionID), "old session must be gone")
	assert.False(t, sessionExists(t, db, ancientSessionID), "ancient session must be gone")

	// CASCADE removed breaks + audit edits for the deleted sessions.
	assert.Equal(t, 0, countBreaksForSession(t, db, oldSessionID))
	assert.Equal(t, 0, countEditsForSession(t, db, oldSessionID))

	// Exactly one audit.data_deletions row for the staff (counts both
	// session and absence deletions but we only have sessions here).
	auditRows := findStaffDeletionRows(t, db, staff.ID)
	require.Len(t, auditRows, 1)
	assert.Equal(t, audit.DeletionTypeTimeTrackingRetention, auditRows[0].DeletionType)
	assert.Equal(t, 2, auditRows[0].RecordsDeleted)
	assert.Equal(t, "system", auditRows[0].DeletedBy)
	require.NotNil(t, auditRows[0].StaffID)
	assert.Equal(t, staff.ID, *auditRows[0].StaffID)
	assert.Nil(t, auditRows[0].StudentID, "staff-scoped row must leave student_id null")

	cleanupDeletionRows(t, db, staff.ID)
}

func TestTimeTrackingCleanup_DeletesOldAbsences(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Cleanup", "Abs")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	oldAbsenceID := insertAbsence(t, db, staff.ID, daysAgo(900))
	freshAbsenceID := insertAbsence(t, db, staff.ID, daysAgo(20))

	repos := repoFactory.NewFactory(db)
	svc := activeSvc.NewTimeTrackingCleanupService(repos.WorkSession, repos.StaffAbsence, repos.DataDeletion, nil, nil)

	result, err := svc.CleanupExpiredTimeTrackingData(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, result.AbsencesDeleted)
	assert.Equal(t, 0, result.SessionsDeleted)
	assert.Equal(t, 1, result.StaffAffected)

	assert.True(t, absenceExists(t, db, freshAbsenceID))
	assert.False(t, absenceExists(t, db, oldAbsenceID))

	// One audit row, RecordsDeleted=1 (the absence).
	auditRows := findStaffDeletionRows(t, db, staff.ID)
	require.Len(t, auditRows, 1)
	assert.Equal(t, 1, auditRows[0].RecordsDeleted)
	cleanupDeletionRows(t, db, staff.ID)
}

func TestTimeTrackingCleanup_PreviewLeavesDataIntact(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Cleanup", "Preview")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	oldSessionID := insertSession(t, db, staff.ID, daysAgo(800))
	oldAbsenceID := insertAbsence(t, db, staff.ID, daysAgo(900))

	repos := repoFactory.NewFactory(db)
	svc := activeSvc.NewTimeTrackingCleanupService(repos.WorkSession, repos.StaffAbsence, repos.DataDeletion, nil, nil)

	preview, err := svc.PreviewExpiredTimeTrackingData(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, preview.SessionsToDelete)
	assert.Equal(t, 1, preview.AbsencesToDelete)
	assert.Equal(t, 1, preview.StaffAffected)

	// Nothing actually deleted.
	assert.True(t, sessionExists(t, db, oldSessionID))
	assert.True(t, absenceExists(t, db, oldAbsenceID))
	// And no audit rows either.
	assert.Empty(t, findStaffDeletionRows(t, db, staff.ID))
}

func TestTimeTrackingCleanup_PreviewOldestOnlyShowsExpiredRows(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Cleanup", "PreviewFresh")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)
	purgeOldRowsInTenant(t, db, testpkg.Tenant(t))

	freshSessionID := insertSession(t, db, staff.ID, daysAgo(10))
	freshAbsenceID := insertAbsence(t, db, staff.ID, daysAgo(10))
	defer cleanupTimeTrackingRows(t, db, freshSessionID, freshAbsenceID)

	repos := repoFactory.NewFactory(db)
	svc := activeSvc.NewTimeTrackingCleanupService(repos.WorkSession, repos.StaffAbsence, repos.DataDeletion, nil, nil)

	preview, err := svc.PreviewExpiredTimeTrackingData(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, preview.SessionsToDelete)
	assert.Equal(t, 0, preview.AbsencesToDelete)
	assert.Nil(t, preview.OldestSession)
	assert.Nil(t, preview.OldestAbsence)
	assert.True(t, sessionExists(t, db, freshSessionID))
	assert.True(t, absenceExists(t, db, freshAbsenceID))
}

func TestTimeTrackingCleanup_NoOpWhenNothingExpired(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Cleanup", "NoOp")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	// Other tests in this package can leave behind stale rows for other
	// staff inside the same tenant. The "noop" assertion only holds when
	// the tenant has zero rows older than the cutoff, so we clear them
	// first. We don't truncate, that would also kill any newly inserted
	// rows from concurrent staff.
	purgeOldRowsInTenant(t, db, testpkg.Tenant(t))

	freshSessionID := insertSession(t, db, staff.ID, daysAgo(10))

	repos := repoFactory.NewFactory(db)
	svc := activeSvc.NewTimeTrackingCleanupService(repos.WorkSession, repos.StaffAbsence, repos.DataDeletion, nil, nil)

	result, err := svc.CleanupExpiredTimeTrackingData(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, result.SessionsDeleted)
	assert.Equal(t, 0, result.AbsencesDeleted)
	assert.Equal(t, 0, result.StaffAffected)
	assert.True(t, sessionExists(t, db, freshSessionID))
	// No audit rows when nothing was deleted.
	assert.Empty(t, findStaffDeletionRows(t, db, staff.ID))
}

func TestTimeTrackingCleanup_AuditRequired(t *testing.T) {
	// Service must refuse to delete when no audit repo is configured,
	// otherwise we'd lose the compliance trail silently.
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Cleanup", "NoAudit")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)
	insertSession(t, db, staff.ID, daysAgo(800))

	repos := repoFactory.NewFactory(db)
	svc := activeSvc.NewTimeTrackingCleanupService(repos.WorkSession, repos.StaffAbsence, nil, nil, nil)
	_, err := svc.CleanupExpiredTimeTrackingData(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "audit repo not configured")
}

func TestTimeTrackingCleanup_UsesBusinessDatesNotCreatedAt(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Cleanup", "BusinessDate")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	freshBusinessSessionID := insertSessionWithBusinessDate(t, db, staff.ID, daysAgo(900), daysAgo(10))
	oldBusinessSessionID := insertSessionWithBusinessDate(t, db, staff.ID, daysAgo(10), daysAgo(900))
	freshBusinessAbsenceID := insertAbsenceWithBusinessDates(t, db, staff.ID, daysAgo(900), daysAgo(10), daysAgo(10))
	oldBusinessAbsenceID := insertAbsenceWithBusinessDates(t, db, staff.ID, daysAgo(10), daysAgo(900), daysAgo(900))

	repos := repoFactory.NewFactory(db)
	svc := activeSvc.NewTimeTrackingCleanupService(repos.WorkSession, repos.StaffAbsence, repos.DataDeletion, nil, nil)

	result, err := svc.CleanupExpiredTimeTrackingData(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, result.SessionsDeleted)
	assert.Equal(t, 1, result.AbsencesDeleted)

	assert.True(t, sessionExists(t, db, freshBusinessSessionID), "fresh business session must remain")
	assert.False(t, sessionExists(t, db, oldBusinessSessionID), "old business session must be deleted")
	assert.True(t, absenceExists(t, db, freshBusinessAbsenceID), "fresh business absence must remain")
	assert.False(t, absenceExists(t, db, oldBusinessAbsenceID), "old business absence must be deleted")

	cleanupDeletionRows(t, db, staff.ID)
}

// --- Helpers ---------------------------------------------------------------

func daysAgo(days int) time.Time {
	return time.Now().AddDate(0, 0, -days)
}

func insertSession(t *testing.T, db *bun.DB, staffID int64, createdAt time.Time) int64 {
	t.Helper()
	return insertSessionWithBusinessDate(t, db, staffID, createdAt, createdAt)
}

func insertSessionWithBusinessDate(t *testing.T, db *bun.DB, staffID int64, createdAt, businessDate time.Time) int64 {
	t.Helper()
	checkOut := createdAt.Add(8 * time.Hour)
	s := &activeModels.WorkSession{
		StaffID:      staffID,
		Date:         timezone.DateFromTime(businessDate),
		Status:       activeModels.WorkSessionStatusPresent,
		Source:       activeModels.WorkSessionSourceApp,
		CheckInTime:  createdAt,
		CheckOutTime: &checkOut,
		BreakMinutes: 30,
		CreatedBy:    staffID,
	}
	s.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().Model(s).Exec(context.Background())
	require.NoError(t, err)
	// Override created_at after the fact. BUN's BeforeAppendModel hook
	// stamps NOW() on insert, which is fine for fresh rows but useless for
	// the cleanup test which needs to control "age".
	_, err = db.NewUpdate().
		Table("active.work_sessions").
		Set("created_at = ?", createdAt).
		Where("id = ?", s.ID).
		Exec(context.Background())
	require.NoError(t, err)
	return s.ID
}

func insertBreak(t *testing.T, db *bun.DB, sessionID int64, createdAt time.Time) int64 {
	t.Helper()
	end := createdAt.Add(30 * time.Minute)
	b := &activeModels.WorkSessionBreak{
		SessionID:       sessionID,
		StartedAt:       createdAt,
		EndedAt:         &end,
		DurationMinutes: 30,
	}
	b.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().Model(b).Exec(context.Background())
	require.NoError(t, err)
	return b.ID
}

func insertEdit(t *testing.T, db *bun.DB, sessionID, staffID int64, createdAt time.Time) int64 {
	t.Helper()
	old := "30"
	newVal := "45"
	notes := "test"
	e := &audit.WorkSessionEdit{
		SessionID: sessionID,
		StaffID:   staffID,
		EditedBy:  staffID,
		FieldName: "break_minutes",
		OldValue:  &old,
		NewValue:  &newVal,
		Notes:     &notes,
		CreatedAt: createdAt,
	}
	e.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().Model(e).Exec(context.Background())
	require.NoError(t, err)
	return e.ID
}

func insertAbsence(t *testing.T, db *bun.DB, staffID int64, createdAt time.Time) int64 {
	t.Helper()
	return insertAbsenceWithBusinessDates(t, db, staffID, createdAt, createdAt, createdAt)
}

func insertAbsenceWithBusinessDates(t *testing.T, db *bun.DB, staffID int64, createdAt, dateStart, dateEnd time.Time) int64 {
	t.Helper()
	a := &activeModels.StaffAbsence{
		StaffID:     staffID,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   timezone.DateFromTime(dateStart),
		DateEnd:     timezone.DateFromTime(dateEnd),
		Status:      activeModels.AbsenceStatusApproved,
		CreatedBy:   staffID,
	}
	a.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().Model(a).Exec(context.Background())
	require.NoError(t, err)
	_, err = db.NewUpdate().
		Table("active.staff_absences").
		Set("created_at = ?", createdAt).
		Where("id = ?", a.ID).
		Exec(context.Background())
	require.NoError(t, err)
	return a.ID
}

func sessionExists(t *testing.T, db *bun.DB, id int64) bool {
	t.Helper()
	n, err := db.NewSelect().Table("active.work_sessions").Where("id = ?", id).Count(context.Background())
	require.NoError(t, err)
	return n > 0
}

func absenceExists(t *testing.T, db *bun.DB, id int64) bool {
	t.Helper()
	n, err := db.NewSelect().Table("active.staff_absences").Where("id = ?", id).Count(context.Background())
	require.NoError(t, err)
	return n > 0
}

func countBreaksForSession(t *testing.T, db *bun.DB, sessionID int64) int {
	t.Helper()
	n, err := db.NewSelect().Table("active.work_session_breaks").Where("session_id = ?", sessionID).Count(context.Background())
	require.NoError(t, err)
	return n
}

func countEditsForSession(t *testing.T, db *bun.DB, sessionID int64) int {
	t.Helper()
	n, err := db.NewSelect().Table("audit.work_session_edits").Where("session_id = ?", sessionID).Count(context.Background())
	require.NoError(t, err)
	return n
}

func findStaffDeletionRows(t *testing.T, db *bun.DB, staffID int64) []*audit.DataDeletion {
	t.Helper()
	var rows []*audit.DataDeletion
	err := db.NewSelect().
		Model(&rows).
		Where("staff_id = ?", staffID).
		Order("id DESC").
		Scan(context.Background())
	require.NoError(t, err)
	return rows
}

// purgeOldRowsInTenant deletes work_sessions and staff_absences with business
// dates older than 730 days for the given tenant. Used by tests that
// assert "nothing happens". They need the tenant slice to be empty of
// expired rows before they begin.
func purgeOldRowsInTenant(t *testing.T, db *bun.DB, tenantID int64) {
	t.Helper()
	cutoff := time.Now().AddDate(0, 0, -730)
	_, err := db.NewDelete().
		Table("active.work_sessions").
		Where("tenant_id = ?", tenantID).
		Where("date < ?", cutoff).
		Exec(context.Background())
	require.NoError(t, err)
	_, err = db.NewDelete().
		Table("active.staff_absences").
		Where("tenant_id = ?", tenantID).
		Where("date_end < ?", cutoff).
		Exec(context.Background())
	require.NoError(t, err)
}

func cleanupDeletionRows(t *testing.T, db *bun.DB, staffID int64) {
	t.Helper()
	_, err := db.NewDelete().
		Table("audit.data_deletions").
		Where("staff_id = ?", staffID).
		Exec(context.Background())
	require.NoError(t, err)
}

func cleanupTimeTrackingRows(t *testing.T, db *bun.DB, sessionID, absenceID int64) {
	t.Helper()
	_, err := db.NewDelete().
		Table("active.staff_absences").
		Where("id = ?", absenceID).
		Exec(context.Background())
	require.NoError(t, err)
	_, err = db.NewDelete().
		Table("active.work_sessions").
		Where("id = ?", sessionID).
		Exec(context.Background())
	require.NoError(t, err)
}
