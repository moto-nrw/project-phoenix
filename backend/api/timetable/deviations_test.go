// Integration tests for the atomic Vertretungsplan save (#1840):
//
//	POST /api/timetable/instances/{id}/deviations
//
// The point of the endpoint is that a whole slide-over save lands in ONE tenant
// transaction. The tests drive the wired handler with a real TimetableData
// facade (real DB writes) and a recording mock InstanceService (cancel + ack),
// then read the DB back to prove:
//   - a substitute swap (remove current, assign another) succeeds in one call,
//   - a mid-save conflict writes NOTHING (dry-run-first atomicity),
//   - adding coverage clears a stale "deliberately unstaffed" acknowledgement,
//   - acknowledging a still-staffed block is rejected.
package timetable

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type devSetup struct {
	res    *Resource
	mock   *mockInstanceService
	db     *bun.DB
	ctx    context.Context
	roomID int64
	staffA int64 // planned person
	staffX int64 // current substitute
	staffY int64 // replacement
	staffB int64 // second planned person
}

func buildDevSetup(t *testing.T) *devSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Dev-Room-%d", suffix))
	a := testpkg.CreateTestStaff(t, db, "Planned", fmt.Sprintf("%d", suffix))
	x := testpkg.CreateTestStaff(t, db, "CurrentSub", fmt.Sprintf("%d", suffix+1))
	y := testpkg.CreateTestStaff(t, db, "Replacement", fmt.Sprintf("%d", suffix+2))
	b := testpkg.CreateTestStaff(t, db, "Planned2", fmt.Sprintf("%d", suffix+3))

	mock := &mockInstanceService{}
	res := NewResource(Dependencies{
		TimetableData:   testTimetableData(db),
		PersonService:   usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{StaffRepo: usersRepo.NewStaffRepository(db)}),
		InstanceService: mock,
		DB:              db,
	})

	t.Cleanup(func() {
		for _, id := range []int64{a.ID, x.ID, y.ID, b.ID} {
			testpkg.CleanupActivityFixtures(t, db, 0, id, 0, 0, 0)
		}
		testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room.ID)
	})

	return &devSetup{
		res: res, mock: mock, db: db, ctx: ctx, roomID: room.ID,
		staffA: a.ID, staffX: x.ID, staffY: y.ID, staffB: b.ID,
	}
}

