package migrations

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// insertStatusDay writes one parent-reported day without an author, the shape
// every row had before 1.15.345 added the column.
func insertStatusDay(t *testing.T, db *bun.DB, tenantID, studentID int64, date timezone.Date, status string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO active.student_status_days
			(tenant_id, student_id, date, status, reported_at, source)
		VALUES (?, ?, ?, ?, NOW(), 'parent')
		RETURNING id
	`, tenantID, studentID, date, status).Scan(context.Background(), &id))
	return id
}

func insertAbsenceRequest(
	t *testing.T, db *bun.DB, tenantID, studentID, submittedBy int64,
	dates []timezone.Date, absenceStatus, status string, reviewedAt time.Time,
) int64 {
	t.Helper()
	payload := make([]string, 0, len(dates))
	for _, date := range dates {
		payload = append(payload, date.String())
	}
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	var id int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO active.excused_absence_requests
			(tenant_id, student_id, submitted_by, dates, note, absence_status, status, reviewed_at)
		VALUES (?, ?, ?, ?::jsonb, 'Backfill', ?, ?, ?)
		RETURNING id
	`, tenantID, studentID, submittedBy, string(encoded), absenceStatus, status, reviewedAt).
		Scan(context.Background(), &id))
	return id
}

func statusDayAuthor(t *testing.T, db *bun.DB, dayID int64) *int64 {
	t.Helper()
	var author *int64
	require.NoError(t, db.NewRaw(
		`SELECT guardian_account_id FROM active.student_status_days WHERE id = ?`, dayID,
	).Scan(context.Background(), &author))
	return author
}

// The backfill only acts on positive evidence: an approved request that names
// the day it wrote. Everything else keeps the unknown author, which the reader
// treats as "hide the note" rather than showing it to the wrong co-guardian.
func TestParentStatusAuthorBackfillStampsApprovedRequestsOnly(t *testing.T) {
	// Parallel-safe: the backfill sweeps the whole clone, and every assertion
	// names a row this test created.
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantA, _ := testpkg.CreateTestTenant(t, db)
	tenantB, _ := testpkg.CreateTestTenant(t, db)
	studentA := testpkg.CreateTestStudentForTenant(t, db, tenantA, "Backfill", "Autor", "1a")
	studentB := testpkg.CreateTestStudentForTenant(t, db, tenantB, "Backfill", "Fremd", "1b")
	author := testpkg.CreateTestAccount(t, db, "status-author-backfill@example.com")
	otherAuthor := testpkg.CreateTestAccount(t, db, "status-author-backfill-other@example.com")

	firstDay := timezone.NewDate(2025, time.March, 3)
	secondDay := timezone.NewDate(2025, time.March, 4)
	rejectedDay := timezone.NewDate(2025, time.March, 5)
	withdrawnDay := timezone.NewDate(2025, time.March, 6)
	directDay := timezone.NewDate(2025, time.March, 7)
	foreignDay := timezone.NewDate(2025, time.March, 10)

	approvedFirst := insertStatusDay(t, db, tenantA, studentA.ID, firstDay, "excused")
	approvedSecond := insertStatusDay(t, db, tenantA, studentA.ID, secondDay, "excused")
	rejected := insertStatusDay(t, db, tenantA, studentA.ID, rejectedDay, "excused")
	withdrawn := insertStatusDay(t, db, tenantA, studentA.ID, withdrawnDay, "excused")
	direct := insertStatusDay(t, db, tenantA, studentA.ID, directDay, "sick")
	foreign := insertStatusDay(t, db, tenantB, studentB.ID, foreignDay, "excused")

	reviewed := time.Now().Add(-24 * time.Hour)
	insertAbsenceRequest(t, db, tenantA, studentA.ID, author.ID,
		[]timezone.Date{firstDay, secondDay}, "excused", "approved", reviewed)
	insertAbsenceRequest(t, db, tenantA, studentA.ID, author.ID,
		[]timezone.Date{rejectedDay}, "excused", "rejected", reviewed)
	insertAbsenceRequest(t, db, tenantA, studentA.ID, author.ID,
		[]timezone.Date{withdrawnDay}, "excused", "withdrawn", reviewed)
	// Same calendar day, another tenant's approved request: the day must not
	// borrow an author across the tenant boundary.
	insertAbsenceRequest(t, db, tenantB, studentB.ID, otherAuthor.ID,
		[]timezone.Date{foreignDay}, "excused", "approved", reviewed)

	require.NoError(t, parentStatusAuthorBackfillUp(ctx, db))

	require.NotNil(t, statusDayAuthor(t, db, approvedFirst))
	assert.Equal(t, author.ID, *statusDayAuthor(t, db, approvedFirst))
	require.NotNil(t, statusDayAuthor(t, db, approvedSecond))
	assert.Equal(t, author.ID, *statusDayAuthor(t, db, approvedSecond), "every date of the request is stamped")
	assert.Nil(t, statusDayAuthor(t, db, rejected), "a rejected request never wrote this day")
	assert.Nil(t, statusDayAuthor(t, db, withdrawn), "a withdrawn request never wrote this day")
	assert.Nil(t, statusDayAuthor(t, db, direct), "a direct sick note has no author source")

	require.NotNil(t, statusDayAuthor(t, db, foreign))
	assert.Equal(t, otherAuthor.ID, *statusDayAuthor(t, db, foreign), "each tenant keeps its own author")

	// Idempotent: a second run neither changes nor clears what the first wrote.
	require.NoError(t, parentStatusAuthorBackfillUp(ctx, db))
	require.NotNil(t, statusDayAuthor(t, db, approvedFirst))
	assert.Equal(t, author.ID, *statusDayAuthor(t, db, approvedFirst))
	assert.Nil(t, statusDayAuthor(t, db, direct))
}

// An author already on the row wins: the live flow wrote it, the backfill only
// fills gaps.
func TestParentStatusAuthorBackfillKeepsExistingAuthor(t *testing.T) {
	// Parallel-safe: the backfill sweeps the whole clone, and every assertion
	// names a row this test created.
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID, _ := testpkg.CreateTestTenant(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Backfill", "Bestand", "2a")
	existing := testpkg.CreateTestAccount(t, db, "status-author-existing@example.com")
	requester := testpkg.CreateTestAccount(t, db, "status-author-requester@example.com")

	day := timezone.NewDate(2025, time.April, 7)
	dayID := insertStatusDay(t, db, tenantID, student.ID, day, "excused")
	_, err := db.ExecContext(ctx,
		`UPDATE active.student_status_days SET guardian_account_id = ? WHERE id = ?`, existing.ID, dayID)
	require.NoError(t, err)
	insertAbsenceRequest(t, db, tenantID, student.ID, requester.ID,
		[]timezone.Date{day}, "excused", "approved", time.Now())

	require.NoError(t, parentStatusAuthorBackfillUp(ctx, db))

	require.NotNil(t, statusDayAuthor(t, db, dayID))
	assert.Equal(t, existing.ID, *statusDayAuthor(t, db, dayID))
}
