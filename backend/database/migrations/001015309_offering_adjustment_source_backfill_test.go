package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
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

func newBackfillFixture(t *testing.T, db *bun.DB) *backfillFixture {
	t.Helper()
	ctx := context.Background()
	tenantID := testpkg.Tenant(t)

	student := testpkg.CreateTestStudent(t, db, "Zbackfill", "Adjustkind", "4c")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("backfill-%d@example.invalid", time.Now().UnixNano()))
	phase := testpkg.CreateTestEnrollmentPhase(t, db)

	request := &enrollmentModels.Request{
		PhaseID:           phase.ID,
		GuardianFirstName: "Erzieh",
		GuardianLastName:  "Ungsberechtigt",
		GuardianEmail:     fmt.Sprintf("backfill-%d@example.test", time.Now().UnixNano()),
		StatusToken:       fmt.Sprintf("bf-tok-%d", time.Now().UnixNano()),
	}
	request.TenantID = tenantID
	_, err := db.NewInsert().Model(request).ModelTableExpr(`enrollment.requests AS "request"`).Exec(ctx)
	require.NoError(t, err)

	child := &enrollmentModels.RequestChild{
		RequestID:        request.ID,
		FirstName:        "Zbackfill",
		LastName:         "Adjustkind",
		DateOfBirth:      timezone.TodayDate().AddDays(-2500),
		Status:           enrollmentModels.ChildStatusApproved,
		CreatedStudentID: &student.ID,
	}
	child.TenantID = tenantID
	_, err = db.NewInsert().Model(child).ModelTableExpr(`enrollment.request_children AS "request_child"`).Exec(ctx)
	require.NoError(t, err)

	return &backfillFixture{
		tenantID:  tenantID,
		requestID: request.ID,
		childID:   child.ID,
		studentID: student.ID,
		actorID:   account.ID,
	}
}

// insertAdjustment writes one append-only adjustment row with an explicit
// source, bypassing the column default so legacy rows can be reproduced.
func (f *backfillFixture) insertAdjustment(t *testing.T, db *bun.DB, reason, source string, changedAt time.Time) int64 {
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
	t.Cleanup(func() {
		_, _ = db.NewRaw(`DELETE FROM audit.enrollment_offering_adjustments WHERE id = ?`, id).Exec(context.Background())
	})
	return id
}

func (f *backfillFixture) insertApprovedChangeRequest(t *testing.T, db *bun.DB, note string, reviewedAt time.Time) {
	t.Helper()
	row := &enrollmentModels.ChangeRequest{
		RequestID:           f.requestID,
		RequestChildID:      &f.childID,
		Origin:              enrollmentModels.ChangeRequestOriginParent,
		Status:              enrollmentModels.ChangeRequestStatusApproved,
		AdminDecisionNote:   &note,
		BaseSnapshot:        map[string]any{},
		ProposedSnapshot:    map[string]any{},
		Diff:                map[string]any{},
		ReviewedByAccountID: &f.actorID,
		ReviewedAt:          &reviewedAt,
	}
	row.TenantID = f.tenantID
	_, err := db.NewInsert().Model(row).ModelTableExpr(`enrollment.change_requests AS "change_request"`).Exec(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.NewRaw(`DELETE FROM enrollment.change_requests WHERE id = ?`, row.ID).Exec(context.Background())
	})
}

func sourceOf(t *testing.T, db *bun.DB, id int64) string {
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
	db := testpkg.SetupTestDB(t)
	f := newBackfillFixture(t, db)

	decidedAt := time.Now().UTC().Add(-72 * time.Hour)

	generated := f.insertAdjustment(t,
		db, "Elternanfrage #4711 freigegeben (gültig ab 01.09.2026): nach Rücksprache",
		auditModels.OfferingAdjustmentSourceUnknown, decidedAt)
	generatedBare := f.insertAdjustment(t,
		db, "Elternanfrage #4712 freigegeben (gültig ab 01.09.2026)",
		auditModels.OfferingAdjustmentSourceUnknown, decidedAt)

	const note = "Angaben nach Rücksprache berichtigt"
	f.insertApprovedChangeRequest(t, db, note, decidedAt.Add(200*time.Millisecond))
	correlated := f.insertAdjustment(t, db, note, auditModels.OfferingAdjustmentSourceUnknown, decidedAt)

	// Same note, but written days apart from the decision: no evidence ties it
	// to the change request, so it stays the office's own correction.
	unrelated := f.insertAdjustment(t, db, note,
		auditModels.OfferingAdjustmentSourceUnknown, decidedAt.Add(-48*time.Hour))
	correction := f.insertAdjustment(t, db, "Ganztag nachgetragen",
		auditModels.OfferingAdjustmentSourceUnknown, decidedAt)

	// Rows the running code already labelled must not be reinterpreted.
	labelledDirect := f.insertAdjustment(t, db, "Elternanfrage #4713 freigegeben (gültig ab 01.09.2026)",
		auditModels.OfferingAdjustmentSourceDirect, decidedAt)
	labelledRequest := f.insertAdjustment(t, db, "Ganztag nachgetragen",
		auditModels.OfferingAdjustmentSourceRequest, decidedAt)

	require.NoError(t, offeringAdjustmentSourceBackfillUp(context.Background(), db))

	assert.Equal(t, auditModels.OfferingAdjustmentSourceRequest, sourceOf(t, db, generated),
		"a generated reason with a staff addition is still the offering queue's own row")
	assert.Equal(t, auditModels.OfferingAdjustmentSourceRequest, sourceOf(t, db, generatedBare))
	assert.Equal(t, auditModels.OfferingAdjustmentSourceRequest, sourceOf(t, db, correlated),
		"same request, reviewer, note and instant as the approved Anmeldungsänderung")
	assert.Equal(t, auditModels.OfferingAdjustmentSourceDirect, sourceOf(t, db, unrelated),
		"a matching note alone is not evidence — the instants are days apart")
	assert.Equal(t, auditModels.OfferingAdjustmentSourceDirect, sourceOf(t, db, correction))
	assert.Equal(t, auditModels.OfferingAdjustmentSourceDirect, sourceOf(t, db, labelledDirect),
		"already-labelled rows are left alone")
	assert.Equal(t, auditModels.OfferingAdjustmentSourceRequest, sourceOf(t, db, labelledRequest))

	// Re-running must not reclassify anything (the migration is idempotent).
	require.NoError(t, offeringAdjustmentSourceBackfillUp(context.Background(), db))
	assert.Equal(t, auditModels.OfferingAdjustmentSourceDirect, sourceOf(t, db, correction))
	assert.Equal(t, auditModels.OfferingAdjustmentSourceRequest, sourceOf(t, db, correlated))

	// The rollback is deliberately a no-op: the classification cannot be undone.
	require.NoError(t, offeringAdjustmentSourceBackfillDown(context.Background(), db))
	assert.Equal(t, auditModels.OfferingAdjustmentSourceDirect, sourceOf(t, db, correction))
}
