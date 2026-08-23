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
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services"
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

	ctx := testpkg.Ctx(t)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Dev-Room-%d", suffix))
	a := testpkg.CreateTestStaff(t, db, "Planned", fmt.Sprintf("%d", suffix))
	x := testpkg.CreateTestStaff(t, db, "CurrentSub", fmt.Sprintf("%d", suffix+1))
	y := testpkg.CreateTestStaff(t, db, "Replacement", fmt.Sprintf("%d", suffix+2))
	b := testpkg.CreateTestStaff(t, db, "Planned2", fmt.Sprintf("%d", suffix+3))

	// The mock records lifecycle calls (ack/cancel assertions) and delegates
	// the deviation writes (#1886) to the real service so the DB-effect
	// assertions keep exercising real rows.
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err)
	mock := &mockInstanceService{real: serviceFactory.Instance}
	res := NewResource(Dependencies{
		TimetableData:   testTimetableData(db),
		PersonService:   usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{PersonRepo: usersRepo.NewPersonRepository(db), StaffRepo: usersRepo.NewStaffRepository(db)}),
		InstanceService: mock,
		DB:              db,
	})

	t.Cleanup(func() {
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
	t.Parallel()

	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Swap"})

	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffX, testpkg.InstanceStaffOpts{IsSubstitute: true})

	w := doDev(t, router, inst.ID, map[string]any{
		"absences":      []map[string]any{{"staff_id": s.staffX}},
		"substitutions": []map[string]any{{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffY}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	rows := devInstanceStaff(t, s.db, s.ctx, inst.ID)
	sawXAbsent, sawYActive := false, false
	for _, r := range rows {
		if r.StaffID == s.staffX {
			assert.True(t, r.IsAbsent, "removed substitute X must be marked absent")
			sawXAbsent = true
		}
		if r.StaffID == s.staffY && r.IsSubstitute && !r.IsAbsent {
			sawYActive = true
		}
	}
	assert.True(t, sawXAbsent, "X row absent")
	assert.True(t, sawYActive, "Y active substitute row created")
}

// Assigning a substitute who ALREADY covers another of the absent person's
// same-day blocks must not 500. The absent person A works block1 + block2; Y
// already substitutes on block2 (covering B). One save assigns Y to cover A.
// On block2, A's row is still non-absent at classification time (A is not in the
// absence-only set), so the old classifier returned "substituted" and Phase B
// tried to insert a SECOND Y row → UNIQUE(instance_id, staff_id) → 500 rolling
// back the whole save. Now block2 classifies as already-on-instance: A's row is
// flagged, no duplicate row is inserted (#1840).
func TestApplyDeviations_SubstituteAlreadyCoversOtherBlock(t *testing.T) {
	t.Parallel()

	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst1 := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Block1", StartHHMM: "08:00", EndHHMM: "09:00"})
	inst2 := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Block2", StartHHMM: "10:00", EndHHMM: "11:00"})

	// A works both blocks and is present. On block2, Y is already a substitute
	// (covering B, who is absent there).
	testpkg.CreateTestInstanceStaff(t, s.db, inst1.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, inst2.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, inst2.ID, s.staffB, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestInstanceStaff(t, s.db, inst2.ID, s.staffY, testpkg.InstanceStaffOpts{IsSubstitute: true})

	w := doDev(t, router, inst1.ID, map[string]any{
		"substitutions": []map[string]any{{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffY}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// block1: A absent, Y freshly substituting.
	sawA1Absent, sawY1New := false, false
	for _, r := range devInstanceStaff(t, s.db, s.ctx, inst1.ID) {
		if r.StaffID == s.staffA {
			sawA1Absent = r.IsAbsent
		}
		if r.StaffID == s.staffY && r.IsSubstitute && !r.IsAbsent {
			sawY1New = true
		}
	}
	assert.True(t, sawA1Absent, "A marked absent on block1")
	assert.True(t, sawY1New, "Y substitute row created on block1")

	// block2: A absent, and Y still has EXACTLY ONE (its pre-existing) row — no
	// duplicate insert.
	sawA2Absent, yRowCount := false, 0
	for _, r := range devInstanceStaff(t, s.db, s.ctx, inst2.ID) {
		if r.StaffID == s.staffA {
			sawA2Absent = r.IsAbsent
		}
		if r.StaffID == s.staffY {
			yRowCount++
		}
	}
	assert.True(t, sawA2Absent, "A marked absent on block2")
	assert.Equal(t, 1, yRowCount, "Y must not be inserted twice on block2")

}

// Under the count-based coverage rule (#1840) a second substitute is accepted
// while the block still has an OPEN absent slot — it fills the gap instead of
// 409ing. A is absent and covered by X; B is newly marked absent in the same
// save; Y is assigned. Two absent positions, two substitutes → the block ends up
// fully covered rather than conflicting. (Previously this 409'd because ANY other
// active substitute blocked covering an already-absent position.)
func TestApplyDeviations_SecondSubstituteFillsOpenGap(t *testing.T) {
	t.Parallel()

	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Pooled"})

	// A absent, X already substituting A (non-absent), B planned & present.
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffX, testpkg.InstanceStaffOpts{IsSubstitute: true})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffB, testpkg.InstanceStaffOpts{})

	// Mark B absent AND assign Y: two open positions (A, B), X covers one, Y fills
	// the remaining gap — no conflict.
	w := doDev(t, router, inst.ID, map[string]any{
		"absences":      []map[string]any{{"staff_id": s.staffB}},
		"substitutions": []map[string]any{{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffY}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	sawBAbsent, sawYActive, xStillActive := false, false, false
	for _, r := range devInstanceStaff(t, s.db, s.ctx, inst.ID) {
		if r.StaffID == s.staffB {
			sawBAbsent = r.IsAbsent
		}
		if r.StaffID == s.staffX && r.IsSubstitute && !r.IsAbsent {
			xStillActive = true
		}
		if r.StaffID == s.staffY && r.IsSubstitute && !r.IsAbsent {
			sawYActive = true
		}
	}
	assert.True(t, sawBAbsent, "B marked absent")
	assert.True(t, xStillActive, "X remains an active substitute")
	assert.True(t, sawYActive, "Y created as an active substitute filling the second gap")
}

// A genuine OVERSTAFFING conflict must still 409 and roll back everything in the
// payload. Under the count-based rule (#1840) a substitute is rejected only when
// the target block is ALREADY fully covered (active substitutes >= absent
// positions); adding one more would overstaff. The co-payload absence (day-wide,
// on another block) must NOT survive the abort — the Phase-A dry-run guarantee.
func TestApplyDeviations_OverstaffConflict_NoPartialWrites(t *testing.T) {
	t.Parallel()

	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst1 := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Covered", StartHHMM: "08:00", EndHHMM: "09:00"})
	inst2 := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Other", StartHHMM: "10:00", EndHHMM: "11:00"})

	// inst1 already fully covered: A absent, X actively substituting (1 absent, 1
	// active sub). inst2 has B present — the atomicity co-write target.
	testpkg.CreateTestInstanceStaff(t, s.db, inst1.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestInstanceStaff(t, s.db, inst1.ID, s.staffX, testpkg.InstanceStaffOpts{IsSubstitute: true})
	rowB := testpkg.CreateTestInstanceStaff(t, s.db, inst2.ID, s.staffB, testpkg.InstanceStaffOpts{})

	// Add Y as a SECOND substitute to the already-covered inst1 → substitute_conflict.
	// B's day-wide absence is in the same payload and must not survive the abort.
	w := doDev(t, router, inst1.ID, map[string]any{
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
	t.Parallel()

	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Acked"})
	setUnderstaffedAckFixture(t, s.db, s.ctx, inst.ID)

	// A already absent → block was legitimately all-absent when acked.
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})

	w := doDev(t, router, inst.ID, map[string]any{
		"substitutions": []map[string]any{{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffY}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	assert.False(t, readInstance(t, s.db, s.ctx, inst.ID).UnderstaffedAck,
		"adding coverage must clear the stale acknowledgement on this instance")

	// Clean up the new substitute row.
}

// Acknowledging a block while non-absent staff remain must be rejected, and
// nothing else in the payload should commit.
func TestApplyDeviations_AckWhileStaffed_Rejected(t *testing.T) {
	t.Parallel()

	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Staffed"})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{})

	w := doDev(t, router, inst.ID, map[string]any{"understaffed_ack": true})
	require.Equal(t, http.StatusConflict, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "understaffed_still_staffed")
	assert.False(t, readInstance(t, s.db, s.ctx, inst.ID).UnderstaffedAck,
		"a rejected acknowledgement must not be written")
}

// Cancel is exclusive and delegates to the shared Cancel service.
func TestApplyDeviations_Cancel(t *testing.T) {
	t.Parallel()

	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Cancelme"})

	// Cancel now runs through the real InstanceService inside ApplyDeviations
	// (no mock seam), so the assertions read the persisted cancellation instead
	// of the old mock recorder.
	reason := "Ausflug"
	w := doDev(t, router, inst.ID, map[string]any{"cancel": true, "cancel_reason": reason})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), `"cancelled":true`)

	after := readInstance(t, s.db, s.ctx, inst.ID)
	assert.Equal(t, scheduleModel.InstanceStatusCancelled, after.Status, "block cancelled in the DB")
	require.NotNil(t, after.CancelReason, "cancel reason persisted")
	assert.Equal(t, reason, *after.CancelReason)
}

// Past blocks are historical record; the endpoint refuses to edit them. Uses the
// timezone helper only to build a past date literal.
func TestApplyDeviations_PastBlock_Rejected(t *testing.T) {
	t.Parallel()

	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	past := timezone.TodayDate().AddDays(-1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, past, s.roomID, testpkg.ActivityInstanceOpts{Title: "Past"})

	w := doDev(t, router, inst.ID, map[string]any{
		"absences": []map[string]any{{"staff_id": s.staffA}},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

// When the absent person has several same-day blocks, the substitute is added to
// each; a DIFFERENT (non-selected) block that was flagged "deliberately
// unstaffed" and is now covered must have its stale acknowledgement reconciled
// too — not only the selected block (#1840). The selected block here is never
// acked, so the only ack touched must be the OTHER block's.
func TestApplyDeviations_ClearsStaleAckOnOtherBlocks(t *testing.T) {
	t.Parallel()

	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	selected := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Selected"})
	other := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Other"})
	// The other block was legitimately all-absent when acked.
	setUnderstaffedAckFixture(t, s.db, s.ctx, other.ID)

	testpkg.CreateTestInstanceStaff(t, s.db, selected.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestInstanceStaff(t, s.db, other.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})

	// Substituting Y for A is day-wide → covers BOTH blocks in one save.
	w := doDev(t, router, selected.ID, map[string]any{
		"substitutions": []map[string]any{{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffY}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// The OTHER block's fixture ack was TRUE; covering it day-wide must clear that
	// stale acknowledgement. The selected block was never acked and stays clear.
	assert.False(t, readInstance(t, s.db, s.ctx, other.ID).UnderstaffedAck,
		"stale ack on the OTHER covered block must be reconciled")
	assert.False(t, readInstance(t, s.db, s.ctx, selected.ID).UnderstaffedAck,
		"the selected block was never acked")
}

// A substitute removed on a prior save is marked absent day-wide; the endpoint
// must be able to clear that absence (bring the substitute back) so an accidental
// removal is correctable without a DB edit. The old planPresences skipped every
// substitute row, so a removed substitute was stuck absent forever (#1840).
func TestApplyDeviations_RestoreRemovedSubstitute(t *testing.T) {
	t.Parallel()

	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Restore"})

	// A absent; X is a substitute who was removed earlier → marked absent day-wide.
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	rowX := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffX, testpkg.InstanceStaffOpts{IsSubstitute: true, IsAbsent: true})

	w := doDev(t, router, inst.ID, map[string]any{
		"presences": []int64{s.staffX},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	rowXAfter := readInstanceStaff(t, s.db, s.ctx, rowX.ID)
	assert.False(t, rowXAfter.IsAbsent, "removed substitute must be restored (absence cleared)")
	assert.True(t, rowXAfter.IsSubstitute, "row stays a substitute row")
}

// Two absent planned staff on one block, two DISTINCT replacements, one atomic
// save. The only DB constraint is UNIQUE(instance_id, staff_id) and the
// sequential /substitute path already produces exactly this, so the atomic path
// must accept it too instead of 409-ing on the second position (#1840).
func TestApplyDeviations_TwoDistinctSubstitutes_OneBlock(t *testing.T) {
	t.Parallel()

	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "TwoSubs"})

	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffB, testpkg.InstanceStaffOpts{})

	w := doDev(t, router, inst.ID, map[string]any{
		"substitutions": []map[string]any{
			{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffX},
			{"absent_staff_id": s.staffB, "substitute_staff_id": s.staffY},
		},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	sawX, sawY := false, false
	for _, r := range devInstanceStaff(t, s.db, s.ctx, inst.ID) {
		if r.StaffID == s.staffX && r.IsSubstitute && !r.IsAbsent {
			sawX = true
		}
		if r.StaffID == s.staffY && r.IsSubstitute && !r.IsAbsent {
			sawY = true
		}
	}
	assert.True(t, sawX, "X substitute row created for A")
	assert.True(t, sawY, "Y substitute row created for B")
}

// The SAME substitute named for both absent positions on one block must NOT
// double-insert (instance_id, X) and 500. The repeat collapses to a single
// covering row while both absent positions are still flagged (#1840).
func TestApplyDeviations_SameSubstituteForTwoAbsent_OneBlock(t *testing.T) {
	t.Parallel()

	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "SameSub"})

	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffB, testpkg.InstanceStaffOpts{})

	w := doDev(t, router, inst.ID, map[string]any{
		"substitutions": []map[string]any{
			{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffX},
			{"absent_staff_id": s.staffB, "substitute_staff_id": s.staffX},
		},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	xRows := 0
	sawAAbsent, sawBAbsent := false, false
	for _, r := range devInstanceStaff(t, s.db, s.ctx, inst.ID) {
		if r.StaffID == s.staffX {
			xRows++
		}
		if r.StaffID == s.staffA && r.IsAbsent {
			sawAAbsent = true
		}
		if r.StaffID == s.staffB && r.IsAbsent {
			sawBAbsent = true
		}
	}
	assert.Equal(t, 1, xRows, "the shared substitute must appear exactly once")
	assert.True(t, sawAAbsent, "A marked absent")
	assert.True(t, sawBAbsent, "B marked absent")
}

// One substitute covering TWO absent staff on the same block must yield each
// time-conflict warning ONCE, not once per absent position. collectDeviationWarnings
// already gathers every plan op for a substitute in a single pass, so iterating
// the substitution rows without de-duplicating the substitute id rebuilt and
// re-appended the identical conflicts — inflating the UI warning count (#1840).
func TestApplyDeviations_SameSubstituteForTwoAbsent_WarningsNotDuplicated(t *testing.T) {
	t.Parallel()

	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	// Target block 14:00-15:00; A and B are planned there.
	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "SameSubWarn", StartHHMM: "14:00", EndHHMM: "15:00",
	})

	// A FOREIGN block the same day 14:30-15:30 that X already staffs (non-absent).
	// It overlaps the target, so covering the target with X raises exactly one
	// substitute_time_conflict against this foreign assignment.
	foreign := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "ForeignForX", StartHHMM: "14:30", EndHHMM: "15:30",
	})

	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffB, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, foreign.ID, s.staffX, testpkg.InstanceStaffOpts{})

	// X covers BOTH absent A and B on the target block in one save.
	w := doDev(t, router, inst.ID, map[string]any{
		"substitutions": []map[string]any{
			{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffX},
			{"absent_staff_id": s.staffB, "substitute_staff_id": s.staffX},
		},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var resp struct {
		Data ApplyDeviationsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	require.Len(t, resp.Data.Warnings, 1, "the conflict must appear once, not once per absent position")
	assert.Equal(t, inst.ID, resp.Data.Warnings[0].InstanceID)
	assert.Equal(t, foreign.ID, resp.Data.Warnings[0].OtherID)

}

// ackRouter wires the standalone acknowledge-understaffed handler with the same
// real TimetableData facade + tenant middleware as devRouter, so the date guard
// (which loads the instance) is exercised end to end.
func ackRouter(parentCtx context.Context, res *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(req.Context(), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Post("/instances/{id}/acknowledge-understaffed", res.acknowledgeUnderstaffed)
	return r
}

func doAck(t *testing.T, router chi.Router, instanceID int64, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/instances/%d/acknowledge-understaffed", instanceID), bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// The standalone /acknowledge-understaffed endpoint must reject a past block just
// like /deviations does. A materialized past occurrence can still be
// planned/active, so a status-only check would let a historical staffing record
// be rewritten through this public endpoint (#1840).
func TestAcknowledgeUnderstaffed_PastBlock_Rejected(t *testing.T) {
	t.Parallel()

	s := buildDevSetup(t)
	router := ackRouter(s.ctx, s.res)
	past := timezone.TodayDate().AddDays(-1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, past, s.roomID, testpkg.ActivityInstanceOpts{Title: "PastAck"})

	w := doAck(t, router, inst.ID, map[string]any{"ack": true})
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "Vergangenheit")
	assert.False(t, readInstance(t, s.db, s.ctx, inst.ID).UnderstaffedAck,
		"a past block's acknowledgement must not be written")
}

// futureSubDate returns a YYYY-MM-DD in the future plus the matching
// timezone.Date for fixture rows. (Moved from the removed substitute endpoint
// tests, #1886.)
func futureSubDate(offsetDays int) (string, timezone.Date) {
	d := timezone.TodayDate().AddDays(offsetDays)
	return d.String(), d
}

// readInstanceStaff pulls the row directly from the DB for atomicity
// assertions. Uses the repo to honour tenant scoping.
func readInstanceStaff(t *testing.T, db *bun.DB, ctx context.Context, id int64) *scheduleModel.InstanceStaff {
	t.Helper()
	repo := scheduleRepo.NewInstanceStaffRepository(db)
	row, err := repo.FindByID(ctx, id)
	require.NoError(t, err)
	return row
}

// readInstance pulls the instance directly from the DB. Since ApplyDeviations
// now owns the ack/cancel writes internally (they no longer route through the
// mock InstanceService seam), the acknowledgement/cancellation assertions read
// the persisted row instead of the old mock recorders.
func readInstance(t *testing.T, db *bun.DB, ctx context.Context, id int64) *scheduleModel.ActivityInstance {
	t.Helper()
	repo := scheduleRepo.NewActivityInstanceRepository(db)
	inst, err := repo.FindByID(ctx, id)
	require.NoError(t, err)
	return inst
}
