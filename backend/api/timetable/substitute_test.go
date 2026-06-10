// Integration tests for the WP-B12 substitute endpoint. Mounts the handler
// without middleware; tenant context is injected directly — same pattern as
// student_day_test.go / gaps_test.go.
//
// The atomicity test explicitly reads the DB after a 409 to verify the
// Dry-Run-First pattern: no Phase-B writes must have happened when any
// target instance is in the 409 path. The tenant middleware rolls back only
// for 5xx, so this guarantee comes from handler structure, not middleware.
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
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type subSetup struct {
	res       *Resource
	db        *bun.DB
	ctx       context.Context
	roomID    int64
	absent    int64
	substitu  int64
	cleanupFn func()
}

func buildSubSetup(t *testing.T) *subSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Sub-Room-%d", suffix))
	absent := testpkg.CreateTestStaff(t, db, "Absent", fmt.Sprintf("%d", suffix))
	substitu := testpkg.CreateTestStaff(t, db, "Sub", fmt.Sprintf("%d", suffix+1))

	res := NewResource(Dependencies{
		ActivityInstanceRepo: scheduleRepo.NewActivityInstanceRepository(db),
		InstanceStaffRepo:    scheduleRepo.NewInstanceStaffRepository(db),
		SupervisorRepo:       activeRepo.NewGroupSupervisorRepository(db),
		StaffRepo:            usersRepo.NewStaffRepository(db),
		DB:                   db,
		// Broadcaster intentionally nil: tests do not exercise SSE.
	})

	cleanup := func() {
		testpkg.CleanupActivityFixtures(t, db, 0, absent.ID, 0, 0, 0)
		testpkg.CleanupActivityFixtures(t, db, 0, substitu.ID, 0, 0, 0)
		testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room.ID)
	}

	return &subSetup{
		res: res, db: db, ctx: ctx, roomID: room.ID,
		absent: absent.ID, substitu: substitu.ID,
		cleanupFn: cleanup,
	}
}

