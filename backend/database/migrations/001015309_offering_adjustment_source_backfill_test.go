package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// backfillFixture is the enrollment child every adjustment row in this test
// hangs off, plus the account acting as the deciding admin.
type backfillFixture struct {
	tenantID  int64
	requestID int64
	childID   int64
	studentID int64
	actorID   int64
}

func newBackfillFixture(t *testing.T, db *testpkg.DB) *backfillFixture {
	t.Helper()
	ctx := context.Background()
	tenantID := testpkg.Tenant(t)

	student := testpkg.CreateTestStudent(t, db, "Zbackfill", "Adjustkind", "4c")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("backfill-%d@example.invalid", time.Now().UnixNano()))
	phase := testpkg.CreateTestEnrollmentPhase(t, db)

	var requestID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.requests
			(tenant_id, phase_id, guardian_first_name, guardian_last_name, guardian_email, status_token)
		VALUES (?, ?, 'Erzieh', 'Ungsberechtigt', ?, ?)
		RETURNING id
	`, tenantID, phase.ID, fmt.Sprintf("backfill-%d@example.test", time.Now().UnixNano()),
		fmt.Sprintf("bf-tok-%d", time.Now().UnixNano())).Scan(ctx, &requestID))

	var childID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.request_children
			(tenant_id, request_id, first_name, last_name, date_of_birth, status, created_student_id)
		VALUES (?, ?, 'Zbackfill', 'Adjustkind', ?, 'approved', ?)
		RETURNING id
	`, tenantID, requestID, testpkg.TodayDate().AddDays(-2500), student.ID).Scan(ctx, &childID))

	return &backfillFixture{
		tenantID:  tenantID,
		requestID: requestID,
		childID:   childID,
		studentID: student.ID,
		actorID:   account.ID,
	}
}

// insertAdjustment writes one append-only adjustment row with an explicit
// source, bypassing the column default so legacy rows can be reproduced.
func (f *backfillFixture) insertAdjustment(t *testing.T, db *testpkg.DB, reason, source string, changedAt time.Time) int64 {
	t.Helper()
	var id int64
	err := db.NewRaw(`
		INSERT INTO audit.enrollment_offering_adjustments
			(tenant_id, request_id, request_child_id, student_id, actor_account_id,
			 actor_role, reason, source, before_json, after_json, changed_at)
		VALUES (?, ?, ?, ?, ?, 'admin', ?, ?, ?, ?, ?)
		RETURNING id
	`, f.tenantID, f.requestID, f.childID, f.studentID, f.actorID,
		reason, source, json.RawMessage(`[]`), json.RawMessage(`[]`), changedAt,
	).Scan(context.Background(), &id)
	require.NoError(t, err)
	return id
}

func (f *backfillFixture) insertApprovedChangeRequest(t *testing.T, db *testpkg.DB, note string, reviewedAt time.Time) {
	t.Helper()
	_, err := db.NewRaw(`
		INSERT INTO enrollment.change_requests
			(tenant_id, request_id, request_child_id, origin, status, admin_decision_note,
			 base_snapshot, proposed_snapshot, diff_json, reviewed_by_account_id, reviewed_at)
		VALUES (?, ?, ?, 'parent', 'approved', ?, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, ?, ?)
	`, f.tenantID, f.requestID, f.childID, note, f.actorID, reviewedAt).Exec(context.Background())
	require.NoError(t, err)
}

func sourceOf(t *testing.T, db *testpkg.DB, id int64) string {
	t.Helper()
	var source string
	require.NoError(t, db.NewRaw(
		`SELECT source FROM audit.enrollment_offering_adjustments WHERE id = ?`, id,
	).Scan(context.Background(), &source))
	return source
}