func devRouter(parentCtx context.Context, res *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(req.Context(), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Post("/instances/{id}/deviations", res.applyDeviations)
	return r
}

func doDev(t *testing.T, router chi.Router, instanceID int64, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/instances/%d/deviations", instanceID), bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func devInstanceStaff(t *testing.T, db *bun.DB, ctx context.Context, instanceID int64) []*scheduleModel.InstanceStaff {
	t.Helper()
	repo := scheduleRepo.NewInstanceStaffRepository(db)
	rows, err := repo.FindByInstanceID(ctx, instanceID)
	require.NoError(t, err)
	return rows
}

func setUnderstaffedAckFixture(t *testing.T, db *bun.DB, ctx context.Context, instanceID int64) {
	t.Helper()
	_, err := db.NewUpdate().
		Model((*scheduleModel.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances`).
		Set("understaffed_ack = ?", true).
		Where("id = ?", instanceID).
		Exec(ctx)
	require.NoError(t, err)
}

// Swapping the current substitute for another in a single save must succeed:
// the old substitute reads as absent (freeing the block), the replacement is
// created, all in one call — the exact multi-edit scenario that used to 409.
func TestApplyDeviations_SwapSubstitute_OneCall(t *testing.T) {
	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Swap"})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	rowA := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	rowX := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffX, testpkg.InstanceStaffOpts{IsSubstitute: true})
	t.Cleanup(func() { testpkg.CleanupInstanceStaffFixtures(t, s.db, rowA.ID, rowX.ID) })

	w := doDev(t, router, inst.ID, map[string]any{
		"absences":      []map[string]any{{"staff_id": s.staffX}},
		"substitutions": []map[string]any{{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffY}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	rows := devInstanceStaff(t, s.db, s.ctx, inst.ID)
	var newSubIDs []int64
	sawXAbsent, sawYActive := false, false
	for _, r := range rows {
		if r.StaffID == s.staffX {
			assert.True(t, r.IsAbsent, "removed substitute X must be marked absent")
			sawXAbsent = true
		}
		if r.StaffID == s.staffY && r.IsSubstitute && !r.IsAbsent {
			sawYActive = true
			newSubIDs = append(newSubIDs, r.ID)
		}
	}
	assert.True(t, sawXAbsent, "X row absent")
	assert.True(t, sawYActive, "Y active substitute row created")
	t.Cleanup(func() { testpkg.CleanupInstanceStaffFixtures(t, s.db, newSubIDs...) })
}

// A mid-save conflict must roll back to nothing: the absence in the same payload
// must NOT be committed when the substitution 409s.
func TestApplyDeviations_Conflict_NoPartialWrites(t *testing.T) {
	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Conflict"})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	// A absent, X already substituting A (non-absent), B planned & present.
	rowA := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	rowX := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffX, testpkg.InstanceStaffOpts{IsSubstitute: true})
	rowB := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffB, testpkg.InstanceStaffOpts{})
	t.Cleanup(func() { testpkg.CleanupInstanceStaffFixtures(t, s.db, rowA.ID, rowX.ID, rowB.ID) })

	// Assign Y to A WITHOUT removing X → substitute_conflict. B's absence is in the
	// same payload and must not survive the abort.
	w := doDev(t, router, inst.ID, map[string]any{
		"absences":      []map[string]any{{"staff_id": s.staffB}},
		"substitutions": []map[string]any{{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffY}},
	})
	require.Equal(t, http.StatusConflict, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "substitute_conflict")

	rowBAfter := readInstanceStaff(t, s.db, s.ctx, rowB.ID)
	assert.False(t, rowBAfter.IsAbsent, "B's absence must NOT be committed after the 409")
}

// Adding coverage to a block that was flagged "deliberately unstaffed" must clear
// the stale acknowledgement in the same tx (finding: /gaps and the amber card
// otherwise contradict).
func TestApplyDeviations_ClearsStaleAckWhenCovered(t *testing.T) {
	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Acked"})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })
	setUnderstaffedAckFixture(t, s.db, s.ctx, inst.ID)

	// A already absent → block was legitimately all-absent when acked.
	rowA := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	t.Cleanup(func() { testpkg.CleanupInstanceStaffFixtures(t, s.db, rowA.ID) })

	w := doDev(t, router, inst.ID, map[string]any{
		"substitutions": []map[string]any{{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffY}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	assert.Equal(t, inst.ID, s.mock.lastAckID, "ack clear targeted this instance")
	assert.False(t, s.mock.lastAckValue, "acknowledgement cleared")

	// Clean up the new substitute row.
	var newSubIDs []int64
	for _, r := range devInstanceStaff(t, s.db, s.ctx, inst.ID) {
		if r.StaffID == s.staffY {
			newSubIDs = append(newSubIDs, r.ID)
		}
	}
	t.Cleanup(func() { testpkg.CleanupInstanceStaffFixtures(t, s.db, newSubIDs...) })
}

// Acknowledging a block while non-absent staff remain must be rejected, and
// nothing else in the payload should commit.
func TestApplyDeviations_AckWhileStaffed_Rejected(t *testing.T) {
	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Staffed"})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })
	rowA := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{})
	t.Cleanup(func() { testpkg.CleanupInstanceStaffFixtures(t, s.db, rowA.ID) })

	w := doDev(t, router, inst.ID, map[string]any{"understaffed_ack": true})
	require.Equal(t, http.StatusConflict, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "understaffed_still_staffed")
	assert.Zero(t, s.mock.lastAckID, "no ack write when rejected")
}

// Cancel is exclusive and delegates to the shared Cancel service.
func TestApplyDeviations_Cancel(t *testing.T) {
	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Cancelme"})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	cancelled := &scheduleModel.ActivityInstance{Status: scheduleModel.InstanceStatusCancelled}
	cancelled.ID = inst.ID
	s.mock.cancelRes = cancelled

	reason := "Ausflug"
	w := doDev(t, router, inst.ID, map[string]any{"cancel": true, "cancel_reason": reason})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.NotNil(t, s.mock.lastCancelReason)
	assert.Equal(t, reason, *s.mock.lastCancelReason)
	assert.Contains(t, w.Body.String(), `"cancelled":true`)
}

// Past blocks are historical record; the endpoint refuses to edit them. Uses the
// timezone helper only to build a past date literal.
func TestApplyDeviations_PastBlock_Rejected(t *testing.T) {
	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	past := timezone.TodayDate().AddDays(-1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, past, s.roomID, testpkg.ActivityInstanceOpts{Title: "Past"})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	w := doDev(t, router, inst.ID, map[string]any{
		"absences": []map[string]any{{"staff_id": s.staffA}},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}