func subRouter(parentCtx context.Context, res *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(req.Context(), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Post("/substitute", res.substitute)
	return r
}

func doSub(t *testing.T, router chi.Router, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/substitute", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func doSubRaw(t *testing.T, router chi.Router, raw string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/substitute", bytes.NewReader([]byte(raw)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func decodeSub(t *testing.T, w *httptest.ResponseRecorder) SubstituteResponse {
	t.Helper()
	var env struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), "body=%s", w.Body.String())
	require.Equal(t, "success", env.Status, "body=%s", w.Body.String())
	var out SubstituteResponse
	require.NoError(t, json.Unmarshal(env.Data, &out))
	return out
}

// futureSubDate returns a YYYY-MM-DD in the future plus a UTC date.Time
// anchored at midnight for fixture rows.
func futureSubDate(offsetDays int) (string, time.Time) {
	d := time.Now().AddDate(0, 0, offsetDays)
	return d.Format("2006-01-02"),
		time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

// readInstanceStaff pulls the row directly from the DB for atomicity
// assertions. Uses the repo to honour tenant scoping.
func readInstanceStaff(t *testing.T, db *bun.DB, ctx context.Context, id int64) *schedule.InstanceStaff {
	t.Helper()
	repo := scheduleRepo.NewInstanceStaffRepository(db)
	row, err := repo.FindByID(ctx, id)
	require.NoError(t, err)
	return row
}

// -----------------------------------------------------------------------------
// 400 / 404
// -----------------------------------------------------------------------------

func TestSubstitute_SameStaff_Rejected(t *testing.T) {
	s := buildSubSetup(t)
	defer s.cleanupFn()
	router := subRouter(s.ctx, s.res)

	dateStr, _ := futureSubDate(1)
	w := doSub(t, router, map[string]any{
		"absent_staff_id":     s.absent,
		"substitute_staff_id": s.absent,
		"date":                dateStr,
	})
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

func TestSubstitute_MissingFields_Rejected(t *testing.T) {
	s := buildSubSetup(t)
	defer s.cleanupFn()
	router := subRouter(s.ctx, s.res)

	// Missing date.
	w := doSub(t, router, map[string]any{
		"absent_staff_id":     s.absent,
		"substitute_staff_id": s.substitu,
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Missing ids.
	w = doSub(t, router, map[string]any{"date": "2099-01-01"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSubstitute_InvalidJSON_Rejected(t *testing.T) {
	s := buildSubSetup(t)
	defer s.cleanupFn()
	router := subRouter(s.ctx, s.res)

	w := doSubRaw(t, router, `{not valid json`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSubstitute_PastDate_Rejected(t *testing.T) {
	s := buildSubSetup(t)
	defer s.cleanupFn()
	router := subRouter(s.ctx, s.res)

	// Yesterday (Berlin-local). Matches /gaps policy: mutating is_absent flags
	// on completed instances would rewrite history.
	past := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	w := doSub(t, router, map[string]any{
		"absent_staff_id":     s.absent,
		"substitute_staff_id": s.substitu,
		"date":                past,
	})
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "today or a future date")
}

func TestSubstitute_StaffNotFound(t *testing.T) {
	s := buildSubSetup(t)
	defer s.cleanupFn()
	router := subRouter(s.ctx, s.res)

	dateStr, _ := futureSubDate(1)

	t.Run("absent not found", func(t *testing.T) {
		w := doSub(t, router, map[string]any{
			"absent_staff_id":     int64(999999999),
			"substitute_staff_id": s.substitu,
			"date":                dateStr,
		})
		assert.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
	})

	t.Run("substitute not found", func(t *testing.T) {
		w := doSub(t, router, map[string]any{
			"absent_staff_id":     s.absent,
			"substitute_staff_id": int64(999999999),
			"date":                dateStr,
		})
		assert.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
	})
}

// -----------------------------------------------------------------------------
// No-op path (no assignments on date)
// -----------------------------------------------------------------------------

func TestSubstitute_NoAssignmentsOnDate_ReturnsEmpty(t *testing.T) {
	s := buildSubSetup(t)
	defer s.cleanupFn()
	router := subRouter(s.ctx, s.res)

	dateStr, _ := futureSubDate(5)
	w := doSub(t, router, map[string]any{
		"absent_staff_id":     s.absent,
		"substitute_staff_id": s.substitu,
		"date":                dateStr,
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	got := decodeSub(t, w)
	assert.Empty(t, got.AffectedInstances)
	assert.Empty(t, got.Warnings)
}

// -----------------------------------------------------------------------------
// Happy path: plain substitution
// -----------------------------------------------------------------------------

func TestSubstitute_HappyPath_Planned(t *testing.T) {
	s := buildSubSetup(t)
	defer s.cleanupFn()
	router := subRouter(s.ctx, s.res)

	dateStr, date := futureSubDate(1)

	inst1 := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Inst-1",
	})
	inst2 := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "15:00", EndHHMM: "16:00", Title: "Inst-2",
	})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst1.ID, inst2.ID) })

	row1 := testpkg.CreateTestInstanceStaff(t, s.db, inst1.ID, s.absent, testpkg.InstanceStaffOpts{})
	row2 := testpkg.CreateTestInstanceStaff(t, s.db, inst2.ID, s.absent, testpkg.InstanceStaffOpts{})
	t.Cleanup(func() { testpkg.CleanupInstanceStaffFixtures(t, s.db, row1.ID, row2.ID) })

	w := doSub(t, router, map[string]any{
		"absent_staff_id":     s.absent,
		"substitute_staff_id": s.substitu,
		"date":                dateStr,
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeSub(t, w)
	require.Len(t, got.AffectedInstances, 2)
	for _, a := range got.AffectedInstances {
		assert.Equal(t, "substituted", a.Action)
	}

	// Verify DB state: original rows now is_absent=true, substitute rows exist.
	row1After := readInstanceStaff(t, s.db, s.ctx, row1.ID)
	assert.True(t, row1After.IsAbsent, "original row on inst1 should be is_absent=true")

	row2After := readInstanceStaff(t, s.db, s.ctx, row2.ID)
	assert.True(t, row2After.IsAbsent, "original row on inst2 should be is_absent=true")

	// Cleanup the newly created substitute rows after DB read.
	var newSubRows []int64
	repo := scheduleRepo.NewInstanceStaffRepository(s.db)
	for _, instID := range []int64{inst1.ID, inst2.ID} {
		rows, err := repo.FindByInstanceID(s.ctx, instID)
		require.NoError(t, err)
		foundSub := false
		for _, r := range rows {
			if r.IsSubstitute && r.StaffID == s.substitu {
				foundSub = true
				newSubRows = append(newSubRows, r.ID)
			}
		}
		assert.True(t, foundSub, "substitute row must exist on instance %d", instID)
	}
	t.Cleanup(func() { testpkg.CleanupInstanceStaffFixtures(t, s.db, newSubRows...) })
}

// -----------------------------------------------------------------------------
// Idempotent replay
// -----------------------------------------------------------------------------

func TestSubstitute_Idempotent_AlreadySubstituted(t *testing.T) {
	s := buildSubSetup(t)
	defer s.cleanupFn()
	router := subRouter(s.ctx, s.res)

	dateStr, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00",
	})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	// Pre-existing absent original row + pre-existing substitute row for subStaff.
	row := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.absent, testpkg.InstanceStaffOpts{IsAbsent: true})
	subRow := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.substitu, testpkg.InstanceStaffOpts{IsSubstitute: true})
	t.Cleanup(func() { testpkg.CleanupInstanceStaffFixtures(t, s.db, row.ID, subRow.ID) })

	w := doSub(t, router, map[string]any{
		"absent_staff_id":     s.absent,
		"substitute_staff_id": s.substitu,
		"date":                dateStr,
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	got := decodeSub(t, w)
	require.Len(t, got.AffectedInstances, 1)
	assert.Equal(t, "already_substituted", got.AffectedInstances[0].Action)
}

// -----------------------------------------------------------------------------
// 409 + ATOMICITY: the critical one. Multi-instance request where ONE
// instance has a different substitute already. Must return 409 and leave
// the OTHER instances unchanged (Dry-Run-First contract).
// -----------------------------------------------------------------------------

func TestSubstitute_Conflict_OtherSubstitute_Exists_AtomicNoWrites(t *testing.T) {
	s := buildSubSetup(t)
	defer s.cleanupFn()
	router := subRouter(s.ctx, s.res)

	dateStr, date := futureSubDate(1)

	// Three instances. Order of origRows from FindByStaffAndDate is by
	// start_time ASC, so inst-early processes first, inst-blocker last.
	instEarly := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "08:00", EndHHMM: "09:00", Title: "Early",
	})
	instMid := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "10:00", EndHHMM: "11:00", Title: "Mid",
	})
	instBlock := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Block",
	})
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances",
			instEarly.ID, instMid.ID, instBlock.ID)
	})

	// absent staff on all three.
	rowEarly := testpkg.CreateTestInstanceStaff(t, s.db, instEarly.ID, s.absent, testpkg.InstanceStaffOpts{})
	rowMid := testpkg.CreateTestInstanceStaff(t, s.db, instMid.ID, s.absent, testpkg.InstanceStaffOpts{})
	rowBlock := testpkg.CreateTestInstanceStaff(t, s.db, instBlock.ID, s.absent, testpkg.InstanceStaffOpts{IsAbsent: true})

	// Block: absent is already is_absent=true AND there's another substitute
	// (not s.substitu) so the classifier trips the 409.
	otherSub := testpkg.CreateTestStaff(t, s.db, "Other", fmt.Sprintf("Sub-%d", time.Now().UnixNano()))
	blockSub := testpkg.CreateTestInstanceStaff(t, s.db, instBlock.ID, otherSub.ID, testpkg.InstanceStaffOpts{IsSubstitute: true})

	t.Cleanup(func() {
		testpkg.CleanupInstanceStaffFixtures(t, s.db, rowEarly.ID, rowMid.ID, rowBlock.ID, blockSub.ID)
		testpkg.CleanupActivityFixtures(t, s.db, 0, otherSub.ID, 0, 0, 0)
	})

	w := doSub(t, router, map[string]any{
		"absent_staff_id":     s.absent,
		"substitute_staff_id": s.substitu,
		"date":                dateStr,
	})
	require.Equal(t, http.StatusConflict, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), `"code":"substitute_conflict"`,
		"frontend relies on the stable conflict code, not the German message")

	// ATOMICITY CHECK — this is the WP-B12 Dry-Run-First contract.
	// Instances Early and Mid must NOT have been written to, even though
	// they appear before the blocker in the classification loop.
	rowEarlyAfter := readInstanceStaff(t, s.db, s.ctx, rowEarly.ID)
	assert.False(t, rowEarlyAfter.IsAbsent, "Early original must remain untouched (Dry-Run-First)")

	rowMidAfter := readInstanceStaff(t, s.db, s.ctx, rowMid.ID)
	assert.False(t, rowMidAfter.IsAbsent, "Mid original must remain untouched (Dry-Run-First)")

	// No substitute rows for s.substitu must exist on Early or Mid.
	repo := scheduleRepo.NewInstanceStaffRepository(s.db)
	for _, instID := range []int64{instEarly.ID, instMid.ID} {
		rows, err := repo.FindByInstanceID(s.ctx, instID)
		require.NoError(t, err)
		for _, r := range rows {
			if r.StaffID == s.substitu {
				t.Fatalf("instance %d must not have a substitute row for subID %d after 409", instID, s.substitu)
			}
		}
	}
}