// #2413: legacy rows must be classified from evidence only — a generated
// reason or a character-identical match with the approved Anmeldungsänderung
// that wrote them. Everything else is the office's own correction and becomes
// visible in the central history.
func TestOfferingAdjustmentSourceBackfill(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	f := newBackfillFixture(t, db)

	decidedAt := time.Now().UTC().Add(-72 * time.Hour)

	generated := f.insertAdjustment(t,
		db, "Elternanfrage #4711 freigegeben (gültig ab 01.09.2026): nach Rücksprache",
		"unknown", decidedAt)
	generatedBare := f.insertAdjustment(t,
		db, "Elternanfrage #4712 freigegeben (gültig ab 01.09.2026)",
		"unknown", decidedAt)

	note := "Angaben nach Rücksprache berichtigt"
	f.insertApprovedChangeRequest(t, db, note, decidedAt.Add(200*time.Millisecond))
	correlated := f.insertAdjustment(t, db, note, "unknown", decidedAt)

	// Same note, but written days apart from the decision: no evidence ties it
	// to the change request, so it stays the office's own correction.
	unrelated := f.insertAdjustment(t, db, note,
		"unknown", decidedAt.Add(-48*time.Hour))
	correction := f.insertAdjustment(t, db, "Ganztag nachgetragen",
		"unknown", decidedAt)

	// Rows the running code already labelled must not be reinterpreted.
	labelledDirect := f.insertAdjustment(t, db, "Elternanfrage #4713 freigegeben (gültig ab 01.09.2026)",
		"direct", decidedAt)
	labelledRequest := f.insertAdjustment(t, db, "Ganztag nachgetragen",
		"request", decidedAt)

	require.NoError(t, offeringAdjustmentSourceBackfillUp(context.Background(), db))

	assert.Equal(t, "request", sourceOf(t, db, generated),
		"a generated reason with a staff addition is still the offering queue's own row")
	assert.Equal(t, "request", sourceOf(t, db, generatedBare))
	assert.Equal(t, "request", sourceOf(t, db, correlated),
		"same request, reviewer, note and instant as the approved Anmeldungsänderung")
	assert.Equal(t, "direct", sourceOf(t, db, unrelated),
		"a matching note alone is not evidence — the instants are days apart")
	assert.Equal(t, "direct", sourceOf(t, db, correction))
	assert.Equal(t, "direct", sourceOf(t, db, labelledDirect),
		"already-labelled rows are left alone")
	assert.Equal(t, "request", sourceOf(t, db, labelledRequest))

	// Re-running must not reclassify anything (the migration is idempotent).
	require.NoError(t, offeringAdjustmentSourceBackfillUp(context.Background(), db))
	assert.Equal(t, "direct", sourceOf(t, db, correction))
	assert.Equal(t, "request", sourceOf(t, db, correlated))

	// The rollback is deliberately a no-op: the classification cannot be undone.
	require.NoError(t, offeringAdjustmentSourceBackfillDown(context.Background(), db))
	assert.Equal(t, "direct", sourceOf(t, db, correction))
}

func TestOfferingAdjustmentSourceBackfill_DoesNotCorrelateAnotherChild(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	f := newBackfillFixture(t, db)
	decidedAt := time.Now().UTC().Add(-72 * time.Hour)
	note := "Angaben nach Rücksprache berichtigt"

	otherStudent := testpkg.CreateTestStudent(t, db, "Zbackfill", "Geschwister", "4c")
	var otherChildID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.request_children
			(tenant_id, request_id, first_name, last_name, date_of_birth, status, created_student_id)
		VALUES (?, ?, 'Zbackfill', 'Geschwister', ?, 'approved', ?)
		RETURNING id
	`, f.tenantID, f.requestID, testpkg.TodayDate().AddDays(-2500), otherStudent.ID).
		Scan(context.Background(), &otherChildID))
	_, err := db.NewRaw(`
		INSERT INTO enrollment.change_requests
			(tenant_id, request_id, request_child_id, origin, status, admin_decision_note,
			 base_snapshot, proposed_snapshot, diff_json, reviewed_by_account_id, reviewed_at)
		VALUES (?, ?, ?, 'parent', 'approved', ?, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, ?, ?)
	`, f.tenantID, f.requestID, otherChildID, note, f.actorID, decidedAt).Exec(context.Background())
	require.NoError(t, err)

	adjustment := f.insertAdjustment(t, db, note, "unknown", decidedAt)
	require.NoError(t, offeringAdjustmentSourceBackfillUp(context.Background(), db))
	assert.Equal(t, "direct", sourceOf(t, db, adjustment))
}