// -----------------------------------------------------------------------------
// Substitute is already a co-supervisor on the instance (UNIQUE would reject
// a second row). Expect action=already_on_instance + absent row flipped.
// -----------------------------------------------------------------------------

func TestSubstitute_SubstituteIsAlreadyCoSupervisor(t *testing.T) {
	s := buildSubSetup(t)
	defer s.cleanupFn()
	router := subRouter(s.ctx, s.res)

	dateStr, date := futureSubDate(1)
	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "CoSup",
	})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	rowAbsent := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.absent, testpkg.InstanceStaffOpts{})
	rowCoSup := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.substitu, testpkg.InstanceStaffOpts{IsPrimary: true})
	t.Cleanup(func() { testpkg.CleanupInstanceStaffFixtures(t, s.db, rowAbsent.ID, rowCoSup.ID) })

	w := doSub(t, router, map[string]any{
		"absent_staff_id":     s.absent,
		"substitute_staff_id": s.substitu,
		"date":                dateStr,
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeSub(t, w)
	require.Len(t, got.AffectedInstances, 1)
	assert.Equal(t, "already_on_instance", got.AffectedInstances[0].Action)

	// Absent row flipped.
	absentAfter := readInstanceStaff(t, s.db, s.ctx, rowAbsent.ID)
	assert.True(t, absentAfter.IsAbsent)

	// Co-supervisor row UNTOUCHED: is_substitute=false, is_primary=true, no room.
	coSupAfter := readInstanceStaff(t, s.db, s.ctx, rowCoSup.ID)
	assert.False(t, coSupAfter.IsSubstitute, "co-supervisor must NOT be reclassified as substitute")
	assert.True(t, coSupAfter.IsPrimary, "co-supervisor is_primary preserved")
	assert.False(t, coSupAfter.IsAbsent, "co-supervisor is_absent preserved")
}

// -----------------------------------------------------------------------------
// Active instance path: supervisor row gets ended + new supervisor created.
// -----------------------------------------------------------------------------

func TestSubstitute_ActiveInstance_EndsAndCreatesSupervisor(t *testing.T) {
	s := buildSubSetup(t)
	defer s.cleanupFn()
	router := subRouter(s.ctx, s.res)

	dateStr, date := futureSubDate(1)

	// Create an active.group first so we can link it to the instance.
	activeGroupRepo := activeRepo.NewGroupRepository(s.db)
	now := time.Now()
	ag := &activeModel.Group{
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
		RoomID:         s.roomID,
	}
	require.NoError(t, activeGroupRepo.Create(s.ctx, ag))
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "active.groups", ag.ID) })

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Active-Inst",
		Status:        schedule.InstanceStatusActive,
		ActiveGroupID: &ag.ID,
	})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })

	rowAbsent := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.absent, testpkg.InstanceStaffOpts{})
	t.Cleanup(func() { testpkg.CleanupInstanceStaffFixtures(t, s.db, rowAbsent.ID) })

	// Pre-existing active supervisor row for absent.
	supervisorRepo := activeRepo.NewGroupSupervisorRepository(s.db)
	absentSup := &activeModel.GroupSupervisor{
		StaffID:   s.absent,
		GroupID:   ag.ID,
		Role:      "supervisor",
		StartDate: timezone.DateFromTime(now),
	}
	require.NoError(t, supervisorRepo.Create(s.ctx, absentSup))
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "active.group_supervisors", absentSup.ID) })

	w := doSub(t, router, map[string]any{
		"absent_staff_id":     s.absent,
		"substitute_staff_id": s.substitu,
		"date":                dateStr,
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	// Absent's supervisor row must be ended.
	absentSupAfter, err := supervisorRepo.FindByID(s.ctx, absentSup.ID)
	require.NoError(t, err)
	assert.NotNil(t, absentSupAfter.EndDate, "absent supervisor must be ended")

	// A new supervisor row must exist for the substitute on this group.
	subSups, err := supervisorRepo.FindActiveByStaffID(s.ctx, s.substitu)
	require.NoError(t, err)
	found := false
	var newSupID int64
	for _, sup := range subSups {
		if sup.GroupID == ag.ID {
			found = true
			newSupID = sup.ID
		}
	}
	assert.True(t, found, "substitute must have a new active supervisor row on this group")
	t.Cleanup(func() {
		if newSupID > 0 {
			testpkg.CleanupTableRecords(t, s.db, "active.group_supervisors", newSupID)
		}
	})

	// Cleanup the new substitute instance_staff row.
	repo := scheduleRepo.NewInstanceStaffRepository(s.db)
	allRows, err := repo.FindByInstanceID(s.ctx, inst.ID)
	require.NoError(t, err)
	var newSubInstStaffID int64
	for _, r := range allRows {
		if r.IsSubstitute && r.StaffID == s.substitu {
			newSubInstStaffID = r.ID
		}
	}
	t.Cleanup(func() {
		if newSubInstStaffID > 0 {
			testpkg.CleanupInstanceStaffFixtures(t, s.db, newSubInstStaffID)
		}
	})
}

// -----------------------------------------------------------------------------
// Soft warning path: substitute has another same-day non-absent assignment
// that time-overlaps with the target.
// -----------------------------------------------------------------------------

func TestSubstitute_TimeConflictWarning(t *testing.T) {
	s := buildSubSetup(t)
	defer s.cleanupFn()
	router := subRouter(s.ctx, s.res)

	dateStr, date := futureSubDate(1)

	// Target: 14:00–15:00 for absent staff.
	target := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Target",
	})
	// Foreign: 14:30–15:30 for substitute staff (overlaps target).
	foreign := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:30", EndHHMM: "15:30", Title: "Foreign",
	})
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", target.ID, foreign.ID)
	})

	absentRow := testpkg.CreateTestInstanceStaff(t, s.db, target.ID, s.absent, testpkg.InstanceStaffOpts{})
	subForeign := testpkg.CreateTestInstanceStaff(t, s.db, foreign.ID, s.substitu, testpkg.InstanceStaffOpts{})
	t.Cleanup(func() { testpkg.CleanupInstanceStaffFixtures(t, s.db, absentRow.ID, subForeign.ID) })

	w := doSub(t, router, map[string]any{
		"absent_staff_id":     s.absent,
		"substitute_staff_id": s.substitu,
		"date":                dateStr,
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	got := decodeSub(t, w)
	require.Len(t, got.AffectedInstances, 1)
	assert.Equal(t, "substituted", got.AffectedInstances[0].Action)

	require.Len(t, got.Warnings, 1)
	assert.Equal(t, "substitute_time_conflict", got.Warnings[0].Kind)
	assert.Equal(t, target.ID, got.Warnings[0].InstanceID)
	assert.Equal(t, foreign.ID, got.Warnings[0].OtherID)
	assert.Contains(t, got.Warnings[0].Message, "14:30")

	// Cleanup the new substitute row on target.
	repo := scheduleRepo.NewInstanceStaffRepository(s.db)
	rows, err := repo.FindByInstanceID(s.ctx, target.ID)
	require.NoError(t, err)
	for _, r := range rows {
		if r.IsSubstitute && r.StaffID == s.substitu {
			t.Cleanup(func() { testpkg.CleanupInstanceStaffFixtures(t, s.db, r.ID) })
		}
	}
}

// -----------------------------------------------------------------------------
// Tenant isolation: staff exists only in tenant 1, but request is made from
// tenant 2 → 404 (FindByID returns no rows).
// -----------------------------------------------------------------------------

func TestSubstitute_TenantIsolation(t *testing.T) {
	s := buildSubSetup(t)
	defer s.cleanupFn()

	// Switch to a different tenant — the staff fixtures live in tenant 1.
	otherCtx := testpkg.TenantContext(999)
	router := subRouter(otherCtx, s.res)

	dateStr, _ := futureSubDate(1)
	w := doSub(t, router, map[string]any{
		"absent_staff_id":     s.absent,
		"substitute_staff_id": s.substitu,
		"date":                dateStr,
	})
	assert.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
}
